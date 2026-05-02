// Package hotfetch downloads the daily-computed hot-score sidecar
// (https://cliff.sh/hot.json) and converts it to a per-app score map
// the UI can layer onto the catalog.
//
// The sidecar is published by the cliff.sh worker once per UTC day,
// gated on at least 14 days of telemetry data (see
// web/worker/src/index.js). Until that gate flips, the URL returns a
// 404 and the client treats it as "hot data not available; hide the
// surface" — same fallback shape as the readme/reel redirectors.
//
// We deliberately do not invent a second pattern callers learn:
// blocking Fetch, Result struct kind, on-disk cache with ETag
// revalidation. Mirrors internal/readme and internal/reelfetch.
package hotfetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultURL is where the worker publishes the daily hot.json. Var,
// not const, so tests can swap it for a httptest server. Setting it
// to "" disables the fetch entirely (the client surface stays
// hidden).
var DefaultURL = "https://cliff.sh/hot.json"

// Row matches the worker's emit shape (web/worker/src/index.js,
// aggregateHot). Kind distinguishes "readme" from "reel" so we
// can layer scores per signal in the future; today the client only
// looks at "readme" — README opens are the strongest engagement
// signal we collect, and aggregating both kinds would double-count
// users who scrolled both surfaces in the same session.
type Row struct {
	Kind        string  `json:"kind"`
	Key         string  `json:"key"`
	HotScore    float64 `json:"hot_score"`
	LifetimeIPs int     `json:"lifetime_ips"`
}

// Sidecar is the wire shape of cliff.sh/hot.json. Field set tracks
// schema_version for forward-compat: a v2 with a different shape
// would just fail to unmarshal here, the result would be a Result
// with no Scores, and the UI would degrade as if the file 404'd.
type Sidecar struct {
	GeneratedAt    time.Time `json:"generated_at"`
	SchemaVersion  int       `json:"schema_version"`
	HalfLifeDays   int       `json:"half_life_days"`
	WindowDays     int       `json:"window_days"`
	DaysSeen       int       `json:"days_seen"`
	MinLifetimeIPs int       `json:"min_lifetime_ips"`
	Rows           []Row     `json:"rows"`
}

// Result is the outcome of a Fetch call. Available is true when we
// got a usable sidecar from somewhere (network or cache); the UI
// keys all of its reveal logic off this and off len(Scores) rather
// than off Err, because a 404 is the *expected* steady state during
// the first ~14 days post-deploy.
type Result struct {
	Available bool
	Scores    map[string]float64 // repo -> hot_score, "readme" kind only
	Sidecar   *Sidecar
	Err       error
	FromCache bool
}

// Fetch returns the latest hot.json with disk-cached fallback. A
// network error or 404 with no cache → Result{Available:false},
// which the UI reads as "no hot surface today." A 404 *with* a
// fresh-enough cache → cached Result{Available:true} so the UI
// stays revealed across one bad fetch.
//
// The "readme" kind is the only one currently surfaced; reel-kind
// rows are loaded into a separate map but ignored by the client UI
// today. The split keeps future "hot reels" easy without a second
// fetch.
func Fetch() Result {
	c := loadCache()
	if DefaultURL == "" {
		if c != nil {
			return result(c.Sidecar, true)
		}
		return Result{}
	}

	req, err := http.NewRequest("GET", DefaultURL, nil)
	if err != nil {
		if c != nil {
			return result(c.Sidecar, true)
		}
		return Result{Err: err}
	}
	if c != nil && c.ETag != "" {
		req.Header.Set("If-None-Match", c.ETag)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if c != nil {
			return result(c.Sidecar, true)
		}
		return Result{Err: err}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		if c != nil {
			return result(c.Sidecar, true)
		}
		return Result{Err: fmt.Errorf("304 without cache")}
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			if c != nil {
				return result(c.Sidecar, true)
			}
			return Result{Err: err}
		}
		var s Sidecar
		if err := json.Unmarshal(body, &s); err != nil {
			// Malformed sidecar: treat as unavailable. Don't
			// cache the bad bytes — leave any prior cache in
			// place so the next fetch can still revalidate.
			if c != nil {
				return result(c.Sidecar, true)
			}
			return Result{Err: fmt.Errorf("parse hot.json: %w", err)}
		}
		etag := resp.Header.Get("Etag")
		saveCache(etag, body, &s)
		return result(&s, false)
	case http.StatusNotFound:
		// Expected during the days-seen gate. If a previous fetch
		// got real data and it's recent enough, keep showing it.
		// Otherwise this is the steady-state "hot surface hidden"
		// response — return Result{} cleanly.
		if c != nil && time.Since(c.FetchedAt) < 7*24*time.Hour {
			return result(c.Sidecar, true)
		}
		return Result{}
	default:
		if c != nil {
			return result(c.Sidecar, true)
		}
		return Result{Err: fmt.Errorf("hot.json: %s", resp.Status)}
	}
}

// result projects a Sidecar into the per-repo score map the UI
// consumes. Only "readme" kind contributes — see the Result doc
// for why we don't combine the two kinds today.
func result(s *Sidecar, fromCache bool) Result {
	scores := make(map[string]float64, len(s.Rows))
	for _, r := range s.Rows {
		if r.Kind != "readme" {
			continue
		}
		// Last-write-wins on duplicate keys; the worker emits one
		// row per (kind, key) so duplicates shouldn't appear, but
		// being defensive here keeps a malformed sidecar from
		// silently choosing the smaller of two scores.
		if existing, ok := scores[r.Key]; !ok || r.HotScore > existing {
			scores[r.Key] = r.HotScore
		}
	}
	return Result{
		Available: true,
		Scores:    scores,
		Sidecar:   s,
		FromCache: fromCache,
	}
}
