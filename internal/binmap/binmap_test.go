package binmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempHome points $HOME at a fresh temp dir so Path() and
// AuditPath() resolve under the test's private filesystem. Restores
// on cleanup via t.Setenv's built-in behavior.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	withTempHome(t)
	got := Load()
	if len(got) != 0 {
		t.Errorf("expected empty map for missing file, got %v", got)
	}
}

func TestRememberAndLoad_RoundTrip(t *testing.T) {
	home := withTempHome(t)

	if err := Remember("cpcloud/minesweep-rs", "minesweep", "minesweep-rs"); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	got := Load()
	if got["cpcloud/minesweep-rs"] != "minesweep" {
		t.Errorf("round-trip: got %v", got)
	}

	// File is under ~/.cliff/cache.
	expected := filepath.Join(home, ".cliff", "cache", "binmap.json")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("cache file not at %s: %v", expected, err)
	}
}

// TestRemember_WritesAuditOnMismatch pins the audit behavior: when
// the detected name differs from the derived guess, a line is
// appended to the audit log. That log is how we'll discover which
// catalog manifests need `binary` overrides.
func TestRemember_WritesAuditOnMismatch(t *testing.T) {
	withTempHome(t)
	if err := Remember("cpcloud/minesweep-rs", "minesweep", "minesweep-rs"); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	p, _ := AuditPath()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "cpcloud/minesweep-rs") ||
		!strings.Contains(s, "derived=minesweep-rs") ||
		!strings.Contains(s, "detected=minesweep") {
		t.Errorf("audit line missing expected fields: %q", s)
	}
}

// TestRemember_NoAuditWhenDerivedMatches: if the installer confirmed
// what the manifest already said, there's nothing to audit. Keeps
// the log focused on actionable discrepancies.
func TestRemember_NoAuditWhenDerivedMatches(t *testing.T) {
	withTempHome(t)
	if err := Remember("charmbracelet/glow", "glow", "glow"); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	p, _ := AuditPath()
	if _, err := os.Stat(p); err == nil {
		data, _ := os.ReadFile(p)
		t.Errorf("expected no audit log for matching derived, got: %q", string(data))
	}
}

// TestRemember_EmptyBinIsNoOp: detection failures must not clobber a
// correct cached entry. A later re-install where dir-diff comes up
// empty (because the file already existed) shouldn't forget the
// override we learned earlier.
func TestRemember_EmptyBinIsNoOp(t *testing.T) {
	withTempHome(t)
	_ = Remember("u/app", "the-real-bin", "app")
	_ = Remember("u/app", "", "app") // simulate a detection-empty re-install
	got := Load()
	if got["u/app"] != "the-real-bin" {
		t.Errorf("empty-bin Remember clobbered a prior entry: %v", got)
	}
}

func TestForget_RemovesEntry(t *testing.T) {
	withTempHome(t)
	_ = Remember("u/app", "bin", "app")
	if err := Forget("u/app"); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	got := Load()
	if _, ok := got["u/app"]; ok {
		t.Errorf("Forget left entry: %v", got)
	}
}

func TestLoad_CorruptJSONReturnsEmpty(t *testing.T) {
	withTempHome(t)
	p, _ := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if len(got) != 0 {
		t.Errorf("expected empty map on corrupt JSON, got %v", got)
	}
}
