package catalog

import (
	"strings"
	"testing"
)

func TestAppBinaryName(t *testing.T) {
	cases := []struct {
		name   string
		app    App
		want   string
	}{
		{"derived from repo basename", App{Repo: "charmbracelet/glow"}, "glow"},
		{"derived, single-segment repo", App{Repo: "standalone"}, "standalone"},
		{"derived, empty repo", App{}, ""},
		{"override wins over derivation", App{Repo: "cli/cli", Binary: "gh"}, "gh"},
		{"override with no repo", App{Binary: "btm"}, "btm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.app.BinaryName(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolvedBinaryName_OverridesWin(t *testing.T) {
	app := App{Repo: "cpcloud/minesweep-rs"} // derived = "minesweep-rs"
	overrides := map[string]string{"cpcloud/minesweep-rs": "minesweep"}
	if got := app.ResolvedBinaryName(overrides); got != "minesweep" {
		t.Errorf("overrides should win, got %q", got)
	}
}

func TestResolvedBinaryName_FallbackWhenNoOverride(t *testing.T) {
	app := App{Repo: "charmbracelet/glow"}
	if got := app.ResolvedBinaryName(nil); got != "glow" {
		t.Errorf("nil overrides should fall through to BinaryName, got %q", got)
	}
	if got := app.ResolvedBinaryName(map[string]string{"other/repo": "x"}); got != "glow" {
		t.Errorf("non-matching overrides should fall through, got %q", got)
	}
}

// TestUninstallCommandWithOverrides_GoUsesDetectedBinary pins the
// bug this whole plumbing was built to fix: for a go-install whose
// manifest-derived binary name is wrong, the derived `rm -f` recipe
// must target the name we actually saw installed, not the repo
// basename. Without this, uninstall silently removes nothing.
func TestUninstallCommandWithOverrides_GoUsesDetectedBinary(t *testing.T) {
	app := App{
		Repo:        "author/repo-with-weird-suffix",
		InstallSpecs: []InstallSpec{{Type: "go", Package: "example.com/author/repo-with-weird-suffix@latest"}},
	}
	overrides := map[string]string{"author/repo-with-weird-suffix": "actualbin"}
	got := app.UninstallCommandWithOverrides(overrides)
	if !strings.Contains(got, "/actualbin") {
		t.Errorf("expected uninstall to target /actualbin, got %q", got)
	}
	if strings.Contains(got, "/repo-with-weird-suffix") {
		t.Errorf("uninstall should NOT target the stale derived name, got %q", got)
	}
}

func TestAppUninstallCommand_OverrideWins(t *testing.T) {
	app := App{
		Repo:          "foo/bar",
		InstallSpecs:  []InstallSpec{{Type: "brew", Package: "bar"}},
		UninstallSpec: &CommandSpec{Command: "rm -rf /custom/path"},
	}
	if got := app.UninstallCommand(); got != "rm -rf /custom/path" {
		t.Errorf("override ignored: got %q", got)
	}
}

func TestAppUninstallCommand_DerivedFallback(t *testing.T) {
	app := App{
		Repo:        "foo/bar",
		InstallSpecs: []InstallSpec{{Type: "brew", Package: "bar"}},
	}
	if got := app.UninstallCommand(); got != "brew uninstall bar" {
		t.Errorf("derived fallback wrong: got %q", got)
	}
}

func TestAppUninstallCommand_ScriptNoSpecReturnsEmpty(t *testing.T) {
	app := App{
		InstallSpecs: []InstallSpec{{Type: "script", Command: "curl ... | sh"}},
	}
	if got := app.UninstallCommand(); got != "" {
		t.Errorf("script without UninstallSpec should be empty, got %q", got)
	}
}

func TestAppUpgradeCommand_GoLatestPathRewrite(t *testing.T) {
	app := App{
		InstallSpecs: []InstallSpec{{Type: "go", Package: "github.com/x/y@v1.2.3"}},
	}
	if got := app.UpgradeCommand(); got != "go install github.com/x/y@latest" {
		t.Errorf("go @version should be rewritten to @latest, got %q", got)
	}
}

func TestAppUninstallCommand_GoUsesRuntimeGoEnv(t *testing.T) {
	// The go-type uninstall must resolve GOBIN/GOPATH at runtime via
	// `go env` — shell-side $GOBIN/$GOPATH are empty for asdf users
	// and a naive shell fallback would point at ~/go/bin where the
	// binary doesn't actually live. Regression guard for that bug.
	app := App{
		Repo:        "foo/bar",
		InstallSpecs: []InstallSpec{{Type: "go", Package: "github.com/foo/bar@latest"}},
	}
	got := app.UninstallCommand()
	if !strings.Contains(got, "go env GOBIN") {
		t.Errorf("go-uninstall must use `go env GOBIN` at runtime, got %q", got)
	}
	if !strings.Contains(got, "/bar") {
		t.Errorf("go-uninstall should target the binary name, got %q", got)
	}
	if strings.Contains(got, "$HOME/go") {
		t.Errorf("go-uninstall should not shell-expand to $HOME/go (breaks under asdf), got %q", got)
	}
}

// TestEmbeddedSnapshotLoads guards that the build-time registry.cliff.sh
// snapshot shipped inside the binary parses and isn't empty. It's the
// fallback users land on when offline, so it needs to be valid at all
// times — not just when we remember to check.
func TestEmbeddedSnapshotLoads(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", c.SchemaVersion)
	}
	if len(c.Apps) == 0 {
		t.Fatal("embedded snapshot has no apps")
	}

	seen := make(map[string]bool)
	for _, app := range c.Apps {
		if app.Name == "" {
			t.Errorf("empty name (repo=%q)", app.Repo)
		}
		if seen[app.Repo] {
			t.Errorf("duplicate repo: %s", app.Repo)
		}
		seen[app.Repo] = true
	}
}
