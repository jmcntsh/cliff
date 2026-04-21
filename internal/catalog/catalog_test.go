package catalog

import "testing"

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
