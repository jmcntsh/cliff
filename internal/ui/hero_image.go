package ui

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// heroImage renders the first usable image from an app's README as
// ANSI half-block characters, spliced into glamour's output at the
// position the markdown referenced. It's the static counterpart to
// the reel strip — apps that don't have a registry-hosted reel get
// a logo or hero shot in roughly the same place as the markdown
// would have shown the link.
//
// V1 scope: PNG / JPEG / GIF (first frame), absolute or readme-
// relative URLs, capped at heroMaxCols × heroMaxRows cells, no disk
// cache, badge hosts skipped. SVG and animated formats are deferred.
// The renderer samples nearest-neighbor — fidelity is modest but
// logos remain recognizable. See hero_pick.go for URL extraction
// and hero_render.go for the fetch + decode + render pipeline.

// heroImage carries the rendered ANSI block plus the markdown URL it
// was extracted from, so the readme view can splice it back into
// glamour's output at the position the markdown referenced. Zero
// value is "not ready, no splice" — the readme view treats it
// unconditionally.
type heroImage struct {
	ansi   string // rendered half-block ANSI, multi-line
	refRaw string // URL as it appears in the markdown source
	rows   int    // visible rows the ansi spans
	cols   int    // visible cols of the ansi block (used to center it)
	width  int    // host terminal width at construction time
	slug   string
	ready  bool
}

// heroImageReadyMsg is delivered after a successful fetch + render.
// The slug routes the message to the right readme model in case the
// user navigated away while the fetch was in flight.
type heroImageReadyMsg struct {
	slug string
	ansi string
	rows int
	cols int
}

// newHeroFromMarkdown returns a heroImage in its zero state plus a
// tea.Cmd that finds the first viable image in the markdown,
// downloads it, and emits a heroImageReadyMsg when rendering
// completes. Returns (zero, nil) if no candidate image is found in
// the markdown — saves a Cmd allocation and an empty fetch.
func newHeroFromMarkdown(slug, raw, readmeURL string, width int) (heroImage, tea.Cmd) {
	h := heroImage{slug: slug, width: width}
	refRaw, resolved := pickHeroImage(raw, readmeURL)
	if resolved == "" {
		return h, nil
	}
	h.refRaw = refRaw
	return h, fetchHeroCmd(slug, resolved)
}

// applyHeroFetched routes a heroImageReadyMsg to the model when the
// slug matches. Returns the (possibly mutated) hero with `ready` set
// so the readme view can splice the rendered ANSI block into the
// markdown content at the original image position.
func (h heroImage) applyHeroFetched(msg heroImageReadyMsg) heroImage {
	if msg.slug != h.slug || h.ready {
		return h
	}
	if msg.ansi == "" || msg.rows == 0 {
		return h
	}
	h.ansi = msg.ansi
	h.rows = msg.rows
	h.cols = msg.cols
	h.ready = true
	return h
}

// Height is unused for the inline splice — the hero contributes
// rows by being substituted into the viewport content, not by
// claiming its own layout slot above the viewport. Kept (always
// returning zero) so the readme view's layout math stays simple
// and the hero behaves like an absent participant.
func (h heroImage) Height() int { return 0 }

// HeroPlaceholder is the sentinel string we inject into the markdown
// before glamour renders it, in place of the picked image's
// `![alt](url)` syntax. Short, ASCII, no markdown-meaningful
// characters — glamour passes it through as plain text without
// wrapping, normalizing, or styling it apart. The splice then
// substitutes it with the rendered ANSI block. This sidesteps every
// glamour transformation that previously broke direct URL matching:
// soft-wrapping long URLs across two or three lines, normalizing
// `./path` to `/path`, and rendering image links inconsistently
// when the alt is empty.
const HeroPlaceholder = "CLIFFHEROANCHORZ7"

// InjectHeroPlaceholder rewrites the first markdown image whose URL
// matches refRaw to a plain CLIFFHEROANCHORZ7 paragraph. Returns md
// unchanged when refRaw is empty or no match is found. Exported so
// the readme renderer can call it from the rendering pipeline.
func InjectHeroPlaceholder(md, refRaw string) string {
	if refRaw == "" {
		return md
	}
	// Match `![alt](refRaw…)` allowing trailing query/fragment. We
	// can't just substring-replace refRaw because that'd alter the
	// URL inside the parens without removing the surrounding
	// `![…]( … )` glamour rendering. Quoting refRaw protects against
	// regex metachars in URLs (notably `?` and `+`).
	pattern := regexp.MustCompile(`!\[[^\]]*\]\(` + regexp.QuoteMeta(refRaw) + `[^)]*\)`)
	return pattern.ReplaceAllString(md, "\n\n"+HeroPlaceholder+"\n\n")
}

// spliceInline replaces the line in rendered that contains the hero
// placeholder with h.ansi (a multi-line block), padded to center
// against h.width. Returns rendered unchanged when the hero isn't
// ready or the placeholder isn't present.
func (h heroImage) spliceInline(rendered string) string {
	if !h.ready || h.ansi == "" {
		return rendered
	}
	idx := strings.Index(rendered, HeroPlaceholder)
	if idx < 0 {
		return rendered
	}
	start := strings.LastIndexByte(rendered[:idx], '\n') + 1
	end := len(rendered)
	if rest := strings.IndexByte(rendered[idx:], '\n'); rest >= 0 {
		end = idx + rest
	}
	return rendered[:start] + h.padded() + rendered[end:]
}

// padded returns h.ansi with each line prefixed by enough spaces to
// center the rendered block against the host terminal width. Falls
// back to the unpadded ansi when we don't have a sane width or the
// block already fills the terminal.
func (h heroImage) padded() string {
	if h.width <= 0 || h.cols <= 0 || h.cols >= h.width {
		return h.ansi
	}
	leftPad := (h.width - h.cols) / 2
	if leftPad <= 0 {
		return h.ansi
	}
	prefix := strings.Repeat(" ", leftPad)
	lines := strings.Split(h.ansi, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
