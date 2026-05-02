package ui

import (
	"context"
	"time"

	"github.com/jmcntsh/cliff/internal/binmap"
	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/install"
	"github.com/jmcntsh/cliff/internal/launcher"
	"github.com/jmcntsh/cliff/internal/pathfix"
	"github.com/jmcntsh/cliff/internal/submit"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type focusState int

const (
	focusGrid focusState = iota
	focusSidebar
)

type mode int

const (
	modeBrowse mode = iota
	modeSidebarOverlay
	modeSearch
	modeHelp
	modeReadme
	modeManage // picker: Update / Uninstall / Readme for installed apps

	// Package-operation modes are shared across install / uninstall /
	// upgrade. The specific verb is carried on r.installOp, which the
	// view/update handlers branch on for labels and op-specific flows
	// (PathWarning + launcher handoff are install-only). Collapsing
	// the previous 3×3 = 9 modes into a single 3-phase machine halves
	// the dispatch table and keeps the three ops in lockstep by
	// construction — adding a new phase (e.g. a post-diagnose retry
	// screen) doesn't require adding three parallel modes.
	modePkgConfirm
	modePkgRunning
	modePkgResult

	modeFixPath // confirm + result screen for auto-adding a dir to $PATH
	modeSubmit  // confirm screen for opening the registry submit form in a browser
)

// submitPhase tracks where we are inside modeSubmit. The flow is
// strictly linear: form (user types fields) → confirm (preview the
// URL we're about to open) → opened (browser hand-off completed,
// either successfully or with an error to display). Splitting the
// phases out of mode (rather than minting three new modes) keeps
// the top-level mode dispatch table flat and lets the submit
// overlay own its own internal state machine.
type submitPhase int

const (
	// submitPhaseForm is the huh-driven entry form. The user is
	// filling in name / repo / description / notes; esc cancels
	// the whole flow, ⏎ on the final field advances to confirm.
	submitPhaseForm submitPhase = iota

	// submitPhaseConfirm is the "about to open <URL>" preview.
	// Same UX as the pre-huh submit overlay: the user sees the
	// URL before we hand off to a browser, so the keypress that
	// caused the navigation is never load-bearing.
	submitPhaseConfirm

	// submitPhaseOpened is the post-hand-off state. submitErr
	// distinguishes success (browser opened) from failure (the
	// URL is shown for manual paste).
	submitPhaseOpened
)

// pkgOp is the active package operation for the shared confirm/running/
// result state machine above. Install is the zero value so paths that
// leave r.installOp unset stay on the install side by default.
type pkgOp int

const (
	pkgOpInstall pkgOp = iota
	pkgOpUninstall
	pkgOpUpgrade
)

// verb returns the short gerund used in running-state headers ("Installing",
// "Uninstalling", "Updating"). Centralizing the strings keeps
// pkgRunningView consistent with the confirm/result headers without
// each call site having to duplicate a switch.
func (o pkgOp) verb() string {
	switch o {
	case pkgOpUninstall:
		return "Uninstall"
	case pkgOpUpgrade:
		return "Update"
	default:
		return "Install"
	}
}

// runningVerb is the -ing form used on the progress modal.
func (o pkgOp) runningVerb() string {
	switch o {
	case pkgOpUninstall:
		return "Uninstalling"
	case pkgOpUpgrade:
		return "Updating"
	default:
		return "Installing"
	}
}

// pastVerb is the past-tense form used on the success line ("Installed foo",
// "Uninstalled foo", "Updated foo").
func (o pkgOp) pastVerb() string {
	switch o {
	case pkgOpUninstall:
		return "Uninstalled"
	case pkgOpUpgrade:
		return "Updated"
	default:
		return "Installed"
	}
}

// sortMode is the user-cyclable sort key. The cycle is descending-only
// by design: ascending sorts (stars ↑, A→Z name) buried high-signal
// apps under low-signal ones, which made the cycle feel like "a full
// rotation = back where I started" rather than "a full rotation = I
// saw the catalog three useful ways." sortHotDesc only enters the
// cycle when hot.json is published and the reveal threshold passes
// (see (Root).visibleSorts).
type sortMode int

const (
	sortStarsDesc sortMode = iota
	sortRecencyDesc
	sortHotDesc
)

func (s sortMode) label() string {
	switch s {
	case sortStarsDesc:
		return "stars ↓"
	case sortRecencyDesc:
		return "recency ↓"
	case sortHotDesc:
		return "hot ↓"
	}
	return ""
}

// hotRevealThreshold is the minimum number of apps in the catalog
// with a nonzero HotScore before the Hot surface (sidebar row +
// sort-cycle step) appears. Below this we'd be ranking ~all apps
// by ~all-zero scores, which is just a degenerate alphabetical
// list.
const hotRevealThreshold = 25

// visibleSorts is the sort cycle the Sort key actually walks, given
// the current reveal state. Hot is appended only when revealed, so
// pressing 's' on a fresh client never lands on a sort that ranks
// every app at zero.
func (r Root) visibleSorts() []sortMode {
	if r.hotRevealed {
		return []sortMode{sortStarsDesc, sortRecencyDesc, sortHotDesc}
	}
	return []sortMode{sortStarsDesc, sortRecencyDesc}
}

// nextSort returns the next sort in r.visibleSorts() after the
// current one, wrapping. Used by the Sort keypress handler. If the
// current sort isn't in the visible list (e.g. the user was on
// sortHotDesc and the reveal flipped back to false because hot.json
// disappeared), we fall back to the first visible sort.
func (r Root) nextSort() sortMode {
	cycle := r.visibleSorts()
	for i, s := range cycle {
		if s == r.sort {
			return cycle[(i+1)%len(cycle)]
		}
	}
	return cycle[0]
}

type Root struct {
	catalog     *catalog.Catalog
	grid        grid
	sidebar     sidebar
	search      textinput.Model
	readme      readmeModel
	focus             focusState
	mode              mode
	helpReturnMode    mode // mode to return to when help is dismissed
	installReturnMode mode // mode to return to when an install modal is dismissed
	sort              sortMode
	// hotRevealed flips true once we've fetched hot.json *and* at least
	// hotRevealThreshold apps carry a non-zero HotScore. Both the Hot
	// sidebar row and the sortHotDesc step in the sort cycle are gated
	// on this flag, so the surface appears in both places at once. The
	// New sidebar row hides on the same flag — they trade places, not
	// stack. False is the sticky steady-state until the worker's
	// days-seen gate flips and the catalog has enough nonzero scores.
	hotRevealed       bool
	layout            layoutMode
	width       int
	height      int
	ready       bool
	flashMsg    string
	flashExpiry time.Time

	installed       map[string]bool    // repo -> installed, derived from $PATH via install.Detect
	// binOverrides is a repo→binary-name map learned from previous
	// installs (internal/binmap). It corrects manifest-derived
	// BinaryName() guesses when they're wrong (e.g. cargo crate
	// "minesweep" installed from repo "cpcloud/minesweep-rs" ships
	// as `minesweep`, not `minesweep-rs`). Loaded once at startup,
	// mutated on successful install, written through binmap.Remember.
	binOverrides    map[string]string
	installCancel   context.CancelFunc // non-nil while an install/uninstall is running
	installLines    []string           // streamed output from the running op (source of truth)
	installViewport viewport.Model     // derived view for scrolling logs
	installApp   *catalog.App
	installRes   *install.Result
	// installOp distinguishes the package operation in flight: install vs
	// uninstall. The running/result modes share one state machine (same
	// command streamer, same viewport, same Result), but the modals need
	// to render different verbs and the install-side-only follow-ups
	// (PathWarning, launcher) must stay suppressed when uninstalling.
	installOp pkgOp

	// Fix-PATH follow-up flow. When a post-install PathWarning fires,
	// Enter on the result modal lifts us into modeFixPath with a
	// plan ready to apply. fixApplied flips to true once we've
	// written the rc file (success or error). fixAlreadyPresent
	// snapshots Plan.Present at Detect time so the result screen can
	// distinguish "just added" from "was already there" after Apply
	// has clobbered Plan.Present to true.
	fixPlan            *pathfix.Plan
	fixErr             error
	fixApplied         bool
	fixAlreadyPresent  bool

	// Post-install launcher state. launchMethod is detected once at
	// startup (via launcher.Detect on CurrentEnv) so every install's
	// result screen can render the right affordance — "⏎ open in new
	// tab" when we can do that, "⏎ copy command" otherwise — without
	// re-detecting on every keypress. launchErr holds the last spawn
	// error if Launch failed, so the result view can surface a hint
	// rather than silently swallowing it.
	launchMethod launcher.Method
	launchErr    error

	// Submit-flow state for modeSubmit. Populated when `+` is pressed
	// from any mode that allows submission; cleared when the overlay
	// closes.
	//
	// The flow has three phases (see submitPhase): a huh-driven form
	// where the user fills in the manifest seed fields, a confirm
	// step that previews the URL we're about to open, and the post-
	// open state that either confirms success or surfaces the URL
	// for manual paste. submitReturnMode is the mode we bounce back
	// to on esc/cancel (browse or readme, depending on where `+`
	// was pressed). submitErr holds the browser.Open error so the
	// post-open phase can fall back to showing the URL.
	//
	// submitFields is the running buffer the huh form writes into;
	// once the form completes, we hand it to submit.Request to derive
	// the prefilled GitHub URL. Keeping the request struct as the
	// source of truth means the CLI verb (cmdSubmit) and the TUI
	// form share one URL builder — change the schema in one place
	// and both surfaces follow.
	submitReturnMode mode
	submitPhase      submitPhase
	submitErr        error
	submitURL        string
	submitFields     submit.Request
	submitForm       *huh.Form

	// Manage-picker state for modeManage. Populated when Enter is
	// pressed on an installed app; emptied when the picker closes.
	// The picker is a horizontal row of actions (Update / Uninstall /
	// Readme), with Update default-selected because it's the most
	// common "what do I want to do with this installed thing" and is
	// benign. Uninstall is destructive so never default; Readme is
	// the escape hatch for "I meant to re-read docs, not manage."
	manageActions []manageAction
	manageCursor  int

	// spinner is a single shared bubbles/spinner reused everywhere
	// cliff is waiting on something the user can see: install
	// startup before the first stdout line, README fetches, reel
	// fetches. One ticker means the glyph rotates in lockstep
	// across surfaces, and we only post one TickMsg per frame
	// regardless of how many "loading" states are visible at once.
	//
	// The ticker is started lazily — Init() returns nil so the very
	// first paint isn't gated on a spinner tick — and re-armed
	// whenever a new loading state begins (see startSpinner). When
	// nothing's loading, the tick goroutine stays parked until the
	// next loading state arms it again.
	spinner spinner.Model

	// titlePhase animates the brand-mark gradient on launch. It
	// starts at 0 (flat fuchsia flash), ticks toward 1.0 over
	// ~500ms, and then stays at 1.0 for the rest of the session.
	// theme.GradientTitlePhase reads this to decide how far each
	// rune has interpolated from start-color toward its final
	// gradient slot, so on launch the brand mark "ignites" instead
	// of just snapping in. Once at 1.0, the rest of the codebase
	// can keep calling theme.GradientTitle (which is just
	// GradientTitlePhase(s, 1.0)) and behavior is unchanged.
	titlePhase float64
}

// spinnerActive reports whether any UI surface currently wants the
// spinner ticking. Centralizing the answer here keeps the "do we
// re-tick?" check in Update consistent with what each view actually
// renders, and makes adding a new spinning state a one-line change.
func (r Root) spinnerActive() bool {
	switch {
	case r.mode == modePkgRunning && len(r.installLines) == 0:
		return true
	case r.mode == modeReadme && r.readme.loading:
		return true
	case r.mode == modeReadme && r.readme.reelLoading():
		return true
	}
	return false
}

// manageAction is one choice on the manage picker. Kind drives what
// happens on Enter; enabled gates arrow navigation and dimming
// (Update is disabled when the app has no UpgradeCommand; Uninstall
// is disabled when the app has no uninstall recipe). The Readme
// action is always enabled and is always the third/last slot, because
// it's the fallback "I meant to read about it, not change it."
type manageAction struct {
	kind    manageKind
	label   string
	enabled bool
}

type manageKind int

const (
	manageUpdate manageKind = iota
	manageUninstall
	manageReadme
)

func New(c *catalog.Catalog) Root {
	ti := textinput.New()
	ti.Prompt = "search  "
	ti.Placeholder = "type to filter apps..."
	ti.CharLimit = 80
	ti.PromptStyle = theme.AccentBold
	ti.TextStyle = theme.FocusText
	ti.PlaceholderStyle = theme.MutedItalic
	ti.Cursor.Style = theme.AccentText

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.ColorAccent)

	overrides := binmap.Load()
	installed := install.InstalledAppsWithOverrides(c.Apps, overrides)
	r := Root{
		catalog:         c,
		grid:            newGrid(),
		sidebar:         newSidebar(c, installed, false),
		search:          ti,
		installed:       installed,
		binOverrides:    overrides,
		installViewport: viewport.New(installLogWidth, installLogHeight),
		launchMethod:    launcher.Detect(launcher.CurrentEnv()),
		spinner:         sp,
	}
	r = r.refilter()
	return r
}

