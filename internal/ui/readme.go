package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/catalog"
	rdm "github.com/jmcntsh/cliff/internal/readme"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/bubbles/key"
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
	// Cached so reel ticks can refresh the viewport without re-running Glamour.
	renderedMarkdown string
}

func (m readmeModel) reelLoading() bool {
	return m.app != nil && !m.reel.ready
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

// ReelInit returns the command that starts an embedded reel or fetches a live one.
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

// applyReelFetched accepts only the reel for the current app and starts playback.
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

func (m readmeModel) applyFetch(msg readmeFetchedMsg) (readmeModel, tea.Cmd) {
	if m.app == nil || msg.repo != m.app.Repo {
		return m, nil
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
	m.renderedMarkdown = renderMarkdown(m.raw, m.contentWidth)
	m.refreshViewportContent()
	return m, nil
}

func (m readmeModel) resize(width, height int) readmeModel {
	m.width = width
	m.height = height
	bodyRows := max(height-3, 1)
	m.reelRightPane = m.canUseReelRightPane(width, bodyRows)

	if m.reelRightPane {
		reelW := m.reel.FramedWidth()
		m.contentWidth = max(width-reelW-readmeReelPaneGap, 20)
		m.reel.width = reelW
		m.viewport = viewport.New(m.contentWidth, bodyRows)
	} else {
		// In stacked mode the reel lives inside the scrollable content.
		if m.reel.Height() > 0 {
			m.reel.width = width
		}
		m.contentWidth = width
		m.viewport = viewport.New(m.contentWidth, bodyRows)
	}
	m.renderedMarkdown = renderMarkdown(m.raw, m.contentWidth)
	m.refreshViewportContent()
	m.ready = true
	return m
}

// refreshViewportContent rebuilds content while preserving scroll offset.
func (m *readmeModel) refreshViewportContent() {
	content := m.renderedMarkdown
	if !m.reelRightPane && m.reel.Height() > 0 {
		content = m.reel.View() + "\n" + m.renderedMarkdown
	}
	yOff := m.viewport.YOffset
	m.viewport.SetContent(content)
	maxOff := max(m.viewport.TotalLineCount()-m.viewport.Height, 0)
	if yOff > maxOff {
		yOff = maxOff
	}
	m.viewport.SetYOffset(yOff)
}

// scrollStep is how many lines a single up/down/j/k press moves the
// readme viewport. The bubbles viewport default is 1 line per press,
// which feels glacial on a 500-line README. 5 is a reasonable
// compromise — fast enough to skim, slow enough to land on a paragraph.
const scrollStep = 5

func (m readmeModel) Update(msg tea.Msg) (readmeModel, tea.Cmd) {
	// Stacked reels are part of the viewport, so each tick refreshes content.
	if _, isTick := msg.(reelTickMsg); isTick {
		var reelCmd tea.Cmd
		m.reel, reelCmd = m.reel.Update(msg)
		if !m.reelRightPane && m.reel.Height() > 0 {
			m.refreshViewportContent()
		}
		return m, reelCmd
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, keys.Up):
			m.viewport.LineUp(scrollStep)
			return m, nil
		case key.Matches(km, keys.Down):
			m.viewport.LineDown(scrollStep)
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the readme without a live spinner.
func (m readmeModel) View() string {
	return m.ViewWithSpinner("")
}

// ViewWithSpinner is the live-render path used by Root.
func (m readmeModel) ViewWithSpinner(spinnerGlyph string) string {
	if !m.ready {
		return ""
	}
	header := m.renderHeader()
	footer := m.renderFooterWithSpinner(spinnerGlyph)
	if m.reelRightPane {
		body := lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.viewport.View(),
			strings.Repeat(" ", readmeReelPaneGap),
			m.reel.View(),
		)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
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
	back := theme.MutedText.Render("◂ back")
	title := theme.GradientTitle(m.app.Name + " · README")
	meta := theme.MutedText.Render(
		fmt.Sprintf("★ %s · %s", formatStars(m.app.Stars), m.app.Language))

	left := back + "   " + title
	spacerW := m.contentWidth - lipgloss.Width(left) - lipgloss.Width(meta)
	if spacerW < 1 {
		spacerW = 1
	}
	return left + strings.Repeat(" ", spacerW) + meta
}

func (m readmeModel) renderFooter() string {
	return m.renderFooterWithSpinner("")
}

func (m readmeModel) renderFooterWithSpinner(spinnerGlyph string) string {
	scroll := theme.MutedText.Render(fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100))

	var status string
	switch {
	case m.loading:
		label := "fetching from github…"
		if spinnerGlyph != "" {
			label = spinnerGlyph + " " + label
		}
		status = theme.MutedItalic.Render(label)
	case m.notFound:
		status = theme.MutedText.Render("no README found on github")
	case m.rateLimited && m.raw != "":
		status = theme.MutedItalic.Render("github rate limited; showing cached/bundled")
	case m.rateLimited:
		reset := ""
		if !m.rateLimitReset.IsZero() {
			reset = fmt.Sprintf(" · resets %s", m.rateLimitReset.Format("15:04"))
		}
		status = theme.MutedText.Render("rate limited" + reset + " · set GITHUB_TOKEN")
	case m.fetchErr != nil:
		status = theme.MutedText.Render("fetch failed: " + m.fetchErr.Error())
	case m.fromCache:
		status = theme.MutedItalic.Render("cached")
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

// placeholderMarkdown is the thin body shown while the live README loads.
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

	// Use the background detected before tea captured the terminal.
	style := "pink"
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
