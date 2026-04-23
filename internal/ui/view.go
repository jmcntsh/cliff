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
		return r.readme.View() + "\n" + r.footer()
	}

	contentH := r.height - 2
	if r.mode == modeSearch {
		contentH -= 3
	}
	contentH = max(contentH, 1)

	gridW, gridH := r.gridDimensions()

	titleStyle := theme.TitleStyle
	if r.focus == focusSidebar {
		titleStyle = theme.DimTitle
	}
	title := titleStyle.Render(r.computeTitle())

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
			BorderForeground(theme.ColorAccent).
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
			pkgRunningView(r.installApp, r.installOp, r.installViewport, len(r.installLines) > 0, r.width))
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
		body = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			submitView(r.submitURL, r.submitOpened, r.submitErr, r.width))
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
		title += " · " + cat
	}
	if query != "" {
		title += fmt.Sprintf(" · %q", query)
	} else {
		title += " · " + r.sort.label()
	}
	return title
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
		if r.submitOpened {
			hints = "⏎ or esc close"
		} else {
			hints = "⏎ open in browser · esc cancel"
		}
	}
	if r.flashMsg != "" && time.Now().Before(r.flashExpiry) {
		return theme.AccentText.Render(r.flashMsg)
	}
	return theme.MutedText.Render(hints)
}
