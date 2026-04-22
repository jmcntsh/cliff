// Package binmap persists a repo→binary-name override map learned
// from installer output. The manifest's binary name is sometimes
// wrong (cargo crate "minesweep" under repo "cpcloud/minesweep-rs"
// where BinaryName() picks the wrong name from the repo basename).
// Rather than require every such manifest to be hand-edited, cliff
// scrapes installer output at install time (install.Result.
// DetectedBinaries) and remembers what it saw here.
//
// This is a cache, not state: if the file disappears, nothing breaks
// — the on-disk $PATH scan in install.InstalledApps still answers
// "installed?" correctly; we just lose the right-name hint until the
// next install. That keeps binmap safely deletable and matches the
// project-wide principle (STATUS.md) of not persisting installed
// state. Honoring external uninstalls is fine: removing the binary
// from disk makes the ✓ disappear; a stale override is harmless
// because no bin of that name exists.
//
// On top of the cache, binmap also writes an audit line whenever a
// detected binary disagrees with the manifest-derived BinaryName().
// That log (~/.cliff/logs/bin-audit.log) becomes the source of
// truth for which registry manifests need a `binary` override —
// exactly the dataset we want for cleaning the catalog at scale.
package binmap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Path returns the override-cache path. Exposed for tests and to
// keep the "where does this live?" question answerable without
// grepping.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cliff", "cache", "binmap.json"), nil
}

// AuditPath returns the audit log path. Writes are append-only; the
// log is human-readable (one line per event) so you can tail it.
func AuditPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cliff", "logs", "bin-audit.log"), nil
}

// Map is the on-disk shape: repo path (e.g. "cpcloud/minesweep-rs")
// to the binary name that install actually produced ("minesweep").
// Using the repo as key — not the app name — because the repo is the
// stable identifier across schema bumps (App.Name is unique today
// but could collide if we ever imported from multiple sources).
type Map map[string]string

// Load reads the cache. A missing file returns an empty map, no
// error — first run must not look like failure. Corrupt JSON is
// logged to stderr and treated as empty so a bad write can't wedge
// cliff; the next successful install overwrites it.
func Load() Map {
	p, err := Path()
	if err != nil {
		return Map{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Map{}
	}
	m := Map{}
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintf(os.Stderr, "cliff: binmap cache corrupt (%v); ignoring\n", err)
		return Map{}
	}
	return m
}

// save writes the map atomically (temp file + rename) so a crash
// mid-write can't leave the cache half-rewritten. MkdirAll is safe
// to repeat; 0o700 on the cache dir because it lives under ~/.cliff
// and there's no reason other users should read it.
var saveMu sync.Mutex

func save(m Map) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Remember adds repo→bin to the on-disk cache and, if bin differs
// from derived (the BinaryName() the manifest derivation produced),
// appends an audit line. derived may be empty when the caller
// doesn't have it; no audit line is written in that case.
//
// A no-op when bin is empty: detection failures mustn't clobber a
// correct cached entry (e.g. re-install where the dir-diff comes up
// empty because the file already existed).
func Remember(repo, bin, derived string) error {
	if repo == "" || bin == "" {
		return nil
	}
	m := Load()
	prev := m[repo]
	m[repo] = bin
	if err := save(m); err != nil {
		return err
	}
	if derived != "" && derived != bin && prev != bin {
		// Only audit the first time we learn a new disagreement —
		// re-installs shouldn't spam the log. Best-effort: audit
		// errors are swallowed so they can't fail an install.
		_ = auditf("bin-mismatch repo=%s derived=%s detected=%s\n", repo, derived, bin)
	}
	return nil
}

// Forget removes a repo from the cache, used after a successful
// uninstall. Tolerates missing entries.
func Forget(repo string) error {
	if repo == "" {
		return nil
	}
	m := Load()
	if _, ok := m[repo]; !ok {
		return nil
	}
	delete(m, repo)
	return save(m)
}

// Sorted returns the entries in repo-sorted order for deterministic
// output in tests and any future `cliff doctor`-style command.
func Sorted(m Map) [][2]string {
	out := make([][2]string, 0, len(m))
	for k, v := range m {
		out = append(out, [2]string{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// auditf appends a timestamped line to the audit log.
func auditf(format string, args ...any) error {
	p, err := AuditPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	prefix := time.Now().UTC().Format("2006-01-02T15:04:05Z ")
	_, err = fmt.Fprintf(f, prefix+format, args...)
	return err
}
