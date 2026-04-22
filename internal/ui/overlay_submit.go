package ui

import (
	"strings"

	"github.com/jmcntsh/cliff/internal/submit"
	"github.com/jmcntsh/cliff/internal/ui/theme"
)

// submitView draws the submit-an-app overlay. Two phases share one
// view, driven by `opened`:
//
//   - !opened: "about to open <url>" preview so the user sees the
//     browser hop coming. Matches the same honesty install-confirm
//     gives about the shell command it's about to run.
//   - opened:  confirmation that we fired browser.Open. If Open
//     itself errored (err != nil), we show the URL so the user can
//     copy it by hand — a failed open-in-browser on a weird headless
//     setup should never leave them stuck.
//
// Split by flag rather than a separate mode because the content flow
// is linear and the only difference is copy + keybinds.
func submitView(url string, opened bool, err error, width int) string {
	header := theme.AccentBold.Render("Submit an app to cliff")

	body := []string{header, ""}

	if !opened {
		body = append(body,
			theme.MutedText.Render("cliff's catalog is a TOML registry on GitHub."),
			theme.MutedText.Render("Submissions go through a short issue form there —"),
			theme.MutedText.Render("no account setup inside cliff, just the GitHub"),
			theme.MutedText.Render("identity you already have."),
			"",
			theme.MutedText.Render("This will open in your browser:"),
			theme.FocusText.Render("  "+truncateURL(url, 72)),
			"",
			theme.MutedText.Render("Opens: github.com/"+submit.RegistryRepo),
			"",
			theme.MutedText.Render("⏎ open in browser     esc cancel"),
		)
		return modalBox(width, strings.Join(body, "\n"))
	}

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
