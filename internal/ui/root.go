package ui

import (
	"context"
	"time"

	"github.com/jmcntsh/cliff/internal/binmap"
	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/install"
	"github.com/jmcntsh/cliff/internal/launcher"
	"github.com/jmcntsh/cliff/internal/pathfix"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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

type sortMode int

const (
	sortStarsDesc sortMode = iota
	sortStarsAsc
	sortName
)

func (s sortMode) label() string {
	switch s {
	case sortStarsDesc:
		return "stars ↓"
	case sortStarsAsc:
		return "stars ↑"
	case sortName:
		return "name"
	}
	return ""
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
	// closes. submitReturnMode is the mode we bounce back to on esc
	// (browse or readme, depending on where `+` was pressed);
	// submitOpened flips after a successful browser.Open so the modal
	// can show the "opened in your browser" confirmation rather than
	// the initial "about to open" preview, without switching modes.
	// submitErr holds the browser.Open error so the post-open phase
	// can fall back to showing the URL for manual paste.
	submitReturnMode mode
	submitOpened     bool
	submitErr        error
	submitURL        string

	// Manage-picker state for modeManage. Populated when Enter is
	// pressed on an installed app; emptied when the picker closes.
	// The picker is a horizontal row of actions (Update / Uninstall /
	// Readme), with Update default-selected because it's the most
	// common "what do I want to do with this installed thing" and is
	// benign. Uninstall is destructive so never default; Readme is
	// the escape hatch for "I meant to re-read docs, not manage."
	manageActions []manageAction
	manageCursor  int
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

	overrides := binmap.Load()
	installed := install.InstalledAppsWithOverrides(c.Apps, overrides)
	r := Root{
		catalog:         c,
		grid:            newGrid(),
		sidebar:         newSidebar(c, installed),
		search:          ti,
		installed:       installed,
		binOverrides:    overrides,
		installViewport: viewport.New(installLogWidth, installLogHeight),
		launchMethod:    launcher.Detect(launcher.CurrentEnv()),
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

func (r Root) Init() tea.Cmd { return nil }

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
	gridH := max(r.height-2, 1)
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
