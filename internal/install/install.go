// Package install runs install commands derived from manifest InstallSpec
// values and answers "is this app installed?" by inspecting $PATH.
//
// The runner shells out via `sh -c` and captures stdout+stderr together.
// We intentionally don't try to sandbox or sanitize — the trust model
// (CLAUDE.md §3) is that installs run with the user's shell privileges,
// same as `brew install`. The confirm modal in the UI shows the exact
// command before it runs.
//
// Installed-state detection is derived from $PATH at runtime rather than
// persisted to disk. This means the UI's ✓ marker always reflects what
// the shell can actually run: a `brew uninstall` outside cliff makes the
// marker disappear on next detection, and an app the user installed
// before cliff existed is recognized immediately.
package install

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
)

// Result is what Stream reports back when the install finishes.
type Result struct {
	App      *catalog.App
	Command  string
	ExitCode int
	Output   string // combined stdout+stderr
	Err      error
}

// Stream runs the install command and invokes onLine for each line of
// combined stdout+stderr as it's produced. Blocks until the process
// exits or ctx is canceled (in which case the process is killed).
// Result.Output contains the full buffered output so callers can read
// it after completion without needing to have subscribed.
//
// onLine may be nil, in which case Stream just buffers the output.
// Callers in a TUI should invoke Stream off the main loop (inside a
// tea.Cmd) since it blocks for the duration of the install.
func Stream(ctx context.Context, app *catalog.App, onLine func(string)) Result {
	res := Result{App: app}
	if app == nil || app.InstallSpec == nil {
		res.Err = errors.New("app has no install spec")
		return res
	}
	cmd := app.InstallSpec.Shell()
	if cmd == "" {
		res.Err = errors.New("install spec produced empty command")
		return res
	}
	res.Command = cmd

	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	var output bytes.Buffer
	lineR, lineW := io.Pipe()
	c.Stdout = io.MultiWriter(&output, lineW)
	c.Stderr = io.MultiWriter(&output, lineW)

	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		sc := bufio.NewScanner(lineR)
		// Some installs emit very long lines (progress bars, long URLs).
		// Bump max token size well above the default 64 KiB.
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if onLine != nil {
				onLine(sc.Text())
			}
		}
		// If sc.Scan stopped early (bufio.ErrTooLong on a line > 1 MiB),
		// we MUST keep reading from the pipe. The command's stdout/stderr
		// go through io.MultiWriter(&output, lineW); a synchronous io.Pipe
		// with no reader would block those writes and deadlock c.Run.
		// Drain the rest so the full bytes still reach the output buffer.
		_, _ = io.Copy(io.Discard, lineR)
	}()

	err := c.Run()
	_ = lineW.Close()
	<-scanDone

	res.Output = output.String()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		} else {
			res.ExitCode = -1
		}
		res.Err = err
		return res
	}
	res.ExitCode = 0
	return res
}

// toolHints maps install.Type → a human-readable diagnosis for the case
// where the package manager itself isn't installed. Keyed by the same
// values that InstallSpec.Type accepts. The UI consults these via
// Diagnose to turn exit-127 failures into actionable guidance.
var toolHints = map[string]string{
	"brew":  "Homebrew isn't installed.\nGet it at https://brew.sh",
	"cargo": "Cargo isn't installed.\nInstall Rust at https://rustup.rs",
	"go":    "Go isn't installed.\nInstall it at https://go.dev/dl/",
	"pipx":  "pipx isn't installed.\nInstall it at https://pipx.pypa.io/stable/installation/",
	"npm":   "npm isn't installed.\nInstall Node.js at https://nodejs.org",
}

// Diagnose turns a failed install Result into a short human-readable
// hint, or "" if the failure isn't one we recognize. Two signals are
// consulted, in order of reliability:
//  1. Exit code 127 + a known InstallSpec.Type — shells set 127 when
//     the command wasn't found, and we know which tool we asked for.
//  2. Stderr scrape for "<tool>: command not found" — catches the
//     cases where the wrapper command exited differently.
//
// Callers display the hint verbatim alongside the raw output, so the
// user sees both "what actually happened" and "what to do about it".
func Diagnose(res Result) string {
	if res.Err == nil {
		return ""
	}
	if res.App != nil && res.App.InstallSpec != nil && res.ExitCode == 127 {
		if h, ok := toolHints[res.App.InstallSpec.Type]; ok {
			return h
		}
	}
	for tool, h := range toolHints {
		// Word-boundary match so "go" doesn't latch inside "cargo": the
		// output "/bin/sh: cargo: command not found" contains the
		// substring "go: command not found", and Go's randomized map
		// iteration would otherwise make the hint nondeterministic.
		if cmdNotFoundRes[tool].MatchString(res.Output) {
			return h
		}
	}
	return ""
}

// cmdNotFoundRes is one pre-compiled regex per tool name. Package-init
// so we compile once rather than on every Diagnose call.
var cmdNotFoundRes = func() map[string]*regexp.Regexp {
	out := make(map[string]*regexp.Regexp, len(toolHints))
	for tool := range toolHints {
		out[tool] = regexp.MustCompile(`\b` + regexp.QuoteMeta(tool) + `:\s*(command\s+)?not found`)
	}
	return out
}()

// Detect walks every directory on $PATH and returns a set of executable
// basenames found. Non-executable files and directories are skipped.
// This is the source of truth for the UI's ✓ marker.
func Detect() map[string]bool {
	out := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			// Any-execute bit set. On Windows this check would need
			// extension-based heuristics (.exe, .bat) — we accept the
			// Unix-only limitation consistent with the sh -c runner.
			if info.Mode()&0o111 == 0 {
				continue
			}
			out[e.Name()] = true
		}
	}
	return out
}

// InstalledApps returns a repo→installed map for the given catalog,
// computed against the current $PATH. Intended to be refreshed on
// startup and after install completion.
func InstalledApps(apps []catalog.App) map[string]bool {
	bins := Detect()
	out := map[string]bool{}
	for i := range apps {
		if bins[BinaryName(&apps[i])] {
			out[apps[i].Repo] = true
		}
	}
	return out
}

// BinaryName returns the expected executable name for an app. Today it's
// just the repo basename (charmbracelet/glow → "glow"), which matches
// the install convention for ~90% of CLI TUIs. Apps whose binary name
// differs from the repo (e.g. cli/cli → "gh", ClementTsang/bottom → "btm")
// would need a manifest binary: field — not wired yet.
func BinaryName(a *catalog.App) string {
	if a == nil {
		return ""
	}
	if i := strings.LastIndex(a.Repo, "/"); i >= 0 {
		return a.Repo[i+1:]
	}
	return a.Repo
}