// installLogWidth/Height size the scrollable log area inside the install
// modals. They're fixed (rather than responsive) because the modal itself
// is fixed-width — tying them together would require threading resize
// through the view path for little gain.
const (
	installLogWidth  = 72
	installLogHeight = 12
)

// Init batches the launch-title sweep with a one-shot background
// hot.json fetch. The fetch is fire-and-forget: if the sidecar is
// 404 (worker still inside its 14-day days-seen gate) or unreachable,
// hotFetchedMsg arrives with Available=false and the UI quietly
// stays in pre-reveal shape. No spinner; nothing to wait for.
func (r Root) Init() tea.Cmd {
	return tea.Batch(launchTitleTick(), fetchHotCmd())
}

// titleTickMsg drives the launch sweep on the brand-mark gradient.
// One message arrives every titleTickInterval until phase hits 1.0,
// at which point the chain self-terminates (see Update). All other
// rendering ignores the phase: it only feeds GradientTitlePhase.
type titleTickMsg struct{}

const (
	// 40fps is the smoothest we can hit without burning frames; below
	// 30fps the torch motion looks steppy on a word as short as
	// "cliff." Apple Terminal handles 40fps fine; iTerm and Ghostty
	// can do more, but it's diminishing returns past 40 for a
	// 1.2-second one-shot.
	titleTickInterval = 25 * time.Millisecond
	// Phase travels 0 → 1.2 (not 0 → 1.0) so the torch sweeps fully
	// off the right edge and every rune gets a final post-torch
	// frame. 0.025 step × 1.2 range = 48 ticks × 25ms ≈ 1200ms total,
	// which is the sweet spot: long enough to register, short enough
	// to not feel like a splash screen.
	titleTickStep = 0.025
	titleTickEnd  = 1.2
)

