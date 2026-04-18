package ui

import (
	"fmt"
	"testing"

	"github.com/jmcntsh/cliff/internal/catalog"
)

func mkApps(n int) []catalog.App {
	out := make([]catalog.App, n)
	for i := 0; i < n; i++ {
		out[i] = catalog.App{
			Name:        "app" + string(rune('A'+i)),
			Repo:        "owner/app" + string(rune('A'+i)),
			Description: "desc",
		}
	}
	return out
}

func TestPickColsRespectsMinWidth(t *testing.T) {
	cases := []struct {
		w        int
		wantCols int
	}{
		{40, 1},
		{60, 1},
		{70, 2},
		{120, 3},
		{160, 4},
	}
	for _, tc := range cases {
		cols, cardW := pickCols(tc.w)
		if cols != tc.wantCols {
			t.Errorf("pickCols(%d): cols = %d, want %d (cardW=%d)", tc.w, cols, tc.wantCols, cardW)
		}
		if cardW < 1 {
			t.Errorf("pickCols(%d): cardW=%d must be >=1", tc.w, cardW)
		}
	}
}

func TestGridMoveClampsAtEdges(t *testing.T) {
	g := newGrid().setApps(mkApps(7), nil).setLayout(120, 30) // 3 cols → 3 rows of 3,3,1
	if g.cols != 3 {
		t.Fatalf("expected 3 cols, got %d", g.cols)
	}
	g = g.move(0, -1)
	if g.cursor != 0 {
		t.Errorf("left at top-left should clamp, got cursor=%d", g.cursor)
	}
	g = g.move(0, 5)
	if g.cursor != 2 {
		t.Errorf("right past edge should clamp to last col of row 0 (cursor=2), got %d", g.cursor)
	}
	g = g.move(10, 0)
	if g.cursor != 6 {
		t.Errorf("down past last app should land on last app (cursor=6), got %d", g.cursor)
	}
}

func TestGridSelectByRepoPreservesCursor(t *testing.T) {
	g := newGrid().setApps(mkApps(5), nil).setLayout(120, 30)
	g = g.move(1, 0) // cursor on app at row 1 col 0 = index 3
	target := g.apps[g.cursor].Repo
	g = g.setApps(mkApps(5), nil).selectByRepo(target)
	if g.apps[g.cursor].Repo != target {
		t.Errorf("expected cursor on %s, got %s", target, g.apps[g.cursor].Repo)
	}
}

// Regression: scrolling down in one category and then switching to a
// category where the previously selected app doesn't exist should land
// the cursor at the top of the new list, not the bottom.
//
// Root cause: setApps clamps an out-of-range cursor to len-1, and
// selectByRepo has no fallback when the previous repo isn't in the new
// apps — so the clamp target (bottom of the new list) survives.
func TestGridResetsToTopWhenSelectedRepoMissing(t *testing.T) {
	big := make([]catalog.App, 20)
	for i := range big {
		big[i] = catalog.App{
			Name: fmt.Sprintf("a%02d", i),
			Repo: fmt.Sprintf("owner/a%02d", i),
		}
	}
	g := newGrid().setApps(big, nil).setLayout(120, 30)
	g = g.move(4, 0) // scroll several rows down
	scrolledRepo := g.apps[g.cursor].Repo
	if g.cursor < 6 {
		t.Fatalf("precondition: expected cursor to have scrolled (>=6); got %d", g.cursor)
	}

	small := []catalog.App{
		{Name: "b0", Repo: "owner/b0"},
		{Name: "b1", Repo: "owner/b1"},
		{Name: "b2", Repo: "owner/b2"},
	}
	g = g.setApps(small, nil).selectByRepo(scrolledRepo).setLayout(120, 30)

	if g.cursor != 0 {
		t.Errorf("expected cursor=0 after switching to category where selected repo is absent; got %d (%s)",
			g.cursor, g.apps[g.cursor].Repo)
	}
}
