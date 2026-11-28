package theme

import (
	"fmt"
	"strings"
)

// Prefix is the CSS custom-property prefix for every token.
const Prefix = "--qj-"

func decl(b *strings.Builder, name, value string) {
	fmt.Fprintf(b, "\t%s%s: %s;\n", Prefix, name, value)
}

func scalars(b *strings.Builder, list []Scalar) {
	for _, s := range list {
		decl(b, s.CSS, s.Value)
	}
}

// themeBlock writes the mode-dependent tokens for one theme.
func themeBlock(b *strings.Builder, dark bool) {
	for _, r := range Roles {
		if r.ModeIndependent {
			continue
		}
		v := r.Pair.Light
		if dark {
			v = r.Pair.Dark
		}
		decl(b, r.CSS, v)
	}
	if dark {
		decl(b, GradientDark.CSS, GradientDark.Value)
		decl(b, GlowSoftDark.CSS, GlowSoftDark.Value)
		decl(b, GlowStrongDk.CSS, GlowStrongDk.Value)
		return
	}
	decl(b, GradientLight.CSS, GradientLight.Value)
	decl(b, GlowSoftDark.CSS, GlowNone)
	decl(b, GlowStrongDk.CSS, GlowNone)
}

// CSS returns the whole token block served to the dashboard. Dark is the
// default; light follows prefers-color-scheme and can be forced with
// data-theme on the root element. Declarations are generated from the Go
// values above, so the stylesheet cannot drift from them.
func CSS() string {
	var b strings.Builder
	b.WriteString("/* Generated from internal/qbit/theme. Do not edit. */\n")

	b.WriteString(":root {\n\tcolor-scheme: dark;\n")
	for _, r := range Roles {
		if r.ModeIndependent {
			decl(&b, r.CSS, r.Pair.Dark)
		}
	}
	scalars(&b, Fonts)
	scalars(&b, Radius)
	scalars(&b, Spacing)
	scalars(&b, Motion)
	themeBlock(&b, true)
	b.WriteString("}\n\n")

	b.WriteString("@media (prefers-color-scheme: light) {\n:root {\n\tcolor-scheme: light;\n")
	themeBlock(&b, false)
	b.WriteString("}\n}\n\n")

	b.WriteString(":root[data-theme=\"dark\"] {\n\tcolor-scheme: dark;\n")
	themeBlock(&b, true)
	b.WriteString("}\n\n")

	b.WriteString(":root[data-theme=\"light\"] {\n\tcolor-scheme: light;\n")
	themeBlock(&b, false)
	b.WriteString("}\n")
	return b.String()
}
