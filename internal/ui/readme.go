package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/catalog"
	rdm "github.com/jmcntsh/cliff/internal/readme"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

const readmeMaxContentWidth = 80
const (
	readmeReelPaneGap     = 2
	readmeMinContentWidth = 60
)

// darkBackground stores the terminal background polarity detected at
// startup. It's set by main via SetDarkBackground before any tea Program
// captures the terminal, because in-program OSC 11 queries are unreliable.
// Defaults to dark — matches the historical assumption.
var darkBackground = true

// SetDarkBackground configures the global terminal-background polarity.
// Called once from main(). Affects glamour style selection and forces
// lipgloss's HasDarkBackground() to agree, so AdaptiveColor pairs and
// the readme renderer pick the same palette.
func SetDarkBackground(dark bool) {
	darkBackground = dark
	lipgloss.SetHasDarkBackground(dark)
}

type readmeFetchedMsg struct {
	repo   string
	result rdm.Result
}

type readmeModel struct {
	app            *catalog.App
	raw            string
	viewport       viewport.Model
	contentWidth   int
	width          int
	height         int
	ready          bool
	reelRightPane  bool
	loading        bool
	rateLimited    bool
	rateLimitReset time.Time
	notFound       bool
	fetchErr       error
	fromCache      bool
	reel           reelStrip
	reelFetchCmd   tea.Cmd
}

func newReadme(app *catalog.App, width, height int) readmeModel {
	raw := placeholderMarkdown(app)
	reel, fetchCmd := newReelStripForApp(app.Name, width)
	m := readmeModel{
		app:          app,
		raw:          raw,
		loading:      true,
		reel:         reel,
		reelFetchCmd: fetchCmd,
	}
	return m.resize(width, height)
}

// ReelInit returns the tea.Cmd(s) that get the reel strip going.
// Two cases are batched together so callers don't have to know which
// applies:
//
//   - For the cliff app (embedded reel), the strip is already
//     populated; this returns the tick-loop start command.
//   - For everything else, the strip is empty and this returns the
//     background fetch command. The tick loop starts later when
//     applyReelFetched succeeds.
//
// Callers entering modeReadme should batch this with fetchReadmeCmd
// so the README and the reel both start loading the moment the user
// presses enter.
func (m readmeModel) ReelInit() tea.Cmd {
	tickCmd := m.reel.Init()
	if m.reelFetchCmd == nil {
		return tickCmd
	}
	if tickCmd == nil {
		return m.reelFetchCmd
	}
	return tea.Batch(tickCmd, m.reelFetchCmd)
}

// applyReelFetched routes a reelFetchedMsg to the strip and re-runs
// layout if the strip went from "not ready" to "ready" (so the
// viewport gives back rows to the now-visible strip). Returns the
// updated model and any tick command the strip needs to start
// animating.
func (m readmeModel) applyReelFetched(msg reelFetchedMsg) (readmeModel, tea.Cmd) {
	wasReady := m.reel.ready
	var cmd tea.Cmd
	m.reel, cmd = m.reel.applyReelFetched(msg)
	if !wasReady && m.reel.ready {
		m = m.resize(m.width, m.height)
	}
	return m, cmd
}

func fetchReadmeCmd(app *catalog.App) tea.Cmd {
	return func() tea.Msg {
		owner, repo := splitRepo(app.Repo)
		token := os.Getenv("GITHUB_TOKEN")
		result := rdm.Fetch(owner, repo, token)
		return readmeFetchedMsg{repo: app.Repo, result: result}
	}
}

func splitRepo(repo string) (string, string) {
	i := strings.Index(repo, "/")
	if i < 0 {
		return repo, ""
	}
	return repo[:i], repo[i+1:]
}

func (m readmeModel) applyFetch(msg readmeFetchedMsg) readmeModel {
	if m.app == nil || msg.repo != m.app.Repo {
		return m
	}
	m.loading = false
	r := msg.result
	switch {
	case r.Markdown != "":
		m.raw = r.Markdown
		m.fromCache = r.FromCache
		m.rateLimited = r.RateLimited
		m.rateLimitReset = r.ResetAt
	case r.NotFound:
		m.notFound = true
	case r.RateLimited:
		m.rateLimited = true
		m.rateLimitReset = r.ResetAt
	case r.Err != nil:
		m.fetchErr = r.Err
	}
	rendered := renderMarkdown(m.raw, m.contentWidth)
	m.viewport.SetContent(rendered)
	return m
}

func (m readmeModel) resize(width, height int) readmeModel {
	m.width = width
	m.height = height
	// Reserve 3 rows for header + footer + the separating blank line
	// between viewport/body and footer that JoinVertical produces at
	// these widths.
	bodyRows := max(height-3, 1)
	m.reelRightPane = m.canUseReelRightPane(width, bodyRows)

	if m.reelRightPane {
		reelW := m.reel.FramedWidth()
		m.contentWidth = max(width-reelW-readmeReelPaneGap, 20)
		// Keep the strip uncentered in right-pane mode.
		m.reel.width = reelW
		m.viewport = viewport.New(m.contentWidth, bodyRows)
	} else {
		reelRows := m.reel.Height()
		if reelRows > 0 {
			m.reel.width = width
		}
		m.contentWidth = width
		vpHeight := max(bodyRows-reelRows, 1)
		m.viewport = viewport.New(m.contentWidth, vpHeight)
	}
	rendered := renderMarkdown(m.raw, m.contentWidth)
	m.viewport.SetContent(rendered)
	m.ready = true
	return m
}

