package readme

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// TrackingRedirectURL routes README fetches through the cliff.sh
// Worker so we can count opens. The Worker logs one Analytics Engine
// data point and 302s to api.github.com/repos/<owner>/<repo>/readme.
//
// Two reasons it's a var, not a const:
//   - tests point it at a httptest server.
//   - operations may need to disable tracking quickly without a
//     client release. A future env var (CLIFF_NO_TRACK=1) can flip
//     this to "" to mean "always direct."
//
// We deliberately do NOT use the redirector when a GITHUB_TOKEN is
// present. Go's HTTP client strips Authorization headers across a
// cross-domain redirect (cliff.sh → api.github.com), which would
// silently degrade those users from a 5000/hr rate limit to 60/hr.
// See fetchURL below.
var TrackingRedirectURL = "https://cliff.sh/r/readme"

type Result struct {
	Markdown    string
	Err         error
	NotFound    bool
	RateLimited bool
	ResetAt     time.Time
	FromCache   bool
}

func Fetch(owner, repo, token string) Result {
	cached := loadCache(owner, repo)
	res := fetchFrom(fetchURL(owner, repo, token), owner, repo, token, cached)

	// Redirector fallback. If we asked the cliff.sh redirector and it
	// returned a hard failure (404 / 5xx / network error and no cache
	// to serve), retry against api.github.com directly. Two cases this
	// covers:
	//   1. Client release ships before the worker is deployed: every
	//      cliff.sh/r/readme/* returns 404. Without this fallback the
	//      whole TUI looks broken until the worker lands.
	//   2. The redirector goes down post-deploy. We'd rather miss a
	//      tracking event than show "no readme" for a real app.
	usedRedirector := token == "" && TrackingRedirectURL != ""
	if usedRedirector && shouldFallback(res) {
		direct := fmt.Sprintf("https://api.github.com/repos/%s/%s/readme", owner, repo)
		return fetchFrom(direct, owner, repo, token, cached)
	}
	return res
}

// shouldFallback reports whether the redirector result is bad enough
// to warrant a direct retry. We only retry on outcomes that have no
// useful information for the user: a 404 or 5xx with no cache, or a
// network error with no cache. A real 304/200, or a rate-limit, or a
// 404 served from cache, all stay as-is.
func shouldFallback(r Result) bool {
	if r.Markdown != "" || r.FromCache {
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

func fetchFrom(url, owner, repo, token string, cached *cached) Result {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Result{Err: err}
	}
	req.Header.Set("Accept", "application/vnd.github.raw")
	if cached != nil && cached.ETag != "" {
		req.Header.Set("If-None-Match", cached.ETag)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if cached != nil {
			return Result{Markdown: cached.Markdown, FromCache: true}
		}
		return Result{Err: err}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		if cached != nil {
			return Result{Markdown: cached.Markdown, FromCache: true}
		}
		return Result{Err: fmt.Errorf("304 without cache")}
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return Result{Err: err}
		}
		etag := resp.Header.Get("Etag")
		saveCache(owner, repo, etag, string(body))
		return Result{Markdown: string(body)}
	case http.StatusNotFound:
		return Result{NotFound: true}
	case http.StatusForbidden, http.StatusTooManyRequests:
		var resetAt time.Time
		if ts, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			resetAt = time.Unix(ts, 0)
		}
		if cached != nil {
			return Result{Markdown: cached.Markdown, FromCache: true, RateLimited: true, ResetAt: resetAt}
		}
		return Result{RateLimited: true, ResetAt: resetAt}
	default:
		return Result{Err: fmt.Errorf("github: %s", resp.Status)}
	}
}

// fetchURL returns the URL to fetch the README from. With a token
// present we hit api.github.com directly: Go strips Authorization on
// cross-domain redirects, and dropping a user from 5000/hr to 60/hr
// silently is the worse failure mode than missing those opens in our
// stats. Without a token we use the redirector, which logs one data
// point and 302s to the same upstream.
func fetchURL(owner, repo, token string) string {
	if token != "" || TrackingRedirectURL == "" {
		return fmt.Sprintf("https://api.github.com/repos/%s/%s/readme", owner, repo)
	}
	return fmt.Sprintf("%s/%s/%s", TrackingRedirectURL, owner, repo)
}
