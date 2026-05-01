package readme

import (
	"errors"
	"testing"
)

func TestFetchURL(t *testing.T) {
	t.Run("no token routes through redirector", func(t *testing.T) {
		got := fetchURL("octocat", "hello-world", "")
		want := "https://cliff.sh/r/readme/octocat/hello-world"
		if got != want {
			t.Errorf("fetchURL: got %q, want %q", got, want)
		}
	})

	t.Run("with token bypasses redirector", func(t *testing.T) {
		got := fetchURL("octocat", "hello-world", "ghp_abc")
		want := "https://api.github.com/repos/octocat/hello-world/readme"
		if got != want {
			t.Errorf("fetchURL: got %q, want %q", got, want)
		}
	})

	t.Run("empty redirect var bypasses redirector", func(t *testing.T) {
		prev := TrackingRedirectURL
		TrackingRedirectURL = ""
		defer func() { TrackingRedirectURL = prev }()

		got := fetchURL("octocat", "hello-world", "")
		want := "https://api.github.com/repos/octocat/hello-world/readme"
		if got != want {
			t.Errorf("fetchURL: got %q, want %q", got, want)
		}
	})
}

func TestShouldFallback(t *testing.T) {
	tests := []struct {
		name string
		r    Result
		want bool
	}{
		{"successful fetch is final", Result{Markdown: "# hi"}, false},
		{"cache-served is final", Result{Markdown: "# hi", FromCache: true}, false},
		{"rate-limited is final", Result{RateLimited: true}, false},
		{"404 with no cache falls back", Result{NotFound: true}, true},
		{"network error with no cache falls back", Result{Err: errors.New("dial tcp: refused")}, true},
		{"empty cache-served NotFound stays final", Result{NotFound: true, FromCache: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldFallback(tt.r); got != tt.want {
				t.Errorf("shouldFallback(%+v) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}
