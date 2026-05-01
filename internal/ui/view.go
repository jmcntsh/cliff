package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/lipgloss"
)

func (r Root) View() string {
	if !r.ready {
		return ""
	}

	if r.mode == modeReadme {
		return r.readme.ViewWithSpinner(r.spinner.View()) + "\n" + r.footer()
	}

	contentH := r.height - 2
	if r.mode == modeSearch {
		contentH -= 3
	}
	contentH = max(contentH, 1)

	gridW, gridH := r.gridDimensions()

	var title string
	if r.focus == focusSidebar {
		title = theme.DimTitle.Render(r.computeTitle())
	} else {
		title = theme.GradientTitlePhase(r.computeTitle(), r.titlePhase)
	}

	gridBody := r.grid.View()
	if len(r.grid.apps) == 0 {
		gridBody = r.emptyGridView(gridW, gridH-2)
	}
	mainCol := lipgloss.NewStyle().Width(gridW).Render(title + "\n\n" + gridBody)

	var body string
	if r.layout == layoutNarrow {
		body = mainCol
		if r.mode == modeSidebarOverlay {
			body = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center, r.sidebar.viewOverlay())
		}
	} else {
		sidebarView := r.sidebar.view(contentH)
		sidebarBlock := lipgloss.NewStyle().Width(sidebarWidth).Render(sidebarView)
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebarBlock, " ", mainCol)
	}

	if r.mode == modeSearch {
		matches := fmt.Sprintf("%d matches", len(r.grid.apps))
		matchesRendered := theme.MutedItalic.Render(matches)

		searchView := r.search.View()
		spacerW := max(r.width-4-lipgloss.Width(searchView)-lipgloss.Width(matchesRendered), 1)
		content := searchView + strings.Repeat(" ", spacerW) + matchesRendered

		searchBar := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderTopForeground(theme.ColorAccent).
			BorderLeftForeground(theme.ColorAccent).
			BorderRightForeground(theme.ColorAccentMid).
			BorderBottomForeground(theme.ColorAccentAlt).
			Padding(0, 1).
			Render(content)
		body = searchBar + "\n" + body
	}

	if r.mode == modeHelp {
		body = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center, helpView(r.layout, r.helpReturnMode))
	}

	if r.mode == modePkgConfirm {
		body = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			pkgConfirmView(r.installApp, r.installOp, r.width))
	}
	if r.mode == modePkgRunning {
		body = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			pkgRunningView(r.installApp, r.installOp, r.installViewport, len(r.installLines) > 0, r.spinner.View(), r.width))
	}
	if r.mode == modePkgResult {
		body = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			pkgResultView(r.installRes, r.installOp, r.installViewport, r.launchMethod, r.launchErr, r.binOverrides, r.width))
	}
	if r.mode == modeManage {
		body = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			manageView(r.installApp, r.manageActions, r.manageCursor, r.width))
	}
	if r.mode == modeFixPath {
		body = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			fixPathView(r.fixPlan, r.fixErr, r.fixApplied, r.fixAlreadyPresent, r.installApp, r.launchMethod, r.launchErr, r.binOverrides, r.width))
	}
	if r.mode == modeSubmit {
		var content string
		switch r.submitPhase {
		case submitPhaseForm:
			if r.submitForm != nil {
				content = submitFormView(r.submitForm, r.width)
			}
		case submitPhaseConfirm:
			content = submitConfirmView(r.submitURL, r.width)
		case submitPhaseOpened:
			content = submitOpenedView(r.submitURL, r.submitErr, r.width)
		}
		body = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center, content)
	}

	return body + "\n" + r.footer()
}

func (r Root) computeTitle() string {
	cat := r.sidebar.selected()
	query := r.search.Value()
	total := len(r.grid.apps)

	var title string
	if total == 0 {
		title = "cliff · 0 apps"
	} else {
		title = fmt.Sprintf("cliff · %d / %d apps", r.grid.cursor+1, total)
	}
	if cat != "" {
		title += " · " + categoryDisplay(cat)
	}
	if query != "" {
		title += fmt.Sprintf(" · %q", query)
	} else {
		title += " · " + sortLabelFor(cat, r.sort)
	}
	return title
}

