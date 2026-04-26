package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

type sidebarItem struct {
	name  string // "" means "All", categoryInstalled means "Installed"
	count int
}

type sidebar struct {
	items   []sidebarItem
	cursor  int
	focused bool
}

// newSidebar builds the sidebar with All, New, and Installed pinned at
// the top, then every catalog category in registry order. The New and
// Installed counts are derived at runtime — New from FreshnessTime,
// Installed from the live install map — and refreshed via setInstalled
// / setNewCount whenever the inputs change.
func newSidebar(c *catalog.Catalog, installed map[string]bool) sidebar {
	items := []sidebarItem{
		{name: "", count: len(c.Apps)},
		{name: categoryNew, count: countNew(c.Apps, time.Now())},
		{name: categoryInstalled, count: len(installed)},
	}
	for _, cat := range c.Categories {
		items = append(items, sidebarItem{name: cat.Name, count: cat.Count})
	}
	return sidebar{items: items}
}

// countNew returns how many apps currently qualify as New relative to
// `now`. Kept as a package-level helper so newSidebar and setNewCount
// agree on the rule (ultimately newSet's rule, which enforces the
// fallback cap — so countNew is exactly len(newSet)).
func countNew(apps []catalog.App, now time.Time) int {
	return len(newSet(apps, now))
}

// setInstalled refreshes the Installed pseudo-category's count after an
// install or uninstall. No-op if there is no Installed row (shouldn't
// happen post-newSidebar, but cheap to guard).
func (s sidebar) setInstalled(installed map[string]bool) sidebar {
	for i, item := range s.items {
		if item.name == categoryInstalled {
			s.items[i].count = len(installed)
			break
		}
	}
	return s
}

// setNewCount refreshes the New pseudo-category's count. Called at
// startup (via newSidebar) and if we ever reload the catalog mid-
// session. Not wired into any per-keypress path: the window is 7 days,
// so the count is stable for the lifetime of a normal cliff session.
func (s sidebar) setNewCount(n int) sidebar {
	for i, item := range s.items {
		if item.name == categoryNew {
			s.items[i].count = n
			break
		}
	}
	return s
}

func (s sidebar) selected() string {
	return s.items[s.cursor].name
}

func (s sidebar) setFocused(b bool) sidebar {
	s.focused = b
	return s
}

func (s sidebar) update(msg tea.Msg) (sidebar, bool) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, false
	}
	prev := s.cursor
	switch {
	case key.Matches(km, keys.Up):
		if s.cursor > 0 {
			s.cursor--
		}
	case key.Matches(km, keys.Down):
		if s.cursor < len(s.items)-1 {
			s.cursor++
		}
	case key.Matches(km, keys.Top):
		s.cursor = 0
	case key.Matches(km, keys.Bottom):
		s.cursor = len(s.items) - 1
	}
	return s, s.cursor != prev
}

func (s sidebar) view(height int) string {
	var lines []string
	// Header uses the gradient brand-mark treatment so the sidebar
	// reads as a labeled section sharing the title bar's brand
	// language. When focused, an accent-color underline-style
	// indicator marks which pane has input.
	header := theme.GradientTitle("CATEGORIES")
	if s.focused {
		header = lipgloss.NewStyle().
			Background(theme.ColorPanel).
			Padding(0, 1).
			Render(header)
	}
	lines = append(lines, header)
	lines = append(lines, "")

	nameBudget := sidebarWidth - 2 // 2 cols for prefix; count appended after truncation
	for i, item := range s.items {
		name := item.name
		switch name {
		case "":
			name = "All"
		case categoryNew:
			name = "New"
		case categoryInstalled:
			name = "Installed"
		}
		countStr := fmt.Sprintf(" (%d)", item.count)
		nameMax := nameBudget - runewidth.StringWidth(countStr)
		if nameMax < 3 {
			nameMax = 3
		}
		label := runewidth.Truncate(name, nameMax, "…") + countStr

		selected := i == s.cursor
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(theme.ColorMuted)

		var rendered string
		switch {
		case selected && s.focused:
			// Focused+selected gets the gradient treatment so the
			// active sidebar row matches the title bar and the
			// active card visually. The panel-background bar
			// extends across the full sidebar width as a
			// peripheral-vision cue, with the gradient label
			// painted on top.
			prefix = theme.SelectionPrefix
			barStyle := lipgloss.NewStyle().
				Background(theme.ColorPanel).
				Width(nameBudget)
			rendered = prefix + barStyle.Render(theme.GradientTitle(label))
		case selected:
			prefix = "▸ "
			style = lipgloss.NewStyle().Foreground(theme.ColorText)
			rendered = prefix + style.Render(label)
		default:
			rendered = prefix + style.Render(label)
		}
		lines = append(lines, rendered)
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (s sidebar) viewOverlay() string {
	body := s.view(len(s.items) + 3)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 2).
		Width(sidebarWidth + 4)
	return box.Render(body)
}
