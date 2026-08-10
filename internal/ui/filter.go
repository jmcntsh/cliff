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

// categoryHot is the registry-backed popularity surface. Unlike New,
// eligibility comes from measured star-growth map membership: a zero
// delta is valid, while a missing key means the app lacked coverage.
const categoryHot = "__hot__"

// newWindow is how far back "new" reaches. One week matches the
// "new this week" language in README/CLAUDE.
const newWindow = 7 * 24 * time.Hour

// newCap bounds how many apps land in the "New" row regardless of
// signal. On launch week the 7-day window is every app, and a row
// that shows everything shows nothing; the cap keeps the row scannable
// and curated-feeling in both the AddedAt and LastCommit branches.
const newCap = 10

// newSet returns the set of repos that qualify as "New" at time `now`.
// When any app has a non-zero AddedAt, we use AddedAt exclusively
// (LastCommit means "pushed code recently," a very different claim);
// otherwise we fall back to LastCommit. In both branches we keep the
// top newCap by timestamp inside newWindow, using stars to rank apps
// from the same weekly batch. Returning a set rather than a predicate
// is what lets the cap be enforced once for the whole render instead
// of per-row.
func newSet(apps []catalog.App, now time.Time) map[string]struct{} {
	pick := func(a *catalog.App) time.Time {
		if !a.AddedAt.IsZero() {
			return a.AddedAt
		}
		return time.Time{}
	}
	hasAddedAt := false
	for i := range apps {
		if !apps[i].AddedAt.IsZero() {
			hasAddedAt = true
			break
		}
	}
	if !hasAddedAt {
		pick = func(a *catalog.App) time.Time { return a.LastCommit }
	}

	type ranked struct {
		repo  string
		t     time.Time
		stars int
	}
	eligible := make([]ranked, 0, len(apps))
	for i := range apps {
		a := &apps[i]
		t := pick(a)
		if !t.IsZero() && now.Sub(t) <= newWindow {
			eligible = append(eligible, ranked{repo: a.Repo, t: t, stars: a.Stars})
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		if !eligible[i].t.Equal(eligible[j].t) {
			return eligible[i].t.After(eligible[j].t)
		}
		if eligible[i].stars != eligible[j].stars {
			return eligible[i].stars > eligible[j].stars
		}
		return eligible[i].repo < eligible[j].repo
	})
	if len(eligible) > newCap {
		eligible = eligible[:newCap]
	}
	set := make(map[string]struct{}, len(eligible))
	for _, r := range eligible {
		set[r.repo] = struct{}{}
	}
	return set
}

type filterCriteria struct {
	category  string
	query     string
	sort      sortMode
	hotWindow string
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
		case c.category == categoryHot:
			if _, ok := app.StarGrowth[c.hotWindow]; !ok {
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
		filtered = applyFuzzy(filtered, c.query)
		if c.category != categoryHot {
			return filtered
		}
	}
	if c.category == categoryHot {
		sortByStarGrowth(filtered, c.hotWindow)
		return filtered
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

func countHot(apps []catalog.App, window string) int {
	n := 0
	for i := range apps {
		if _, ok := apps[i].StarGrowth[window]; ok {
			n++
		}
	}
	return n
}

// sortByStarGrowth is the fixed order for Hot. Absolute net growth is the
// primary signal; lifetime stars and name make ties deterministic.
func sortByStarGrowth(apps []catalog.App, window string) {
	sort.Slice(apps, func(i, j int) bool {
		di, dj := apps[i].StarGrowth[window], apps[j].StarGrowth[window]
		if di != dj {
			return di > dj
		}
		if apps[i].Stars != apps[j].Stars {
			return apps[i].Stars > apps[j].Stars
		}
		return apps[i].Name < apps[j].Name
	})
}

// sortByFreshness sorts newest-first by FreshnessTime, then by stars
// within a weekly batch, with name as the deterministic final tie-break.
func sortByFreshness(apps []catalog.App) {
	sort.Slice(apps, func(i, j int) bool {
		ti, tj := apps[i].FreshnessTime(), apps[j].FreshnessTime()
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		if apps[i].Stars != apps[j].Stars {
			return apps[i].Stars > apps[j].Stars
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
		case sortRecencyDesc:
			// FreshnessTime falls back across AddedAt and
			// LastCommit (see catalog.App.FreshnessTime), so a
			// catalog where some entries lack AddedAt still
			// produces a meaningful order — same source of
			// truth as the New-row freshness sort.
			ti, tj := apps[i].FreshnessTime(), apps[j].FreshnessTime()
			if !ti.Equal(tj) {
				return ti.After(tj)
			}
			return apps[i].Name < apps[j].Name
		default:
			if apps[i].Stars != apps[j].Stars {
				return apps[i].Stars > apps[j].Stars
			}
			return apps[i].Name < apps[j].Name
		}
	})
}
