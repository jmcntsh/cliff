package readme

import (
	"fmt"
	"testing"
)

func TestGalleryURLsManifestWins(t *testing.T) {
	got := GalleryURLs(
		[]string{"https://example.com/a.png", "https://example.com/b.png"},
		"o/r",
		"https://raw.githubusercontent.com/o/r/main/README.md",
		"![ignored](https://example.com/x.png)",
	)
	if len(got) != 2 || got[0] != "https://example.com/a.png" {
		t.Fatalf("got %v", got)
	}
}

func TestGalleryURLsExtractSkipsBadges(t *testing.T) {
	md := `# App

![build](https://img.shields.io/badge/build-passing)
![hero](./assets/hero.png)
`
	base := "https://raw.githubusercontent.com/o/r/main/README.md"
	got := GalleryURLs(nil, "o/r", base, md)
	if len(got) != 1 {
		t.Fatalf("got %v, want one non-badge image", got)
	}
	want := "https://raw.githubusercontent.com/o/r/HEAD/assets/hero.png"
	if got[0] != want {
		t.Fatalf("got %q want %q", got[0], want)
	}
}

func TestGalleryURLsHTMLImg(t *testing.T) {
	md := `<p align="center"><img src="./demo.gif" alt="demo"></p>`
	base := "https://raw.githubusercontent.com/o/r/master/README.md"
	got := GalleryURLs(nil, "o/r", base, md)
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if got[0] != "https://raw.githubusercontent.com/o/r/HEAD/demo.gif" {
		t.Fatalf("got %q", got[0])
	}
}

func TestGalleryURLsCap(t *testing.T) {
	var md string
	for i := range MaxGalleryImages + 3 {
		md += fmt.Sprintf("![x](https://example.com/%d.png)\n", i)
	}
	got := GalleryURLs(nil, "o/r", "", md)
	if len(got) != MaxGalleryImages {
		t.Fatalf("got %d urls, want cap %d", len(got), MaxGalleryImages)
	}
}

func TestReadmeBaseURL(t *testing.T) {
	if got := ReadmeBaseURL("o/r", ""); got != "https://raw.githubusercontent.com/o/r/HEAD/README.md" {
		t.Fatalf("got %q", got)
	}
	if got := ReadmeBaseURL("o/r", "https://custom/readme.md"); got != "https://custom/readme.md" {
		t.Fatalf("got %q", got)
	}
}