func launchTitleTick() tea.Cmd {
	return tea.Tick(titleTickInterval, func(time.Time) tea.Msg {
		return titleTickMsg{}
	})
}

func (r Root) selectedApp() *catalog.App { return r.grid.selected() }

// gridDimensions returns (width, height) available to the card grid,
// after subtracting the sidebar (when visible) and the search bar
// (when search mode is active). Height accounts for the footer.
func (r Root) gridDimensions() (int, int) {
	gridW := r.width
	if r.layout != layoutNarrow {
		gridW -= sidebarWidth + sidebarGap
	}
	gridW = max(gridW, 20)
	// Reserve rows for: title (1) + blank under title (1) + newline
	// before footer (1) + footer (1) = 4. The previous "-2" only
	// accounted for the title and blank, which left the grid one row
	// taller than the available space; the terminal then scrolled
	// the title and blank off the top so the footer could land at
	// the bottom row, which is the bug that prompted this fix
	// (top of screen showed cards directly, no title visible).
	gridH := max(r.height-4, 1)
	if r.mode == modeSearch {
		gridH = max(gridH-3, 1)
	}
	return gridW, gridH
}

func (r Root) resize() Root {
	r.layout = modeFor(r.width)
	gridW, gridH := r.gridDimensions()
	r.search.Width = 50
	r.grid = r.grid.setLayout(gridW, gridH)
	if r.layout == layoutNarrow && r.focus == focusSidebar {
		r.focus = focusGrid
	}
	r = r.syncFocus()
	return r
}

