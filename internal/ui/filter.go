package ui

import (
	"sort"
	"time"

	"github.com/jmcntsh/cliff/internal/catalog"

	"github.com/sahilm/fuzzy"
)

// categoryInstalled is the sentinel the sidebar uses for the
// "Installed" pseudo-category. It isn't a real catalog.Category —
// it filters by runtime install state rather than manifest metadata.
// The value is deliberately unlikely to collide with any real
// category string coming out of the registry.
const categoryInstalled = "__installed__"

// categoryNew is the sentinel for the "New" pseudo-category: apps
// whose FreshnessTime falls inside newWindow. Like Installed, it
// filters by a runtime rule rather than a manifest field, so it
// doesn't live in catalog.Categories.
const categoryNew = "__new__"

// newWindow is how far back "new" reaches when we have a reliable
// "added to cliff" timestamp (App.AddedAt). One week matches the
// "new this week" language in README/CLAUDE.
const newWindow = 7 * 24 * time.Hour

// newFallbackWindow is the window we use when falling back to
// LastCommit. LastCommit means "this project pushed code recently,"
// not "this project is new to cliff" — two very different claims.
// We still use a week here so mid-week visitors see something, but
// the fallback is additionally capped to newFallbackCap entries (via
// fallbackFreshest) so the row reads as a curated surface rather
// than "every actively-maintained tool in the catalog."
const newFallbackWindow = 7 * 24 * time.Hour

// newFallbackCap bounds the LastCommit-fallback variant of "New" so
// it stays a small, curated-feeling row. Irrelevant once the registry
// starts emitting AddedAt — at that point the 7-day AddedAt window is
// naturally small and the cap drops away.
const newFallbackCap = 10

// newSet returns the set of repos that qualify as "New" at time `now`.
// When any app has a non-zero AddedAt, the AddedAt branch wins
// exclusively: every qualifying app is one whose AddedAt is inside
// newWindow. Otherwise we fall back to the top-newFallbackCap apps by
// LastCommit that are inside newFallbackWindow. Returning a set
// (rather than a predicate) is what lets the fallback enforce a cap
// without each isNew call needing to re-rank the whole catalog.
func newSet(apps []catalog.App, now time.Time) map[string]struct{} {
	set := make(map[string]struct{})
	hasAddedAt := false
	for i := range apps {
		if !apps[i].AddedAt.IsZero() {
			hasAddedAt = true
			break
		}
	}
	if hasAddedAt {
		for i := range apps {
			a := &apps[i]
			if !a.AddedAt.IsZero() && now.Sub(a.AddedAt) <= newWindow {
				set[a.Repo] = struct{}{}
			}
		}
		return set
	}
	type ranked struct {
		repo string
		t    time.Time
	}
	eligible := make([]ranked, 0, len(apps))
	for i := range apps {
		a := &apps[i]
		if !a.LastCommit.IsZero() && now.Sub(a.LastCommit) <= newFallbackWindow {
			eligible = append(eligible, ranked{repo: a.Repo, t: a.LastCommit})
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		if !eligible[i].t.Equal(eligible[j].t) {
			return eligible[i].t.After(eligible[j].t)
		}
		return eligible[i].repo < eligible[j].repo
	})
	if len(eligible) > newFallbackCap {
		eligible = eligible[:newFallbackCap]
	}
	for _, r := range eligible {
		set[r.repo] = struct{}{}
	}
	return set
}

type filterCriteria struct {
	category  string
	query     string
	sort      sortMode
	installed map[string]bool // required when category == categoryInstalled
	now       time.Time       // injection point for isNew; zero means time.Now()
}

func filterAndSort(apps []catalog.App, c filterCriteria) []catalog.App {
	now := c.now
	if now.IsZero() {
		now = time.Now()
	}
	var newRepos map[string]struct{}
	if c.category == categoryNew {
		newRepos = newSet(apps, now)
	}
	filtered := make([]catalog.App, 0, len(apps))
	for _, app := range apps {
		switch {
		case c.category == categoryInstalled:
			if !c.installed[app.Repo] {
				continue
			}
		case c.category == categoryNew:
			if _, ok := newRepos[app.Repo]; !ok {
				continue
			}
		case c.category != "":
			if app.Category != c.category {
				continue
			}
		}
		filtered = append(filtered, app)
	}
	if c.query != "" {
		return applyFuzzy(filtered, c.query)
	}
	// For the New surface, default-sort by freshness (newest first)
	// so the row actually reads as "new this week" rather than
	// "new this week, sorted by stars." Explicit sort toggles still
	// work — user's choice wins when they cycle the sort key.
	if c.category == categoryNew && c.sort == sortStarsDesc {
		sortByFreshness(filtered)
		return filtered
	}
	sortApps(filtered, c.sort)
	return filtered
}

// sortByFreshness sorts newest-first by FreshnessTime, tie-breaking on
// name for deterministic ordering when two apps share a timestamp.
func sortByFreshness(apps []catalog.App) {
	sort.Slice(apps, func(i, j int) bool {
		ti, tj := apps[i].FreshnessTime(), apps[j].FreshnessTime()
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return apps[i].Name < apps[j].Name
	})
}

func applyFuzzy(apps []catalog.App, query string) []catalog.App {
	haystack := make([]string, len(apps))
	for i, app := range apps {
		haystack[i] = app.Name + " " + app.Description
	}
	matches := fuzzy.Find(query, haystack)
	out := make([]catalog.App, len(matches))
	for i, m := range matches {
		out[i] = apps[m.Index]
	}
	return out
}

func sortApps(apps []catalog.App, mode sortMode) {
	sort.Slice(apps, func(i, j int) bool {
		switch mode {
		case sortStarsAsc:
			if apps[i].Stars != apps[j].Stars {
				return apps[i].Stars < apps[j].Stars
			}
			return apps[i].Name < apps[j].Name
		case sortName:
			return apps[i].Name < apps[j].Name
		default:
			if apps[i].Stars != apps[j].Stars {
				return apps[i].Stars > apps[j].Stars
			}
			return apps[i].Name < apps[j].Name
		}
	})
}
