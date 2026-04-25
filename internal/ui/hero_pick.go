package ui

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// imgRe matches markdown image syntax: ![alt](url).
// The alt text is allowed to be empty or contain anything except ].
// The URL is anything up to whitespace or the closing paren.
var imgRe = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)`)

// htmlImgRe matches the common HTML image syntax that GitHub READMEs
// use for centered or aligned hero shots — the markdown image syntax
// can't express alignment, so authors fall back to <img src="...">.
// We accept attribute order variation (alt before/after src) by just
// looking for src=. Quotes can be single, double, or absent.
var htmlImgRe = regexp.MustCompile(`<img\s[^>]*src\s*=\s*["']?([^"'\s>]+)`)

// htmlImgTagRe matches a full <img …> tag for substitution into
// markdown image syntax. Glamour strips HTML blocks wholesale, so
// without this conversion any logo declared as <img> never reaches
// the rendered output and the inline splice has no anchor to bind
// to. Converting to ![alt](src) before glamour renders gives both
// glamour and the splice a stable URL to work with.
var htmlImgTagRe = regexp.MustCompile(`(?i)<img\s+[^>]*?>`)
var htmlAttrSrcRe = regexp.MustCompile(`(?i)src\s*=\s*["']([^"']+)["']`)
var htmlAttrAltRe = regexp.MustCompile(`(?i)alt\s*=\s*["']([^"']*)["']`)

// badgeHosts lists hostnames known to serve status badges or icons
// rather than logos / hero screenshots. Most readmes lead with a
// row of these; we skip past them to find the actual hero.
var badgeHosts = map[string]bool{
	"img.shields.io":        true,
	"shields.io":            true,
	"badge.fury.io":         true,
	"badges.greenkeeper.io": true,
	"codecov.io":            true,
	"codeclimate.com":       true,
	"app.codacy.com":        true,
	"codacy.com":            true,
	"travis-ci.org":         true,
	"travis-ci.com":         true,
	"circleci.com":          true,
	"appveyor-matrix.com":   true,
	"snyk.io":               true,
	"goreportcard.com":      true,
	"pkg.go.dev":            true,
	"deps.dev":              true,
	"img.youtube.com":       true,
	"badgen.net":            true,
	"gitter.im":             true,
	"gitpod.io":             true,
}

// HTMLImgToMarkdown rewrites every <img src="X" alt="Y" …> in md as
// ![Y](X) wrapped in blank lines. The wrap matters: GitHub READMEs
// often nest <img> inside <a><div>…</div></a> for sizing/centering,
// and goldmark (glamour's parser) treats the whole nest as a single
// HTML block — markdown inside an HTML block isn't re-parsed. The
// blank lines force the converted image out of the surrounding HTML
// block and into its own paragraph so glamour renders it as an
// actual image link with the URL visible in the output. Tags
// without a src are left alone. Exported so readme.go can call it
// from the rendering path.
func HTMLImgToMarkdown(md string) string {
	return htmlImgTagRe.ReplaceAllStringFunc(md, func(tag string) string {
		srcM := htmlAttrSrcRe.FindStringSubmatch(tag)
		if len(srcM) < 2 || srcM[1] == "" {
			return tag
		}
		alt := ""
		if altM := htmlAttrAltRe.FindStringSubmatch(tag); len(altM) >= 2 {
			alt = altM[1]
		}
		return fmt.Sprintf("\n\n![%s](%s)\n\n", alt, srcM[1])
	})
}

// imageRef is the byte offset and URL of a single ![](url) or
// <img src="url"> hit; package-scoped so collectImageRefs can hand
// the slice to sortImageRefs without an anonymous-struct mismatch.
type imageRef struct {
	idx int
	raw string
}

// collectImageRefs returns every image URL referenced in md, in
// document order, deduplicated. Markdown ![alt](url) and HTML
// <img src="url"> are both included so a candidate the markdown
// regex misses (alignment-wrapped HTML) still gets considered.
func collectImageRefs(md string) []string {
	hits := []imageRef{}
	for _, m := range imgRe.FindAllStringSubmatchIndex(md, -1) {
		// m is [start, end, group1Start, group1End]; the URL is at [2:3].
		hits = append(hits, imageRef{idx: m[0], raw: md[m[2]:m[3]]})
	}
	for _, m := range htmlImgRe.FindAllStringSubmatchIndex(md, -1) {
		hits = append(hits, imageRef{idx: m[0], raw: md[m[2]:m[3]]})
	}
	sortImageRefs(hits)
	seen := map[string]bool{}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		if seen[h.raw] {
			continue
		}
		seen[h.raw] = true
		out = append(out, h.raw)
	}
	return out
}

