package readme

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchHitsGitHubReadmePath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	prev := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = prev }()

	res := Fetch("octocat", "hello-world", "")
	if gotPath != "/repos/octocat/hello-world/readme" {
		t.Errorf("path: got %q, want %q", gotPath, "/repos/octocat/hello-world/readme")
	}
	if !res.NotFound {
		t.Errorf("expected NotFound result, got %+v", res)
	}
}
