package ui

import (
	"fmt"
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
	if got[len(got)-1].Name != "yazi" {
		t.Errorf("expected yazi last, got %s", got[len(got)-1].Name)
	}
}

func TestSort_StarsAsc(t *testing.T) {
	got := filterAndSort(sample(), filterCriteria{sort: sortStarsAsc})
	if got[0].Name != "yazi" {
		t.Errorf("expected yazi first, got %s", got[0].Name)
	}
}

func TestSort_Name(t *testing.T) {
	got := filterAndSort(sample(), filterCriteria{sort: sortName})
	if got[0].Name != "gh" {
		t.Errorf("expected gh first (alphabetical), got %s", got[0].Name)
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

func TestFilter_New_AddedAtBranchExcludesLastCommitOnly(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	apps := []catalog.App{
		// Inside window via AddedAt; LastCommit is old → qualifies.
		{Name: "fresh", Repo: "a/fresh", AddedAt: now.Add(-48 * time.Hour), LastCommit: now.Add(-365 * 24 * time.Hour)},
		// AddedAt outside window even though LastCommit is recent —
		// once any app in the catalog has AddedAt, we trust that signal
		// exclusively and ignore LastCommit. This is the important
		// invariant: mixing branches would let a well-maintained old
		// project leak into a "new" surface.
		{Name: "stale-add", Repo: "a/stale-add", AddedAt: now.Add(-30 * 24 * time.Hour), LastCommit: now.Add(-1 * time.Hour)},
		// AddedAt unset; a peer app has AddedAt so we don't fall back.
		// This app is excluded even though LastCommit is very recent.
		{Name: "commit-fresh-but-no-added", Repo: "a/commit-fresh-but-no-added", LastCommit: now.Add(-1 * time.Hour)},
	}
	got := filterAndSort(apps, filterCriteria{category: categoryNew, now: now})
	if len(got) != 1 || got[0].Name != "fresh" {
		t.Fatalf("expected [fresh] only under AddedAt-exclusive branch, got %+v", got)
	}
}

func TestFilter_New_LastCommitFallbackWhenAddedAtAbsent(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	apps := []catalog.App{
		{Name: "commit-fresh", Repo: "a/commit-fresh", LastCommit: now.Add(-24 * time.Hour)},
		{Name: "cold", Repo: "a/cold", LastCommit: now.Add(-30 * 24 * time.Hour)},
	}
	got := filterAndSort(apps, filterCriteria{category: categoryNew, now: now})
	if len(got) != 1 || got[0].Name != "commit-fresh" {
		t.Fatalf("expected [commit-fresh] under fallback, got %+v", got)
	}
}

func TestFilter_New_FallbackCapsToTopN(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	// 15 apps, all inside the 7-day LastCommit window, none with
	// AddedAt → fallback branch triggers and the cap kicks in at
	// newCap (10). The 5 oldest should be dropped.
	apps := make([]catalog.App, 0, 15)
	for i := 0; i < 15; i++ {
		apps = append(apps, catalog.App{
			Name:       fmt.Sprintf("app-%02d", i),
			Repo:       fmt.Sprintf("a/app-%02d", i),
			LastCommit: now.Add(-time.Duration(i) * time.Hour),
		})
	}
	got := filterAndSort(apps, filterCriteria{category: categoryNew, now: now})
	if len(got) != newCap {
		t.Fatalf("expected cap of %d, got %d", newCap, len(got))
	}
	for i, app := range got {
		wantName := fmt.Sprintf("app-%02d", i)
		if app.Name != wantName {
			t.Errorf("at %d: expected %s, got %s", i, wantName, app.Name)
		}
	}
}

func TestFilter_New_AddedAtCapsToTopN(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	// Launch-week shape: 15 apps all added within the past week via
	// AddedAt. Without a cap this returns the whole catalog and the
	// "New" row shows everything — useless. With the cap we keep the
	// 10 most recently added.
	apps := make([]catalog.App, 0, 15)
	for i := 0; i < 15; i++ {
		apps = append(apps, catalog.App{
			Name:    fmt.Sprintf("app-%02d", i),
			Repo:    fmt.Sprintf("a/app-%02d", i),
			AddedAt: now.Add(-time.Duration(i) * time.Hour),
		})
	}
	got := filterAndSort(apps, filterCriteria{category: categoryNew, now: now})
	if len(got) != newCap {
		t.Fatalf("expected cap of %d, got %d", newCap, len(got))
	}
	for i, app := range got {
		wantName := fmt.Sprintf("app-%02d", i)
		if app.Name != wantName {
			t.Errorf("at %d: expected %s, got %s", i, wantName, app.Name)
		}
	}
}

func TestFilter_New_EmptyWhenNoTimestamps(t *testing.T) {
	got := filterAndSort(sample(), filterCriteria{category: categoryNew, now: time.Now()})
	if len(got) != 0 {
		t.Errorf("sample() has no freshness timestamps; expected 0 results, got %d", len(got))
	}
}

func TestFilter_New_RespectsExplicitSort(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	apps := []catalog.App{
		{Name: "b-app", Repo: "a/b", Stars: 10, AddedAt: now.Add(-24 * time.Hour)},
		{Name: "a-app", Repo: "a/a", Stars: 100, AddedAt: now.Add(-48 * time.Hour)},
	}
	// Name sort overrides freshness sort when user picks it explicitly.
	got := filterAndSort(apps, filterCriteria{category: categoryNew, sort: sortName, now: now})
	if got[0].Name != "a-app" {
		t.Errorf("expected a-app first under name sort, got %s", got[0].Name)
	}
}
