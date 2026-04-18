package theme

import "github.com/charmbracelet/lipgloss"

// Colors are AdaptiveColor pairs (Light, Dark) so the UI stays readable
// on both light and dark terminal backgrounds. ColorAccent (pink) is kept
// chromatic on both sides — it has enough contrast against either.
//
// The Light values are tuned for a near-white background; the Dark values
// are tuned for a near-black background. If you're testing on a mid-gray
// terminal, both variants should still read.
var (
	ColorAccent = lipgloss.AdaptiveColor{Light: "#a83ba0", Dark: "#c586c0"}
	ColorFocus  = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#ffffff"}
	ColorText   = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#e5e5e5"}
	ColorMuted  = lipgloss.AdaptiveColor{Light: "#6a6a6a", Dark: "#9a9a9a"}
	ColorDim    = lipgloss.AdaptiveColor{Light: "#8a8a8a", Dark: "#5a5a5a"}
	ColorBorder = lipgloss.AdaptiveColor{Light: "#c0c0c0", Dark: "#3a3a3a"}
	ColorPanel  = lipgloss.AdaptiveColor{Light: "#eeeeee", Dark: "236"}

	ColorOK    = lipgloss.AdaptiveColor{Light: "#1f7a1f", Dark: "#5fbf5f"}
	ColorWarn  = lipgloss.AdaptiveColor{Light: "#a06a00", Dark: "#e5b567"}
	ColorError = lipgloss.AdaptiveColor{Light: "#a82828", Dark: "#e07a7a"}
	ColorStar  = lipgloss.AdaptiveColor{Light: "#8a6a1f", Dark: "#d4a847"}
)

var languageColors = map[string]lipgloss.Color{
	"Go":           lipgloss.Color("#00ADD8"),
	"Rust":         lipgloss.Color("#DEA584"),
	"Python":       lipgloss.Color("#3572A5"),
	"TypeScript":   lipgloss.Color("#3178C6"),
	"JavaScript":   lipgloss.Color("#F1E05A"),
	"C":            lipgloss.Color("#A8B9CC"),
	"C++":          lipgloss.Color("#F34B7D"),
	"Shell":        lipgloss.Color("#89E051"),
	"Ruby":         lipgloss.Color("#CC342D"),
	"Haskell":      lipgloss.Color("#A972EF"),
	"Zig":          lipgloss.Color("#EC915C"),
	"Lua":          lipgloss.Color("#7D99E0"),
	"Nim":          lipgloss.Color("#FFC200"),
	"Crystal":      lipgloss.Color("#D8D8D8"),
	"Perl":         lipgloss.Color("#0298C3"),
	"OCaml":        lipgloss.Color("#EF7A08"),
	"V":            lipgloss.Color("#5D87BF"),
	"D":            lipgloss.Color("#B03931"),
	"Elixir":       lipgloss.Color("#6E4A7E"),
	"Clojure":      lipgloss.Color("#DB5855"),
	"Scheme":       lipgloss.Color("#1E4AEC"),
	"Racket":       lipgloss.Color("#3C5CAA"),
	"Erlang":       lipgloss.Color("#B83998"),
	"Elm":          lipgloss.Color("#60B5CC"),
	"Kotlin":       lipgloss.Color("#A97BFF"),
	"Swift":        lipgloss.Color("#F05138"),
	"Java":         lipgloss.Color("#B07219"),
	"CoffeeScript": lipgloss.Color("#244776"),
}

func LanguageColor(lang string) lipgloss.TerminalColor {
	if c, ok := languageColors[lang]; ok {
		return c
	}
	return ColorMuted
}

var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	SelectionPrefix = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true).
			Render("▸ ")

	UnselectedPrefix = "  "

	SelectedName = lipgloss.NewStyle().
			Foreground(ColorFocus).
			Bold(true)

	UnselectedName = lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(true)

	Description = lipgloss.NewStyle().
			Foreground(ColorMuted)

	Stars = lipgloss.NewStyle().
		Foreground(ColorStar).
		Bold(true)

	MutedText = lipgloss.NewStyle().Foreground(ColorMuted)

	MutedItalic = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)

	FocusText = lipgloss.NewStyle().Foreground(ColorFocus).Bold(true)

	AccentText = lipgloss.NewStyle().Foreground(ColorAccent)

	AccentBold = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	DimTitle = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)

	InstalledMark = lipgloss.NewStyle().Foreground(ColorOK).Bold(true)

	WarnText = lipgloss.NewStyle().Foreground(ColorWarn).Bold(true)

	ErrorText = lipgloss.NewStyle().Foreground(ColorError).Bold(true)

	OKText = lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
)
