package ui

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	heroMaxCols      = 60
	heroMaxRows      = 15
	heroFetchTimeout = 5 * time.Second
	heroMaxBytes     = 5 << 20 // 5 MiB
)

// fetchHeroCmd does the work that produces a heroImageReadyMsg:
// HTTP GET, image decode, ANSI render. Runs on bubbletea's command
// goroutine so the UI thread stays responsive on slow links.
func fetchHeroCmd(slug, imageURL string) tea.Cmd {
	return func() tea.Msg {
		ansi, rows, cols := fetchAndRender(imageURL)
		return heroImageReadyMsg{slug: slug, ansi: ansi, rows: rows, cols: cols}
	}
}

// fetchAndRender downloads imageURL, decodes it, and renders it to
// ANSI half-block characters. Returns ("", 0, 0) on any failure —
// the readme view treats that as "no hero" and renders the original
// markdown placeholder (briefly, before placeholder substitution
// only kicks in once a hero is ready).
func fetchAndRender(imageURL string) (ansi string, rows, cols int) {
	ctx, cancel := context.WithTimeout(context.Background(), heroFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return "", 0, 0
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", 0, 0
	}
	// SVG passes the URL-path check when the URL has no extension
	// (common with badge services and shorthand routes), so re-check
	// here against the response Content-Type. The std image package
	// can't decode SVG, and the failure mode without this is a noisy
	// "unknown format" we silently swallow — better to skip cleanly
	// at the type level.
	if ct := res.Header.Get("Content-Type"); strings.Contains(ct, "svg") {
		return "", 0, 0
	}
	img, _, err := image.Decode(io.LimitReader(res.Body, heroMaxBytes))
	if err != nil {
		return "", 0, 0
	}
	return renderHalfBlock(img, heroMaxCols, heroMaxRows)
}

// renderHalfBlock samples img into a (cols × rows) grid of cells,
// each cell being two vertical pixels rendered as a U+2580 ("▀")
// glyph: fg = top pixel color, bg = bottom pixel color. This packs
// 2× vertical resolution into the same row count, which is the
// standard trick for terminal image rendering.
//
// We sample nearest-neighbor (no interpolation) because the only
// std-only options would be hand-rolling bilinear — at these sizes
// the quality difference isn't worth the extra code. If the result
// looks noisy in practice we can swap for a real resize later.
//
// Aspect ratio is preserved: we shrink to fit inside (cols × rows*2)
// pixels with the longer side touching the box. Empty pixel rows
// at the top/bottom are trimmed so the rendered block hugs the image.
func renderHalfBlock(img image.Image, maxCols, maxRows int) (string, int, int) {
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return "", 0, 0
	}

	// Each cell is 2 vertical pixels, so the destination pixel grid
	// is maxCols × (maxRows*2). Scale to fit inside that, preserving
	// aspect ratio.
	dstPxW, dstPxH := maxCols, maxRows*2
	scale := minFloat(float64(dstPxW)/float64(srcW), float64(dstPxH)/float64(srcH))
	if scale > 1 {
		scale = 1 // never upscale — small logos render at native size
	}
	cols := int(float64(srcW) * scale)
	pxRows := int(float64(srcH) * scale)
	if cols < 1 {
		cols = 1
	}
	if pxRows < 2 {
		pxRows = 2
	}
	// Round pxRows down to even so half-block cells aren't half-empty.
	if pxRows%2 == 1 {
		pxRows--
	}
	rows := pxRows / 2

	var out strings.Builder
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			topX := bounds.Min.X + (col*srcW)/cols
			topY := bounds.Min.Y + ((row*2)*srcH)/pxRows
			botY := bounds.Min.Y + ((row*2+1)*srcH)/pxRows
			fg := rgbHex8(img.At(topX, topY))
			bg := rgbHex8(img.At(topX, botY))
			// One lipgloss.Render per cell is fine for ≤ heroMaxCols
			// × heroMaxRows = 900 cells. Coalescing equal-style runs
			// is the next optimization if rendering shows up in a
			// profile.
			cell := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#" + fg)).
				Background(lipgloss.Color("#" + bg)).
				Render("▀")
			out.WriteString(cell)
		}
		if row < rows-1 {
			out.WriteByte('\n')
		}
	}
	return out.String(), rows, cols
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// rgbHex8 returns a 6-digit hex string for the given color, ignoring
// alpha. Premultiplied-alpha sources (PNG with transparency) get
// flattened against black, which renders as black borders around
// transparent regions — visually fine for logos, marginally less
// great for hero screenshots; not worth a checkerboard.
func rgbHex8(c interface{}) string {
	type rgba interface {
		RGBA() (uint32, uint32, uint32, uint32)
	}
	col, ok := c.(rgba)
	if !ok {
		return "000000"
	}
	r, g, b, _ := col.RGBA()
	return fmt.Sprintf("%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}
