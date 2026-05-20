package ui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/blacktop/go-termimg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "golang.org/x/image/webp"
)

const (
	galleryFetchTimeout = 10 * time.Second
	galleryMaxBytes     = 5 << 20
	galleryMaxRows      = 18
	galleryBorderRows   = 2
)

type galleryStrip struct {
	repo     string
	urls     []string
	index    int
	rendered string
	loading  bool
	fetchErr error
	width    int
}

type galleryImageReadyMsg struct {
	repo     string
	index    int
	rendered string
	err      error
}

func newGalleryStrip(repo string, urls []string, width int) galleryStrip {
	return galleryStrip{repo: repo, urls: urls, width: width}
}

func (g galleryStrip) hasURLs() bool {
	return len(g.urls) > 0
}

func (g galleryStrip) loadingActive() bool {
	return g.hasURLs() && g.loading
}

func (g galleryStrip) currentURL() string {
	if g.index < 0 || g.index >= len(g.urls) {
		return ""
	}
	return g.urls[g.index]
}

func (g galleryStrip) fetchCurrentCmd() tea.Cmd {
	if !g.hasURLs() {
		return nil
	}
	repo := g.repo
	index := g.index
	url := g.urls[index]
	width := g.width
	return func() tea.Msg {
		rendered, err := downloadAndRenderGalleryImage(url, width)
		return galleryImageReadyMsg{repo: repo, index: index, rendered: rendered, err: err}
	}
}

func (g galleryStrip) applyFetched(msg galleryImageReadyMsg) galleryStrip {
	if msg.repo != g.repo || msg.index != g.index {
		return g
	}
	g.loading = false
	g.fetchErr = msg.err
	g.rendered = msg.rendered
	return g
}

func (g galleryStrip) step(delta int) (galleryStrip, tea.Cmd) {
	if len(g.urls) <= 1 {
		return g, nil
	}
	next := (g.index + delta + len(g.urls)) % len(g.urls)
	if next == g.index {
		return g, nil
	}
	g.index = next
	g.loading = true
	g.rendered = ""
	g.fetchErr = nil
	return g, g.fetchCurrentCmd()
}

func (g galleryStrip) Height() int {
	if !g.hasURLs() {
		return 0
	}
	if g.rendered == "" && !g.loading && g.fetchErr == nil {
		return 0
	}
	rows := lipgloss.Height(g.body())
	if rows == 0 {
		rows = 1
	}
	// Caption row when multiple screenshots.
	if len(g.urls) > 1 {
		rows++
	}
	return rows + galleryBorderRows
}

func (g galleryStrip) body() string {
	switch {
	case g.loading && g.rendered == "":
		return theme.MutedItalic.Render("loading screenshot…")
	case g.fetchErr != nil:
		return theme.MutedText.Render("screenshot unavailable")
	default:
		return g.rendered
	}
}

func (g galleryStrip) View() string {
	if g.Height() == 0 {
		return ""
	}
	body := g.body()
	if len(g.urls) > 1 {
		caption := theme.MutedText.Render(fmt.Sprintf("screenshot %d / %d · [ ] browse", g.index+1, len(g.urls)))
		body = body + "\n" + caption
	}
	framed := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Render(body)

	framedWidth := lipgloss.Width(strings.Split(framed, "\n")[0])
	if g.width > framedWidth+2 {
		leftPad := (g.width - framedWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}
		prefix := strings.Repeat(" ", leftPad)
		lines := strings.Split(framed, "\n")
		for i := range lines {
			lines[i] = prefix + lines[i]
		}
		framed = strings.Join(lines, "\n")
	}
	return framed
}

func downloadAndRenderGalleryImage(imageURL string, width int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), galleryFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); strings.Contains(ct, "svg") {
		return "", fmt.Errorf("SVG not supported")
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, galleryMaxBytes))
	if err != nil {
		return "", err
	}

	img, err := termimg.From(bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	cellW := max(width-8, 20)
	return img.
		Width(cellW).
		Height(galleryMaxRows).
		Scale(termimg.ScaleFit).
		Protocol(termimg.Auto).
		Render()
}
