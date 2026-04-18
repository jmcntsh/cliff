package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRegistryURL is the canonical published index. Override via the
// CLIFF_REGISTRY_URL env var (also handy for local dev with file:// URLs
// or self-hosted private registries).
const DefaultRegistryURL = "https://registry.cliff.sh/index.json"

// RegistryURL returns the URL to fetch the registry index from, honoring
// CLIFF_REGISTRY_URL if set.
func RegistryURL() string {
	if u := strings.TrimSpace(os.Getenv("CLIFF_REGISTRY_URL")); u != "" {
		return u
	}
	return DefaultRegistryURL
}

// Source describes how the active catalog was obtained — useful for the
// UI footer and for explaining "why are you showing the old list" when
// offline.
type Source int

const (
	SourceUnknown Source = iota
	SourceRegistry
	SourceCache
	SourceEmbedded
)

func (s Source) String() string {
	switch s {
	case SourceRegistry:
		return "registry"
	case SourceCache:
		return "cache"
	case SourceEmbedded:
		return "embedded"
	}
	return "unknown"
}

// LoadOptions controls catalog fetching. Zero value is safe and uses the
// default registry URL, the user's cache dir, and a 10s HTTP timeout.
type LoadOptions struct {
	URL      string
	CacheDir string
	Timeout  time.Duration
	Client   *http.Client
}

// LoadResult bundles the catalog with metadata about where it came from
// and any non-fatal error the fetch produced (e.g. network failure that
// triggered a fallback).
type LoadResult struct {
	Catalog *Catalog
	Source  Source
	URL     string
	Err     error
}

// LoadWithFallback tries, in order: fetch from URL (using ETag-cached copy
// if present), fall back to the cached copy, fall back to the embedded
// scrape. It only returns an error if even the embedded catalog fails to
// load; partial failures are reported via LoadResult.Err.
func LoadWithFallback(opts LoadOptions) LoadResult {
	if opts.URL == "" {
		opts.URL = RegistryURL()
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.CacheDir == "" {
		opts.CacheDir = defaultCacheDir()
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: opts.Timeout}
	}

	cat, source, fetchErr := tryFetch(client, opts.URL, opts.CacheDir)
	if cat != nil {
		return LoadResult{Catalog: cat, Source: source, URL: opts.URL, Err: fetchErr}
	}

	if cat, err := loadCache(opts.CacheDir); err == nil && cat != nil {
		return LoadResult{Catalog: cat, Source: SourceCache, URL: opts.URL, Err: fetchErr}
	}

	cat, err := Load()
	if err != nil {
		return LoadResult{Err: fmt.Errorf("all catalog sources failed: fetch=%v, embedded=%v", fetchErr, err)}
	}
	// The embedded catalog is the awesome-tuis scrape, which has no
	// InstallSpecs. Overlay curated registry manifests so glow/lazygit/etc.
	// are still installable when the live registry isn't reachable.
	overlayRegistry(cat)
	return LoadResult{Catalog: cat, Source: SourceEmbedded, URL: opts.URL, Err: fetchErr}
}

func tryFetch(client *http.Client, raw, cacheDir string) (*Catalog, Source, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, SourceUnknown, fmt.Errorf("parse url: %w", err)
	}

	if u.Scheme == "file" || u.Scheme == "" {
		return loadLocal(u.Path)
	}

	cachePath, etagPath := cachePaths(cacheDir)
	prevETag, _ := os.ReadFile(etagPath)

	req, err := http.NewRequest("GET", raw, nil)
	if err != nil {
		return nil, SourceUnknown, err
	}
	req.Header.Set("User-Agent", "cliff/0 (+https://cliff.sh)")
	if len(prevETag) > 0 {
		req.Header.Set("If-None-Match", strings.TrimSpace(string(prevETag)))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, SourceUnknown, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		cat, err := loadCache(cacheDir)
		if err != nil {
			return nil, SourceUnknown, fmt.Errorf("304 but cache unreadable: %w", err)
		}
		return cat, SourceCache, nil
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, SourceUnknown, err
		}
		var c Catalog
		if err := json.Unmarshal(body, &c); err != nil {
			return nil, SourceUnknown, fmt.Errorf("parse remote index: %w", err)
		}
		writeCache(cachePath, etagPath, body, resp.Header.Get("ETag"))
		return &c, SourceRegistry, nil
	default:
		return nil, SourceUnknown, fmt.Errorf("registry returned %s", resp.Status)
	}
}

func loadLocal(path string) (*Catalog, Source, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, SourceUnknown, err
	}
	var c Catalog
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, SourceUnknown, fmt.Errorf("parse %s: %w", path, err)
	}
	return &c, SourceRegistry, nil
}

func loadCache(cacheDir string) (*Catalog, error) {
	if cacheDir == "" {
		return nil, errors.New("no cache dir")
	}
	cachePath, _ := cachePaths(cacheDir)
	body, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func writeCache(cachePath, etagPath string, body []byte, etag string) {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(cachePath, body, 0o644)
	if etag != "" {
		_ = os.WriteFile(etagPath, []byte(etag), 0o644)
	} else {
		_ = os.Remove(etagPath)
	}
}

// cachePaths returns the on-disk paths for the cached index and its ETag
// sidecar. We cache one index at a time; if/when CLIFF_REGISTRY_URL
// becomes a real multi-tenant story (private registries, teams), this
// can grow a key.
func cachePaths(cacheDir string) (cachePath, etagPath string) {
	cachePath = filepath.Join(cacheDir, "index.json")
	etagPath = filepath.Join(cacheDir, "index.json.etag")
	return
}

func defaultCacheDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cliff", "cache")
	}
	return ""
}
