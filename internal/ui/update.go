package ui

import (
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/binmap"
	"github.com/jmcntsh/cliff/internal/browser"
	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/clipboard"
	"github.com/jmcntsh/cliff/internal/install"
	"github.com/jmcntsh/cliff/internal/launcher"
	"github.com/jmcntsh/cliff/internal/pathfix"

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
		switch r.installOp {
		case pkgOpUninstall:
			r.mode = modeUninstallResult
		case pkgOpUpgrade:
			r.mode = modeUpgradeResult
		default:
			r.mode = modeInstallResult
		}
		// Reset per-modal transient error from any previous install.
		r.launchErr = nil
		// Replace with the canonical full output from Result — Stream's
		// onLine callback misses any partial final line (no trailing \n),
		// and having the result view show the same bytes Result.Output
		// holds keeps the two modes consistent.
		r.installViewport.SetContent(strings.TrimRight(res.Output, "\n"))
		r.installViewport.GotoTop()
		if res.Err == nil && res.App != nil {
			// Learn/forget the binary-name override before re-scanning
			// so the fresh InstalledAppsWithOverrides call sees the
			// correct name and the ✓ lights up immediately, even when
			// the manifest's derived BinaryName() is wrong. Best-
			// effort: we ignore write errors so an unwritable
			// ~/.cliff can't wedge the TUI.
			switch r.installOp {
			case pkgOpInstall:
				if len(res.DetectedBinaries) > 0 {
					_ = binmap.Remember(res.App.Repo, res.DetectedBinaries[0], res.App.BinaryName())
					if r.binOverrides == nil {
						r.binOverrides = map[string]string{}
					}
					r.binOverrides[res.App.Repo] = res.DetectedBinaries[0]
				}
			case pkgOpUninstall:
				_ = binmap.Forget(res.App.Repo)
				delete(r.binOverrides, res.App.Repo)
			}
			// Re-scan $PATH rather than blindly marking res.App.Repo
			// installed or uninstalled. This keeps the ✓ markers honest:
			// if an install reported success but didn't actually land a
			// binary on PATH (unusual but possible for odd scripts), or
			// if an uninstall "succeeded" but the binary is still there
			// (wrong GOBIN, asdf, etc.), we don't lie.
			r.installed = install.InstalledAppsWithOverrides(r.catalog.Apps, r.binOverrides)
			r.sidebar = r.sidebar.setInstalled(r.installed)
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
		case modeUninstallConfirm:
			return r.updateUninstallConfirm(m)
		case modeUninstallRunning:
			return r.updateUninstallRunning(m)
		case modeUninstallResult:
			return r.updateUninstallResult(m)
		case modeManage:
			return r.updateManage(m)
		case modeUpgradeConfirm:
			return r.updateUpgradeConfirm(m)
		case modeUpgradeRunning:
			return r.updateUpgradeRunning(m)
		case modeUpgradeResult:
			return r.updateUpgradeResult(m)
		case modeFixPath:
			return r.updateFixPath(m)
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
			r = r.setFocus(focusSidebar)
		} else {
			r = r.setFocus(focusGrid)
		}
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
			// Installed apps open the manage picker instead of the
			// readme. The assumption is that if you've installed it,
			// your next Enter is almost always "change something
			// about it" (update, uninstall) rather than "re-read the
			// README"; the picker still exposes Readme as the third
			// option for the "wait, I wanted to skim docs again" case.
			if r.installed[app.Repo] {
				r.installApp = app
				r.installReturnMode = modeBrowse
				r.manageActions, r.manageCursor = manageActionsFor(app)
				r.mode = modeManage
				return r, nil
			}
			r.readme = newReadme(app, r.width, r.height)
			r.mode = modeReadme
			return r, fetchReadmeCmd(app)
		}
		return r, nil
	case key.Matches(msg, keys.Install):
		if app := r.selectedApp(); app != nil {
			r.installApp = app
			r.installOp = pkgOpInstall
			r.installReturnMode = modeBrowse
			r.mode = modeInstallConfirm
		}
		return r, nil
	case key.Matches(msg, keys.Upgrade):
		// Direct-keybind path for "update" — skips the manage picker
		// for users who know they want to upgrade. Only meaningful
		// when the app is both installed and has an upgrade recipe;
		// silently no-ops otherwise (same pattern as `u`).
		if app := r.selectedApp(); app != nil && r.installed[app.Repo] && app.UpgradeCommand() != "" {
			r.installApp = app
			r.installOp = pkgOpUpgrade
			r.installReturnMode = modeBrowse
			r.mode = modeUpgradeConfirm
		}
		return r, nil
	case key.Matches(msg, keys.Uninstall):
		// `u` is only meaningful when the selected app is actually
		// installed. Silently no-op otherwise: showing a confirm
		// modal for "uninstall something you don't have" would just
		// confuse, and making `u` flash-warn on every stray press
		// would be noisy.
		if app := r.selectedApp(); app != nil && r.installed[app.Repo] {
			r.installApp = app
			r.installOp = pkgOpUninstall
			r.installReturnMode = modeBrowse
			r.mode = modeUninstallConfirm
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
			r = r.setFocus(focusGrid)
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
			return r.setFocus(focusSidebar)
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
	// In the readme, ⏎ is "go deeper" = install for uninstalled apps,
	// or the manage picker for installed ones. Keeping ⏎ = primary
	// action is consistent with browse mode and keeps the spatial
	// model intact. `i` always triggers install (even on installed
	// apps — counts as reinstall); `U`/`u` give direct access to
	// update/uninstall without routing through the picker.
	if key.Matches(msg, keys.Enter) {
		if app := r.selectedApp(); app != nil {
			if r.installed[app.Repo] {
				r.installApp = app
				r.installReturnMode = modeReadme
				r.manageActions, r.manageCursor = manageActionsFor(app)
				r.mode = modeManage
				return r, nil
			}
			r.installApp = app
			r.installOp = pkgOpInstall
			r.installReturnMode = modeReadme
			r.mode = modeInstallConfirm
			return r, nil
		}
	}
	if key.Matches(msg, keys.Install) {
		if app := r.selectedApp(); app != nil {
			r.installApp = app
			r.installOp = pkgOpInstall
			r.installReturnMode = modeReadme
			r.mode = modeInstallConfirm
			return r, nil
		}
	}
	if key.Matches(msg, keys.Upgrade) {
		if app := r.selectedApp(); app != nil && r.installed[app.Repo] && app.UpgradeCommand() != "" {
			r.installApp = app
			r.installOp = pkgOpUpgrade
			r.installReturnMode = modeReadme
			r.mode = modeUpgradeConfirm
			return r, nil
		}
	}
	if key.Matches(msg, keys.Uninstall) {
		if app := r.selectedApp(); app != nil && r.installed[app.Repo] {
			r.installApp = app
			r.installOp = pkgOpUninstall
			r.installReturnMode = modeReadme
			r.mode = modeUninstallConfirm
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
	// Enter has three different meanings depending on the install
	// outcome, so the footer view labels them explicitly and this
	// switch branches on the same conditions:
	//
	//   1. PathWarning pending → "fix PATH" (jump into modeFixPath).
	//   2. Clean success + launcher supported → "open in new tab".
	//   3. Clean success + launcher unsupported → "copy command".
	//   4. Install failed or no binary → plain dismiss.
	//
	// esc/q/← always means "back out" to whatever called the install.
	if key.Matches(msg, keys.Enter) {
		if r.installRes != nil && r.installRes.Err == nil && r.installRes.PathWarning != nil {
			plan, err := pathfix.Detect(r.installRes.PathWarning.Dir)
			r.fixPlan = plan
			r.fixErr = err
			r.fixApplied = false
			if plan != nil {
				r.fixAlreadyPresent = plan.Present
			} else {
				r.fixAlreadyPresent = false
			}
			r.mode = modeFixPath
			r.launchErr = nil
			return r, nil
		}
		// Clean success path: try to launch.
		if r.installRes != nil && r.installRes.Err == nil && r.installApp != nil {
			bin := r.installApp.ResolvedBinaryName(r.binOverrides)
			if bin != "" {
				return r.tryLaunchOrCopy(bin)
			}
		}
		// Fallback: plain dismiss (install failed, or no binary).
		r.mode = modeBrowse
		r.installApp = nil
		r.installRes = nil
		r.launchErr = nil
		return r, nil
	}
	// `c` as an explicit "copy command" shortcut — labeled in the
	// footer when the launch affordance is also available, so users
	// can choose the fallback without triggering a tab they don't
	// want. Harmless no-op when there's no binary.
	if msg.String() == "c" {
		if r.installRes != nil && r.installRes.Err == nil && r.installApp != nil {
			bin := r.installApp.ResolvedBinaryName(r.binOverrides)
			if bin != "" {
				if err := clipboard.Write(bin); err != nil {
					return r.flash("couldn't copy — run: " + bin), clearFlashCmd()
				}
				return r.flash("copied: " + bin), clearFlashCmd()
			}
		}
		return r, nil
	}
	if key.Matches(msg, keys.Escape, keys.Quit, keys.Left) {
		r.mode = r.installReturnMode
		r.installApp = nil
		r.installRes = nil
		r.launchErr = nil
		return r, nil
	}
	// Everything else scrolls the log viewport.
	var cmd tea.Cmd
	r.installViewport, cmd = r.installViewport.Update(msg)
	return r, cmd
}

// updateUninstallConfirm handles the "Uninstall <app>?" modal. ⏎ runs
// the derived UninstallCommand via StreamCmd; esc backs out. Mirrors
// updateInstallConfirm except there's no PathWarning / launcher flow
// to worry about on the result side.
func (r Root) updateUninstallConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape, keys.Quit):
		r.mode = r.installReturnMode
		r.installApp = nil
		return r, nil
	case key.Matches(msg, keys.Enter):
		if r.installApp == nil {
			r.mode = r.installReturnMode
			r.installApp = nil
			return r, nil
		}
		cmd := r.installApp.UninstallCommand()
		if cmd == "" {
			// No uninstall recipe available (script-type without
			// manifest [uninstall] block). The view will have
			// already communicated that; bail on ⏎ as a dismiss.
			r.mode = r.installReturnMode
			r.installApp = nil
			return r, nil
		}
		app := r.installApp
		r.installLines = nil
		r.installViewport.SetContent("")
		r.installViewport.GotoTop()
		r.mode = modeUninstallRunning
		return r, runUninstallCmd(app, cmd)
	}
	return r, nil
}

