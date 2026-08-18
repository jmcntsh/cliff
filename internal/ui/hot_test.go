package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/jmcntsh/cliff/internal/catalog"

	tea "github.com/charmbracelet/bubbletea"
)

func selectSidebarCategory(t *testing.T, r Root, category string) Root {
	t.Helper()
	for i := range r.sidebar.items {
		if r.sidebar.items[i].name == category {
			r.sidebar.cursor = i
			return r.refilter()
		}
	}
	t.Fatalf("sidebar category %q not found", category)
	return r
}

func TestSidebarUsesHotWithoutNew(t *testing.T) {
	r := New(&catalog.Catalog{})
	want := []string{"", categoryHot, categoryInstalled}
	if len(r.sidebar.items) != len(want) {
		t.Fatalf("sidebar items = %d, want %d", len(r.sidebar.items), len(want))
	}
	for i, name := range want {
		if got := r.sidebar.items[i].name; got != name {
			t.Errorf("sidebar item %d = %q, want %q", i, got, name)
		}
	}
}

func TestHotTimeframeSwitchesAndRefilters(t *testing.T) {
	c := &catalog.Catalog{Apps: []catalog.App{
		{Name: "weekly", Repo: "a/weekly", Stars: 20, StarGrowth: map[string]int{"7d": 10, "30d": 1}},
		{Name: "monthly", Repo: "a/monthly", Stars: 10, StarGrowth: map[string]int{"7d": 2, "30d": 20}},
		{Name: "weekly-only", Repo: "a/weekly-only", StarGrowth: map[string]int{"7d": 1}},
	}}
	r := selectSidebarCategory(t, New(c), categoryHot)
	if got := r.grid.apps[0].Name; got != "weekly" {
		t.Fatalf("weekly Hot leader = %q, want weekly", got)
	}

	model, _ := r.updateBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	r = model.(Root)
	if r.hot.key() != "30d" {
		t.Fatalf("Hot period = %q, want 30d", r.hot.key())
	}
	if got := r.grid.apps[0].Name; got != "monthly" {
		t.Fatalf("monthly Hot leader = %q, want monthly", got)
	}
	if len(r.grid.apps) != 2 {
		t.Fatalf("monthly Hot apps = %d, want 2", len(r.grid.apps))
	}
	if r.sidebar.items[r.sidebar.cursor].count != 2 {
		t.Fatalf("monthly Hot sidebar count = %d, want 2", r.sidebar.items[r.sidebar.cursor].count)
	}
}

func TestHotIgnoresRegularSortKey(t *testing.T) {
	c := &catalog.Catalog{Apps: []catalog.App{{
		Name: "app", Repo: "a/app", StarGrowth: map[string]int{"7d": 1},
	}}}
	r := selectSidebarCategory(t, New(c), categoryHot)

	model, _ := r.updateBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := model.(Root)
	if got.sort != r.sort {
		t.Fatalf("sort changed on Hot: got %v, want %v", got.sort, r.sort)
	}
}

func TestHotTitleShowsPartialObservedInterval(t *testing.T) {
	from := time.Date(2026, 8, 8, 4, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 11, 4, 0, 0, 0, time.UTC)
	c := &catalog.Catalog{
		Apps: []catalog.App{{
			Name: "app", Repo: "a/app", StarGrowth: map[string]int{"7d": 3},
		}},
		StarWindows: map[string]catalog.StarWindow{
			"7d": {RequestedDays: 7, From: &from, To: &to, Complete: false},
		},
	}
	r := selectSidebarCategory(t, New(c), categoryHot)
	title := r.computeTitle()
	for _, want := range []string{"hot", "7d net stars", "Aug 8–Aug 11 collected"} {
		if !strings.Contains(title, want) {
			t.Errorf("title %q missing %q", title, want)
		}
	}
}

func TestHotUnavailableStateExplainsCollection(t *testing.T) {
	c := &catalog.Catalog{
		StarWindows: map[string]catalog.StarWindow{
			"7d": {RequestedDays: 7},
		},
	}
	r := selectSidebarCategory(t, New(c), categoryHot)
	if !strings.Contains(r.computeTitle(), "collecting history") {
		t.Fatalf("title should explain unavailable history: %q", r.computeTitle())
	}
	empty := r.emptyGridView(80, 10)
	if !strings.Contains(empty, "Collecting star history.") {
		t.Fatalf("empty state should explain collection: %q", empty)
	}
}

func TestFormatSignedStars(t *testing.T) {
	cases := map[int]string{
		1200: "+1.2k",
		0:    "+0",
		-4:   "-4",
	}
	for input, want := range cases {
		if got := formatSignedStars(input); got != want {
			t.Errorf("formatSignedStars(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestHotCardShowsPeriodDelta(t *testing.T) {
	app := catalog.App{
		Name:        "tool",
		Repo:        "a/tool",
		Description: "tool description",
		Stars:       1200,
		StarGrowth:  map[string]int{"7d": 12},
	}
	hot := renderCard(app, 34, cardHeightCompact, false, false, true, "7d")
	if !strings.Contains(hot, "★ +12 / 7d") {
		t.Fatalf("Hot card missing period delta: %q", hot)
	}
	regular := renderCard(app, 34, cardHeightCompact, false, false, true, "")
	if !strings.Contains(regular, "★ 1.2k") || strings.Contains(regular, "/ 7d") {
		t.Fatalf("regular card should retain lifetime stars: %q", regular)
	}
}
