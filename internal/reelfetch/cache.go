package reelfetch

import (
	"os"
	"path/filepath"
)

// Cache layout:
//
//   ~/.cache/cliff/reels/<slug>.reel    bytes
//   ~/.cache/cliff/reels/<slug>.etag    optional etag from last fetch
//
// Flat (no per-owner subdirectory like the readme cache) because slugs
// are globally unique inside the registry — there is no /<owner>/<repo>
// shape to mirror. Same parent dir as the readme cache so a single
// `rm -rf ~/.cache/cliff` clears everything cliff has ever cached.

type cached struct {
	Bytes []byte
	ETag  string
}

func cacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "cliff", "reels")
}

func loadCache(slug string) *cached {
	dir := cacheDir()
	if dir == "" {
		return nil
	}
	bytes, err := os.ReadFile(filepath.Join(dir, slug+".reel"))
	if err != nil {
		return nil
	}
	etag, _ := os.ReadFile(filepath.Join(dir, slug+".etag"))
	return &cached{Bytes: bytes, ETag: string(etag)}
}

func saveCache(slug, etag string, bytes []byte) {
	dir := cacheDir()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	// Best-effort writes. A failed cache write means we'll re-fetch
	// next time — annoying but not broken — so we don't propagate the
	// error to the caller, who can't do anything useful with it.
	_ = os.WriteFile(filepath.Join(dir, slug+".reel"), bytes, 0o644)
	if etag != "" {
		_ = os.WriteFile(filepath.Join(dir, slug+".etag"), []byte(etag), 0o644)
	}
}
