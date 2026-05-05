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
	"github.com/jmcntsh/cliff/internal/submit"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type flashClearMsg struct{}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Spinner ticks are global, but only re-arm while a visible surface
	// still needs animation.
	if tick, ok := msg.(spinner.TickMsg); ok {
		var cmd tea.Cmd
		r.spinner, cmd = r.spinner.Update(tick)
		if !r.spinnerActive() {
			return r, nil
		}
		return r, cmd
	}

	if _, ok := msg.(titleTickMsg); ok {
		// Overshoot so the launch sweep fully clears the word before
		// the tick chain stops.
		r.titlePhase += titleTickStep
		if r.titlePhase >= titleTickEnd {
			r.titlePhase = titleTickEnd
			return r, nil
		}
		return r, launchTitleTick()
	}

	// Huh forms need non-key messages too; key and resize messages stay
	// in the main switch so esc-cancel and form rebuilds happen first.
	if r.mode == modeSubmit && r.submitPhase == submitPhaseForm && r.submitForm != nil {
		switch msg.(type) {
		case tea.KeyMsg, tea.WindowSizeMsg:
			// fall through to the main switch below
		default:
			form, cmd := r.submitForm.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				r.submitForm = f
			}
			return r, cmd
		}
	}

	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = m.Width
		r.height = m.Height
		r.ready = true
		if r.readme.ready {
			r.readme = r.readme.resize(m.Width, m.Height)
		}
		// Rebuild the huh form on resize; typed values live in submitFields.
		if r.mode == modeSubmit && r.submitPhase == submitPhaseForm {
			r.submitForm = newSubmitForm(
				&r.submitFields,
				submitFormWidth(r.width),
				submitFormHeight(r.height),
			)
			return r.resize(), r.submitForm.Init()
		}
		return r.resize(), nil

	case readmeFetchedMsg:
		var cmd tea.Cmd
		r.readme, cmd = r.readme.applyFetch(m)
		return r, cmd

	case hotFetchedMsg:
		// Hot scores are mode-agnostic and may arrive behind any overlay.
		r = r.applyHotScores(m)
		return r.refilter(), nil

	case reelFetchedMsg:
		// Unconditional; stale or unrelated reel fetches are ignored by
		// the readme model.
		var cmd tea.Cmd
		r.readme, cmd = r.readme.applyReelFetched(m)
		return r, cmd

	case reelTickMsg:
		// Keep reel playback moving through brief overlay mode switches.
		var cmd tea.Cmd
		r.readme, cmd = r.readme.Update(m)
		return r, cmd

	case installStartedMsg:
		r.installCancel = m.Cancel
		return r, nil

	case installLineMsg:
		r.installLines = append(r.installLines, m.Line)
		// Keep only recent output without pinning the old backing array.
		if len(r.installLines) > 2000 {
			copy(r.installLines, r.installLines[len(r.installLines)-2000:])
			r.installLines = r.installLines[:2000]
		}
		// Tail-follow only while the user is already at the bottom.
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
		r.mode = modePkgResult
		r.launchErr = nil
		// Result.Output includes any final partial line missed by onLine.
		r.installViewport.SetContent(strings.TrimRight(res.Output, "\n"))
		r.installViewport.GotoTop()
		if res.Err == nil && res.App != nil {
			// Update binary-name overrides before re-scanning installed state.
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
			// Re-scan instead of trusting the package command's exit status.
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
		case modePkgConfirm:
			return r.updatePkgConfirm(m)
		case modePkgRunning:
			return r.updatePkgRunning(m)
		case modePkgResult:
			return r.updatePkgResult(m)
		case modeManage:
			return r.updateManage(m)
		case modeFixPath:
			return r.updateFixPath(m)
		case modeSubmit:
			return r.updateSubmit(m)
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
		r.sort = r.nextSort()
		return r.refilter(), nil
	case key.Matches(msg, keys.Search):
		r.mode = modeSearch
		r.search.Focus()
		return r.resize(), textinput.Blink
	case key.Matches(msg, keys.Help):
		r.helpReturnMode = modeBrowse
		r.mode = modeHelp
		return r, nil
	case key.Matches(msg, keys.Submit):
		// Submit always starts blank; the current app is already listed.
		r.submitReturnMode = modeBrowse
		return r.openSubmitForm()
	case key.Matches(msg, keys.Enter):
		if app := r.selectedApp(); app != nil {
			if r.installed[app.Repo] {
				r.installApp = app
				r.installReturnMode = modeBrowse
				r.manageActions, r.manageCursor = manageActionsFor(app)
				r.mode = modeManage
				return r, nil
			}
			r.readme = newReadme(app, r.width, r.height)
			r.mode = modeReadme
			return r, tea.Batch(fetchReadmeCmd(app), r.readme.ReelInit(), r.spinner.Tick)
		}
		return r, nil
	case key.Matches(msg, keys.Install):
		if app := r.selectedApp(); app != nil {
			r.installApp = app
			r.installOp = pkgOpInstall
			r.installReturnMode = modeBrowse
			r.mode = modePkgConfirm
		}
		return r, nil
	case key.Matches(msg, keys.Upgrade):
		if app := r.selectedApp(); app != nil && r.installed[app.Repo] && app.UpgradeCommand() != "" {
			r.installApp = app
			r.installOp = pkgOpUpgrade
			r.installReturnMode = modeBrowse
			r.mode = modePkgConfirm
		}
		return r, nil
	case key.Matches(msg, keys.Uninstall):
		if app := r.selectedApp(); app != nil && r.installed[app.Repo] {
			r.installApp = app
			r.installOp = pkgOpUninstall
			r.installReturnMode = modeBrowse
			r.mode = modePkgConfirm
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
		if key.Matches(msg, keys.Right) {
			r = r.setFocus(focusGrid)
			return r, nil
		}
		newSB, changed := r.sidebar.update(msg)
		r.sidebar = newSB
		if changed {
			r = r.refilter()
			r.grid = r.grid.jumpTop()
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
	if key.Matches(msg, keys.Escape, keys.Quit, keys.Left) {
		r.mode = modeBrowse
		return r, nil
	}
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
			r.mode = modePkgConfirm
			return r, nil
		}
	}
	if key.Matches(msg, keys.Install) {
		if app := r.selectedApp(); app != nil {
			r.installApp = app
			r.installOp = pkgOpInstall
			r.installReturnMode = modeReadme
			r.mode = modePkgConfirm
			return r, nil
		}
	}
	if key.Matches(msg, keys.Upgrade) {
		if app := r.selectedApp(); app != nil && r.installed[app.Repo] && app.UpgradeCommand() != "" {
			r.installApp = app
			r.installOp = pkgOpUpgrade
			r.installReturnMode = modeReadme
			r.mode = modePkgConfirm
			return r, nil
		}
	}
	if key.Matches(msg, keys.Uninstall) {
		if app := r.selectedApp(); app != nil && r.installed[app.Repo] {
			r.installApp = app
			r.installOp = pkgOpUninstall
			r.installReturnMode = modeReadme
			r.mode = modePkgConfirm
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
	if key.Matches(msg, keys.Submit) {
		r.submitReturnMode = modeReadme
		return r.openSubmitForm()
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
	// nav (hjkl) belongs to the input itself, so we match by key string
	// against the arrow-only inputs rather than by binding.
	if s := msg.String(); s == "up" || s == "down" || s == "left" || s == "right" {
		return r.gridNav(msg), nil
	}
	var cmd tea.Cmd
	r.search, cmd = r.search.Update(msg)
	return r.refilter(), cmd
}

// updatePkgConfirm handles the shared install / uninstall / upgrade confirm modal.
func (r Root) updatePkgConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape, keys.Quit):
		r.mode = r.installReturnMode
		r.installApp = nil
		r.installOp = pkgOpInstall
		return r, nil
	case key.Matches(msg, keys.Enter):
		cmd := pkgOpCommand(r.installApp, r.installOp)
		if cmd == "" {
			r.mode = r.installReturnMode
			r.installApp = nil
			r.installOp = pkgOpInstall
			return r, nil
		}
		app := r.installApp
		r.installLines = nil
		r.installViewport.SetContent("")
		r.installViewport.GotoTop()
		r.mode = modePkgRunning
		return r, tea.Batch(runPkgCmd(app, cmd), r.spinner.Tick)
	}
	return r, nil
}

