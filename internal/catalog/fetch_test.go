package catalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sampleCatalog() Catalog {
	return Catalog{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC(),
		SourceCommit:  "test",
		Apps: []App{{
			Name: "x", Repo: "u/x", Description: "y", Category: "Other",
		}},
		Categories: []Category{{Name: "Other", Count: 1}},
	}
}

func TestLoadWithFallback_Registry200(t *testing.T) {
	cat := sampleCatalog()
	body, _ := json.Marshal(cat)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc"`)
		w.WriteHeader(200)
		w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	res := LoadWithFallback(LoadOptions{URL: srv.URL, CacheDir: dir})

	if res.Err != nil {
		t.Fatalf("err: %v", res.Err)
	}
	if res.Source != SourceRegistry {
		t.Errorf("source = %s, want registry", res.Source)
	}
	if got := res.Catalog.Apps[0].Name; got != "x" {
		t.Errorf("app name = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.json")); err != nil {
		t.Errorf("expected cache file: %v", err)
	}
	if etag, _ := os.ReadFile(filepath.Join(dir, "index.json.etag")); string(etag) != `"abc"` {
		t.Errorf("etag not written: %q", etag)
	}
}

func TestLoadWithFallback_304UsesCache(t *testing.T) {
	dir := t.TempDir()
	cat := sampleCatalog()
	body, _ := json.Marshal(cat)
	os.WriteFile(filepath.Join(dir, "index.json"), body, 0o644)
	os.WriteFile(filepath.Join(dir, "index.json.etag"), []byte(`"abc"`), 0o644)

	var sentETag string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentETag = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	res := LoadWithFallback(LoadOptions{URL: srv.URL, CacheDir: dir})

	if res.Err != nil {
		t.Fatalf("err: %v", res.Err)
	}
	if sentETag != `"abc"` {
		t.Errorf("If-None-Match = %q, want \"abc\"", sentETag)
	}
	if res.Source != SourceCache {
		t.Errorf("source = %s, want cache", res.Source)
	}
}

func TestLoadWithFallback_NetworkFailFallsBackToCache(t *testing.T) {
	dir := t.TempDir()
	cat := sampleCatalog()
	body, _ := json.Marshal(cat)
	os.WriteFile(filepath.Join(dir, "index.json"), body, 0o644)

	res := LoadWithFallback(LoadOptions{
		URL: "http://127.0.0.1:1/never", CacheDir: dir,
		Timeout: 100 * time.Millisecond,
	})

	if res.Catalog == nil {
		t.Fatalf("expected fallback catalog, got nil (err=%v)", res.Err)
	}
	if res.Source != SourceCache {
		t.Errorf("source = %s, want cache", res.Source)
	}
	if res.Err == nil {
		t.Errorf("expected non-nil err to record the fetch failure")
	}
}

func TestLoadWithFallback_NoCacheFallsBackToEmbedded(t *testing.T) {
	dir := t.TempDir()
	res := LoadWithFallback(LoadOptions{
		URL: "http://127.0.0.1:1/never", CacheDir: dir,
		Timeout: 100 * time.Millisecond,
	})
	if res.Catalog == nil {
		t.Fatalf("expected embedded catalog, got nil (err=%v)", res.Err)
	}
	if res.Source != SourceEmbedded {
		t.Errorf("source = %s, want embedded", res.Source)
	}
}

func TestLoadWithFallback_FileURL(t *testing.T) {
	dir := t.TempDir()
	cat := sampleCatalog()
	body, _ := json.Marshal(cat)
	path := filepath.Join(dir, "index.json")
	os.WriteFile(path, body, 0o644)

	res := LoadWithFallback(LoadOptions{URL: "file://" + path, CacheDir: dir})
	if res.Err != nil {
		t.Fatalf("err: %v", res.Err)
	}
	if res.Catalog.Apps[0].Name != "x" {
		t.Errorf("app name = %q", res.Catalog.Apps[0].Name)
	}
}

func TestRegistryURL_EnvOverride(t *testing.T) {
	t.Setenv("CLIFF_REGISTRY_URL", "https://example.com/i.json")
	if got := RegistryURL(); got != "https://example.com/i.json" {
		t.Errorf("got %q", got)
	}
	t.Setenv("CLIFF_REGISTRY_URL", "")
	if got := RegistryURL(); got != DefaultRegistryURL {
		t.Errorf("got %q, want default", got)
	}
}
