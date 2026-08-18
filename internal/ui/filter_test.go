package ui

import (
	"testing"
	"time"

	"github.com/jmcntsh/cliff/internal/catalog"
)

func sample() []catalog.App {
	return []catalog.App{
		{Name: "lazygit", Repo: "jesseduffield/lazygit", Description: "git tui", Category: "Git", Language: "Go", Stars: 52000},
		{Name: "gh", Repo: "cli/cli", Description: "github cli", Category: "Git", Language: "Go", Stars: 18000},
		{Name: "gitui", Repo: "extrawurst/gitui", Description: "fast git tui", Category: "Git", Language: "Rust", Stars: 12000},
		{Name: "yazi", Repo: "sxyazi/yazi", Description: "file manager", Category: "Files", Language: "Rust", Stars: 9000},
		{Name: "ranger", Repo: "ranger/ranger", Description: "vim-inspired fm", Category: "Files", Language: "Python", Stars: 15000},
		{Name: "balatro-tui", Repo: "Passeriform/BalatroTUI", Description: "terminal card game", Category: "Games", Language: "Rust", Stars: 200},
		{Name: "tetrigo", Repo: "Broderick-Westrope/tetrigo", Description: "terminal tetris", Category: "Games", Language: "Go", Stars: 100},
		{Name: "draw", Repo: "jmcntsh/draw", Description: "terminal canvas", Category: "Creative", Language: "Go", Stars: 1},
	}
}

func TestFilter_Category(t *testing.T) {
	got := filterAndSort(sample(), filterCriteria{category: "Files"})
	if len(got) != 2 {
		t.Fatalf("expected 2 Files apps, got %d", len(got))
	}
	for _, app := range got {
		if app.Category != "Files" {
			t.Errorf("got non-Files app: %+v", app)
		}
	}
}

func TestSort_StarsDesc(t *testing.T) {
	got := filterAndSort(sample(), filterCriteria{sort: sortStarsDesc})
	if got[0].Name != "lazygit" {
		t.Errorf("expected lazygit first, got %s", got[0].Name)
	}
	if got[len(got)-1].Name != "draw" {
		t.Errorf("expected draw last, got %s", got[len(got)-1].Name)
	}
}

func TestSort_RecencyDesc(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	apps := []catalog.App{
		{Name: "old", Repo: "a/old", Stars: 100, LastCommit: now.Add(-90 * 24 * time.Hour)},
		{Name: "mid", Repo: "a/mid", Stars: 1, LastCommit: now.Add(-30 * 24 * time.Hour)},
		{Name: "new", Repo: "a/new", Stars: 50, LastCommit: now.Add(-1 * time.Hour)},
	}
	got := filterAndSort(apps, filterCriteria{sort: sortRecencyDesc})
	if got[0].Name != "new" || got[2].Name != "old" {
		t.Errorf("expected newest first / oldest last, got %v", got)
	}
}

func TestSort_RecencyDescPrefersAddedAt(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	apps := []catalog.App{
		{Name: "older-add", Repo: "a/older", AddedAt: now.Add(-365 * 24 * time.Hour), LastCommit: now.Add(-1 * time.Minute)},
		{Name: "recent-fallback", Repo: "a/recent", LastCommit: now.Add(-2 * time.Hour)},
	}
	got := filterAndSort(apps, filterCriteria{sort: sortRecencyDesc})
	if got[0].Name != "recent-fallback" {
		t.Errorf("expected AddedAt to be the primary recency signal, got %s", got[0].Name)
	}
}

func TestSearch_Fuzzy(t *testing.T) {
	got := filterAndSort(sample(), filterCriteria{query: "git"})
	if len(got) == 0 {
		t.Fatal("expected matches for 'git'")
	}
	found := false
	for _, app := range got {
		if app.Name == "lazygit" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'lazygit' in fuzzy results for 'git'")
	}
}

