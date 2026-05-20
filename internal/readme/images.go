package readme

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const MaxGalleryImages = 8

var (
	imgRe         = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)`)
	htmlImgRe     = regexp.MustCompile(`<img\s[^>]*src\s*=\s*["']?([^"'\s>]+)`)
	htmlImgTagRe  = regexp.MustCompile(`(?i)<img\b[^>]*?>`)
	htmlAttrSrcRe = regexp.MustCompile(`(?i)src\s*=\s*["']([^"']+)["']`)
	htmlAttrAltRe = regexp.MustCompile(`(?i)alt\s*=\s*["']([^"']*)["']`)
)

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
	"static.pepy.tech":      true,
	"pepy.tech":             true,
	"workflow-badge.vercel.app": true,
}

type imageRef struct {
	idx int
	raw string
}

// ReadmeBaseURL returns the URL used to resolve relative image paths in a
// README. Manifest readme fields win; otherwise derive a HEAD-rooted raw URL
// from the GitHub repo slug.
func ReadmeBaseURL(repo, readmeField string) string {
	if readmeField != "" {
		return readmeField
	}
	i := strings.Index(repo, "/")
	if i < 0 {
		return ""
	}
	return fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/HEAD/README.md",
		repo[:i], repo[i+1:],
	)
}

// GalleryURLs returns screenshot URLs for the gallery overlay. Manifest
// screenshots win when present; otherwise non-badge images are extracted
// from markdown in document order, capped at MaxGalleryImages.
func GalleryURLs(manifest []string, repo, readmeURL, markdown string) []string {
	baseURL := readmeURL
	if baseURL == "" {
		baseURL = ReadmeBaseURL(repo, "")
	}
	if len(manifest) > 0 {
		return resolveGalleryURLs(manifest, baseURL, MaxGalleryImages)
	}
	return extractGalleryURLs(markdown, baseURL, MaxGalleryImages)
}

func extractGalleryURLs(md, readmeURL string, limit int) []string {
	base, _ := url.Parse(readmeURL)
	md = htmlImgToMarkdown(md)
	var out []string
	seen := map[string]bool{}
	for _, raw := range collectImageRefs(md) {
		if resolved := viableImageURL(base, raw); resolved != "" && !seen[resolved] {
			seen[resolved] = true
			out = append(out, resolved)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func resolveGalleryURLs(urls []string, readmeURL string, limit int) []string {
	base, _ := url.Parse(readmeURL)
	var out []string
	seen := map[string]bool{}
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		resolved := resolveImageURL(base, raw, u)
		if resolved == "" || seen[resolved] {
			continue
		}
		seen[resolved] = true
		out = append(out, resolved)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func htmlImgToMarkdown(md string) string {
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

func collectImageRefs(md string) []string {
	hits := []imageRef{}
	for _, m := range imgRe.FindAllStringSubmatchIndex(md, -1) {
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

func sortImageRefs(hits []imageRef) {
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j-1].idx > hits[j].idx; j-- {
			hits[j-1], hits[j] = hits[j], hits[j-1]
		}
	}
}

func viableImageURL(base *url.URL, raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.IsAbs() && u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(u.Path), ".svg") {
		return ""
	}
	if u.IsAbs() && badgeHosts[strings.ToLower(u.Host)] {
		return ""
	}
	return resolveImageURL(base, raw, u)
}

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
