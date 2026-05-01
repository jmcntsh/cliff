package reelfetch

import (
	"errors"
	"testing"
)

func TestFetchURL(t *testing.T) {
	t.Run("default routes through redirector with no extension", func(t *testing.T) {
		got := fetchURL("lazygit")
		want := "https://cliff.sh/r/reel/lazygit"
		if got != want {
			t.Errorf("fetchURL: got %q, want %q", got, want)
		}
	})

	t.Run("empty redirect var falls back to direct .reel URL", func(t *testing.T) {
		prev := TrackingRedirectURL
		TrackingRedirectURL = ""
		defer func() { TrackingRedirectURL = prev }()

		got := fetchURL("lazygit")
		want := "https://registry.cliff.sh/reels/lazygit.reel"
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
		{"successful fetch is final", Result{Bytes: []byte("x")}, false},
		{"cache-served is final", Result{Bytes: []byte("x"), FromCache: true}, false},
		{"404 with no cache falls back", Result{NotFound: true}, true},
		{"network error with no cache falls back", Result{Err: errors.New("dial tcp")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldFallback(tt.r); got != tt.want {
				t.Errorf("shouldFallback(%+v) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}