// updateUninstallRunning is the direct analog of updateInstallRunning:
// esc cancels the in-flight process (via context), everything else
// scrolls the log viewport. Completion arrives as installResultMsg,
// which the receiver routes to modeUninstallResult because r.installOp
// is pkgOpUninstall.
func (r Root) updateUninstallRunning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Escape, keys.Quit) {
		if r.installCancel != nil {
			r.installCancel()
		}
		return r, nil
	}
	var cmd tea.Cmd
	r.installViewport, cmd = r.installViewport.Update(msg)
	return r, cmd
}

// updateUninstallResult shows the final state of an uninstall. There's
// no PathWarning / launcher branching here — once the app is gone, the
// only hand-off is "close the modal", so ⏎ and esc both dismiss.
func (r Root) updateUninstallResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Enter, keys.Escape, keys.Quit, keys.Left) {
		r.mode = r.installReturnMode
		r.installApp = nil
		r.installRes = nil
		r.installOp = pkgOpInstall
		return r, nil
	}
	var cmd tea.Cmd
	r.installViewport, cmd = r.installViewport.Update(msg)
	return r, cmd
}

// updateManage drives the horizontal picker shown when ⏎ is pressed on
// an installed app. Left/Right (and h/l) move the cursor between
// actions, skipping disabled ones so the user can't pick a no-op.
// Enter runs the focused action, routing to the appropriate confirm/
// readme mode. esc backs out to whatever opened the picker.
func (r Root) updateManage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape, keys.Quit):
		r.mode = r.installReturnMode
		r.installApp = nil
		r.manageActions = nil
		r.manageCursor = 0
		return r, nil
	case key.Matches(msg, keys.Left):
		r.manageCursor = manageStep(r.manageActions, r.manageCursor, -1)
		return r, nil
	case key.Matches(msg, keys.Right):
		r.manageCursor = manageStep(r.manageActions, r.manageCursor, +1)
		return r, nil
	case key.Matches(msg, keys.Enter):
		if r.manageCursor < 0 || r.manageCursor >= len(r.manageActions) {
			return r, nil
		}
		a := r.manageActions[r.manageCursor]
		if !a.enabled {
			// Shouldn't happen (manageStep skips disabled), but
			// guard anyway — silently no-op rather than running the
			// wrong command.
			return r, nil
		}
		app := r.installApp
		switch a.kind {
		case manageUpdate:
			r.installOp = pkgOpUpgrade
			r.manageActions = nil
			r.manageCursor = 0
			r.mode = modeUpgradeConfirm
			return r, nil
		case manageUninstall:
			r.installOp = pkgOpUninstall
			r.manageActions = nil
			r.manageCursor = 0
			r.mode = modeUninstallConfirm
			return r, nil
		case manageReadme:
			// The manage picker might have been opened from readme
			// mode itself (when the user pressed ⏎ on an installed
			// app from the readme). Going back there on "Readme" is
			// a no-op; open a fresh readme anyway to guarantee a
			// refetched copy. Cheap and keeps the behavior uniform.
			r.manageActions = nil
			r.manageCursor = 0
			if app != nil {
				r.readme = newReadme(app, r.width, r.height)
				r.mode = modeReadme
				return r, fetchReadmeCmd(app)
			}
			r.mode = r.installReturnMode
			return r, nil
		}
	}
	return r, nil
}