// updatePkgRunning handles cancellation and log scrolling for any package op.
func (r Root) updatePkgRunning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

// updatePkgResult handles the terminal modal for any package op.
func (r Root) updatePkgResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	installing := r.installOp == pkgOpInstall

	if key.Matches(msg, keys.Enter) {
		// Install follow-ups: fix PATH first, then offer launch.
		if installing && r.installRes != nil && r.installRes.Err == nil && r.installRes.PathWarning != nil {
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
		if installing && r.installRes != nil && r.installRes.Err == nil && r.installApp != nil {
			bin := r.installApp.ResolvedBinaryName(r.binOverrides)
			if bin != "" {
				return r.tryLaunchOrCopy(bin)
			}
		}
		r.mode = r.installReturnMode
		r.installApp = nil
		r.installRes = nil
		r.installOp = pkgOpInstall
		r.launchErr = nil
		return r, nil
	}
	if installing && msg.String() == "c" {
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
		r.installOp = pkgOpInstall
		r.launchErr = nil
		return r, nil
	}
	var cmd tea.Cmd
	r.installViewport, cmd = r.installViewport.Update(msg)
	return r, cmd
}

// pkgOpCommand returns the shell command for an app/op pair, or "".
func pkgOpCommand(app *catalog.App, op pkgOp) string {
	if app == nil {
		return ""
	}
	switch op {
	case pkgOpUninstall:
		return app.UninstallCommand()
	case pkgOpUpgrade:
		return app.UpgradeCommand()
	default:
		s := app.PrimaryInstallSpec()
		if s == nil {
			return ""
		}
		return s.Shell()
	}
}

