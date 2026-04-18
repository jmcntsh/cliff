package readme

import (
	"os"
	"path/filepath"
	"time"
)

type cached struct {
	Markdown  string
	ETag      string
	FetchedAt time.Time
}

func cacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "cliff", "readme")
}

func loadCache(owner, repo string) *cached {
	base := cacheDir()
	if base == "" {
		return nil
	}
	dir := filepath.Join(base, owner)
	mdPath := filepath.Join(dir, repo+".md")
	md, err := os.ReadFile(mdPath)
	if err != nil {
		return nil
	}
	etag, _ := os.ReadFile(filepath.Join(dir, repo+".etag"))
	info, _ := os.Stat(mdPath)
	var fetchedAt time.Time
	if info != nil {
		fetchedAt = info.ModTime()
	}
	return &cached{
		Markdown:  string(md),
		ETag:      string(etag),
		FetchedAt: fetchedAt,
	}
}

func saveCache(owner, repo, etag, markdown string) {
	base := cacheDir()
	if base == "" {
		return
	}
	dir := filepath.Join(base, owner)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, repo+".md"), []byte(markdown), 0o644)
	if etag != "" {
		_ = os.WriteFile(filepath.Join(dir, repo+".etag"), []byte(etag), 0o644)
	}
}