// manageStep advances the manage-picker cursor by delta (±1), skipping
// disabled actions so the user can't land on a dimmed option. Clamps
// at the ends rather than wrapping — wrapping in a 3-item horizontal
// row is surprising ("I pressed Right and the cursor jumped to the
// leftmost one?"), and the picker is small enough that clamping never
// gets in the way.
func manageStep(actions []manageAction, cursor, delta int) int {
	if len(actions) == 0 {
		return 0
	}
	i := cursor + delta
	for i >= 0 && i < len(actions) {
		if actions[i].enabled {
			return i
		}
		i += delta
	}
	// Off the end without finding an enabled slot — stay put.
	return cursor
}

// updateUpgradeConfirm mirrors updateUninstallConfirm — ⏎ runs the
// upgrade command via StreamCmd, esc backs out to whatever opened the
// confirm modal. Structurally identical to the uninstall path because
// upgrade is "install but with a different command string."
func (r Root) updateUpgradeConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape, keys.Quit):
		r.mode = r.installReturnMode
		r.installApp = nil
		r.installOp = pkgOpInstall
		return r, nil
	case key.Matches(msg, keys.Enter):
		if r.installApp == nil {
			r.mode = r.installReturnMode
			r.installApp = nil
			r.installOp = pkgOpInstall
			return r, nil
		}
		cmd := r.installApp.UpgradeCommand()
		if cmd == "" {
			// No upgrade recipe. The view surfaces this; ⏎ here is
			// a dismiss.
			r.mode = r.installReturnMode
			r.installApp = nil
			r.installOp = pkgOpInstall
			return r, nil
		}
		app := r.installApp
		r.installLines = nil
		r.installViewport.SetContent("")
		r.installViewport.GotoTop()
		r.mode = modeUpgradeRunning
		return r, runUpgradeCmd(app, cmd)
	}
	return r, nil
}

