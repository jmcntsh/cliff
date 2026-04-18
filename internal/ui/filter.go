package ui

import (
	"sort"

	"github.com/jmcntsh/cliff/internal/catalog"

	"github.com/sahilm/fuzzy"
)

type filterCriteria struct {
	category string
	query    string
	sort     sortMode
}

func filterAndSort(apps []catalog.App, c filterCriteria) []catalog.App {
	filtered := make([]catalog.App, 0, len(apps))
	for _, app := range apps {
		if c.category != "" && app.Category != c.category {
			continue
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
