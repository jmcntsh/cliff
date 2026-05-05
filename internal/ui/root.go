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

	// Package-operation modes are shared across install / uninstall / upgrade;
	// r.installOp carries the active verb.
	modePkgConfirm
	modePkgRunning
	modePkgResult

	modeFixPath // confirm + result screen for auto-adding a dir to $PATH
	modeSubmit  // confirm screen for opening the registry submit form in a browser
)

// submitPhase tracks the form → confirm → opened flow inside modeSubmit.
type submitPhase int

const (
	submitPhaseForm submitPhase = iota

	submitPhaseConfirm

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

// sortMode is the user-cyclable, descending-only sort key.
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

// hotRevealThreshold avoids showing Hot while almost every score is zero.
const hotRevealThreshold = 25

func (r Root) visibleSorts() []sortMode {
	if r.hotRevealed {
		return []sortMode{sortStarsDesc, sortRecencyDesc, sortHotDesc}
	}
	return []sortMode{sortStarsDesc, sortRecencyDesc}
}

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
	catalog *catalog.Catalog
	grid    grid
	sidebar sidebar
	search  textinput.Model
	readme  readmeModel
	focus   focusState
	mode    mode
	sort    sortMode

	modeState
	pkgState
	fixPathState
	launchState
	submitState
	manageState
	spinnerState

	// Gates the Hot sidebar row and sort step; New hides when Hot appears.
	hotRevealed bool
	layout      layoutMode
	width       int
	height      int
	ready       bool
	flashMsg    string
	flashExpiry time.Time

	installed    map[string]bool // repo -> installed, derived from $PATH via install.Detect
	binOverrides map[string]string
}

// modeState remembers where modal-style screens should return.
type modeState struct {
	helpReturnMode    mode
	installReturnMode mode
}

// pkgState is the shared install / uninstall / upgrade flow.
type pkgState struct {
	installCancel        context.CancelFunc // non-nil while an install/uninstall is running
	installLines         []string           // streamed output from the running op (source of truth)
	installViewport      viewport.Model     // derived view for scrolling logs
	installApp           *catalog.App
	installRes           *install.Result
	installOp            pkgOp
	installBootstrapping bool
	installBootstrapType string
	installRunningCmd    string
}

// fixPathState backs the post-install PATH follow-up flow.
type fixPathState struct {
	fixPlan           *pathfix.Plan
	fixErr            error
	fixApplied        bool
	fixAlreadyPresent bool
}

// launchState is detected once and reused on install/fix-PATH result screens.
type launchState struct {
	launchMethod launcher.Method
	launchErr    error
}

// submitState holds the three-phase `+` flow: form, confirm, opened.
type submitState struct {
	submitReturnMode mode
	submitPhase      submitPhase
	submitErr        error
	submitURL        string
	submitFields     submit.Request
	submitForm       *huh.Form
}

// manageState backs the installed-app action picker.
type manageState struct {
	manageActions []manageAction
	manageCursor  int
}

// spinnerState groups transient animation state.
type spinnerState struct {
	spinner spinner.Model

	// titlePhase drives the one-shot launch gradient.
	titlePhase float64
}

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
		catalog:      c,
		grid:         newGrid(),
		sidebar:      newSidebar(c, installed, false),
		search:       ti,
		installed:    installed,
		binOverrides: overrides,
		pkgState:     pkgState{installViewport: viewport.New(installLogWidth, installLogHeight)},
		launchState:  launchState{launchMethod: launcher.Detect(launcher.CurrentEnv())},
		spinnerState: spinnerState{spinner: sp},
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