// setFocus is the one place that changes which pane has input and
// keeps the two panes' focused flags in sync. Callers used to update
// r.focus and r.sidebar.setFocused by hand; they forgot the grid
// flag when it was added, which is the exact class of bug this
// helper exists to prevent.
func (r Root) setFocus(f focusState) Root {
	r.focus = f
	return r.syncFocus()
}

// syncFocus pushes r.focus down to the grid and sidebar. Idempotent;
// safe to call whenever focus may have moved.
func (r Root) syncFocus() Root {
	r.sidebar = r.sidebar.setFocused(r.focus == focusSidebar)
	r.grid = r.grid.setFocused(r.focus == focusGrid)
	return r
}

// refilter recomputes the visible app slice from current sidebar/filter/search/sort
// state, preserving cursor selection by repo when possible. Called whenever any
// of those inputs change, or when install state changes (so the ✓ markers update).
func (r Root) refilter() Root {
	var selectedRepo string
	if app := r.grid.selected(); app != nil {
		selectedRepo = app.Repo
	}

	apps := filterAndSort(r.catalog.Apps, filterCriteria{
		category:  r.sidebar.selected(),
		query:     r.search.Value(),
		sort:      r.sort,
		installed: r.installed,
	})

	r.grid = r.grid.setApps(apps, r.installed)
	r.grid = r.grid.selectByRepo(selectedRepo)
	gridW, gridH := r.gridDimensions()
	r.grid = r.grid.setLayout(gridW, gridH)
	return r
}
