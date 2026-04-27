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
	hero           heroImage
	// renderedMarkdown is the glamour-rendered, hero-spliced markdown
	// body cached between SetContent calls. In stacked-mode with a
	// reel, every reel tick re-prepends the live reel.View() to this
	// cached body and SetContents the result; caching avoids re-running
	// glamour 60 times a second for content that hasn't changed.
	renderedMarkdown string
}

// reelLoading reports whether the reel strip is still being fetched.
// Used by the Root spinner to decide whether to keep ticking. The
// embedded "cliff" reel is ready synchronously, so this only fires
// for live registry fetches; once applyReelFetched populates the
// strip and flips reel.ready, the spinner stops.
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
		hero:         heroImage{slug: app.Name, width: width},
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
	m.renderedMarkdown = m.hero.spliceInline(renderMarkdown(m.raw, m.hero.refRaw, m.contentWidth))
	m.refreshViewportContent()

	// Look for a hero image once we have real markdown. The hero
	// fetches in the background; when it lands, applyHeroFetched
	// re-renders with the rendered ANSI block spliced in at the
	// markdown URL's position. Reel and hero coexist now — reel
	// keeps its slot above (or to the right of) the viewport, hero
	// lives inline within the viewport content.
	var heroCmd tea.Cmd
	if r.Markdown != "" && !m.hero.ready {
		var newHero heroImage
		newHero, heroCmd = newHeroFromMarkdown(m.app.Name, m.raw, m.app.Readme, m.contentWidth)
		// Only replace the existing hero if we actually launched a
		// fetch — otherwise keep the zero value so applyHeroFetched
		// stays a no-op for stale messages.
		if heroCmd != nil {
			m.hero = newHero
		}
	}
	return m, heroCmd
}

// applyHeroFetched routes a heroImageReadyMsg to the hero and re-runs
// the markdown render so the rendered ANSI block gets spliced in at
// the image's markdown position. The viewport keeps its current
// scroll position; bubbles handles the line-count change.
func (m readmeModel) applyHeroFetched(msg heroImageReadyMsg) readmeModel {
	wasReady := m.hero.ready
	m.hero = m.hero.applyHeroFetched(msg)
	if !wasReady && m.hero.ready {
		m.renderedMarkdown = m.hero.spliceInline(renderMarkdown(m.raw, m.hero.refRaw, m.contentWidth))
		m.refreshViewportContent()
	}
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
		// Stacked mode: the reel (if any) lives inside the viewport's
		// content stream as a hero block above the markdown, so the
		// viewport gets the full body height. Scrolling past the reel's
		// rows lets it leave the top of the panel naturally — same UX
		// as a hero image on a web page.
		if m.reel.Height() > 0 {
			m.reel.width = width
		}
		m.contentWidth = width
		m.viewport = viewport.New(m.contentWidth, bodyRows)
	}
	// Keep the hero's centering width in sync with the readme content
	// width — the right-pane reel mode narrows the readme column, so a
	// hero centered against the full terminal width would sit off to
	// the right of the surrounding markdown.
	m.hero.width = m.contentWidth
	m.renderedMarkdown = m.hero.spliceInline(renderMarkdown(m.raw, m.hero.refRaw, m.contentWidth))
	m.refreshViewportContent()
	m.ready = true
	return m
}

// refreshViewportContent rebuilds the viewport's content from the
// cached markdown render plus, in stacked-with-reel mode, the live
// reel.View() prepended as a hero block. Called from three places:
// (1) resize, (2) applyFetch / applyHeroFetched when the markdown
// changes, and (3) the reel tick path so each animation frame
// repaints the reel's portion of the scroll buffer.
//
// Scroll position is preserved across the SetContent call: bubbles
// resets YOffset to 0 on SetContent, so we save it, restore it, and
// clamp to the new content's max so we don't end up scrolled past
// the bottom (e.g. when the reel finishes loading and content gets
// taller, or a resize shortens the content).
//
// In right-pane mode and stacked-without-reel mode, the reel is
// either drawn separately or absent, and this just sets the cached
// markdown verbatim.
func (m *readmeModel) refreshViewportContent() {
	content := m.renderedMarkdown
	if !m.reelRightPane && m.reel.Height() > 0 {
		// reel.View() is already a multi-line styled block; a single
		// "\n" between it and the markdown gives one blank-ish row of
		// breathing space without the markdown's own leading whitespace
		// stacking on top of the reel's framed border.
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
	// Reel tick messages go to the strip and nowhere else. Handle
	// them before the key/viewport dispatch so the animation keeps
	// running regardless of what else the readme model is doing.
	//
	// In stacked mode the reel is part of the viewport's scroll
	// buffer (so it scrolls off when the user scrolls down), which
	// means a fresh reel frame requires re-spilling the content into
	// the viewport. In right-pane mode the reel is drawn separately
	// in View() and no content refresh is needed.
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

// View renders the readme without a live spinner. Used by tests and
// any caller that doesn't have a Root in scope. Production callers
// should prefer ViewWithSpinner so the "fetching from github…"
// footer animates.
func (m readmeModel) View() string {
	return m.ViewWithSpinner("")
}

// ViewWithSpinner is the live-render path. spinnerGlyph is the
// current frame of the shared Root spinner; when non-empty and the
// readme is still loading, the footer shows it next to the
// "fetching from github…" label so the wait state is visible.
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
	// Stacked mode: the reel (when present) is spliced into the
	// viewport's content by refreshViewportContent, so the viewport
	// alone draws the body. The reel scrolls off the top with the
	// rest of the readme — same UX as a hero image on a web page.
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

func renderMarkdown(md, heroRefRaw string, termWidth int) string {
	// Pre-process HTML <img> tags into markdown image syntax so
	// glamour surfaces their URLs in its output. Many GitHub
	// READMEs use HTML for alignment/sizing; without this step
	// glamour strips the whole tag and the inline-image splice
	// has no anchor.
	md = HTMLImgToMarkdown(md)

	// If a hero has been picked for this readme, swap its markdown
	// image syntax for the splice placeholder before glamour sees
	// it. This sidesteps glamour's URL wrapping/normalization
	// quirks that broke earlier direct-match approaches.
	md = InjectHeroPlaceholder(md, heroRefRaw)

	wrap := readmeMaxContentWidth
	if termWidth-8 < wrap {
		wrap = max(termWidth-8, 20)
	}

	// We pre-detect the terminal background in main() before tea captures
	// the terminal — glamour's WithAutoStyle queries OSC 11 from inside
	// the renderer, which fails once tea is in raw mode + alt screen, so
	// it always falls back to dark. Pass the explicit style instead.
	//
	// On dark terminals we use glamour's "pink" stylesheet — it ships
	// with the Charm fuchsia for headings, links, and list bullets, so
	// READMEs render as part of the same brand language as the rest of
	// the UI rather than as generic dark-mode markdown. Light terminals
	// stay on the stock "light" style: the pink stylesheet was tuned for
	// dark backgrounds and washes out on white.
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