// categoryDisplay maps the internal category sentinels (__new__,
// __installed__) to the user-facing strings the sidebar uses, so the
// title bar doesn't leak implementation details. Real category names
// pass through unchanged.
func categoryDisplay(cat string) string {
	switch cat {
	case categoryNew:
		return "new"
	case categoryInstalled:
		return "installed"
	default:
		return cat
	}
}

// sortLabelFor returns the sort label that matches what the user is
// actually seeing in the grid. The New surface overrides the default
// stars-descending sort with a freshness sort (see filterAndSort), so
// the title bar must say "freshness ↓" in that one case to stay
// truthful. Any explicit sort toggle (stars ↑, name) wins, since the
// filter respects that too.
func sortLabelFor(cat string, sort sortMode) string {
	if cat == categoryNew && sort == sortStarsDesc {
		return "freshness ↓"
	}
	return sort.label()
}

func (r Root) emptyGridView(w, h int) string {
	msg := theme.MutedItalic.Render("No apps match these filters.")

	var hint string
	switch {
	case r.search.Value() != "":
		hint = "esc clear search"
	default:
		hint = "try a different category"
	}
	hintLine := theme.MutedText.Render(hint)
	// When the user is searching and has hit zero results, the most
	// useful next action is often "cliff should list what I was
	// looking for" — surface the submit flow right here rather than
	// relying on them finding `+` via help. Shown only in search
	// mode; for an empty category filter, "try a different category"
	// is the better prod.
	var submitLine string
	if r.search.Value() != "" {
		submitLine = theme.MutedText.Render("+ submit this app to cliff")
	}

	block := "  " + msg + "\n  " + hintLine
	if submitLine != "" {
		block += "\n  " + submitLine
	}
	return lipgloss.NewStyle().Width(w).Height(h).Render(block)
}

func (r Root) footer() string {
	// Footer hints lead with the action verbs you actually want to use
	// from this mode. Arrows + ⏎ + esc are universal and don't need to
	// be listed every time; ? always gets you the full reference.
	// When the selected app is already installed, swap `i install` for
	// `U update` — upgrade is the most common "do something with this
	// installed thing" action, so the direct keybind gets the footer
	// slot. Uninstall is reachable via ⏎ (manage picker) or `u`, but
	// showing all three verbs in the footer would be noisy; the help
	// overlay has the complete list.
	primaryVerb := "i install"
	enterVerb := "⏎ readme"
	if app := r.selectedApp(); app != nil && r.installed[app.Repo] {
		primaryVerb = "U update"
		enterVerb = "⏎ manage"
	}
	hints := "/ search · s sort · " + primaryVerb + " · " + enterVerb + " · ? help · q quit"
	if r.layout == layoutNarrow {
		hints = "/ search · c categories · " + primaryVerb + " · ? help · q quit"
	}
	switch r.mode {
	case modeSidebarOverlay:
		hints = "⏎ apply · esc close"
	case modeSearch:
		hints = "type to search · ↑↓←→ pick · ⏎ commit · esc cancel"
	case modeHelp:
		hints = "? or esc to close"
	case modeReadme:
		readmeVerb := "⏎ install"
		if app := r.selectedApp(); app != nil && r.installed[app.Repo] {
			readmeVerb = "⏎ manage · U update · u uninstall"
		}
		hints = readmeVerb + " · o github · ? help · ← back"
	case modePkgConfirm:
		hints = "⏎ run · esc cancel"
	case modePkgRunning:
		// Label the cancel with the op's verb so "esc" is unambiguous —
		// "cancel install" reads differently from "cancel uninstall,"
		// and we want the user to see exactly which child process
		// they're about to kill.
		hints = "↑↓ scroll logs  esc cancel " + strings.ToLower(r.installOp.verb())
	case modePkgResult:
		if r.installOp == pkgOpInstall && r.installRes != nil && r.installRes.Err == nil && r.installRes.PathWarning != nil {
			hints = "⏎ fix PATH · esc close"
		} else {
			hints = "↑↓/pgup/pgdn scroll logs  ⏎ or esc to close"
		}
	case modeManage:
		hints = "←→ move · ⏎ go · esc cancel"
	case modeFixPath:
		if r.fixApplied {
			hints = "⏎ or esc close"
		} else {
			hints = "⏎ apply · esc cancel"
		}
	case modeSubmit:
		switch r.submitPhase {
		case submitPhaseForm:
			hints = "tab/⏎ next field · ⏎ on last field to confirm · esc cancel"
		case submitPhaseConfirm:
			hints = "⏎ open in browser · esc cancel"
		case submitPhaseOpened:
			hints = "⏎ or esc close"
		}
	}
	if r.flashMsg != "" && time.Now().Before(r.flashExpiry) {
		// Even on flash, keep the brand mark anchored on the left so
		// the bottom row always reads as "this is cliff." The flash
		// message replaces the hints, not the wordmark.
		return theme.GradientTitlePhase("cliff", r.titlePhase) + theme.MutedText.Render("  ") + theme.AccentText.Render(r.flashMsg)
	}
	// Brand mark on the left, hint text on the right, separated by a
	// muted dot. The wordmark is the consistent "Charm app" tell;
	// every screen ends with it. The hint chunks are styled per-keycap
	// so the actionable letters pop in fuchsia and the descriptions
	// sit muted, which makes the footer scannable instead of a flat
	// gray ribbon.
	return theme.GradientTitlePhase("cliff", r.titlePhase) +
		theme.MutedText.Render("  ·  ") +
		styleHints(hints)
}

