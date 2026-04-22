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

type filterCriteria struct {
	category  string
	query     string
	sort      sortMode
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
	sortApps(filtered, c.sort)
	return filtered
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
