package hotfetch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// withTempCache redirects the on-disk cache into a temp directory
// for the duration of the test. Restores HOME afterwards. Used
// across the table to keep cache state from leaking between cases.
func withTempCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	old := os.Getenv("HOME")
	if err := os.Setenv("HOME", dir); err != nil {
		t.Fatalf("setenv HOME: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("HOME", old) })
}

const sampleSidecar = `{
  "generated_at": "2026-05-01T00:05:00Z",
  "schema_version": 1,
  "half_life_days": 7,
  "window_days": 21,
  "days_seen": 14,
  "min_lifetime_ips": 5,
  "rows": [
    {"kind":"readme","key":"foo/bar","hot_score":42.5,"lifetime_ips":120},
    {"kind":"readme","key":"baz/qux","hot_score":7.1,"lifetime_ips":40},
    {"kind":"reel","key":"foo","hot_score":99.9,"lifetime_ips":200}
  ]
}`

func TestFetch_OK(t *testing.T) {
	withTempCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", `"abc"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSidecar))
	}))
	t.Cleanup(srv.Close)
	old := DefaultURL
	DefaultURL = srv.URL + "/hot.json"
	t.Cleanup(func() { DefaultURL = old })

	got := Fetch()
	if !got.Available {
		t.Fatalf("expected Available; got Err=%v", got.Err)
	}
	if got.FromCache {
		t.Errorf("first fetch should not be FromCache")
	}
	if score, ok := got.Scores["foo/bar"]; !ok || score != 42.5 {
		t.Errorf("expected foo/bar=42.5 from readme rows, got score=%v ok=%v", score, ok)
	}
	if _, ok := got.Scores["foo"]; ok {
		t.Errorf("reel-kind rows must not appear in Scores")
	}
}

func TestFetch_404_NoCache(t *testing.T) {
	withTempCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	old := DefaultURL
	DefaultURL = srv.URL + "/hot.json"
	t.Cleanup(func() { DefaultURL = old })

	got := Fetch()
	if got.Available {
		t.Errorf("expected Available=false on 404 with no cache, got %+v", got)
	}
	if got.Err != nil {
		t.Errorf("expected nil Err on plain 404, got %v", got.Err)
	}
}

func TestFetch_304_RevalidatesCache(t *testing.T) {
	withTempCache(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.Header().Set("Etag", `"abc"`)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sampleSidecar))
			return
		}
		// Second hit: assert the client sent If-None-Match and reply 304.
		if r.Header.Get("If-None-Match") != `"abc"` {
			t.Errorf("expected If-None-Match=\"abc\" on second fetch, got %q", r.Header.Get("If-None-Match"))
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(srv.Close)
	old := DefaultURL
	DefaultURL = srv.URL + "/hot.json"
	t.Cleanup(func() { DefaultURL = old })

	first := Fetch()
	if !first.Available || first.FromCache {
		t.Fatalf("first fetch: want Available && !FromCache, got %+v", first)
	}
	second := Fetch()
	if !second.Available || !second.FromCache {
		t.Fatalf("second fetch should serve from cache via 304, got %+v", second)
	}
	if score := second.Scores["foo/bar"]; score != 42.5 {
		t.Errorf("cached score lost: got %v", score)
	}
}

func TestFetch_404_FallsBackToFreshCache(t *testing.T) {
	withTempCache(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sampleSidecar))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	old := DefaultURL
	DefaultURL = srv.URL + "/hot.json"
	t.Cleanup(func() { DefaultURL = old })

	if !Fetch().Available {
		t.Fatal("first fetch should populate cache")
	}
	got := Fetch()
	if !got.Available || !got.FromCache {
		t.Errorf("a 404 with a fresh cache should serve cached, got %+v", got)
	}
}

func TestFetch_DisabledURL(t *testing.T) {
	withTempCache(t)
	old := DefaultURL
	DefaultURL = ""
	t.Cleanup(func() { DefaultURL = old })

	got := Fetch()
	if got.Available {
		t.Errorf("empty DefaultURL should disable fetch entirely, got %+v", got)
	}
}

func TestCache_Roundtrip(t *testing.T) {
	withTempCache(t)
	saveCache(`"v1"`, []byte(sampleSidecar), nil)
	c := loadCache()
	if c == nil {
		t.Fatal("expected non-nil cache after save")
	}
	if c.ETag != `"v1"` {
		t.Errorf("etag roundtrip: got %q", c.ETag)
	}
	if c.Sidecar == nil || len(c.Sidecar.Rows) != 3 {
		t.Errorf("expected 3 rows in cached sidecar, got %+v", c.Sidecar)
	}
}

func TestCache_Dir(t *testing.T) {
	withTempCache(t)
	saveCache("", []byte(sampleSidecar), nil)
	want := filepath.Join(os.Getenv("HOME"), ".cache", "cliff", "hot", "hot.json")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected cache file at %s, got: %v", want, err)
	}
}
