package ui

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/ui/theme"
	"github.com/jmcntsh/reel/format"
	"github.com/jmcntsh/reel/player"
	"github.com/jmcntsh/reel/screen"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// cliffdemoReelBytes is the hand-recorded tour of cliff that plays
// above the cliff entry's own readme view. Embedded rather than
// fetched so the very first readme a new user lands on renders
// instantly with no network call and no hosting decision in our way.
// When other apps get reels, they'll come via URL in the manifest —
// see notes on the "author-hosted" plan — and the embed will stay as
// a cliff-specific special case, not a general mechanism.
//
//go:embed assets/cliffdemo.reel
var cliffdemoReelBytes []byte

// reelTickInterval paces the player's internal clock. 16ms ≈ 60Hz,
// the same cadence reel's recorder samples at, so we reproduce the
// author's timing faithfully without over-drawing.
const reelTickInterval = 16 * time.Millisecond

// reelTickMsg is the bubbletea message that wakes the player up on
// every frame. We carry the wall-clock time so Advance gets a real
// dt (tea.Tick fires on a best-effort basis; a jittery host should
// not produce jittery playback).
type reelTickMsg time.Time

// reelStrip is a small tea-model wrapper around reel.Player. It owns
// the player state and the tick schedule; the readme view owns the
// strip's layout position above the markdown viewport.
//
// Zero value is "no reel to play" — all methods tolerate it, which
// keeps the readme view's conditional rendering simple (just always
// call View; an empty strip draws nothing and occupies zero rows).
type reelStrip struct {
	player   *player.Player
	lastTick time.Time
	width    int
	ready    bool
}

// newReelStripForApp returns a strip populated for apps we have an
// embedded reel for, or a zero-value strip otherwise. Today that's
// exactly one app: cliff itself. Anything else renders no strip.
// The readme view's layout math treats a zero strip as occupying
// zero rows, so non-cliff readmes are visually unchanged.
func newReelStripForApp(appName string, width int) reelStrip {
	if appName != "cliff" || len(cliffdemoReelBytes) == 0 {
		return reelStrip{}
	}
	r, err := format.Decode(strings.NewReader(string(cliffdemoReelBytes)))
	if err != nil {
		// Swallow: a broken embedded reel is a bug in the binary,
		// not a runtime problem the user can act on. Silently
		// falling back to "no strip" keeps the readme usable.
		return reelStrip{}
	}
	return reelStrip{
		player:   player.New(r),
		lastTick: time.Now(),
		width:    width,
		ready:    true,
	}
}

// reelBorderRows is how many rows the framing border adds on top of
// the reel's own height (one for the top edge, one for the bottom).
// Pulled out as a constant so Height() and the layout math in
// readme.go can't drift apart — if the border style ever grows a
// padding row, both callers update together.
const reelBorderRows = 2

// Height returns the row count the strip will occupy when rendered,
// including the framing border. Callers use this to shrink the
// adjacent viewport. Zero means the strip renders nothing and should
// be omitted from layout entirely.
func (s reelStrip) Height() int {
	if !s.ready || s.player == nil {
		return 0
	}
	_, rows := s.player.Size()
	return rows + reelBorderRows
}

// Init returns the first tick command. Must be wired into the
// host's tea.Cmd chain when the readme view is entered — the strip
// won't animate without it.
func (s reelStrip) Init() tea.Cmd {
	if !s.ready {
		return nil
	}
	return tickReel()
}

// Update handles a single reelTickMsg: advances the player by the
// real elapsed time since the last tick and schedules the next one.
// Other message types are ignored. Returns the updated strip and
// the next tick command.
func (s reelStrip) Update(msg tea.Msg) (reelStrip, tea.Cmd) {
	if !s.ready {
		return s, nil
	}
	tick, ok := msg.(reelTickMsg)
	if !ok {
		return s, nil
	}
	now := time.Time(tick)
	dt := now.Sub(s.lastTick)
	s.lastTick = now
	s.player.Advance(dt)
	return s, tickReel()
}

// tickReel schedules the next animation frame. Using tea.Tick (rather
// than a self-rescheduling goroutine) keeps the strip cooperative
// with bubbletea's model and avoids a separate lifecycle to manage.
func tickReel() tea.Cmd {
	return tea.Tick(reelTickInterval, func(t time.Time) tea.Msg {
		return reelTickMsg(t)
	})
}

// View renders the current screen state as a block of styled text,
// wrapped in a subtle rounded border so the reel reads as a framed
// preview rather than bleeding into the surrounding chrome. Returns
// an empty string when there's nothing to draw, which
// lipgloss.JoinVertical handles by just skipping the line.
//
// Centering: the border wraps at the reel's native width + 2 for
// the side edges. If the host terminal is wider, we left-pad the
// whole framed block to match the markdown body's centering in
// readme.go so the reel and the README column share a visual axis.
func (s reelStrip) View() string {
	if !s.ready || s.player == nil {
		return ""
	}
	sc := s.player.Screen()
	cols, rows := s.player.Size()
	var out strings.Builder
	for row := 0; row < rows; row++ {
		renderRow(&out, sc, cols, row)
		if row < rows-1 {
			out.WriteByte('\n')
		}
	}
	// Wrap in a rounded border. Width is set to the reel's native
	// col count so the border's right edge sits exactly one cell
	// past the last painted cell — no stretch, no gap. The muted
	// border color keeps the frame visually subordinate to the
	// reel's own content.
	framed := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Width(cols).
		Render(out.String())

	framedWidth := cols + 2 // border adds 1 col on each side
	if s.width > framedWidth+2 {
		leftPad := (s.width - framedWidth) / 2
		prefix := strings.Repeat(" ", leftPad)
		lines := strings.Split(framed, "\n")
		for i := range lines {
			lines[i] = prefix + lines[i]
		}
		framed = strings.Join(lines, "\n")
	}
	return framed
}

