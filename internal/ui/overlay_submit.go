package ui

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/jmcntsh/cliff/internal/submit"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/huh"
)

// newSubmitForm builds the huh form that backs the submit overlay's
// first phase. Fields write through pointers into the caller's
// submit.Request so the form's "current values" are always the same
// struct the URL builder consumes — no marshal step on completion.
//
// Field ordering follows what the registry issue template asks for
// (name, repo, description, notes), which is also the order users
// naturally think about a submission: "what's it called, where does
// it live, what does it do, anything else."
//
// Validation is intentionally minimal here:
//
//   - Name: optional, but if present must be a slug-ish lowercase
//     identifier (the manifest validator on the registry side enforces
//     the strict regex; we just guide the user away from obvious
//     mistakes like spaces).
//   - Repo: optional, but if present must look like "owner/name". The
//     curator handles non-GitHub repos in freeform notes.
//   - Description: optional, but if present must be ≤120 chars to
//     match the manifest cap. Tighten here so the user fixes it now,
//     not in the issue form.
//   - Notes: freeform.
//
// Validation runs on the live value (huh validates on field exit), so
// a user who types junk gets feedback immediately rather than at
// submission time.
//
// width is the available content width inside the modal; we pass it
// to huh so the form lays out to the same column the surrounding
// padding reserves. height is currently a hint — huh's group layout
// adapts within the cap.
func newSubmitForm(seed *submit.Request, width, height int) *huh.Form {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("name").
				Title("Name").
				Description("Lowercase slug, e.g. lazygit. Optional.").
				Placeholder("lazygit").
				Value(&seed.Name).
				Validate(validateSubmitName),

			huh.NewInput().
				Key("repo").
				Title("GitHub repo").
				Description("owner/name. Optional, but it's the easiest way for the curator to find it.").
				Placeholder("jesseduffield/lazygit").
				Value(&seed.Repo).
				Validate(validateSubmitRepo),

			huh.NewInput().
				Key("description").
				Title("One-line description").
				Description("≤120 chars. Optional.").
				Placeholder("Simple terminal UI for git commands").
				Value(&seed.Description).
				Validate(validateSubmitDescription),

			huh.NewText().
				Key("notes").
				Title("Notes").
				Description("Anything you want the curator to see. Optional.").
				Placeholder("e.g. why this belongs in cliff, install quirks, install command if non-obvious").
				Lines(4).
				Value(&seed.Notes),
		),
	).
		WithTheme(theme.HuhTheme()).
		WithShowHelp(false). // we render our own footer hints
		WithWidth(width).
		WithHeight(height)

	return form
}

// validateSubmitName accepts an empty string (the field is optional)
// or a slug-ish identifier. Strict-but-not-too-strict: the registry
// linter will catch the rest, and we don't want to bounce a user for
// a borderline character when the curator can normalize it.
func validateSubmitName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.ContainsAny(s, " \t/\\") {
		return errors.New("no spaces or slashes — slug only (e.g. lazygit)")
	}
	if strings.ToLower(s) != s {
		return errors.New("lowercase only — registry slugs are [a-z0-9-]")
	}
	return nil
}

// validateSubmitRepo accepts empty or "owner/name". We don't try to
// HEAD the URL — that would block the form on a network round-trip
// and a 404 from a private repo isn't necessarily disqualifying
// (the curator might know the maintainer). Shape check only.
func validateSubmitRepo(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Tolerate a pasted full URL by stripping the github.com prefix.
	if strings.HasPrefix(s, "https://github.com/") {
		s = strings.TrimPrefix(s, "https://github.com/")
	}
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return errors.New(`expected "owner/name"`)
	}
	// Catch the common copy-paste trailing slash early.
	if strings.Contains(parts[1], "/") {
		return errors.New(`extra slash — just "owner/name", no path`)
	}
	if _, err := url.Parse("https://github.com/" + s); err != nil {
		return errors.New("invalid characters")
	}
	return nil
}

// validateSubmitDescription enforces the manifest's 120-char cap so
// the user fixes overlong copy here rather than after submission.
// Counts runes, not bytes, so emoji in a description don't trip the
// limit prematurely.
func validateSubmitDescription(s string) error {
	if utf8.RuneCountInString(s) > 120 {
		return errors.New("≤120 chars (manifest cap)")
	}
	return nil
}