// updateManage drives the installed-app action picker.
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
			return r, nil
		}
		app := r.installApp
		switch a.kind {
		case manageUpdate:
			r.installOp = pkgOpUpgrade
			r.manageActions = nil
			r.manageCursor = 0
			r.mode = modePkgConfirm
			return r, nil
		case manageUninstall:
			r.installOp = pkgOpUninstall
			r.manageActions = nil
			r.manageCursor = 0
			r.mode = modePkgConfirm
			return r, nil
		case manageReadme:
			r.manageActions = nil
			r.manageCursor = 0
			if app != nil {
				r.readme = newReadme(app, r.width, r.height)
				r.mode = modeReadme
				return r, tea.Batch(fetchReadmeCmd(app), r.readme.ReelInit(), r.spinner.Tick)
			}
			r.mode = r.installReturnMode
			return r, nil
		}
	}
	return r, nil
}

// manageStep advances by delta, skipping disabled actions and clamping at ends.
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
	return cursor
}

// tryLaunchOrCopy runs the post-install "open in new tab" action.
func (r Root) tryLaunchOrCopy(bin string) (tea.Model, tea.Cmd) {
	if r.launchMethod == launcher.MethodUnsupported {
		err := clipboard.Write(bin)
		r.mode = modeBrowse
		r.installApp = nil
		r.installRes = nil
		r.launchErr = nil
		if err != nil {
			return r.flash("couldn't copy — run: " + bin), clearFlashCmd()
		}
		return r.flash("copied: " + bin + " — paste in a new terminal"), clearFlashCmd()
	}
	if err := launcher.Launch(r.launchMethod, bin); err != nil {
		// Keep the modal open and let `c` be the explicit copy fallback.
		r.launchErr = err
		return r, nil
	}
	r.mode = modeBrowse
	r.installApp = nil
	r.installRes = nil
	r.launchErr = nil
	return r.flash("launched " + bin + " in new tab"), clearFlashCmd()
}

