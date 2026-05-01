// Package reelfetch downloads and caches per-app reel artifacts from
// the registry's static host (https://registry.cliff.sh/reels/<slug>.reel).
//
// It mirrors the shape of the readme package on purpose: same
// blocking Fetch, same on-disk cache layout, same etag-revalidation
// flow, same Result struct kind. Two artifacts that are fetched the
// same way and cached the same way should be plumbed the same way —
// the readme package's design works fine, so we copy it rather than
// invent a second pattern callers have to learn.
//
// Reels are content-addressable in practice (we publish a fresh file
// when a demo changes; nothing rewrites in place), so cache freshness
// is not critical — even a multi-day stale entry plays the right
// thing for the listed app. The etag check exists because GitHub
// Pages returns one for free and it lets us avoid re-downloading on
// every focus, not because the user would notice if we skipped it.
//
// Tracking: by default Fetch routes through the cliff.sh Worker
// (cliff.sh/r/reel/<slug>), which logs one Analytics Engine data
// point and 302s to the registry. There is no Authorization header
// involved, so the cross-domain redirect is lossless. Set
// TrackingRedirectURL to "" to bypass the redirector and hit the
// registry directly.
package reelfetch

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// TrackingRedirectURL is the cliff.sh Worker route that logs one
// data point per reel fetch and 302s to the registry. Var, not
// const, so tests can swap it for a httptest server, and so we can
// disable tracking at runtime without a client release.
var TrackingRedirectURL = "https://cliff.sh/r/reel"

// DirectBaseURL is the registry's static reel host. Used when
// TrackingRedirectURL is empty (tests, opt-out flag in future, or
// the redirector being down — though Cloudflare and GitHub Pages are
// in different failure domains so a redirector outage is rare).
var DirectBaseURL = "https://registry.cliff.sh/reels"

// Result is the outcome of a Fetch call. Exactly one of Bytes, Err,
// or NotFound is meaningful per result; FromCache tags the Bytes
// path when the returned data came from disk rather than the wire.
//
// We intentionally don't surface "rate limited" the way the readme
// fetcher does — GitHub Pages doesn't apply the API rate limit to
// static asset requests, so the failure modes are just network
// errors and 404s.
type Result struct {
	Bytes     []byte
	Err       error
	NotFound  bool
	FromCache bool
}

// Fetch returns the reel bytes for the given slug, with a single
// GET against the redirector (or the registry host if the
// redirector is disabled) and a local file cache for repeat
// requests. Network errors fall through to cache when one exists,
// matching the readme fetcher's offline-friendly contract.
//
// The slug is the app's name field from its manifest (lowercase,
// hyphenated), which is also the URL path component the registry's
// publish step writes to (`public/reels/<slug>.reel` in the
// workflow). No URL-escaping needed — slugs are validated by the
// registry's lint step to be `[a-z0-9-]+` only.
func Fetch(slug string) Result {
	cached := loadCache(slug)
	res := fetchFrom(fetchURL(slug), slug, cached)

	// Redirector fallback. If we asked the cliff.sh redirector and it
	// returned a hard failure (404 / 5xx / network error and no cache
	// to serve), retry against the registry directly. Mirrors the
	// readme fetcher's fallback for the same two reasons:
	//   1. Client may ship before the worker is deployed.
	//   2. Cloudflare and GitHub Pages are in different failure
	//      domains; we'd rather miss a tracking event than show
	//      "no reel" for an app that has one.
	// 404 is a special case: a real "this app has no reel" 404 from
	// the registry is the *expected* answer for many apps. We let the
	// fallback path produce that same NotFound, which lands at the
	// caller with the same meaning.
	if TrackingRedirectURL != "" && shouldFallback(res) {
		return fetchFrom(fmt.Sprintf("%s/%s.reel", DirectBaseURL, slug), slug, cached)
	}
	return res
}

// shouldFallback reports whether the redirector result is bad enough
// to warrant a direct retry. We retry on 404 and on network errors
// when there's no cache to serve. A successful fetch, a cache-served
// result, or a real registry response (3xx/2xx) all stay as-is.
func shouldFallback(r Result) bool {
	if len(r.Bytes) > 0 || r.FromCache {
		return false
	}
	if r.NotFound {
		return true
	}
	if r.Err != nil {
		return true
	}
	return false
}

func fetchFrom(url, slug string, cached *cached) Result {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Result{Err: err}
	}
	if cached != nil && cached.ETag != "" {
		req.Header.Set("If-None-Match", cached.ETag)
	}

	// 15s matches readme.Fetch — same network, same patience budget.
	// Reels are larger on average (median ~6KB, p99 ~280KB for the
	// animated weathr capture), but on any working connection that's
	// well under a second. The timeout is for the connection-stuck
	// case, not the slow-but-progressing case.
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if cached != nil {
			return Result{Bytes: cached.Bytes, FromCache: true}
		}
		return Result{Err: err}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		if cached != nil {
			return Result{Bytes: cached.Bytes, FromCache: true}
		}
		// GitHub Pages returned 304 but we have no local copy. Should
		// be impossible (we only send If-None-Match when we have one),
		// but if it happens, surface as a real error rather than an
		// empty success — empty bytes would render as a blank strip.
		return Result{Err: fmt.Errorf("304 without cache")}
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			if cached != nil {
				return Result{Bytes: cached.Bytes, FromCache: true}
			}
			return Result{Err: err}
		}
		etag := resp.Header.Get("Etag")
		saveCache(slug, etag, body)
		return Result{Bytes: body}
	case http.StatusNotFound:
		// 404 is a normal outcome: not every registry app has a reel
		// yet (during a transitional period after a new submission),
		// and the client should silently skip the strip rather than
		// flag it as an error. Distinct from Err so callers can tell.
		return Result{NotFound: true}
	default:
		if cached != nil {
			return Result{Bytes: cached.Bytes, FromCache: true}
		}
		return Result{Err: fmt.Errorf("registry: %s", resp.Status)}
	}
}

// fetchURL builds the URL for a reel fetch. Redirector path emits
// `<base>/<slug>` with no extension (the Worker appends `.reel` on
// the upstream side); direct path emits `<base>/<slug>.reel`.
func fetchURL(slug string) string {
	if TrackingRedirectURL != "" {
		return fmt.Sprintf("%s/%s", TrackingRedirectURL, slug)
	}
	return fmt.Sprintf("%s/%s.reel", DirectBaseURL, slug)
}
