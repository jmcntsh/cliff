package ui

import (
	"fmt"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/install"
	"github.com/jmcntsh/cliff/internal/ui/theme"

	"github.com/charmbracelet/bubbles/viewport"
)

// manageView draws the small horizontal picker shown when ⏎ is pressed
// on an already-installed app. Three actions sit side-by-side, with the
// focused one bracketed; arrow keys (or h/l) move the cursor, ⏎ runs
// the selected action. Disabled actions (e.g. "Update" when the app
// has no upgrade recipe) render in a muted tone and are skipped over
// by the cursor navigation logic in updateManage.
//
// Design rationale: a horizontal row keeps the modal compact and makes
// the picker feel lightweight — this is not a menu, it's a prompt with
// two-or-three choices. Update is the default (leftmost, benign);
// Uninstall is deliberately not the default because it's destructive.
func manageView(app *catalog.App, actions []manageAction, cursor int, width int) string {
	if app == nil || len(actions) == 0 {
		return modalBox(width,
			theme.WarnText.Render("Nothing to manage")+"\n\n"+
				theme.MutedText.Render("esc close"))
	}

	header := theme.AccentBold.Render("Manage ") + theme.FocusText.Render(app.Name)
	subline := theme.MutedText.Render("Already installed — what would you like to do?")

	// Render each action as either "[ label ]" (focused, enabled),
	// "  label  " (unfocused, enabled), or "  label  " in a muted
	// tone (disabled). Equal padding on both sides regardless of
	// focus keeps the row from jittering horizontally as the cursor
	// moves.
	var cells []string
	for i, a := range actions {
		label := a.label
		switch {
		case i == cursor && a.enabled:
			cells = append(cells, theme.AccentBold.Render("[ "+label+" ]"))
		case a.enabled:
			cells = append(cells, theme.MutedText.Render("  "+label+"  "))
		default:
			// Disabled: dim it further so it clearly reads as "not
			// available" rather than "just unfocused". The cursor
			// navigation in updateManage skips these.
			cells = append(cells, theme.MutedItalic.Render("  "+label+"  "))
		}
	}
	row := strings.Join(cells, "  ")

	// Per-action context line below the picker explains what the
	// focused action will actually do. Copied from the underlying
	// commands so it stays honest (per CLAUDE.md §3).
	var context string
	if cursor >= 0 && cursor < len(actions) {
		a := actions[cursor]
		if !a.enabled {
			context = theme.MutedItalic.Render(manageDisabledReason(a.kind, app))
		} else {
			switch a.kind {
			case manageUpdate:
				context = theme.MutedText.Render(app.UpgradeCommand())
			case manageUninstall:
				context = theme.MutedText.Render(app.UninstallCommand())
			case manageReadme:
				context = theme.MutedText.Render("Re-read the app's README.")
			}
		}
	}

	body := []string{
		header,
		subline,
		"",
		row,
		"",
		context,
		"",
		theme.MutedText.Render("←→ move · ⏎ go · esc cancel"),
	}
	return modalBox(width, strings.Join(body, "\n"))
}

// manageDisabledReason is a short explanation rendered under the
// picker when the focused action is disabled. Keeps the "why can't I
// pick this?" answer one glance away rather than hiding it behind
// "nothing happens when I press ⏎."
func manageDisabledReason(kind manageKind, app *catalog.App) string {
	switch kind {
	case manageUpdate:
		typeLabel := "unknown"
		if app != nil && app.InstallSpec != nil && app.InstallSpec.Type != "" {
			typeLabel = app.InstallSpec.Type
		}
		return fmt.Sprintf("No upgrade recipe available for install type: %s.", typeLabel)
	case manageUninstall:
		typeLabel := "unknown"
		if app != nil && app.InstallSpec != nil && app.InstallSpec.Type != "" {
			typeLabel = app.InstallSpec.Type
		}
		return fmt.Sprintf("No uninstall recipe available for install type: %s.", typeLabel)
	default:
		return "This action isn't available."
	}
}

