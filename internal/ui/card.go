package ui

import (
	"fmt"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func formatStars(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 10000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	case n < 1000000:
		return fmt.Sprintf("%dk", n/1000)
	default:
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
}

// renderCard draws one app card. The card is fixed-width so rows align;
// height is fixed at cardHeightCompact for unselected cards. Selected
// cards share the same outer height (so neighbors don't shift) but use
// a different border + accent treatment to stand out.
//
// focused tracks whether the grid is the active pane. When the user
// moves focus to the sidebar, the selection mark stays put (so they
// know where they'll land when they return) but drops the accent
// color: the thick border and the panel fill go away, and the border
// falls back to ColorText — "black on a light terminal, white on a
// dark one". This keeps the accent pink meaningful: it only appears
// where input is actually going.
func renderCard(app catalog.App, width, height int, selected, installed, focused bool) string {
	// Selection cues are stacked on purpose: shape (thick vs. rounded
	// border), color (accent vs. muted border), and a panel background
	// fill. Any one of these alone is easy to lose track of on a busy
	// grid or a low-contrast terminal; together they're unmissable
	// without making unselected cards look noisy.
	//
	// active means "selected AND the grid has focus" — this is the
	// fully-lit treatment. A merely-selected card (grid unfocused)
	// keeps the thick border shape but drops color and fill.
	active := selected && focused

	border := lipgloss.RoundedBorder()
	var borderColor lipgloss.TerminalColor = theme.ColorBorder
	switch {
	case active:
		border = lipgloss.ThickBorder()
		borderColor = theme.ColorAccent
	case selected:
		border = lipgloss.ThickBorder()
		borderColor = theme.ColorText
	}

	innerW := width - 2 // border on left+right
	if innerW < 1 {
		innerW = 1
	}

	name := app.Name
	if installed {
		name = "✓ " + name
	}
	nameStyle := lipgloss.NewStyle().Bold(true)
	if active {
		nameStyle = nameStyle.Foreground(theme.ColorAccent)
	} else {
		nameStyle = nameStyle.Foreground(theme.ColorText)
	}
	nameLine := nameStyle.Render(runewidth.Truncate(name, innerW, "…"))

	// Every styled chunk in the meta row needs the panel bg on an
	// active card. Each Render call emits a trailing ANSI reset that
	// drops the box's inherited Background for subsequent cells, so
	// separators and plain-text tails (like the language name) render
	// on terminal bg otherwise. applyBG threads the conditional once.
	applyBG := func(s lipgloss.Style) lipgloss.Style {
		if active {
			return s.Background(theme.ColorPanel)
		}
		return s
	}
	metaParts := []string{applyBG(theme.Stars).Render("★ " + formatStars(app.Stars))}
	if app.Language != "" {
		dotStyle := applyBG(lipgloss.NewStyle().Foreground(theme.LanguageColor(app.Language)))
		langNameStyle := applyBG(lipgloss.NewStyle())
		metaParts = append(metaParts, dotStyle.Render("●")+langNameStyle.Render(" "+app.Language))
	}
	sep := applyBG(lipgloss.NewStyle()).Render("  ")
	meta := strings.Join(metaParts, sep)

	var descColor lipgloss.TerminalColor = theme.ColorMuted
	if active {
		// Lift the description out of muted-grey on the active card so
		// the whole tile reads as "lit up", not just its frame. When
		// the grid is unfocused we leave the description muted — the
		// card shouldn't shout while input is going somewhere else.
		descColor = theme.ColorText
	}
	desc := wrapTextColored(app.Description, innerW, 2, descColor)

	bodyLines := []string{nameLine, meta}
	bodyLines = append(bodyLines, desc...)

	innerH := height - 2 // border top+bottom
	if innerH < 1 {
		innerH = 1
	}
	for len(bodyLines) < innerH {
		bodyLines = append(bodyLines, "")
	}
	if len(bodyLines) > innerH {
		bodyLines = bodyLines[:innerH]
	}

	body := strings.Join(bodyLines, "\n")

	box := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Width(innerW).
		Height(innerH)
	if active {
		// Background fill on the body and border catches peripheral
		// vision in a way color-only changes don't. Only applied when
		// the grid is the focused pane; a selected-but-unfocused card
		// keeps just the thick-border shape.
		box = box.Background(theme.ColorPanel).BorderBackground(theme.ColorPanel)
	}

	return box.Render(body)
}

// wrapTextColored word-wraps s into at most maxLines lines of width w,
// rendered in the given foreground color. The selected-card path lifts
// the description out of muted-grey by passing ColorText here; unselected
// cards pass ColorMuted.
func wrapTextColored(s string, w, maxLines int, fg lipgloss.TerminalColor) []string {
	if w <= 0 || maxLines <= 0 || s == "" {
		out := make([]string, maxLines)
		return out
	}
	words := strings.Fields(s)
	var lines []string
	var cur string
	for _, word := range words {
		if cur == "" {
			cur = word
			continue
		}
		if runewidth.StringWidth(cur)+1+runewidth.StringWidth(word) <= w {
			cur += " " + word
			continue
		}
		lines = append(lines, cur)
		cur = word
		if len(lines) == maxLines {
			break
		}
	}
	if len(lines) < maxLines && cur != "" {
		lines = append(lines, cur)
	}

	if len(lines) == maxLines {
		// We may have stopped early; if there were remaining words, mark with ellipsis.
		joined := strings.Join(lines, " ")
		if runewidth.StringWidth(joined) < runewidth.StringWidth(s) {
			last := lines[maxLines-1]
			lines[maxLines-1] = runewidth.Truncate(last, w, "…")
		}
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}

	style := lipgloss.NewStyle().Foreground(fg)
	for i := range lines {
		lines[i] = style.Render(lines[i])
	}
	return lines
}

