package ui

import (
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/browser"
	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/clipboard"
	"github.com/jmcntsh/cliff/internal/install"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type flashClearMsg struct{}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = m.Width
		r.height = m.Height
		r.ready = true
		if r.readme.ready {
			r.readme = r.readme.resize(m.Width, m.Height)
		}
		return r.resize(), nil

	case readmeFetchedMsg:
		r.readme = r.readme.applyFetch(m)
		return r, nil

	case installStartedMsg:
		r.installCancel = m.Cancel
		return r, nil

	case installLineMsg:
		r.installLines = append(r.installLines, m.Line)
		// Cap buffered lines so chatty installs don't grow unbounded.
		// Copy the tail in place rather than re-slicing, so the dropped
		// prefix doesn't stay pinned by the backing array.
		if len(r.installLines) > 2000 {
			copy(r.installLines, r.installLines[len(r.installLines)-2000:])
			r.installLines = r.installLines[:2000]
		}
		// Tail-follow: auto-scroll to bottom only if the user was already
		// at the bottom. If they've scrolled up to read earlier output,
		// respect that and let them stay.
		wasAtBottom := r.installViewport.AtBottom()
		r.installViewport.SetContent(strings.Join(r.installLines, "\n"))
		if wasAtBottom {
			r.installViewport.GotoBottom()
		}
		return r, nil

	case installResultMsg:
		res := m.Result
		r.installRes = &res
		r.installCancel = nil
		r.mode = modeInstallResult
		// Replace with the canonical full output from Result — Stream's
		// onLine callback misses any partial final line (no trailing \n),
		// and having the result view show the same bytes Result.Output
		// holds keeps the two modes consistent.
		r.installViewport.SetContent(strings.TrimRight(res.Output, "\n"))
		r.installViewport.GotoTop()
		if res.Err == nil && res.App != nil {
			// Re-scan $PATH rather than blindly marking res.App.Repo
			// installed. This keeps the ✓ markers honest: if the install
			// reported success but didn't actually land a binary on PATH
			// (unusual but possible for odd scripts), we don't lie.
			r.installed = install.InstalledApps(r.catalog.Apps)
			r = r.refilter()
		}
		return r, nil

	case flashClearMsg:
		if time.Now().After(r.flashExpiry) {
			r.flashMsg = ""
		}
		return r, nil

	case tea.KeyMsg:
		if m.String() == "ctrl+c" {
			return r, tea.Quit
		}
		switch r.mode {
		case modeSidebarOverlay:
			return r.updateSidebarOverlay(m)
		case modeSearch:
			return r.updateSearch(m)
		case modeHelp:
			return r.updateHelp(m)
		case modeReadme:
			return r.updateReadme(m)
		case modeInstallConfirm:
			return r.updateInstallConfirm(m)
		case modeInstallRunning:
			return r.updateInstallRunning(m)
		case modeInstallResult:
			return r.updateInstallResult(m)
		default:
			return r.updateBrowse(m)
		}
	}

	return r, nil
}

