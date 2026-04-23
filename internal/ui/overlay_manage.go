package ui

import (
	"fmt"
	"strings"

	"github.com/jmcntsh/cliff/internal/catalog"
	"github.com/jmcntsh/cliff/internal/ui/theme"
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

