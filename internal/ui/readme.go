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
	width          int
	height         int
	ready          bool
	loading        bool
	rateLimited    bool
	rateLimitReset time.Time
	notFound       bool
	fetchErr       error
	fromCache      bool
}

func newReadme(app *catalog.App, width, height int) readmeModel {
	raw := bundledMarkdown(app)
	m := readmeModel{app: app, raw: raw, loading: true}
	return m.resize(width, height)
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
	rendered := renderMarkdown(m.raw, m.width)
	m.viewport.SetContent(rendered)
	return m
}

func (m readmeModel) resize(width, height int) readmeModel {
	m.width = width
	m.height = height
	m.viewport = viewport.New(width, max(height-3, 1))
	rendered := renderMarkdown(m.raw, width)
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
	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View(), footer)
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
	spacerW := m.width - lipgloss.Width(left) - lipgloss.Width(meta)
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
	spacer := m.width - lipgloss.Width(status) - lipgloss.Width(scroll)
	if spacer < 1 {
		spacer = 1
	}
	return status + strings.Repeat(" ", spacer) + scroll
}

func bundledMarkdown(app *catalog.App) string {
	if app == nil {
		return "# No app selected"
	}
	var sb strings.Builder
	sb.WriteString("# " + app.Name + "\n\n")

	owner := app.Repo
	if i := strings.Index(app.Repo, "/"); i > 0 {
		owner = app.Repo[:i]
	}
	sb.WriteString("*by " + owner + "*\n\n")
	sb.WriteString("---\n\n")
	sb.WriteString(app.Description + "\n\n")

	sb.WriteString("## Metadata\n\n")
	sb.WriteString(fmt.Sprintf("- **Stars:** %d\n", app.Stars))
	if app.Language != "" {
		sb.WriteString(fmt.Sprintf("- **Language:** %s\n", app.Language))
	}
	if app.License != "" {
		sb.WriteString(fmt.Sprintf("- **License:** %s\n", app.License))
	}
	sb.WriteString("\n")

	if app.InstallSpec != nil {
		if cmd := app.InstallSpec.Shell(); cmd != "" {
			sb.WriteString("## Install\n\n")
			sb.WriteString(fmt.Sprintf("```sh\n# %s\n%s\n```\n\n", app.InstallSpec.Type, cmd))
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("GitHub: [github.com/" + app.Repo + "](https://github.com/" + app.Repo + ")\n\n")
	sb.WriteString("*Press `esc` or `q` to return to the list.*\n")
	return sb.String()
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