// styleHints renders a "·"-separated hint string with each chunk's
// keycap (the first whitespace-delimited token) in fuchsia bold and
// the rest of the chunk muted. Examples of inputs it handles:
//
//	"/ search"           → ⟦/⟧ search
//	"⏎ install"          → ⟦⏎⟧ install
//	"↑↓/pgup/pgdn scroll logs  ⏎ or esc to close"
//	                     → ⟦↑↓/pgup/pgdn⟧ scroll logs   ⟦⏎⟧ or esc to close
//	"type to search · ↑↓←→ pick · ⏎ commit · esc cancel"
//	                     → type to search · ⟦↑↓←→⟧ pick · ⟦⏎⟧ commit · ⟦esc⟧ cancel
//
// The split rule: each chunk is split on the first space, and the
// left side is treated as the keycap. Chunks without a space (rare,
// like a standalone phrase) render fully muted. The function is
// stateless and re-parses every render — cheap given hint strings
// are tens of bytes.
func styleHints(s string) string {
	const sep = " · "
	chunks := strings.Split(s, sep)
	for i, chunk := range chunks {
		chunks[i] = styleHintChunk(chunk)
	}
	return strings.Join(chunks, theme.MutedText.Render(sep))
}

func styleHintChunk(c string) string {
	// Some chunks have a double-space sub-separator (e.g. "↑↓
	// scroll logs  esc cancel install"). Recurse on those so each
	// half gets its own keycap styling.
	if i := strings.Index(c, "  "); i >= 0 {
		return styleHintChunk(c[:i]) + theme.MutedText.Render("  ") + styleHintChunk(c[i+2:])
	}

	// Chunks like "type to search" or "?" with no real keycap
	// fall back to all-muted. The detection rule is "first token
	// is shorter than the rest" — i.e. it really is a key, not the
	// start of a sentence.
	sp := strings.IndexByte(c, ' ')
	if sp <= 0 || sp >= len(c)-1 {
		return theme.MutedText.Render(c)
	}
	key := c[:sp]
	desc := c[sp:]

	// "type to search" starts with a long word that isn't a
	// keycap; exempt anything where the first token is > 4 chars
	// AND alphabetic. ⏎/↑↓/pgup/esc all stay short or contain
	// non-letters so they pass the keycap test.
	if len(key) > 4 && isAlpha(key) {
		return theme.MutedText.Render(c)
	}
	return theme.AccentBold.Render(key) + theme.MutedText.Render(desc)
}

func isAlpha(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}
