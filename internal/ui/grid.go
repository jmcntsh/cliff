package ui

import (
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"

	"github.com/charmbracelet/lipgloss"
)

// grid is the tiled card view that replaces the row-based bubbles/list.
// It owns the visible app slice, the cursor (single index into apps),
// and a viewport (topRow). The layout decides cols/rows/cardSize each
// frame based on the available width and height.
type grid struct {
	apps      []catalog.App
	installed map[string]bool

	cursor  int // index into apps; clamped to [0, len(apps)-1]
	topRow  int // first visible row of cards

	cols       int
	rows       int
	cardWidth  int
	cardHeight int
}

func newGrid() grid { return grid{} }

func (g grid) setApps(apps []catalog.App, installed map[string]bool) grid {
	g.apps = apps
	g.installed = installed
	if g.cursor >= len(apps) {
		g.cursor = max(len(apps)-1, 0)
	}
	g.ensureCursorVisible()
	return g
}

// setLayout recomputes cols/rows/card size from the available viewport.
// It does not change the cursor — only adjusts viewport so the cursor
// stays visible after a resize.
func (g grid) setLayout(width, height int) grid {
	g.cols, g.cardWidth = pickCols(width)
	g.cardHeight = cardHeightCompact // selected card overflows visually but stays in its slot
	g.rows = max(height/(g.cardHeight+cardVGap), 1)
	g.ensureCursorVisible()
	return g
}

func (g grid) cursorRowCol() (row, col int) {
	if g.cols == 0 {
		return 0, 0
	}
	return g.cursor / g.cols, g.cursor % g.cols
}

func (g grid) ensureCursorVisible() grid {
	if g.cols == 0 || g.rows == 0 {
		return g
	}
	row, _ := g.cursorRowCol()
	if row < g.topRow {
		g.topRow = row
	}
	if row >= g.topRow+g.rows {
		g.topRow = row - g.rows + 1
	}
	if g.topRow < 0 {
		g.topRow = 0
	}
	return g
}

func (g grid) selected() *catalog.App {
	if g.cursor < 0 || g.cursor >= len(g.apps) {
		return nil
	}
	return &g.apps[g.cursor]
}

// move applies a (drow, dcol) delta to the cursor with bounds checking.
// Out-of-bounds moves are clamped, not wrapped — wrapping in a 2D grid
// is more disorienting than helpful.
func (g grid) move(drow, dcol int) grid {
	if len(g.apps) == 0 || g.cols == 0 {
		return g
	}
	row, col := g.cursorRowCol()
	row += drow
	col += dcol

	if col < 0 {
		col = 0
	}
	if col >= g.cols {
		col = g.cols - 1
	}
	if row < 0 {
		row = 0
	}
	maxRow := (len(g.apps) - 1) / g.cols
	if row > maxRow {
		row = maxRow
	}

	idx := row*g.cols + col
	if idx >= len(g.apps) {
		idx = len(g.apps) - 1
	}
	g.cursor = idx
	return g.ensureCursorVisible()
}

func (g grid) jumpTop() grid {
	g.cursor = 0
	g.topRow = 0
	return g
}

func (g grid) jumpBottom() grid {
	g.cursor = max(len(g.apps)-1, 0)
	return g.ensureCursorVisible()
}

func (g grid) pageDown() grid { return g.move(g.rows, 0) }
func (g grid) pageUp() grid   { return g.move(-g.rows, 0) }

// selectByRepo restores selection after a refilter when the previous
// repo is still present; otherwise resets to the top of the new list.
// Falling through without a fallback leaves the cursor at whatever
// setApps clamped it to (len-1), which shows up as "switching category
// lands you at the bottom" — confusing UX.
func (g grid) selectByRepo(repo string) grid {
	if repo == "" {
		return g.jumpTop()
	}
	for i, a := range g.apps {
		if a.Repo == repo {
			g.cursor = i
			return g.ensureCursorVisible()
		}
	}
	return g.jumpTop()
}

func (g grid) View() string {
	if len(g.apps) == 0 {
		return ""
	}
	if g.cols == 0 || g.rows == 0 {
		return ""
	}

	var rows []string
	end := g.topRow + g.rows
	maxRow := (len(g.apps) - 1) / g.cols
	if end > maxRow+1 {
		end = maxRow + 1
	}

	for row := g.topRow; row < end; row++ {
		var rowCards []string
		for col := 0; col < g.cols; col++ {
			idx := row*g.cols + col
			if idx >= len(g.apps) {
				rowCards = append(rowCards, lipgloss.NewStyle().Width(g.cardWidth).Render(""))
				continue
			}
			app := g.apps[idx]
			selected := idx == g.cursor
			installed := g.installed[app.Repo]
			rowCards = append(rowCards, renderCard(app, g.cardWidth, g.cardHeight, selected, installed))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, joinWithGap(rowCards, cardHGap)...))
	}

	return strings.Join(rows, strings.Repeat("\n", cardVGap+1))
}

func joinWithGap(parts []string, gap int) []string {
	if len(parts) <= 1 || gap <= 0 {
		return parts
	}
	out := make([]string, 0, len(parts)*2-1)
	gapStr := strings.Repeat(" ", gap)
	for i, p := range parts {
		if i > 0 {
			out = append(out, gapStr)
		}
		out = append(out, p)
	}
	return out
}

// pickCols returns (number of columns, width per card) for the given
// total width. The card width target is ~32 cols including borders;
// we pack as many as fit with a 1-col gap between them, falling back
// to 1 column on narrow terminals.
func pickCols(width int) (cols, cardW int) {
	if width <= 0 {
		return 1, 1
	}
	const ideal = 34
	const minCard = 28
	cols = (width + cardHGap) / (ideal + cardHGap)
	if cols < 1 {
		cols = 1
	}
	cardW = (width - cardHGap*(cols-1)) / cols
	if cardW < minCard && cols > 1 {
		cols--
		cardW = (width - cardHGap*(cols-1)) / cols
	}
	if cardW < 1 {
		cardW = 1
	}
	return cols, cardW
}

const (
	cardHGap          = 1
	cardVGap          = 0
	cardHeightCompact = 7 // border(2) + name(1) + meta(1) + desc(2) + footer(1)
)
