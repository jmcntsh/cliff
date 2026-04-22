package ui

import (
	"fmt"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/ui/theme"

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

// newSidebar builds the sidebar with All + Installed pinned at the top,
// then every catalog category in registry order. Installed's count is
// derived from the runtime install map and refreshed via setInstalled
// whenever the user installs or uninstalls something.
func newSidebar(c *catalog.Catalog, installed map[string]bool) sidebar {
	items := []sidebarItem{
		{name: "", count: len(c.Apps)},
		{name: categoryInstalled, count: len(installed)},
	}
	for _, cat := range c.Categories {
		items = append(items, sidebarItem{name: cat.Name, count: cat.Count})
	}
	return sidebar{items: items}
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

func (s sidebar) selected() string {
	return s.items[s.cursor].name
}

func (s sidebar) setFocused(b bool) sidebar {
	s.focused = b
	return s
}

func (s sidebar) update(msg tea.Msg) (sidebar, bool) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, false
	}
	prev := s.cursor
	switch key.String() {
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.items)-1 {
			s.cursor++
		}
	case "home", "g":
		s.cursor = 0
	case "end", "G":
		s.cursor = len(s.items) - 1
	}
	return s, s.cursor != prev
}

func (s sidebar) view(height int) string {
	var lines []string
	lines = append(lines, theme.TitleStyle.Render("CATEGORIES"))
	lines = append(lines, "")

	nameBudget := sidebarWidth - 2 // 2 cols for prefix; count appended after truncation
	for i, item := range s.items {
		name := item.name
		switch name {
		case "":
			name = "All"
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
		switch {
		case selected && s.focused:
			prefix = theme.SelectionPrefix
			style = lipgloss.NewStyle().Foreground(theme.ColorFocus).Bold(true)
		case selected:
			prefix = "▸ "
			style = lipgloss.NewStyle().Foreground(theme.ColorText)
		}
		lines = append(lines, prefix+style.Render(label))
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