// updateUpgradeRunning: esc cancels the child process via context, all
// other keys scroll the log viewport. Completion arrives via
// installResultMsg which the top-level receiver routes to
// modeUpgradeResult based on r.installOp == pkgOpUpgrade.
func (r Root) updateUpgradeRunning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Escape, keys.Quit) {
		if r.installCancel != nil {
			r.installCancel()
		}
		return r, nil
	}
	var cmd tea.Cmd
	r.installViewport, cmd = r.installViewport.Update(msg)
	return r, cmd
}

// updateUpgradeResult mirrors updateUninstallResult: no launcher or
// PathWarning branching (the app was already on PATH pre-upgrade, so
// there's no "open in new tab" load-bearing step), ⏎ and esc dismiss.
func (r Root) updateUpgradeResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Enter, keys.Escape, keys.Quit, keys.Left) {
		r.mode = r.installReturnMode
		r.installApp = nil
		r.installRes = nil
		r.installOp = pkgOpInstall
		return r, nil
	}
	var cmd tea.Cmd
	r.installViewport, cmd = r.installViewport.Update(msg)
	return r, cmd
}

// tryLaunchOrCopy runs the post-install "open in new tab" action.
// When the host terminal exposes a tab-spawn mechanism we call it and,
// on success, dismiss the modal and return the user to the catalog —
// their new app is now running next door. When no launcher is
// available, we copy the command to the clipboard (native tool
// preferred, OSC52 fallback) and flash an honest toast based on
// whether the copy actually worked. Either way the user gets a single
// keystroke path to "go try it," which is the whole point.
func (r Root) tryLaunchOrCopy(bin string) (tea.Model, tea.Cmd) {
	if r.launchMethod == launcher.MethodUnsupported {
		err := clipboard.Write(bin)
		r.mode = modeBrowse
		r.installApp = nil
		r.installRes = nil
		r.launchErr = nil
		if err != nil {
			// Copy failed — don't claim it worked. Show the command
			// so the user can copy it by hand (or by mouse) from the
			// toast. Keeping it short so it doesn't overflow the
			// flash line: the full command is still on the modal
			// behind, so this is supplementary.
			return r.flash("couldn't copy — run: " + bin), clearFlashCmd()
		}
		return r.flash("copied: " + bin + " — paste in a new terminal"), clearFlashCmd()
	}
	if err := launcher.Launch(r.launchMethod, bin); err != nil {
		// Leave the modal open, record the error, let the view
		// surface it alongside the "run this yourself" fallback.
		// We deliberately don't also copy-to-clipboard on error —
		// that would steal the user's clipboard after a failed
		// action; better to show the hint and let them retry or
		// press `c` to copy explicitly.
		r.launchErr = err
		return r, nil
	}
	// Success: dismiss back to the catalog. The newly spawned tab has
	// the app running in it; cliff stays open here.
	r.mode = modeBrowse
	r.installApp = nil
	r.installRes = nil
	r.launchErr = nil
	return r.flash("launched " + bin + " in new tab"), clearFlashCmd()
}