func (r Root) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return r, tea.Quit
	case key.Matches(msg, keys.Tab):
		if r.layout == layoutNarrow {
			return r, nil
		}
		if r.focus == focusGrid {
			r.focus = focusSidebar
		} else {
			r.focus = focusGrid
		}
		r.sidebar = r.sidebar.setFocused(r.focus == focusSidebar)
		return r, nil
	case key.Matches(msg, keys.Categories):
		if r.layout == layoutNarrow {
			r.mode = modeSidebarOverlay
			r.sidebar = r.sidebar.setFocused(true)
			return r, nil
		}
	case key.Matches(msg, keys.Sort):
		r.sort = (r.sort + 1) % 3
		return r.refilter(), nil
	case key.Matches(msg, keys.Search):
		r.mode = modeSearch
		r.search.Focus()
		return r.resize(), textinput.Blink
	case key.Matches(msg, keys.Help):
		r.helpReturnMode = modeBrowse
		r.mode = modeHelp
		return r, nil
	case key.Matches(msg, keys.Enter):
		if app := r.selectedApp(); app != nil {
			r.readme = newReadme(app, r.width, r.height)
			r.mode = modeReadme
			return r, fetchReadmeCmd(app)
		}
		return r, nil
	case key.Matches(msg, keys.Install):
		if app := r.selectedApp(); app != nil {
			r.installApp = app
			r.installReturnMode = modeBrowse
			r.mode = modeInstallConfirm
		}
		return r, nil
	case key.Matches(msg, keys.CopyInstall):
		if app := r.selectedApp(); app != nil {
			if cmd := preferredInstall(app); cmd != "" {
				clipboard.WriteOSC52(cmd)
				return r.flash("copied: " + cmd), clearFlashCmd()
			}
			url := app.Homepage
			if url == "" {
				url = "https://github.com/" + app.Repo
			}
			clipboard.WriteOSC52(url)
			return r.flash("no install command; copied github URL: " + url), clearFlashCmd()
		}
		return r, nil
	}

	if r.focus == focusSidebar {
		// Right arrow / l from the sidebar hops focus into the grid.
		// Mirrors the leftmost-column-Left behavior in gridNav so the
		// two panes feel like one continuous 2D space.
		if key.Matches(msg, keys.Right) {
			r.focus = focusGrid
			r.sidebar = r.sidebar.setFocused(false)
			return r, nil
		}
		newSB, changed := r.sidebar.update(msg)
		r.sidebar = newSB
		if changed {
			r = r.refilter()
		}
		return r, nil
	}

	return r.gridNav(msg), nil
}

// gridNav routes navigation keys to the grid. Up/Down/Left/Right (and
// hjkl) move by one cell; g/G jump to first/last; pgup/pgdn page by
// rows. Left from the leftmost column hops focus to the sidebar
// instead of clamping (when there's a sidebar to hop to).
func (r Root) gridNav(msg tea.KeyMsg) Root {
	switch {
	case key.Matches(msg, keys.Up):
		r.grid = r.grid.move(-1, 0)
	case key.Matches(msg, keys.Down):
		r.grid = r.grid.move(1, 0)
	case key.Matches(msg, keys.Left):
		_, col := r.grid.cursorRowCol()
		if col == 0 && r.layout != layoutNarrow {
			r.focus = focusSidebar
			r.sidebar = r.sidebar.setFocused(true)
			return r
		}
		r.grid = r.grid.move(0, -1)
	case key.Matches(msg, keys.Right):
		r.grid = r.grid.move(0, 1)
	case key.Matches(msg, keys.Top):
		r.grid = r.grid.jumpTop()
	case key.Matches(msg, keys.Bottom):
		r.grid = r.grid.jumpBottom()
	case key.Matches(msg, keys.PageUp):
		r.grid = r.grid.pageUp()
	case key.Matches(msg, keys.PageDown):
		r.grid = r.grid.pageDown()
	}
	return r
}

func (r Root) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Help, keys.Escape, keys.Quit, keys.Left) {
		r.mode = r.helpReturnMode
	}
	return r, nil
}

func (r Root) updateReadme(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Left arrow / h = back to the grid. Mirrors the "◂ back" affordance
	// in the readme header. esc/q also work.
	if key.Matches(msg, keys.Escape, keys.Quit, keys.Left) {
		r.mode = modeBrowse
		return r, nil
	}
	// In the readme, ⏎ is "go deeper" = install. There's no further
	// drill-down available, so promoting ⏎ to the primary action keeps
	// the spatial model consistent. `i` still works for muscle memory.
	if key.Matches(msg, keys.Enter, keys.Install) {
		if app := r.selectedApp(); app != nil {
			r.installApp = app
			r.installReturnMode = modeReadme
			r.mode = modeInstallConfirm
			return r, nil
		}
	}
	if key.Matches(msg, keys.OpenGithub) {
		if app := r.selectedApp(); app != nil {
			_ = browser.Open(app.Homepage)
			return r.flash("opening " + app.Homepage), clearFlashCmd()
		}
		return r, nil
	}
	if key.Matches(msg, keys.Help) {
		r.helpReturnMode = modeReadme
		r.mode = modeHelp
		return r, nil
	}
	var cmd tea.Cmd
	r.readme, cmd = r.readme.Update(msg)
	return r, cmd
}

