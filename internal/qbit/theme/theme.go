// Package theme is the single source of qbit-jay's visual tokens (spec S14).
//
// Every colour, font stack, radius, spacing step and motion curve is declared
// here once. The TUI reads the colours through Lip Gloss (Pair.Adaptive) and
// the web dashboard reads the same values as CSS custom properties (CSS).
//
// The values come from design/system.md: the qbitdev tokens are copied
// unchanged, the qbit-jay additions are marked as such there. A role whose
// Source is empty is a qbit-jay extension that design/system.md does not list
// (text-safe variants and the error colour, which the brand has no token for);
// the tests still check their WCAG AA contrast.
package theme

import "github.com/charmbracelet/lipgloss"

// Pair holds one role's value for the dark and the light theme, plus the
// design/system.md token each value is copied from (empty for an extension).
type Pair struct {
	Dark, Light             string
	DarkSource, LightSource string
}

// Adaptive returns the Lip Gloss colour that follows the terminal background.
func (p Pair) Adaptive() lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: p.Light, Dark: p.Dark}
}

// Color returns the Lip Gloss colour for one theme, ignoring the terminal.
func (p Pair) Color(dark bool) lipgloss.Color {
	if dark {
		return lipgloss.Color(p.Dark)
	}
	return lipgloss.Color(p.Light)
}

// same builds a Pair whose value is identical in both themes.
func same(value, source string) Pair {
	return Pair{Dark: value, Light: value, DarkSource: source, LightSource: source}
}

// Colour roles. Names follow the role, not the hue; the CSS variable is
// --qj-<css name> (see Roles).
var (
	// Surfaces and lines.
	Bg    = Pair{"#04070D", "#F4F8FC", "--qb-bg-deep", "--qb-light-bg"}
	Panel = Pair{"#0B121B", "#FFFFFF", "--qb-panel", "--qb-light-panel"}
	Line  = Pair{"#16222F", "#D6E0EA", "--qb-line-dark", "--qb-light-line"}

	// Text.
	Ink   = Pair{"#E8F2FF", "#0B1521", "--qb-ink", "--qb-light-ink"}
	Muted = Pair{"#8DA2B8", "#4F6275", "--qb-ink-muted", "--qb-light-ink-muted"}

	// Accents. In light mode cyan and green are lines only (2.7:1).
	Blue  = Pair{"#2D7FFF", "#1F6FEB", "--qb-blue", "--qb-light-blue"}
	Cyan  = Pair{"#35E0FF", "#0FA6C7", "--qb-cyan", "--qb-light-cyan"}
	Green = Pair{"#3DFF8B", "#17B36A", "--qb-green", "--qb-light-green"}
	Teal  = Pair{Dark: "#1FB6C9", Light: "#0FA6C7", DarkSource: "--qb-teal", LightSource: "--qb-light-cyan"}
	White = Pair{"#FFFFFF", "#FFFFFF", "--qb-white", "--qb-white"}

	// Extensions (not in design/system.md): accent text that passes AA on
	// both light surfaces (--qb-light-blue is 4.3:1 on --qb-light-bg), and
	// text on a filled accent.
	AccentText = Pair{Dark: "#35E0FF", Light: "#1A5FCF", DarkSource: "--qb-cyan"}
	OnAccent   = Pair{Dark: "#04070D", Light: "#FFFFFF", DarkSource: "--qb-bg-deep", LightSource: "--qb-white"}

	// Semantic roles mapped from tokens. The brand has no red, so Error is
	// an extension; Success text in light mode is a darker green because
	// --qb-light-green is lines only.
	Success = Pair{Dark: "#3DFF8B", Light: "#0B7A45", DarkSource: "--qb-green"}
	Warn    = Pair{"#D9923A", "#8A5427", "--jay-acorn", "--jay-acorn-cap"}
	Error   = Pair{Dark: "#FF6B7A", Light: "#B3261E"}
)

// Jay colours (qbit-jay additions). Same in both themes.
var (
	JayBlue     = same("#2F6FDB", "--jay-blue")
	JayBlueDeep = same("#1B459E", "--jay-blue-deep")
	JayBlueSky  = same("#5B9BFF", "--jay-blue-sky")
	JayMask     = same("#070B12", "--jay-mask")
	JayBeak     = same("#1C2633", "--jay-beak")
	JayBeakHi   = same("#3A4A5E", "--jay-beak-hi")
	JayFace     = same("#F2F7FD", "--jay-face")
	JayBelly    = same("#B8C5D6", "--jay-belly")
	JayAcorn    = same("#D9923A", "--jay-acorn")
	JayAcornCap = same("#8A5427", "--jay-acorn-cap")
	JayBark     = same("#5B3A1E", "--jay-bark")
)

// Role names one colour role for CSS output and tests.
type Role struct {
	CSS  string // variable name without the --qj- prefix
	Pair Pair
	// ModeIndependent roles are emitted once in :root, not per theme.
	ModeIndependent bool
}