func TestSearch_NoMatch(t *testing.T) {
	got := filterAndSort(sample(), filterCriteria{query: "xyzzy"})
	if len(got) != 0 {
		t.Errorf("expected 0 matches for 'xyzzy', got %d", len(got))
	}
}

func TestFilter_Installed(t *testing.T) {
	installed := map[string]bool{
		"jesseduffield/lazygit": true,
		"sxyazi/yazi":           true,
	}
	got := filterAndSort(sample(), filterCriteria{
		category:  categoryInstalled,
		installed: installed,
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 installed apps, got %d", len(got))
	}
	for _, app := range got {
		if !installed[app.Repo] {
			t.Errorf("filter returned non-installed app: %s", app.Repo)
		}
	}
}

func TestFilter_Installed_Empty(t *testing.T) {
	got := filterAndSort(sample(), filterCriteria{
		category:  categoryInstalled,
		installed: map[string]bool{},
	})
	if len(got) != 0 {
		t.Errorf("expected 0 apps when nothing installed, got %d", len(got))
	}
}

func TestFilter_Installed_SpansCategories(t *testing.T) {
	installed := map[string]bool{
		"cli/cli":     true,
		"sxyazi/yazi": true,
	}
	got := filterAndSort(sample(), filterCriteria{
		category:  categoryInstalled,
		installed: installed,
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 apps across categories, got %d", len(got))
	}
}

func TestFilter_HotUsesMeasuredGrowthAndKeepsZero(t *testing.T) {
	apps := []catalog.App{
		{Name: "fast", Repo: "a/fast", Stars: 100, StarGrowth: map[string]int{"7d": 20}},
		{Name: "flat", Repo: "a/flat", Stars: 200, StarGrowth: map[string]int{"7d": 0}},
		{Name: "down", Repo: "a/down", Stars: 300, StarGrowth: map[string]int{"7d": -4}},
		{Name: "missing", Repo: "a/missing", Stars: 1000},
		{Name: "month-only", Repo: "a/month", Stars: 500, StarGrowth: map[string]int{"30d": 50}},
	}

	got := filterAndSort(apps, filterCriteria{category: categoryHot, hotWindow: "7d"})
	if len(got) != 3 {
		t.Fatalf("expected 3 measured weekly apps, got %d", len(got))
	}
	want := []string{"fast", "flat", "down"}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("at %d: got %s, want %s", i, got[i].Name, want[i])
		}
	}
	if countHot(apps, "7d") != 3 || countHot(apps, "30d") != 1 {
		t.Errorf("countHot did not distinguish window membership")
	}
}

func TestFilter_HotTieBreaksByLifetimeStarsThenName(t *testing.T) {
	apps := []catalog.App{
		{Name: "z-low", Repo: "a/z", Stars: 10, StarGrowth: map[string]int{"30d": 5}},
		{Name: "z-high", Repo: "a/zh", Stars: 20, StarGrowth: map[string]int{"30d": 5}},
		{Name: "a-high", Repo: "a/ah", Stars: 20, StarGrowth: map[string]int{"30d": 5}},
	}

	got := filterAndSort(apps, filterCriteria{category: categoryHot, hotWindow: "30d"})
	want := []string{"a-high", "z-high", "z-low"}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("at %d: got %s, want %s", i, got[i].Name, want[i])
		}
	}
}

func TestFilter_HotSearchRetainsGrowthOrder(t *testing.T) {
	apps := []catalog.App{
		{Name: "alpha fast", Repo: "a/fast", Description: "tool", StarGrowth: map[string]int{"7d": 20}},
		{Name: "alpha slow", Repo: "a/slow", Description: "tool", StarGrowth: map[string]int{"7d": 2}},
	}

	got := filterAndSort(apps, filterCriteria{category: categoryHot, hotWindow: "7d", query: "alpha"})
	if len(got) != 2 || got[0].Name != "alpha fast" {
		t.Fatalf("Hot search should retain ranking order, got %+v", got)
	}
}
