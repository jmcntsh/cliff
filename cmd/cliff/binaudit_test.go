package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadMismatches_DedupesAndCounts verifies that repeated entries
// for the same repo collapse into a single mismatch with Seen
// incrementing, and that the most recent (derived, detected) pair
// wins — important because a manifest edit between installs could
// legitimately change the "derived" name and we want to report the
// latest state, not a stale one.
func TestReadMismatches_DedupesAndCounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin-audit.log")
	body := `2026-04-22T10:00:00Z bin-mismatch repo=cpcloud/minesweep-rs derived=minesweep-rs detected=minesweep
2026-04-22T11:00:00Z bin-mismatch repo=cli/cli derived=cli detected=gh
2026-04-22T12:00:00Z bin-mismatch repo=cpcloud/minesweep-rs derived=minesweep-rs detected=minesweep
something unrelated
2026-04-22T13:00:00Z bin-mismatch repo=ClementTsang/bottom derived=bottom detected=btm
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readMismatches(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 unique repos, got %d: %+v", len(got), got)
	}
	// Sorted by Seen desc, so minesweep-rs (Seen=2) should be first.
	if got[0].Repo != "cpcloud/minesweep-rs" || got[0].Seen != 2 {
		t.Errorf("expected minesweep-rs first with Seen=2, got %+v", got[0])
	}
	if got[0].Detected != "minesweep" {
		t.Errorf("expected Detected=minesweep, got %s", got[0].Detected)
	}
}

// TestReadMismatches_MissingFileNoError ensures the audit-log path not
// existing yet (fresh install, first run) is a normal "zero mismatches"
// case rather than an error. Matches the file's comment contract.
func TestReadMismatches_MissingFileNoError(t *testing.T) {
	got, err := readMismatches(filepath.Join(t.TempDir(), "does-not-exist.log"))
	if err != nil {
		t.Fatalf("missing file should return (nil, nil), got err=%v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 mismatches for missing file, got %d", len(got))
	}
}

// TestAppSlugFromRepo_BasenameLowercase covers the slug derivation
// used for the toml-patches output. Since this is the "where do I
// paste this?" hint, the common cases (owner/repo, already-lowercase,
// mixed case) all need to come out as the registry expects.
func TestAppSlugFromRepo_BasenameLowercase(t *testing.T) {
	cases := map[string]string{
		"cli/cli":               "cli",
		"ClementTsang/bottom":   "bottom",
		"cpcloud/minesweep-rs":  "minesweep-rs",
		"charmbracelet/GlowDev": "glowdev",
	}
	for repo, want := range cases {
		if got := appSlugFromRepo(repo); got != want {
			t.Errorf("appSlugFromRepo(%q) = %q, want %q", repo, got, want)
		}
	}
}
