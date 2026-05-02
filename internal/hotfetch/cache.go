package hotfetch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// On-disk cache layout:
//
//   ~/.cache/cliff/hot/hot.json       latest fetched body
//   ~/.cache/cliff/hot/hot.json.etag  optional etag for revalidation
//
// One file, not slug-keyed: hot.json *is* the per-app data, there's
// nothing to shard by. Same parent dir as the readme/reel caches so
// `rm -rf ~/.cache/cliff` clears everything cliff has cached.

type cached struct {
	Sidecar   *Sidecar
	ETag      string
	FetchedAt time.Time
}

func cacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "cliff", "hot")
}

func loadCache() *cached {
	dir := cacheDir()
	if dir == "" {
		return nil
	}
	body, err := os.ReadFile(filepath.Join(dir, "hot.json"))
	if err != nil {
		return nil
	}
	var s Sidecar
	if err := json.Unmarshal(body, &s); err != nil {
		return nil
	}
	etag, _ := os.ReadFile(filepath.Join(dir, "hot.json.etag"))
	info, _ := os.Stat(filepath.Join(dir, "hot.json"))
	var fetchedAt time.Time
	if info != nil {
		fetchedAt = info.ModTime()
	}
	return &cached{
		Sidecar:   &s,
		ETag:      string(etag),
		FetchedAt: fetchedAt,
	}
}

func saveCache(etag string, body []byte, _ *Sidecar) {
	dir := cacheDir()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	// Best-effort writes. A failed cache write means we'll
	// re-fetch next time — annoying but not broken — so we don't
	// propagate the error to the caller, who can't do anything
	// useful with it.
	_ = os.WriteFile(filepath.Join(dir, "hot.json"), body, 0o644)
	if etag != "" {
		_ = os.WriteFile(filepath.Join(dir, "hot.json.etag"), []byte(etag), 0o644)
	} else {
		_ = os.Remove(filepath.Join(dir, "hot.json.etag"))
	}
}
