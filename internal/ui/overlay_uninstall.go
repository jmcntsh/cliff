package ui

import (
	"fmt"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/install"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/bubbles/viewport"
)

// uninstallConfirmView is the modal shown after `u` from the grid or
// readme. It surfaces the literal uninstall command that will run —
// same honesty principle as installConfirmView (CLAUDE.md §3).
//
// When no uninstall recipe can be derived (script-type installs
// without an author-provided [uninstall] block), we say so plainly and
// point at the CLI's equivalent message so the user knows this isn't
// a cliff bug, just a gap in the manifest.
func uninstallConfirmView(app *catalog.App, width int) string {
	if app == nil {
		return modalBox(width,
			theme.WarnText.Render("No app selected")+"\n\n"+
				theme.MutedText.Render("esc close"))
	}

	cmd := app.UninstallCommand()
	if cmd == "" {
		typeLabel := "unknown"
		if app.InstallSpec != nil && app.InstallSpec.Type != "" {
			typeLabel = app.InstallSpec.Type
		}
		body := []string{
			theme.WarnText.Render("Can't uninstall " + app.Name),
			"",
			theme.MutedText.Render(fmt.Sprintf("cliff has no uninstall recipe for install type: %s", typeLabel)),
		}
		if typeLabel == "script" {
			body = append(body,
				theme.MutedText.Render("Script-type apps need an author-provided [uninstall] block"),
				theme.MutedText.Render("in the registry manifest."),
			)
		}
		body = append(body, "", theme.MutedText.Render("esc close"))
		return modalBox(width, strings.Join(body, "\n"))
	}

	header := theme.AccentBold.Render("Uninstall ") +
		theme.FocusText.Render(app.Name) +
		theme.AccentBold.Render("?")
	subline := theme.MutedText.Render(cmd)

	body := []string{
		header,
		subline,
		"",
		theme.MutedText.Render("⏎ run     esc cancel"),
	}
	return modalBox(width, strings.Join(body, "\n"))
}

// uninstallRunningView is the running/streaming modal for uninstall.
// Identical shape to installRunningView so the muscle memory carries
// over; only the verb in the header changes.
func uninstallRunningView(app *catalog.App, vp viewport.Model, hasOutput bool, width int) string {
	header := theme.AccentBold.Render("Uninstalling ") + theme.FocusText.Render(app.Name)

	outputBlock := renderLogViewport(vp, hasOutput, "(starting…)")

	body := []string{
		header,
		"",
		theme.MutedText.Render(app.UninstallCommand()),
		"",
		outputBlock,
		"",
		theme.MutedText.Render("↑↓/jk scroll  esc cancel"),
	}
	return modalBox(width, strings.Join(body, "\n"))
}

// uninstallResultView is the terminal modal for uninstall. Unlike
// install's result view there's no follow-up action — the app is (or
// isn't) gone, and ⏎/esc both dismiss. We still surface Diagnose hints
// for recognized failures (e.g. "brew isn't installed") so "uninstall
// failed because the manager is missing" isn't just a raw exit code.
func uninstallResultView(res *install.Result, vp viewport.Model, width int) string {
	if res == nil {
		return modalBox(width, "no result")
	}
	app := res.App

	var status string
	appName := "app"
	if app != nil {
		appName = app.Name
	}
	if res.Err == nil {
		status = theme.OKText.Render("✓ Uninstalled " + appName)
	} else {
		status = theme.ErrorText.Render(fmt.Sprintf("✗ Uninstall failed (exit %d)", res.ExitCode))
	}

	hasOutput := strings.TrimSpace(res.Output) != ""
	outputBlock := renderLogViewport(vp, hasOutput, "(no output)")

	body := []string{status}
	if hint := install.Diagnose(*res); hint != "" {
		body = append(body,
			"",
			theme.WarnText.Render(hint),
		)
	}
	body = append(body,
		"",
		theme.MutedText.Render(res.Command),
		"",
		outputBlock,
		"",
		theme.MutedText.Render("↑↓/jk scroll  pgup/pgdn page  ⏎ or esc close"),
	)
	return modalBox(width, strings.Join(body, "\n"))
}
