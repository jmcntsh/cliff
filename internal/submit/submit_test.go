package submit

import (
	"net/url"
	"strings"
	"testing"
)

func TestURL_Empty(t *testing.T) {
	got := Request{}.URL()
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("URL() produced unparseable URL: %v (%q)", err, got)
	}
	if u.Host != "github.com" {
		t.Errorf("host = %q, want github.com", u.Host)
	}
	if !strings.HasSuffix(u.Path, "/issues/new") {
		t.Errorf("path = %q, want .../issues/new", u.Path)
	}
	q := u.Query()
	if q.Get("template") != "new-app.yml" {
		t.Errorf("template = %q, want new-app.yml", q.Get("template"))
	}
	if q.Get("title") != "Submit: new app" {
		t.Errorf("empty-request title = %q, want fallback", q.Get("title"))
	}
	if q.Get("name") != "" {
		t.Errorf("empty Name should not emit a name param, got %q", q.Get("name"))
	}
}

func TestURL_PrefilledFromRepo(t *testing.T) {
	got := Request{Repo: "ClementTsang/bottom"}.URL()
	u, _ := url.Parse(got)
	q := u.Query()
	if q.Get("title") != "Submit: ClementTsang/bottom" {
		t.Errorf("title = %q, want repo-derived", q.Get("title"))
	}
	if q.Get("repo") != "ClementTsang/bottom" {
		t.Errorf("repo = %q, want passthrough", q.Get("repo"))
	}
}

func TestURL_FullRequest(t *testing.T) {
	r := Request{
		Name:        "bottom",
		Repo:        "ClementTsang/bottom",
		Description: "Graphical process/system monitor",
		Notes:       "Popular htop-alternative, already cross-platform.",
	}
	got := r.URL()
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("unparseable: %v", err)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"title":       "Submit: bottom",
		"name":        "bottom",
		"repo":        "ClementTsang/bottom",
		"description": "Graphical process/system monitor",
		"notes":       "Popular htop-alternative, already cross-platform.",
		"template":    "new-app.yml",
		"labels":      "submission",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestURL_TrimsWhitespace(t *testing.T) {
	r := Request{Name: "  bottom  ", Notes: "   "}
	q, _ := url.Parse(r.URL())
	if got := q.Query().Get("name"); got != "bottom" {
		t.Errorf("name = %q, want trimmed", got)
	}
	if got := q.Query().Get("notes"); got != "" {
		t.Errorf("whitespace-only notes should be omitted, got %q", got)
	}
}