// scrollStep is how many lines a single up/down/j/k press moves the
// readme viewport. The bubbles viewport default is 1 line per press,
// which feels glacial on a 500-line README. 5 is a reasonable
// compromise — fast enough to skim, slow enough to land on a paragraph.
const scrollStep = 5

func (m readmeModel) Update(msg tea.Msg) (readmeModel, tea.Cmd) {
	// Reel tick messages go to the strip and nowhere else. Handle
	// them before the key/viewport dispatch so the animation keeps
	// running regardless of what else the readme model is doing.
	if _, isTick := msg.(reelTickMsg); isTick {
		var reelCmd tea.Cmd
		m.reel, reelCmd = m.reel.Update(msg)
		return m, reelCmd
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			m.viewport.LineUp(scrollStep)
			return m, nil
		case "down", "j":
			m.viewport.LineDown(scrollStep)
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m readmeModel) View() string {
	if !m.ready {
		return ""
	}
	header := m.renderHeader()
	footer := m.renderFooter()
	if m.reelRightPane {
		body := lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.viewport.View(),
			strings.Repeat(" ", readmeReelPaneGap),
			m.reel.View(),
		)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	}
	if m.reel.Height() > 0 {
		return lipgloss.JoinVertical(lipgloss.Left, header, m.reel.View(), m.viewport.View(), footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View(), footer)
}

func (m readmeModel) canUseReelRightPane(width, bodyRows int) bool {
	reelW := m.reel.FramedWidth()
	reelH := m.reel.Height()
	if reelW == 0 || reelH == 0 {
		return false
	}
	// Only split when both panes remain useful: README keeps a readable
	// column and the reel fits in the body area without forcing extra rows.
	return width-reelW-readmeReelPaneGap >= readmeMinContentWidth && reelH <= bodyRows
}

func (m readmeModel) renderHeader() string {
	if m.app == nil {
		return ""
	}
	back := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("◂ back")
	title := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render(m.app.Name + " · README")
	meta := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(
		fmt.Sprintf("★ %s · %s", formatStars(m.app.Stars), m.app.Language))

	left := back + "   " + title
	spacerW := m.contentWidth - lipgloss.Width(left) - lipgloss.Width(meta)
	if spacerW < 1 {
		spacerW = 1
	}
	return left + strings.Repeat(" ", spacerW) + meta
}

func (m readmeModel) renderFooter() string {
	pct := fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100)
	mutedStyle := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	scroll := mutedStyle.Render(pct)

	var status string
	switch {
	case m.loading:
		status = mutedStyle.Italic(true).Render("fetching from github…")
	case m.notFound:
		status = lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("no README found on github")
	case m.rateLimited && m.raw != "":
		status = mutedStyle.Italic(true).Render("github rate limited; showing cached/bundled")
	case m.rateLimited:
		reset := ""
		if !m.rateLimitReset.IsZero() {
			reset = fmt.Sprintf(" · resets %s", m.rateLimitReset.Format("15:04"))
		}
		status = lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("rate limited" + reset + " · set GITHUB_TOKEN")
	case m.fetchErr != nil:
		status = lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("fetch failed: " + m.fetchErr.Error())
	case m.fromCache:
		status = mutedStyle.Italic(true).Render("cached")
	}

	if status == "" {
		return scroll
	}
	spacer := m.contentWidth - lipgloss.Width(status) - lipgloss.Width(scroll)
	if spacer < 1 {
		spacer = 1
	}
	return status + strings.Repeat(" ", spacer) + scroll
}

// placeholderMarkdown is what shows while the live README is being
// fetched. It's deliberately thin: a name and a fetching note. The
// footer's "fetching from github…" / "rate limited" / "fetch failed"
// status line is the real signal; duplicating metadata the user just
// saw on the card would be noise, not content.
func placeholderMarkdown(app *catalog.App) string {
	if app == nil {
		return "# No app selected"
	}
	return "# " + app.Name + "\n\n*fetching from github…*\n"
}

func renderMarkdown(md string, termWidth int) string {
	wrap := readmeMaxContentWidth
	if termWidth-8 < wrap {
		wrap = max(termWidth-8, 20)
	}

	// We pre-detect the terminal background in main() before tea captures
	// the terminal — glamour's WithAutoStyle queries OSC 11 from inside
	// the renderer, which fails once tea is in raw mode + alt screen, so
	// it always falls back to dark. Pass the explicit style instead.
	style := "dark"
	if !darkBackground {
		style = "light"
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(wrap),
	)
	if err != nil {
		return md
	}
	out, err := renderer.Render(md)
	if err != nil {
		return md
	}

	if termWidth > wrap+4 {
		leftPad := (termWidth - wrap) / 2
		prefix := strings.Repeat(" ", leftPad)
		lines := strings.Split(out, "\n")
		for i, line := range lines {
			lines[i] = prefix + line
		}
		out = strings.Join(lines, "\n")
	}
	return out
}
