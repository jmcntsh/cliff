package ui

import (
	"sort"

	"github.com/jmcntsh/cliff/internal/catalog"

	"github.com/sahilm/fuzzy"
)

// categoryInstalled is the sentinel the sidebar uses for the
// "Installed" pseudo-category. It isn't a real catalog.Category —
// it filters by runtime install state rather than manifest metadata.
// The value is deliberately unlikely to collide with any real
// category string coming out of the registry.
const categoryInstalled = "__installed__"

// categoryHot is the registry-backed popularity surface. Eligibility
// comes from measured star-growth map membership: a zero delta is
// valid, while a missing key means the app lacked coverage.
const categoryHot = "__hot__"

type filterCriteria struct {
	category  string
	query     string
	sort      sortMode
	hotWindow string
	installed map[string]bool // required when category == categoryInstalled
}

func filterAndSort(apps []catalog.App, c filterCriteria) []catalog.App {
	filtered := make([]catalog.App, 0, len(apps))
	for _, app := range apps {
		switch {
		case c.category == categoryInstalled:
			if !c.installed[app.Repo] {
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
			// FreshnessTime falls back from AddedAt to LastCommit,
			// so a catalog where some entries lack AddedAt still
			// produces a meaningful order.
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