// sortImageRefs orders refs by their byte offset in the source
// markdown so the markdown and HTML matches interleave correctly.
// Insertion sort because len(hits) is bounded — the number of image
// references in a README is well under 50 in practice; pulling in
// sort.Slice would be more code than this.
func sortImageRefs(hits []imageRef) {
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j-1].idx > hits[j].idx; j-- {
			hits[j-1], hits[j] = hits[j], hits[j-1]
		}
	}
}

// pickHeroImage extracts the first image reference from markdown that
// isn't an SVG and isn't on a known badge host, returning both the
// raw form (as it appears in the markdown source — used to find the
// image syntax for placeholder substitution) and the resolved
// absolute URL (used for the HTTP fetch). Returns ("", "") when no
// viable candidate is found. Walks markdown image syntax first, then
// HTML <img> tags, in document order — many readmes mix both, with
// the hero often as an HTML tag for alignment and badges in markdown
// syntax.
func pickHeroImage(md, readmeURL string) (refRaw, resolved string) {
	base, _ := url.Parse(readmeURL)
	for _, raw := range collectImageRefs(md) {
		// Allow only schemes we'll actually fetch. http / https /
		// relative paths are fine; anything else (data: URIs, mailto,
		// gh-flavored attachment shorthands) is skipped.
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if u.IsAbs() && u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		// SVG: std image package can't decode. Defer.
		if strings.HasSuffix(strings.ToLower(u.Path), ".svg") {
			continue
		}
		if u.IsAbs() && badgeHosts[strings.ToLower(u.Host)] {
			continue
		}
		if abs := resolveImageURL(base, raw, u); abs != "" {
			return raw, abs
		}
	}
	return "", ""
}

// resolveImageURL returns the absolute URL for raw, given the readme
// URL it appears in.
//
//   - raw is already absolute → return as-is.
//   - raw is relative on a raw.githubusercontent.com README → resolve
//     against the same owner/repo but pin the branch to HEAD. The
//     catalog's readme URL sometimes has a stale branch baked in
//     ("/main/README.md" for a repo whose default is "master"); the
//     readme itself loads via the GitHub API which auto-resolves the
//     default, but image URLs derived from the raw URL inherit the
//     wrong branch and 404. HEAD on raw.githubusercontent.com
//     auto-resolves to the actual default branch, so the result is
//     correct regardless of catalog drift. Also handles absolute
//     paths ("/assets/banner.png") by preserving the owner/repo
//     prefix that URL.ResolveReference would otherwise drop.
//   - everything else → standard URL.ResolveReference.
//
// Returns "" if the URL can't be resolved (no base + relative ref).
func resolveImageURL(base *url.URL, raw string, parsed *url.URL) string {
	if parsed.IsAbs() {
		return parsed.String()
	}
	if base == nil {
		return ""
	}
	if base.Host == "raw.githubusercontent.com" {
		parts := strings.SplitN(strings.TrimPrefix(base.Path, "/"), "/", 4)
		if len(parts) >= 3 {
			prefix := "/" + parts[0] + "/" + parts[1] + "/HEAD"
			if strings.HasPrefix(raw, "/") {
				return base.Scheme + "://" + base.Host + prefix + raw
			}
			// Relative path: rebuild a HEAD-rooted base whose path
			// includes the readme's directory so ./assets/x resolves
			// against the right folder, then defer to ResolveReference.
			tail := ""
			if len(parts) == 4 {
				tail = "/" + parts[3]
			}
			rebuilt := &url.URL{
				Scheme: base.Scheme,
				Host:   base.Host,
				Path:   prefix + tail,
			}
			return rebuilt.ResolveReference(parsed).String()
		}
	}
	return base.ResolveReference(parsed).String()
}