// submitFormView renders the huh form inside cliff's modal box. The
// header keeps the gradient brand-mark treatment we use across all
// modal-style overlays so the form reads as part of cliff, not as a
// separate widget.
func submitFormView(form *huh.Form, width int) string {
	header := theme.GradientTitle("Submit an app to cliff")
	hint := theme.MutedText.Render(
		"Fill what you know — every field is optional. Tab/⏎ next, esc cancel.",
	)
	body := strings.Join([]string{header, hint, "", form.View()}, "\n")
	return modalBox(width, body)
}

// submitConfirmView is the "about to open <URL>" preview. Mirrors
// the pre-huh submit overlay's pre-open phase: the user sees the
// browser hop coming and can cancel before it fires.
func submitConfirmView(url string, width int) string {
	header := theme.GradientTitle("Submit an app to cliff")

	body := []string{
		header,
		"",
		theme.MutedText.Render("Your answers will prefill a GitHub issue on"),
		theme.MutedText.Render(submit.RegistryRepo + ". The curator picks it up from there;"),
		theme.MutedText.Render("no account setup needed inside cliff."),
		"",
		theme.MutedText.Render("This will open in your browser:"),
		theme.FocusText.Render("  " + truncateURL(url, 72)),
		"",
		theme.MutedText.Render("⏎ open in browser     esc cancel"),
	}
	return modalBox(width, strings.Join(body, "\n"))
}

// submitOpenedView is the post-hand-off state: success message, or
// the URL printed for manual paste if browser.Open errored.
func submitOpenedView(url string, err error, width int) string {
	header := theme.GradientTitle("Submit an app to cliff")

	body := []string{header, ""}

	if err != nil {
		body = append(body,
			theme.ErrorText.Render("× Couldn't open your browser: "+err.Error()),
			"",
			theme.MutedText.Render("Paste this URL into any browser:"),
			theme.FocusText.Render("  "+url),
			"",
			theme.MutedText.Render("⏎ or esc close"),
		)
		return modalBox(width, strings.Join(body, "\n"))
	}

	body = append(body,
		theme.OKText.Render("✓ Opened in your browser."),
		"",
		theme.MutedText.Render("Finish the form there — the curator gets an issue"),
		theme.MutedText.Render("with your submission and picks it up on the next"),
		theme.MutedText.Render("registry review."),
		"",
		theme.MutedText.Render("Thanks for sending one in."),
		"",
		theme.MutedText.Render("⏎ or esc close"),
	)
	return modalBox(width, strings.Join(body, "\n"))
}

// submitFormWidth derives the form's inner width from the screen
// width by mirroring modalBox's clamp (40..90, minus borders and
// padding) so huh lays out into exactly the column the modal will
// render. Keeping the math here means modalBox internals stay
// private to overlay_install.go.
func submitFormWidth(screenWidth int) int {
	maxW := screenWidth - 8
	if maxW < 40 {
		maxW = 40
	}
	if maxW > 90 {
		maxW = 90
	}
	// Subtract the modal's horizontal padding (3 each side) and the
	// rounded-border columns (1 each) so the form's content fits.
	inner := maxW - (3*2 + 1*2)
	if inner < 30 {
		inner = 30
	}
	return inner
}

// submitFormHeight gives huh a height budget for its group layout.
// Generous so the 4-line notes textarea has room without forcing
// scroll; clamped so a tall terminal doesn't make the form sprawl.
func submitFormHeight(screenHeight int) int {
	h := screenHeight - 8
	if h < 14 {
		h = 14
	}
	if h > 24 {
		h = 24
	}
	return h
}

// truncateURL shortens a long URL for display in the confirm preview,
// keeping the scheme+host intact and truncating the (typically query-
// heavy) tail. Display only — the actual browser.Open call uses the
// untruncated string.
func truncateURL(u string, max int) string {
	if len(u) <= max {
		return u
	}
	if max < 4 {
		return u[:max]
	}
	return u[:max-1] + "…"
}
