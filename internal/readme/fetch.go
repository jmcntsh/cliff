package readme

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Result struct {
	Markdown       string
	Err            error
	NotFound       bool
	RateLimited    bool
	ResetAt        time.Time
	FromCache      bool
}

func Fetch(owner, repo, token string) Result {
	cached := loadCache(owner, repo)

	req, err := http.NewRequest("GET",
		fmt.Sprintf("https://api.github.com/repos/%s/%s/readme", owner, repo), nil)
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
