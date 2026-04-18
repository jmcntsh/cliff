package ui

import (
	"testing"

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
