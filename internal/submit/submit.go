// Package submit builds the URL that lets a user nominate a new app
// for the cliff registry by filing a prefilled GitHub issue against
// the registry repo.
//
// cliff has no backend and no account system, so the submit flow is
// deliberately a one-bounce handoff: show the user what they're about
// to open, then open the registry's new-app issue form in the browser.
// GitHub already solves identity, rate-limiting, and triage; we just
// prefill the fields we know and let the curator do the review.
//
// The URL built here works against the issue template at
// .github/ISSUE_TEMPLATE/new-app.yml in the registry repo (field ids
// match its `id:` keys). If the template is missing or renamed, GitHub
// silently falls back to a blank issue with the title prefilled —
// still functional, just less structured.
package submit

import (
	"net/url"
	"strings"
)

// RegistryRepo is the GitHub repo that accepts submissions. Exported so
// the caller can show it in the confirmation overlay ("opens on
// github.com/<this>") without hard-coding the string twice.
const RegistryRepo = "jmcntsh/cliff-registry"

// Request is the seed for a submission. All fields are optional — the
// user can open the flow from the catalog with no context (Name and
// Repo empty) and fill the issue form themselves, or hit submit from
// a readme where we know the Repo already and prefill it.
type Request struct {
	// Name is a proposed app slug. Lowercase, [a-z0-9-]. If empty
	// the form renders a blank field with the placeholder from the
	// issue template.
	Name string
	// Repo is a canonical "owner/name" GitHub repo, when known. The
	// homepage field falls back to https://github.com/<Repo> in the
	// issue body if the user doesn't paste a different one.
	Repo string
	// Description is a one-line summary (≤120 chars per manifest v0).
	// Longer strings are passed through — GitHub's form will flag
	// them during validation rather than silently truncate.
	Description string
	// Notes is freeform "why cliff should list this" context — often
	// the thing the submitter cares most about. Goes into the
	// free-text field of the issue form.
	Notes string
}

// URL returns the fully-qualified https://github.com/... URL to open
// in a browser. Uses the issue-form `template=` parameter so the right
// template loads; falls back gracefully to a generic prefilled issue
// if the template has been removed on the registry side.
//
// Field names here (`name`, `repo`, `description`, `notes`) must stay
// in sync with the `id:` keys in
// .github/ISSUE_TEMPLATE/new-app.yml in the registry repo. We keep
// them short and self-explanatory so a renamed id won't silently stop
// filling fields.
func (r Request) URL() string {
	u := &url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   "/" + RegistryRepo + "/issues/new",
	}
	q := url.Values{}
	q.Set("template", "new-app.yml")
	q.Set("labels", "submission")
	q.Set("title", r.title())
	// Issue-form prefill works via one query param per form field id.
	// Empty strings are omitted so GitHub renders the template's
	// placeholders rather than literal empty values that look like
	// the user already typed a blank.
	if s := strings.TrimSpace(r.Name); s != "" {
		q.Set("name", s)
	}
	if s := strings.TrimSpace(r.Repo); s != "" {
		q.Set("repo", s)
	}
	if s := strings.TrimSpace(r.Description); s != "" {
		q.Set("description", s)
	}
	if s := strings.TrimSpace(r.Notes); s != "" {
		q.Set("notes", s)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// title derives the issue title. When we know the app name we lead
// with it ("Submit: bottom") so the curator can triage from the list
// view; otherwise a bare "Submit: new app" is honest about the
// unknown and still distinct from a bug report.
func (r Request) title() string {
	name := strings.TrimSpace(r.Name)
	if name == "" {
		name = strings.TrimSpace(r.Repo)
	}
	if name == "" {
		return "Submit: new app"
	}
	return "Submit: " + name
}
