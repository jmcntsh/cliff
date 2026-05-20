package ui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/browser"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/blacktop/go-termimg"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	galleryFetchTimeout = 10 * time.Second
	galleryMaxBytes     = 5 << 20
)

type galleryState struct {
	index    int
	rendered string
	loading  bool
	fetchErr error
}

type galleryImageReadyMsg struct {
	index    int
	rendered string
	err      error
}

func (g galleryState) currentURL(screenshots []string) string {
	if g.index < 0 || g.index >= len(screenshots) {
		return ""
	}
	return screenshots[g.index]
}

func fetchGalleryImageCmd(index int, imageURL string, width, height int) tea.Cmd {
	return func() tea.Msg {
		rendered, err := downloadAndRenderGalleryImage(imageURL, width, height)
		return galleryImageReadyMsg{index: index, rendered: rendered, err: err}
	}
}

func downloadAndRenderGalleryImage(imageURL string, width, height int) (string, error) {
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

	cellW := max(width-12, 20)
	cellH := max(height-12, 8)
	return img.
		Width(cellW).
		Height(cellH).
		Scale(termimg.ScaleFit).
		Protocol(termimg.Auto).
		Render()
}

func (r Root) openGallery() (Root, tea.Cmd) {
	if len(r.readme.screenshots) == 0 {
		return r, nil
	}
	r.gallery = galleryState{index: 0, loading: true}
	r.mode = modeGallery
	return r, fetchGalleryImageCmd(0, r.readme.screenshots[0], r.width, r.height)
}

func (r Root) applyGalleryImage(msg galleryImageReadyMsg) (Root, tea.Cmd) {
	if r.mode != modeGallery || msg.index != r.gallery.index {
		return r, nil
	}
	r.gallery.loading = false
	r.gallery.fetchErr = msg.err
	r.gallery.rendered = msg.rendered
	return r, nil
}

func (r Root) galleryStep(delta int) (Root, tea.Cmd) {
	n := len(r.readme.screenshots)
	if n == 0 {
		return r, nil
	}
	next := (r.gallery.index + delta + n) % n
	if next == r.gallery.index {
		return r, nil
	}
	r.gallery.index = next
	r.gallery.loading = true
	r.gallery.rendered = ""
	r.gallery.fetchErr = nil
	url := r.readme.screenshots[next]
	return r, fetchGalleryImageCmd(next, url, r.width, r.height)
}

func galleryView(appName string, shots []string, g galleryState, width, height int, spinner string) string {
	contentH := max(height-2, 1)
	title := theme.GradientTitle(appName + " · screenshots")
	count := theme.MutedText.Render(fmt.Sprintf("%d / %d", g.index+1, len(shots)))

	var body string
	switch {
	case g.loading:
		label := "loading image…"
		if spinner != "" {
			label = spinner + " " + label
		}
		body = theme.MutedItalic.Render(label)
	case g.fetchErr != nil:
		body = theme.WarnText.Render("could not load image") + "\n\n" +
			theme.MutedText.Render(truncateMiddle(g.currentURL(shots), 72)) + "\n\n" +
			theme.MutedItalic.Render(g.fetchErr.Error())
	case g.rendered != "":
		body = g.rendered
	default:
		body = theme.MutedItalic.Render("no preview")
	}

	urlLine := theme.MutedText.Render(truncateMiddle(g.currentURL(shots), max(width-12, 40)))
	hint := theme.MutedItalic.Render("← → browse · o open · esc back")

	inner := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		count,
		"",
		body,
		"",
		urlLine,
		"",
		hint,
	)

	return lipgloss.Place(width, contentH, lipgloss.Center, lipgloss.Center, modalBox(width, inner))
}

func truncateMiddle(s string, maxLen int) string {
	if maxLen <= 3 || len(s) <= maxLen {
		return s
	}
	keep := maxLen - 3
	left := keep / 2
	right := keep - left
	return s[:left] + "..." + s[len(s)-right:]
}

func (r Root) updateGallery(msg tea.KeyMsg) (Root, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape, keys.Quit):
		r.mode = modeReadme
		r.gallery = galleryState{}
		return r, nil
	case key.Matches(msg, keys.Left):
		return r.galleryStep(-1)
	case key.Matches(msg, keys.Right):
		return r.galleryStep(1)
	case key.Matches(msg, keys.OpenGalleryImage):
		if url := r.gallery.currentURL(r.readme.screenshots); url != "" {
			_ = browser.Open(url)
			return r.flash("opening image"), clearFlashCmd()
		}
	}
	return r, nil
}
