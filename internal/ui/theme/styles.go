package theme

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// Colors are AdaptiveColor pairs (Light, Dark) so the UI stays readable
// on both light and dark terminal backgrounds. ColorAccent (pink) is kept
// chromatic on both sides — it has enough contrast against either.
//
// The Light values are tuned for a near-white background; the Dark values
// are tuned for a near-black background. If you're testing on a mid-gray
// terminal, both variants should still read.
// Adaptive colors that fail catastrophically across themes — pure
// black/white pairs (Focus), near-black/near-white (Text), and the
// only Background-target (Panel) — get gated by recordingSafe. Under
// REEL_RECORDING=1 (set automatically by `reel record` for the
// spawned subshell) they drop to NoColor so the viewer's terminal
// renders them with its own default fg/bg. The viewer's defaults are
// readable against the viewer's bg by definition, so a reel recorded
// in light mode plays correctly in dark mode and vice versa.
//
// Other adaptive values (Muted, Dim, Border, semantic colors) survive
// theme flip with reduced contrast but no readability cliff, so they
// stay as RGB AdaptiveColor pairs for live-use fidelity.
// Palette anchored on Charm's brand colors so cliff sits visually next
// to the rest of the ecosystem (Glow, Glamour-rendered docs, gum, huh,
// VHS recordings) rather than adjacent to it. Source-of-truth values
// come from charmbracelet/x/colors and charmbracelet/huh's ThemeCharm:
//
//   - Fuchsia (#EE6FF8) — the Charm pink, used as the primary accent
//     for titles, focus, and selection prefixes.
//   - Indigo  (#5A56E0 / #7571F9) — used as the gradient endpoint on
//     the title and as the focus-ring color on cards/modals.
//   - Cream   (#FFFDF5) — high-contrast text on the fuchsia button
//     fill (matches huh).
//
// All colors stay AdaptiveColor pairs so cliff is readable on light
// terminals; the dark-side values are the canonical brand hexes.
var (
	ColorAccent    = adaptive("#C13EBC", "#EE6FF8")
	ColorAccentAlt = adaptive("#5A56E0", "#7571F9")
	// ColorAccentMid is the HCL midpoint between Accent (Charm
	// fuchsia) and AccentAlt (Charm indigo), pre-computed at the
	// dark-side hex pair so it can be used as the right and bottom
	// edges of multi-color borders. Without a midpoint, two-color
	// 4-edge borders show abrupt transitions at the corners (top
	// fuchsia meeting right indigo, etc.). Routing the right and
	// bottom edges through this midpoint smooths the diagonal so
	// the border reads as a continuous fuchsia→indigo gradient
	// rather than two solid halves.
	ColorAccentMid = adaptive("#8E4ACE", "#B070F0")
	ColorFocus     = recordingSafe("#000000", "#ffffff")
	ColorText      = recordingSafe("#1a1a1a", "#e5e5e5")
	ColorMuted     = adaptive("#6a6a6a", "#9a9a9a")
	ColorDim       = adaptive("#8a8a8a", "#5a5a5a")
	ColorBorder    = adaptive("#c0c0c0", "#3a3a3a")
	ColorPanel     = recordingSafe("#eeeeee", "236")

	ColorOK    = adaptive("#04B575", "#02BF87")
	ColorWarn  = adaptive("#a06a00", "#e5b567")
	ColorError = adaptive("#FF4672", "#ED567A")
	ColorStar  = adaptive("#8a6a1f", "#d4a847")
)

// adaptive returns an AdaptiveColor under normal use, but collapses
// to a static Color when CLIFF_THEME forces a side. This is the
// escape hatch for terminals that don't expose COLORFGBG (Apple
// Terminal often doesn't set it for default profiles), where lipgloss
// would otherwise pick the light branch and wash out the brand
// fuchsia. CLIFF_THEME=dark|light wins; anything else (or unset)
// keeps the AdaptiveColor and lets lipgloss decide.
func adaptive(light, dark string) lipgloss.TerminalColor {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CLIFF_THEME"))) {
	case "dark":
		return lipgloss.Color(dark)
	case "light":
		return lipgloss.Color(light)
	}
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

func recordingSafe(light, dark string) lipgloss.TerminalColor {
	if os.Getenv("REEL_RECORDING") != "" {
		return lipgloss.NoColor{}
	}
	return adaptive(light, dark)
}

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

	Warn = lipgloss.NewStyle().Foreground(ColorWarn)

	ErrorText = lipgloss.NewStyle().Foreground(ColorError).Bold(true)

	OKText = lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
)

// GradientTitle renders s with a per-character fuchsia→indigo gradient,
// the same brand-pop treatment Charm uses on its own marquees. The
// gradient is applied only to the first space- or "·"-separated
// segment so titles like "cliff · 12 / 44 apps · stars ↓" still read
// cleanly: the leading word pops, the metadata stays muted.
//
// Under REEL_RECORDING=1 we fall back to a flat accent render so the
// recorded frames don't bake in 16+ distinct foreground colors that
// the reel format would have to roundtrip per-cell. The visual
// difference is small for a one-word marquee.
func GradientTitle(s string) string {
	// Pass a phase past the torch's right-edge overshoot so all
	// runes are rendered in steady-state. 1.0 alone would leave the
	// last rune still under the torch (cream-bright); 2.0 puts the
	// torch well off the right side so every rune is post-torch.
	return GradientTitlePhase(s, 2.0)
}