func (r Root) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		r.mode = modeBrowse
		r.search.SetValue("")
		r.search.Blur()
		return r.resize().refilter(), nil
	case key.Matches(msg, keys.Enter):
		r.mode = modeBrowse
		r.search.Blur()
		return r.resize(), nil
	}
	// While search is open, allow grid navigation with arrow keys so the
	// user can pick a result without leaving the input. Letter-based
	// nav (hjkl) belongs to the input itself.
	if msg.String() == "up" || msg.String() == "down" || msg.String() == "left" || msg.String() == "right" {
		return r.gridNav(msg), nil
	}
	var cmd tea.Cmd
	r.search, cmd = r.search.Update(msg)
	return r.refilter(), cmd
}

func (r Root) updateInstallConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape, keys.Quit):
		r.mode = r.installReturnMode
		r.installApp = nil
		return r, nil
	case key.Matches(msg, keys.Enter):
		if r.installApp == nil || r.installApp.InstallSpec == nil {
			// "No install available" dismissal on ⏎: return to caller
			// (readme if that's where ⏎/i was pressed) rather than
			// dumping the user back to the catalog.
			r.mode = r.installReturnMode
			r.installApp = nil
			return r, nil
		}
		app := r.installApp
		// Clear any previous install's line buffer and viewport when
		// entering the running view so each install starts blank.
		r.installLines = nil
		r.installViewport.SetContent("")
		r.installViewport.GotoTop()
		r.mode = modeInstallRunning
		return r, runInstallCmd(app)
	}
	return r, nil
}

func (r Root) updateInstallRunning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// esc/q cancels the in-flight install. The context cancellation
	// kills the child via exec.CommandContext; Stream then returns with
	// a non-nil Err and we transition to modeInstallResult normally.
	if key.Matches(msg, keys.Escape, keys.Quit) {
		if r.installCancel != nil {
			r.installCancel()
		}
		return r, nil
	}
	// All other keys get routed to the log viewport so the user can
	// scroll through output while the install is still running.
	var cmd tea.Cmd
	r.installViewport, cmd = r.installViewport.Update(msg)
	return r, cmd
}

func (r Root) updateInstallResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Two dismissal flavors with different intent:
	//   ⏎       = "I'm done, what's next" → always land in the grid so
	//             the user can pick another app to browse/install.
	//   esc/q/← = "back out" → return to whatever called the install
	//             (grid or readme), keeping the spatial model intact.
	if key.Matches(msg, keys.Enter) {
		r.mode = modeBrowse
		r.installApp = nil
		r.installRes = nil
		return r, nil
	}
	if key.Matches(msg, keys.Escape, keys.Quit, keys.Left) {
		r.mode = r.installReturnMode
		r.installApp = nil
		r.installRes = nil
		return r, nil
	}
	// Everything else scrolls the log viewport.
	var cmd tea.Cmd
	r.installViewport, cmd = r.installViewport.Update(msg)
	return r, cmd
}

func (r Root) updateSidebarOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape, keys.Categories):
		r.mode = modeBrowse
		r.sidebar = r.sidebar.setFocused(false)
		return r, nil
	case key.Matches(msg, keys.Enter):
		r.mode = modeBrowse
		r.sidebar = r.sidebar.setFocused(false)
		r = r.refilter()
		return r, nil
	}
	newSB, changed := r.sidebar.update(msg)
	r.sidebar = newSB
	if changed {
		r = r.refilter()
	}
	return r, nil
}

func (r Root) flash(msg string) Root {
	r.flashMsg = msg
	r.flashExpiry = time.Now().Add(2 * time.Second)
	return r
}

func clearFlashCmd() tea.Cmd {
	return tea.Tick(2*time.Second+100*time.Millisecond, func(time.Time) tea.Msg {
		return flashClearMsg{}
	})
}

func preferredInstall(app *catalog.App) string {
	if app == nil || app.InstallSpec == nil {
		return ""
	}
	return app.InstallSpec.Shell()
}
