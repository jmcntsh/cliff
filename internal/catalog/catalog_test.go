package catalog

import "testing"

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

func TestAppUninstallCommand_OverrideWins(t *testing.T) {
	app := App{
		Repo:          "foo/bar",
		InstallSpec:   &InstallSpec{Type: "brew", Package: "bar"},
		UninstallSpec: &CommandSpec{Command: "rm -rf /custom/path"},
	}
	if got := app.UninstallCommand(); got != "rm -rf /custom/path" {
		t.Errorf("override ignored: got %q", got)
	}
}

func TestAppUninstallCommand_DerivedFallback(t *testing.T) {
	app := App{
		Repo:        "foo/bar",
		InstallSpec: &InstallSpec{Type: "brew", Package: "bar"},
	}
	if got := app.UninstallCommand(); got != "brew uninstall bar" {
		t.Errorf("derived fallback wrong: got %q", got)
	}
}

func TestAppUninstallCommand_ScriptNoSpecReturnsEmpty(t *testing.T) {
	app := App{
		InstallSpec: &InstallSpec{Type: "script", Command: "curl ... | sh"},
	}
	if got := app.UninstallCommand(); got != "" {
		t.Errorf("script without UninstallSpec should be empty, got %q", got)
	}
}

func TestAppUpgradeCommand_GoLatestPathRewrite(t *testing.T) {
	app := App{
		InstallSpec: &InstallSpec{Type: "go", Package: "github.com/x/y@v1.2.3"},
	}
	if got := app.UpgradeCommand(); got != "go install github.com/x/y@latest" {
		t.Errorf("go @version should be rewritten to @latest, got %q", got)
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