// Roles lists every colour role in output order.
var Roles = []Role{
	{CSS: "bg", Pair: Bg},
	{CSS: "panel", Pair: Panel},
	{CSS: "line", Pair: Line},
	{CSS: "ink", Pair: Ink},
	{CSS: "muted", Pair: Muted},
	{CSS: "blue", Pair: Blue},
	{CSS: "cyan", Pair: Cyan},
	{CSS: "green", Pair: Green},
	{CSS: "teal", Pair: Teal},
	{CSS: "white", Pair: White, ModeIndependent: true},
	{CSS: "accent-text", Pair: AccentText},
	{CSS: "on-accent", Pair: OnAccent},
	{CSS: "success", Pair: Success},
	{CSS: "warn", Pair: Warn},
	{CSS: "error", Pair: Error},
	{CSS: "jay-blue", Pair: JayBlue, ModeIndependent: true},
	{CSS: "jay-blue-deep", Pair: JayBlueDeep, ModeIndependent: true},
	{CSS: "jay-blue-sky", Pair: JayBlueSky, ModeIndependent: true},
	{CSS: "jay-mask", Pair: JayMask, ModeIndependent: true},
	{CSS: "jay-beak", Pair: JayBeak, ModeIndependent: true},
	{CSS: "jay-beak-hi", Pair: JayBeakHi, ModeIndependent: true},
	{CSS: "jay-face", Pair: JayFace, ModeIndependent: true},
	{CSS: "jay-belly", Pair: JayBelly, ModeIndependent: true},
	{CSS: "jay-acorn", Pair: JayAcorn, ModeIndependent: true},
	{CSS: "jay-acorn-cap", Pair: JayAcornCap, ModeIndependent: true},
	{CSS: "jay-bark", Pair: JayBark, ModeIndependent: true},
}

// Other tokens copied from design/system.md (source token -> value).
// The dashboard emits them as --qj-<css name>.
type Scalar struct {
	CSS    string
	Value  string
	Source string // design/system.md token, empty for a qbit-jay decision
}

// Typography: font-family stacks for the web (qbitdev values).
var Fonts = []Scalar{
	{"font-display", `"Outfit", system-ui, sans-serif`, "--qb-font-display"},
	{"font-body", `"Inter", system-ui, sans-serif`, "--qb-font-body"},
	{"font-mono", `"JetBrains Mono", ui-monospace, monospace`, "--qb-font-mono"},
}

// Radius (qbitdev values).
var Radius = []Scalar{
	{"radius-field", "12px", "--qb-radius-field"},
	{"radius-card", "20px", "--qb-radius-card"},
	{"radius-pill", "999px", "--qb-radius-pill"},
}

// Spacing: design/system.md defines no spacing tokens, so this 4 px based
// scale is a qbit-jay decision (no Source).
var Spacing = []Scalar{
	{"space-1", "4px", ""},
	{"space-2", "8px", ""},
	{"space-3", "12px", ""},
	{"space-4", "16px", ""},
	{"space-5", "24px", ""},
	{"space-6", "32px", ""},
	{"space-7", "48px", ""},
}

// Motion easings (qbitdev values).
var Motion = []Scalar{
	{"ease-standard", "cubic-bezier(0.2, 0.8, 0.2, 1)", "--qb-ease-standard"},
	{"ease-enter", "cubic-bezier(0, 0, 0.2, 1)", "--qb-ease-enter"},
	{"ease-exit", "cubic-bezier(0.4, 0, 1, 1)", "--qb-ease-exit"},
	{"ease-sine", "cubic-bezier(0.45, 0, 0.55, 1)", "--qb-ease-sine"},
}

// Per-theme effects. Light mode has no glow (brand rule); its emphasis is the
// flat --qb-light-blue.
var (
	GradientDark  = Scalar{"gradient", "linear-gradient(135deg, #2D7FFF 0%, #35E0FF 50%, #3DFF8B 100%)", "--qb-gradient"}
	GradientLight = Scalar{"gradient", "#1F6FEB", "--qb-light-blue"}
	GlowSoftDark  = Scalar{"glow-soft", "0 0 24px rgba(53, 224, 255, 0.33)", "--qb-glow-soft"}
	GlowStrongDk  = Scalar{"glow-strong", "0 0 18px #35E0FF, 0 0 48px rgba(53, 224, 255, 0.4)", "--qb-glow-strong"}
	GlowNone      = "none"
)

// Brand strings shared by every surface.
const (
	// Name is the lowercase wordmark; never split or capitalised in UI chrome.
	Name = "qbit-jay"
	// Mark is the text-mode brand mark used where the isotype cannot be drawn.
	Mark = "◆"
	// Tagline is the line under the wordmark.
	Tagline = "by qbitdev"
)