// manageActionsFor builds the picker's action list for the given app,
// flagging each entry as enabled based on whether the manifest
// actually supports it. Readme is always enabled (the fallback). The
// cursor starts on the first enabled slot — usually Update — but
// falls through to the next enabled one if Update isn't available.
func manageActionsFor(app *catalog.App) ([]manageAction, int) {
	actions := []manageAction{
		{kind: manageUpdate, label: "Update", enabled: app != nil && app.UpgradeCommand() != ""},
		{kind: manageUninstall, label: "Uninstall", enabled: app != nil && app.UninstallCommand() != ""},
		{kind: manageReadme, label: "Readme", enabled: true},
	}
	for i, a := range actions {
		if a.enabled {
			return actions, i
		}
	}
	// All disabled shouldn't happen (Readme is always enabled), but
	// handle the degenerate case by pointing cursor at 0 anyway.
	return actions, 0
}

// upgradeConfirmView is the modal shown after choosing "Update" from
// the manage picker (or pressing the direct `U` keybind). Surfaces the
// literal command per CLAUDE.md §3. When no upgrade recipe exists, we
// say so — the manage picker should have already dimmed the Update
// option, but this guards the direct-keybind path.
func upgradeConfirmView(app *catalog.App, width int) string {
	if app == nil {
		return modalBox(width,
			theme.WarnText.Render("No app selected")+"\n\n"+
				theme.MutedText.Render("esc close"))
	}

	cmd := app.UpgradeCommand()
	if cmd == "" {
		typeLabel := "unknown"
		if app.InstallSpec != nil && app.InstallSpec.Type != "" {
			typeLabel = app.InstallSpec.Type
		}
		body := []string{
			theme.WarnText.Render("Can't upgrade " + app.Name),
			"",
			theme.MutedText.Render(fmt.Sprintf("cliff has no upgrade recipe for install type: %s", typeLabel)),
		}
		if typeLabel == "script" {
			body = append(body,
				theme.MutedText.Render("Script-type apps need an author-provided [upgrade] block"),
				theme.MutedText.Render("in the registry manifest."),
			)
		}
		body = append(body, "", theme.MutedText.Render("esc close"))
		return modalBox(width, strings.Join(body, "\n"))
	}

	header := theme.AccentBold.Render("Update ") +
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

// upgradeRunningView is the streaming-logs modal for an in-flight
// upgrade. Same shape as installRunningView / uninstallRunningView;
// only the verb in the header changes.
func upgradeRunningView(app *catalog.App, vp viewport.Model, hasOutput bool, width int) string {
	header := theme.AccentBold.Render("Updating ") + theme.FocusText.Render(app.Name)

	outputBlock := renderLogViewport(vp, hasOutput, "(starting…)")

	body := []string{
		header,
		"",
		theme.MutedText.Render(app.UpgradeCommand()),
		"",
		outputBlock,
		"",
		theme.MutedText.Render("↑↓/jk scroll  esc cancel"),
	}
	return modalBox(width, strings.Join(body, "\n"))
}

// upgradeResultView is the terminal modal for upgrade. Unlike
// installResultView, there's no launcher follow-up — the app was
// already on $PATH before the upgrade (we're only here because it was
// installed), so the "open in new tab" affordance isn't the load-
// bearing finish line it is for a fresh install. ⏎/esc both dismiss.
func upgradeResultView(res *install.Result, vp viewport.Model, width int) string {
	if res == nil {
		return modalBox(width, "no result")
	}
	app := res.App

	appName := "app"
	if app != nil {
		appName = app.Name
	}

	var status string
	if res.Err == nil {
		status = theme.OKText.Render("✓ Updated " + appName)
	} else {
		status = theme.ErrorText.Render(fmt.Sprintf("✗ Update failed (exit %d)", res.ExitCode))
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
