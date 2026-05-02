package ui

import (
	"testing"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/hotfetch"
)

// makeApps returns n apps with sequential repos so callers can ask
// for "5 of these have hot scores" without inventing names.
func makeApps(n int) []catalog.App {
	apps := make([]catalog.App, n)
	for i := 0; i < n; i++ {
		repo := "owner/" + string(rune('a'+i%26)) + "-" + string(rune('a'+i/26))
		apps[i] = catalog.App{
			Name: repo, Repo: repo, Stars: 100 - i,
		}
	}
	return apps
}

// rootForHotTests builds a minimal Root suitable for exercising the
// hot-overlay logic. We don't go through New() because that pulls in
// the full launcher/install detection chain — for hot-score behavior
// we only need catalog + sidebar + installed map.
func rootForHotTests(apps []catalog.App) Root {
	c := &catalog.Catalog{Apps: apps}
	installed := map[string]bool{}
	return Root{
		catalog:   c,
		sidebar:   newSidebar(c, installed, false),
		installed: installed,
	}
}

func TestApplyHotScores_BelowThreshold_HidesSurface(t *testing.T) {
	apps := makeApps(50)
	r := rootForHotTests(apps)

	scores := map[string]float64{}
	for i := 0; i < hotRevealThreshold-1; i++ { // one short
		scores[apps[i].Repo] = float64(i + 1)
	}
	r = r.applyHotScores(hotFetchedMsg{result: hotfetch.Result{Available: true, Scores: scores}})

	if r.hotRevealed {
		t.Errorf("hotRevealed should stay false when only %d apps have nonzero scores (threshold=%d)",
			hotRevealThreshold-1, hotRevealThreshold)
	}
	for _, item := range r.sidebar.items {
		if item.name == categoryHot {
			t.Errorf("Hot row should not appear below threshold")
		}
	}
	hasNew := false
	for _, item := range r.sidebar.items {
		if item.name == categoryNew {
			hasNew = true
		}
	}
	if !hasNew {
		t.Errorf("New row should still be present below threshold")
	}
}

func TestApplyHotScores_AtThreshold_RevealsHot_HidesNew(t *testing.T) {
	apps := makeApps(50)
	r := rootForHotTests(apps)

	scores := map[string]float64{}
	for i := 0; i < hotRevealThreshold; i++ {
		scores[apps[i].Repo] = float64(i + 1)
	}
	r = r.applyHotScores(hotFetchedMsg{result: hotfetch.Result{Available: true, Scores: scores}})

	if !r.hotRevealed {
		t.Fatalf("hotRevealed should flip true at exactly %d nonzero scores", hotRevealThreshold)
	}
	hasHot, hasNew := false, false
	for _, item := range r.sidebar.items {
		if item.name == categoryHot {
			hasHot = true
		}
		if item.name == categoryNew {
			hasNew = true
		}
	}
	if !hasHot {
		t.Errorf("Hot row should appear at threshold")
	}
	if hasNew {
		t.Errorf("New row should be removed when Hot reveals — they trade places, not stack")
	}
}

func TestApplyHotScores_Unavailable_NoChange(t *testing.T) {
	apps := makeApps(50)
	r := rootForHotTests(apps)
	r = r.applyHotScores(hotFetchedMsg{result: hotfetch.Result{Available: false}})

	if r.hotRevealed {
		t.Errorf("Available=false must not flip hotRevealed")
	}
	for i := range r.catalog.Apps {
		if r.catalog.Apps[i].HotScore != 0 {
			t.Errorf("HotScore should remain zero when result unavailable")
		}
	}
}

func TestApplyHotScores_ZeroScores_DoNotCount(t *testing.T) {
	apps := makeApps(50)
	r := rootForHotTests(apps)

	// Lots of "scores," all zero or negative — should not reveal.
	scores := map[string]float64{}
	for i := 0; i < 40; i++ {
		scores[apps[i].Repo] = 0
	}
	r = r.applyHotScores(hotFetchedMsg{result: hotfetch.Result{Available: true, Scores: scores}})

	if r.hotRevealed {
		t.Errorf("zero-valued scores must not count toward reveal threshold")
	}
}

func TestVisibleSorts(t *testing.T) {
	r := Root{}
	if got := r.visibleSorts(); len(got) != 2 || got[0] != sortStarsDesc || got[1] != sortRecencyDesc {
		t.Errorf("pre-reveal cycle should be [stars↓ recency↓], got %v", got)
	}
	r.hotRevealed = true
	if got := r.visibleSorts(); len(got) != 3 || got[2] != sortHotDesc {
		t.Errorf("post-reveal cycle should append hot↓, got %v", got)
	}
}

func TestNextSort_Wraps(t *testing.T) {
	r := Root{sort: sortStarsDesc}
	if got := r.nextSort(); got != sortRecencyDesc {
		t.Errorf("stars↓ → recency↓; got %v", got)
	}
	r.sort = sortRecencyDesc
	if got := r.nextSort(); got != sortStarsDesc {
		t.Errorf("recency↓ wraps to stars↓ pre-reveal; got %v", got)
	}
	r.hotRevealed = true
	r.sort = sortRecencyDesc
	if got := r.nextSort(); got != sortHotDesc {
		t.Errorf("recency↓ → hot↓ post-reveal; got %v", got)
	}
	r.sort = sortHotDesc
	if got := r.nextSort(); got != sortStarsDesc {
		t.Errorf("hot↓ wraps to stars↓; got %v", got)
	}
}

func TestNextSort_StaleSortFallsBackCleanly(t *testing.T) {
	// User had cycled to sortHotDesc when reveal was true; now the
	// sidecar disappeared and reveal flipped back to false. The
	// next press of `s` shouldn't crash or stay stuck — should
	// fall through to the first sort in the visible cycle.
	r := Root{sort: sortHotDesc, hotRevealed: false}
	if got := r.nextSort(); got != sortStarsDesc {
		t.Errorf("stale sort should fall back to first visible sort, got %v", got)
	}
}

func TestApplyHotScores_PopulatesAppHotScore(t *testing.T) {
	apps := makeApps(5)
	r := rootForHotTests(apps)
	scores := map[string]float64{
		apps[0].Repo: 7.5,
		apps[2].Repo: 12.0,
	}
	r = r.applyHotScores(hotFetchedMsg{result: hotfetch.Result{Available: true, Scores: scores}})

	if r.catalog.Apps[0].HotScore != 7.5 {
		t.Errorf("expected app[0] HotScore=7.5, got %v", r.catalog.Apps[0].HotScore)
	}
	if r.catalog.Apps[1].HotScore != 0 {
		t.Errorf("expected app[1] HotScore unchanged at 0, got %v", r.catalog.Apps[1].HotScore)
	}
	if r.catalog.Apps[2].HotScore != 12.0 {
		t.Errorf("expected app[2] HotScore=12.0, got %v", r.catalog.Apps[2].HotScore)
	}
}