// updateFixPath runs the modeFixPath screen. It has two phases:
//
//   - !fixApplied: we're asking "OK to append this line to <rc>?"
//     Enter confirms and runs Apply; esc/q/← backs out without
//     writing anything.
//   - fixApplied: we've written (or tried). Enter or esc dismisses
//     back to the grid.
//
// Split into phases so the keybinds stay simple and the user can't
// accidentally double-apply by holding Enter.
func (r Root) updateFixPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if r.fixApplied {
		// Post-apply, Enter means "open in new tab" (if we can) and
		// esc/q/← means "done, back to the catalog". This mirrors
		// updateInstallResult — after a successful hand-off step,
		// Enter is always "forward motion," never just dismiss.
		if key.Matches(msg, keys.Enter) {
			if r.fixErr == nil && r.installApp != nil {
				bin := r.installApp.ResolvedBinaryName(r.binOverrides)
				if bin != "" {
					// clearFixPath first so the modeBrowse fall-through
					// in tryLaunchOrCopy doesn't land on a stale plan.
					r = r.clearFixPath()
					return r.tryLaunchOrCopy(bin)
				}
			}
			// No launch possible — plain dismiss (existing behavior).
			r = r.clearFixPath()
			r.mode = modeBrowse
			r.installApp = nil
			r.installRes = nil
			return r, nil
		}
		if key.Matches(msg, keys.Escape, keys.Quit, keys.Left) {
			r = r.clearFixPath()
			r.mode = modeBrowse
			r.installApp = nil
			r.installRes = nil
			r.launchErr = nil
			return r, nil
		}
		return r, nil
	}
	if key.Matches(msg, keys.Enter) {
		// Only Apply for shells we support. Detect already returned
		// ErrShellUnsupported for fish/unknown — honor that and show
		// the user the hand-edit fallback rather than writing bash
		// syntax into a fish config.
		if r.fixErr == nil && r.fixPlan != nil {
			r.fixErr = pathfix.Apply(r.fixPlan)
		}
		r.fixApplied = true
		return r, nil
	}
	if key.Matches(msg, keys.Escape, keys.Quit, keys.Left) {
		r = r.clearFixPath()
		r.mode = modeInstallResult
		return r, nil
	}
	return r, nil
}

func (r Root) clearFixPath() Root {
	r.fixPlan = nil
	r.fixErr = nil
	r.fixApplied = false
	r.fixAlreadyPresent = false
	return r
}

func (r Root) updateSidebarOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape, keys.Categories):
		r.mode = modeBrowse
		r = r.syncFocus()
		return r, nil
	case key.Matches(msg, keys.Enter):
		r.mode = modeBrowse
		r = r.syncFocus()
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
