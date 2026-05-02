package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jmcntsh/cliff/internal/hotfetch"
)

// hotFetchedMsg arrives once the background hotfetch.Fetch call
// returns. The TUI handler overlays Scores onto r.catalog.Apps and
// flips r.hotRevealed if the catalog now has at least
// hotRevealThreshold apps with a non-zero HotScore. Both are
// best-effort: an unavailable sidecar leaves HotScore at zero
// across the board and the Hot surfaces stay hidden, which is
// indistinguishable from the steady-state during the worker's
// 14-day days-seen warmup.
type hotFetchedMsg struct {
	result hotfetch.Result
}

// fetchHotCmd kicks the hotfetch off the main goroutine. Returned
// from Init so it runs once on launch — refreshing on a longer
// cadence isn't worth the complexity given the sidecar regenerates
// once per UTC day.
func fetchHotCmd() tea.Cmd {
	return func() tea.Msg {
		return hotFetchedMsg{result: hotfetch.Fetch()}
	}
}

// applyHotScores overlays the fetched Scores map onto the catalog,
// computes whether the reveal threshold passes, and rebuilds the
// sidebar to show/hide the Hot and New rows accordingly. Returns
// the updated Root and a refilter command so the grid picks up
// any new sort ordering.
//
// Score lookup is by Repo (e.g. "owner/name") because that's the
// stable per-app identifier the worker logs at the redirector — the
// `key` blob in cliff_events_v1 is exactly the readme path that
// the client built from app.Repo.
func (r Root) applyHotScores(msg hotFetchedMsg) Root {
	if !msg.result.Available {
		return r
	}
	scores := msg.result.Scores
	if len(scores) == 0 {
		return r
	}

	// Mutate the catalog's app list in place. This is one of the
	// few places we write to catalog.Apps post-load — every other
	// branch reads it — so the mutation is contained here for
	// auditability.
	nonzero := 0
	for i := range r.catalog.Apps {
		s, ok := scores[r.catalog.Apps[i].Repo]
		if !ok || s <= 0 {
			continue
		}
		r.catalog.Apps[i].HotScore = s
		nonzero++
	}

	if nonzero >= hotRevealThreshold {
		r.hotRevealed = true
	}
	// Sidebar reflects the new reveal state regardless of whether
	// it changed: rebuilding is cheap, and an unconditional rebuild
	// keeps the rule "sidebar shape == hotRevealed value" instead
	// of "sidebar shape == whatever it was when hotRevealed last
	// flipped." Easier to reason about across hot reloads.
	r.sidebar = newSidebar(r.catalog, r.installed, r.hotRevealed)
	return r
}
