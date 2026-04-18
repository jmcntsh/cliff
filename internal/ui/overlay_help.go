package ui

import (
	"strings"

	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

type helpSection struct {
	title    string
	bindings []key.Binding
}

// The help overlay is laid out in two columns: MOVE + FIND on the left,
// DO + APP on the right. Sections lead with MOVE because the entire
// nav model — arrows go where you look, ⏎ goes deeper, ←/esc comes
// back — is the one thing users have to internalize. Everything else
// is a single-shot action they can look up.
var helpLeft = []helpSection{
	{"MOVE", []key.Binding{
		keys.Up, keys.Down, keys.Left, keys.Right,
		keys.Top, keys.Bottom, keys.PageUp, keys.PageDown,
		keys.Tab, keys.Enter, keys.Escape,
	}},
}

// helpRight is built dynamically because its contents depend on
// context: `c` categories only does anything in narrow layouts (the
// sidebar is always visible otherwise), and `o` open-on-github only
// does anything from inside the README view. Showing either one in
// contexts where it's a no-op misleads the user, so we hide them.
func helpRightFor(layout layoutMode, from mode) []helpSection {
	find := []key.Binding{keys.Search, keys.Sort}
	if layout == layoutNarrow {
		find = append(find, keys.Categories)
	}
	do := []key.Binding{keys.Install, keys.CopyInstall}
	if from == modeReadme {
		do = append(do, keys.OpenGithub)
	}
	return []helpSection{
		{"FIND", find},
		{"DO", do},
		{"APP", []key.Binding{keys.Help, keys.Quit}},
	}
}

func helpView(layout layoutMode, from mode) string {
	intro := theme.MutedText.Render(
		"arrows move where you look · ⏎ opens · ← or esc goes back",
	)

	left := renderHelpColumn(helpLeft)
	right := renderHelpColumn(helpRightFor(layout, from))
	cols := lipgloss.JoinHorizontal(lipgloss.Top, left, "    ", right)

	body := intro + "\n\n" + cols

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(1, 3).
		Render(body)
}

func renderHelpColumn(sections []helpSection) string {
	keyStyle := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Width(8)
	descStyle := theme.MutedText

	var blocks []string
	for _, sec := range sections {
		var lines []string
		lines = append(lines, theme.AccentBold.Render(sec.title))
		for _, b := range sec.bindings {
			h := b.Help()
			lines = append(lines, "  "+keyStyle.Render(h.Key)+descStyle.Render(h.Desc))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}