// updateFixPath handles the confirm/result flow for adding a bin dir to PATH.
func (r Root) updateFixPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if r.fixApplied {
		if key.Matches(msg, keys.Enter) {
			if r.fixErr == nil && r.installApp != nil {
				bin := r.installApp.ResolvedBinaryName(r.binOverrides)
				if bin != "" {
					r = r.clearFixPath()
					return r.tryLaunchOrCopy(bin)
				}
			}
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
		if r.fixErr == nil && r.fixPlan != nil {
			r.fixErr = pathfix.Apply(r.fixPlan)
		}
		r.fixApplied = true
		return r, nil
	}
	if key.Matches(msg, keys.Escape, keys.Quit, keys.Left) {
		r = r.clearFixPath()
		r.mode = modePkgResult
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

// openSubmitForm resets submit state and enters the huh form phase.
func (r Root) openSubmitForm() (tea.Model, tea.Cmd) {
	r.submitFields = submit.Request{}
	r.submitURL = ""
	r.submitErr = nil
	r.submitPhase = submitPhaseForm
	r.mode = modeSubmit
	r.submitForm = newSubmitForm(
		&r.submitFields,
		submitFormWidth(r.width),
		submitFormHeight(r.height),
	)
	return r, r.submitForm.Init()
}

// updateSubmit drives the form, confirm, and post-open submit phases.
func (r Root) updateSubmit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch r.submitPhase {
	case submitPhaseForm:
		return r.updateSubmitForm(msg)
	case submitPhaseConfirm:
		return r.updateSubmitConfirm(msg)
	case submitPhaseOpened:
		return r.updateSubmitOpened(msg)
	}
	return r, nil
}

func (r Root) updateSubmitForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if r.submitForm == nil {
		// Defensive: no form means nothing to drive. Bounce back to
		// the return mode so the user isn't stuck on a blank modal.
		r.mode = r.submitReturnMode
		r.submitPhase = submitPhaseForm
		return r, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(km, keys.Escape, keys.Quit) {
			r.mode = r.submitReturnMode
			r.submitForm = nil
			r.submitFields = submit.Request{}
			return r, nil
		}
	}

	form, cmd := r.submitForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		r.submitForm = f
	}

	if r.submitForm.State == huh.StateCompleted {
		r.submitURL = r.submitFields.URL()
		r.submitPhase = submitPhaseConfirm
		r.submitForm = nil
		return r, nil
	}
	if r.submitForm.State == huh.StateAborted {
		r.mode = r.submitReturnMode
		r.submitForm = nil
		r.submitFields = submit.Request{}
		return r, nil
	}
	return r, cmd
}

func (r Root) updateSubmitConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return r, nil
	}
	switch {
	case key.Matches(km, keys.Escape, keys.Quit, keys.Left):
		r.mode = r.submitReturnMode
		r.submitURL = ""
		r.submitFields = submit.Request{}
		return r, nil
	case key.Matches(km, keys.Enter):
		r.submitErr = browser.Open(r.submitURL)
		r.submitPhase = submitPhaseOpened
		return r, nil
	}
	return r, nil
}

func (r Root) updateSubmitOpened(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return r, nil
	}
	if key.Matches(km, keys.Enter, keys.Escape, keys.Quit, keys.Left) {
		r.mode = r.submitReturnMode
		r.submitPhase = submitPhaseForm
		r.submitErr = nil
		r.submitURL = ""
		r.submitFields = submit.Request{}
		return r, nil
	}
	return r, nil
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
		r.grid = r.grid.jumpTop()
		return r, nil
	}
	newSB, changed := r.sidebar.update(msg)
	r.sidebar = newSB
	if changed {
		r = r.refilter()
		r.grid = r.grid.jumpTop()
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
	s := app.PrimaryInstallSpec()
	if s == nil {
		return ""
	}
	return s.Shell()
}
