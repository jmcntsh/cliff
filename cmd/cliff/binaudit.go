package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jmcntsh/cliff/internal/binmap"
)

// cmdBinAudit drains the local bin-audit log into a printable list of
// suggested registry manifest edits. The audit log records every time
// a detected binary name disagreed with the manifest-derived name
// (classic case: repo "cpcloud/minesweep-rs" whose cargo crate is
// "minesweep" — BinaryName() guesses from the repo basename and
// guesses wrong).
//
// Running `cliff bin-audit` against a real user's ~/.cliff/logs is
// the lazy backfill path: the client has been collecting ground truth
// every time anyone installs a mismatched app, and this subcommand
// turns that into an easy-to-paste set of `binary = "…"` diffs for
// PRs against cliff-registry. Nothing about this touches the network
// or the registry directly — it's plain text in, plain text out.
//
// Flags:
//
//	--log <path>   Audit log path (default: ~/.cliff/logs/bin-audit.log)
//	--format <f>   "summary" (default) | "toml-patches"
//
// Summary format is a human-scannable table of repo→(derived, detected,
// seen_count). toml-patches is a paste-friendly list of per-manifest
// diffs in the shape the registry expects (`binary = "foo"` under the
// app's top-level TOML block). Both formats skip entries where a
// subsequent audit line reverted the mismatch (e.g. a manifest was
// already PR'd and the user re-installed at the new name) by keying
// on the most recent observation per repo.
func cmdBinAudit(args []string) int {
	fs := flag.NewFlagSet("bin-audit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	logFlag := fs.String("log", "", "audit log path (default: ~/.cliff/logs/bin-audit.log)")
	format := fs.String("format", "summary", "output format: summary | toml-patches")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	logPath := *logFlag
	if logPath == "" {
		p, err := binmap.AuditPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "bin-audit:", err)
			return 1
		}
		logPath = p
	}

	mismatches, err := readMismatches(logPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bin-audit:", err)
		return 1
	}
	if len(mismatches) == 0 {
		fmt.Fprintln(os.Stderr, "bin-audit: no mismatches recorded (log is empty or missing)")
		return 0
	}

	switch *format {
	case "toml-patches":
		printTOMLPatches(mismatches)
	case "summary":
		printSummary(mismatches)
	default:
		fmt.Fprintln(os.Stderr, "bin-audit: unknown --format:", *format)
		return 2
	}
	return 0
}

// mismatch is a single repo's most-recent observation from the audit
// log. seen counts how many times we recorded this disagreement; higher
// counts are stronger "yes, this is really the right name" signal.
type mismatch struct {
	Repo     string
	Derived  string
	Detected string
	Seen     int
}

// readMismatches parses bin-audit.log into deduped, most-recent-wins
// entries. The log format is:
//
//	<rfc3339> bin-mismatch repo=<repo> derived=<derived> detected=<detected>
//
// Lines that don't match (future events, malformed writes) are
// skipped silently — forward-compatible by design.
func readMismatches(path string) ([]mismatch, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	byRepo := map[string]*mismatch{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if !strings.Contains(line, "bin-mismatch") {
			continue
		}
		repo := kvField(line, "repo=")
		derived := kvField(line, "derived=")
		detected := kvField(line, "detected=")
		if repo == "" || detected == "" {
			continue
		}
		entry, ok := byRepo[repo]
		if !ok {
			entry = &mismatch{Repo: repo}
			byRepo[repo] = entry
		}
		entry.Derived = derived
		entry.Detected = detected
		entry.Seen++
	}
	if err := s.Err(); err != nil {
		return nil, err
	}

	out := make([]mismatch, 0, len(byRepo))
	for _, m := range byRepo {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seen != out[j].Seen {
			return out[i].Seen > out[j].Seen
		}
		return out[i].Repo < out[j].Repo
	})
	return out, nil
}

// kvField extracts the space-terminated value for `<key>=<value>` out
// of a flat log line. Empty when not found. No shell-quoting support:
// the logger is ours, we know it writes one word per value.
func kvField(line, key string) string {
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	rest := line[i+len(key):]
	if j := strings.IndexAny(rest, " \n"); j >= 0 {
		return rest[:j]
	}
	return rest
}

func printSummary(ms []mismatch) {
	fmt.Printf("%-40s  %-18s  %-18s  %s\n", "REPO", "DERIVED", "DETECTED", "SEEN")
	for _, m := range ms {
		fmt.Printf("%-40s  %-18s  %-18s  %d\n", m.Repo, m.Derived, m.Detected, m.Seen)
	}
	fmt.Fprintf(os.Stderr, "\n%d manifests could use a `binary = \"…\"` override.\n", len(ms))
	fmt.Fprintln(os.Stderr, "Re-run with --format=toml-patches for paste-ready diffs.")
}

func printTOMLPatches(ms []mismatch) {
	for _, m := range ms {
		fmt.Printf("# %s (observed %d time(s))\n", m.Repo, m.Seen)
		fmt.Printf("# apps/%s.toml — add this under the top-level table:\n",
			appSlugFromRepo(m.Repo))
		fmt.Printf("binary = %q\n\n", m.Detected)
	}
}

// appSlugFromRepo is a best-guess for which manifest file to edit:
// the registry's convention is apps/<name>.toml and name usually
// matches the repo basename in lowercase. Not authoritative — the
// operator should double-check the filename — but good enough that
// most patches won't need manual path fixups.
func appSlugFromRepo(repo string) string {
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		return strings.ToLower(repo[i+1:])
	}
	return strings.ToLower(repo)
}
