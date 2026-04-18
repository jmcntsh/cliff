package catalog

import "testing"

func TestEmbeddedCatalogInvariants(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if c.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", c.SchemaVersion)
	}
	if len(c.Apps) == 0 {
		t.Fatal("catalog has no apps")
	}

	seenRepo := make(map[string]bool)
	for _, app := range c.Apps {
		if app.Name == "" {
			t.Errorf("empty name (repo=%q)", app.Repo)
		}
		if app.Description == "" {
			t.Errorf("empty description (repo=%q)", app.Repo)
		}
		if seenRepo[app.Repo] {
			t.Errorf("duplicate repo: %s", app.Repo)
		}
		seenRepo[app.Repo] = true
	}

	knownCats := make(map[string]bool)
	for _, cat := range c.Categories {
		knownCats[cat.Name] = true
	}
	for _, app := range c.Apps {
		if !knownCats[app.Category] {
			t.Errorf("app %s references unknown category %q", app.Repo, app.Category)
		}
	}
}