// renderRow walks one row of the screen and coalesces same-styled
// runs of cells into a single lipgloss.Render call. Cell-by-cell
// would also work at these sizes (80x24 is ~2000 cells), but the
// escape-sequence overhead of one Render per cell is ugly in the
// output and makes bubbletea's diff pipeline work harder than it
// needs to.
func renderRow(out *strings.Builder, sc *screen.Screen, cols, row int) {
	if cols == 0 {
		return
	}
	runStart := 0
	runStyle := cellStyle(sc.At(0, row))
	var runText strings.Builder
	runText.WriteRune(safeRune(sc.At(0, row).R))
	for col := 1; col < cols; col++ {
		cell := sc.At(col, row)
		style := cellStyle(cell)
		if style == runStyle {
			runText.WriteRune(safeRune(cell.R))
			continue
		}
		out.WriteString(applyStyle(runStyle, runText.String()))
		runStart = col
		runStyle = style
		runText.Reset()
		runText.WriteRune(safeRune(cell.R))
	}
	_ = runStart
	out.WriteString(applyStyle(runStyle, runText.String()))
}

// safeRune replaces the zero rune with a space. The screen package
// stores blank cells as rune 0 in practice for cells never painted;
// writing a literal NUL to the terminal is at best invisible and at
// worst confuses line-oriented renderers.
func safeRune(r rune) rune {
	if r == 0 {
		return ' '
	}
	return r
}

// cellStyleKey is a compact comparable representation of a cell's
// visual style. We key runs by this string so "same style" can be
// checked with ==, which is cheaper than deep-equalling a lipgloss
// style struct. The format is intentionally lossy in one direction
// only: two cells with the same key render identically, and that's
// the only property renderRow relies on.
type cellStyleKey string

func cellStyle(c screen.Cell) cellStyleKey {
	var b strings.Builder
	b.WriteString(colorKey(c.Fg))
	b.WriteByte('|')
	b.WriteString(colorKey(c.Bg))
	b.WriteByte('|')
	if c.Bold {
		b.WriteByte('B')
	}
	if c.Italic {
		b.WriteByte('I')
	}
	if c.Underline {
		b.WriteByte('U')
	}
	if c.Reverse {
		b.WriteByte('R')
	}
	return cellStyleKey(b.String())
}

func colorKey(c format.Color) string {
	switch c.Kind {
	case format.ColorDefault:
		return "d"
	case format.ColorANSI:
		return fmt.Sprintf("a%d", c.N)
	case format.ColorANSI256:
		return fmt.Sprintf("x%d", c.N)
	case format.ColorRGB:
		return fmt.Sprintf("r%d,%d,%d", c.R, c.G, c.B)
	}
	return "d"
}

// applyStyle builds a lipgloss style for the given key and renders
// text through it. We rebuild the lipgloss.Style from the key on
// every run rather than caching, because the key set per row is
// small in practice (a handful of colors/attributes, not 2000).
// Caching would be a premature optimization and would need
// invalidation on theme changes.
func applyStyle(key cellStyleKey, text string) string {
	style := lipgloss.NewStyle()
	parts := strings.SplitN(string(key), "|", 3)
	if len(parts) == 3 {
		if c := colorFromKey(parts[0]); c != nil {
			style = style.Foreground(*c)
		}
		if c := colorFromKey(parts[1]); c != nil {
			style = style.Background(*c)
		}
		attrs := parts[2]
		if strings.ContainsRune(attrs, 'B') {
			style = style.Bold(true)
		}
		if strings.ContainsRune(attrs, 'I') {
			style = style.Italic(true)
		}
		if strings.ContainsRune(attrs, 'U') {
			style = style.Underline(true)
		}
		if strings.ContainsRune(attrs, 'R') {
			style = style.Reverse(true)
		}
	}
	return style.Render(text)
}

// colorFromKey reverses colorKey back to a lipgloss color. Default
// returns nil (caller skips applying any color, inheriting whatever
// the terminal's default is — which is exactly what we want for
// "the recording's palette should stay slots").
func colorFromKey(s string) *lipgloss.Color {
	if s == "" || s == "d" {
		return nil
	}
	switch s[0] {
	case 'a':
		c := lipgloss.Color(fmt.Sprintf("%d", ansiToLipglossN(s[1:])))
		return &c
	case 'x':
		c := lipgloss.Color(s[1:])
		return &c
	case 'r':
		c := lipgloss.Color("#" + rgbHex(s[1:]))
		return &c
	}
	return nil
}

// ansiToLipglossN parses the N from an ANSI-palette key ("a0".."a15")
// back to the integer slot. lipgloss.Color accepts ANSI slots as a
// decimal string, so the mapping is just atoi.
func ansiToLipglossN(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// rgbHex parses "r,g,b" decimal components back to a 6-digit hex
// string. Used only for true-color cells, which are rare in most
// terminal UIs but common in things like btop and charm TUIs.
func rgbHex(s string) string {
	parts := strings.Split(s, ",")
	if len(parts) != 3 {
		return "000000"
	}
	r := atoi(parts[0])
	g := atoi(parts[1])
	b := atoi(parts[2])
	return fmt.Sprintf("%02x%02x%02x", r&0xff, g&0xff, b&0xff)
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