// GradientTitlePhase is the animation entry point. phase is in [0,1]:
//
//   - 0.0   — pre-launch: head invisible (NoColor on terminal bg)
//   - 0..1  — sweep: a bright cream "torch" slides left-to-right
//             across the head, leaving behind chars colored at their
//             final gradient slot. Chars ahead of the torch are dim
//             (terminal default), chars under the torch are cream-
//             white-bright, chars behind are at brand fuchsia/indigo.
//   - 1.0   — steady state: the static fuchsia→indigo gradient with
//             every char at its final slot.
//
// The torch metaphor is the trick: a light source moving across the
// word "lights up" each glyph as it passes, so you actually see the
// brand mark being drawn rather than just fading in. The phase value
// is consumed unchanged here; easing happens in Root before this is
// called.
func GradientTitlePhase(s string, phase float64) string {
	if s == "" {
		return ""
	}
	if os.Getenv("REEL_RECORDING") != "" {
		return TitleStyle.Render(s)
	}
	if phase < 0 {
		phase = 0
	}
	// Phase upper bound is 2.0 instead of 1.0: callers driving the
	// launch animation walk phase up to titleTickEnd (1.2) so the
	// torch has room to sweep off the right edge of the word, and
	// the steady-state shortcut passes 2.0 so every rune is fully
	// post-torch. Capping at 1.0 here would re-introduce the cream
	// bright on the last rune in steady state.
	if phase > 2 {
		phase = 2
	}

	head, tail := splitTitleHead(s)
	rendered := renderTorchSweep(head, phase)
	if tail == "" {
		return rendered
	}
	return rendered + MutedText.Render(tail)
}

// splitTitleHead pulls off the leading segment up to the first " · "
// separator. If there's no separator, the whole string is the head.
func splitTitleHead(s string) (head, tail string) {
	const sep = " · "
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i:]
	}
	return s, ""
}

// gradientStart and gradientEnd are the brand fuchsia and indigo in
// HSL form, which lerps more naturally than RGB. Pre-parsed once so
// renderGradient stays a tight loop.
var (
	gradientStart, _ = colorful.Hex("#EE6FF8") // Charm fuchsia
	gradientEnd, _   = colorful.Hex("#7571F9") // Charm indigo (dark)
)

// renderTorchSweep is the launch animation renderer. For each rune,
// it computes the rune's position in the word as a normalized t in
// [0,1] and compares against the torch position (phase). Three zones:
//
//   - rune ahead of torch (t > phase + halfWidth): not yet lit — dim
//   - rune under torch    (|t - phase| ≤ halfWidth): bright cream
//   - rune behind torch   (t < phase - halfWidth): at its final slot
//                          on the fuchsia→indigo gradient
//
// The torch isn't a single point; it's a soft band whose intensity
// falls off with distance from `phase`. That gives the sweep a glow
// instead of a hard edge, which reads as "lighting" rather than
// "flipping."
//
// At phase ≥ 1.0 + halfWidth, every rune is behind the torch, so
// the function naturally collapses to the steady-state gradient.
// Callers don't need to special-case the end-of-animation frame.
func renderTorchSweep(s string, phase float64) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	n := len(runes)
	var b strings.Builder
	b.Grow(len(s) * 12)

	for i, r := range runes {
		t := 0.0
		if n > 1 {
			t = float64(i) / float64(n-1)
		}

		// Distance from the torch center. The torch starts off the
		// left side at phase=0, sweeps to phase=1 (right edge), and
		// continues out to phase=1+halfWidth so every rune gets a
		// final "torch passed me" frame before settling.
		const halfWidth = 0.18
		dist := t - phase

		var c colorful.Color
		switch {
		case dist > halfWidth:
			// Ahead of torch: render in muted gradient color so the
			// word is faintly visible while the torch approaches.
			// Without this, pre-torch chars are invisible and the
			// word "appears from nothing" — less satisfying than
			// "fills in from dim."
			final := gradientStart.BlendHcl(gradientEnd, t).Clamped()
			c = blendToward(final, dimGradient, 0.7)
		case dist < -halfWidth:
			// Behind torch: final gradient slot.
			c = gradientStart.BlendHcl(gradientEnd, t).Clamped()
		default:
			// Under torch: blend from final-slot color toward bright
			// cream proportionally to closeness to torch center.
			// closeness = 1.0 at center, 0.0 at edge of band.
			closeness := 1.0 - abs(dist)/halfWidth
			final := gradientStart.BlendHcl(gradientEnd, t).Clamped()
			c = final.BlendHcl(torchHot, closeness).Clamped()
		}

		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(c.Hex())).
			Bold(true).
			Render(string(r)))
	}
	return b.String()
}

// blendToward is BlendHcl with a clearer call-site name for the
// "fade toward a third color" use case.
func blendToward(c, target colorful.Color, amount float64) colorful.Color {
	return c.BlendHcl(target, amount).Clamped()
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

var (
	// torchHot is the bright cream the sweep highlights with.
	// Matches huh's "cream" button-text color so the bright frame
	// reads as Charm-brand rather than as a generic white flash.
	torchHot, _ = colorful.Hex("#FFFDF5")

	// dimGradient is the "not yet lit" color for chars ahead of
	// the torch — a muted neutral that the actual fuchsia/indigo
	// gradient blends toward. Picking a dark gray keeps the
	// pre-torch frame visible without competing with the brand
	// colors that arrive a moment later.
	dimGradient, _ = colorful.Hex("#5A5A5A")
)
