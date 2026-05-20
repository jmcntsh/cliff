package ui

import (
	"os"
	"strings"

	"github.com/blacktop/go-termimg"
)

// galleryMaxPixelHeight caps screenshot height in pixels for graphics
// protocols.
const galleryMaxPixelHeight = 420

// WarmGalleryProtocolCache primes go-termimg's terminal feature detection
// on the main thread before Bubble Tea captures stdin. Detection uses
// TERM_PROGRAM and related env vars without CSI queries when possible.
func WarmGalleryProtocolCache() {
	_ = termimg.QueryTerminalFeatures()
}

// IsAppleTerminal reports macOS Terminal.app, which supports no inline
// graphics protocols.
func IsAppleTerminal() bool {
	switch strings.TrimSpace(os.Getenv("TERM_PROGRAM")) {
	case "Apple_Terminal", "Terminal":
		return true
	default:
		return false
	}
}

// shouldShowInlineGallery reports whether README screenshots should be
// fetched and shown inline. Terminals without a graphics protocol (macOS
// Terminal, most basic emulators) are skipped — blocky half-block fallback
// is not shown unless CLIFF_IMAGE_PROTOCOL=halfblocks is set explicitly.
func shouldShowInlineGallery(urls []string) bool {
	if len(urls) == 0 {
		return false
	}
	return galleryInlinePreviewSupported()
}

func galleryInlinePreviewSupported() bool {
	if IsAppleTerminal() {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CLIFF_IMAGE_PROTOCOL"))) {
	case "halfblocks", "blocks", "mosaic":
		return true
	}
	return galleryUsesGraphicsProtocol(galleryRenderProtocol())
}

func galleryRenderProtocol() termimg.Protocol {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CLIFF_IMAGE_PROTOCOL"))) {
	case "kitty":
		return termimg.Kitty
	case "iterm", "iterm2":
		return termimg.ITerm2
	case "sixel":
		return termimg.Sixel
	case "halfblocks", "blocks", "mosaic":
		return termimg.Halfblocks
	case "auto", "":
		return termimg.DetectProtocol()
	default:
		return termimg.DetectProtocol()
	}
}

func configureGalleryImage(img *termimg.Image, protocol termimg.Protocol, cellW int) *termimg.Image {
	features := termimg.QueryTerminalFeatures()
	fontW, fontH := features.FontWidth, features.FontHeight
	if fontW <= 0 {
		fontW = 8
	}
	if fontH <= 0 {
		fontH = 16
	}

	switch protocol {
	case termimg.Kitty:
		return img.
			Protocol(termimg.Kitty).
			UseUnicode(true).
			WidthPixels(cellW * fontW).
			HeightPixels(galleryMaxPixelHeight).
			Scale(termimg.ScaleFit)
	case termimg.ITerm2:
		cellH := max(galleryMaxPixelHeight/fontH, 10)
		return img.
			Protocol(termimg.ITerm2).
			Width(cellW).
			Height(cellH).
			Scale(termimg.ScaleFit)
	case termimg.Sixel:
		cellH := max(galleryMaxPixelHeight/fontH, 12)
		return img.
			Protocol(termimg.Sixel).
			Width(cellW).
			Height(cellH).
			Scale(termimg.ScaleFit).
			Dither(true)
	default:
		return img.
			Protocol(termimg.Halfblocks).
			Width(cellW).
			Height(galleryMaxRows).
			Scale(termimg.ScaleFit)
	}
}

func galleryUsesGraphicsProtocol(p termimg.Protocol) bool {
	switch p {
	case termimg.Kitty, termimg.ITerm2, termimg.Sixel:
		return true
	default:
		return false
	}
}
