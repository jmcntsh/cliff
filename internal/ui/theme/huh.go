package theme

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// HuhTheme returns a huh form theme that matches cliff's palette so
// in-app forms (today: submit) read as part of the same surface as
// the catalog rather than as a bolted-on widget. We start from
// huh.ThemeCharm (already fuchsia/indigo-flavored) and overwrite the
// color tokens to use cliff's adaptive variants — that way the form
// honors CLIFF_THEME=dark|light the same way the rest of the UI does.
//
// Notes on the choices below:
//
//   - Field titles use ColorAccent (Charm fuchsia). They're the
//     headings the user actually scans for, so they get the brand pop.
//   - The thick left-edge border on the focused field uses
//     ColorAccentAlt (indigo). Pairing fuchsia titles with an indigo
//     focus rail mirrors the per-edge gradient on the rest of the
//     modal borders.
//   - Description text uses ColorMuted, identical to MutedText —
//     keeps secondary copy readable but not competing with the title.
//   - Buttons (Submit/Next) get the cream-on-fuchsia treatment from
//     ThemeCharm; the focused-button background pulls our ColorAccent
//     so it's the same exact pink as the rest of the brand.
//   - Errors use ColorError so a validation failure here looks like
//     an install failure or any other red diagnostic in cliff.
func HuhTheme() *huh.Theme {
	t := huh.ThemeBase()

	cream := lipgloss.Color("#FFFDF5")

	t.Focused.Base = t.Focused.Base.BorderForeground(ColorAccentAlt)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(ColorAccent).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.
		Foreground(ColorAccent).
		Bold(true).
		MarginBottom(1)
	t.Focused.Description = t.Focused.Description.Foreground(ColorMuted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(ColorError)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(ColorError)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(ColorAccent)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(ColorAccent)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(ColorAccent)
	t.Focused.Option = t.Focused.Option.Foreground(ColorText)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(ColorAccent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(ColorOK)
	t.Focused.SelectedPrefix = lipgloss.NewStyle().
		Foreground(ColorOK).
		SetString("✓ ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().
		Foreground(ColorMuted).
		SetString("• ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(ColorText)
	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Foreground(cream).
		Background(ColorAccent)
	t.Focused.Next = t.Focused.FocusedButton
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Foreground(ColorMuted).
		Background(ColorPanel)

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(ColorAccent)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(ColorDim)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(ColorAccent)

	// Blurred mirrors focused but drops the visible left rail so only
	// the field the user is currently in shows the indigo bar; the
	// others sit flush. Matches huh.ThemeCharm's structural choice.
	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description

	return t
}
