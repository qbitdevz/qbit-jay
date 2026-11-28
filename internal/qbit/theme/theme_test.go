package theme

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// systemTokens parses the markdown tables of design/system.md into
// token name -> value.
func systemTokens(t *testing.T) map[string]string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "design", "system.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read design/system.md: %v", err)
	}
	row := regexp.MustCompile("(?m)^[|] `(--[a-z0-9-]+)` [|] `([^`]+)` [|]")
	out := map[string]string{}
	for _, m := range row.FindAllStringSubmatch(string(data), -1) {
		out[m[1]] = m[2]
	}
	if len(out) < 40 {
		t.Fatalf("parsed only %d tokens from design/system.md, table format changed?", len(out))
	}
	return out
}

// resolve replaces var(--x) references with the referenced token's value.
func resolve(t *testing.T, tokens map[string]string, v string) string {
	t.Helper()
	ref := regexp.MustCompile(`var\((--[a-z0-9-]+)\)`)
	return ref.ReplaceAllStringFunc(v, func(s string) string {
		name := ref.FindStringSubmatch(s)[1]
		val, ok := tokens[name]
		if !ok {
			t.Fatalf("design/system.md references unknown %s", name)
		}
		return val
	})
}

func TestTokensMatchDesignSystem(t *testing.T) {
	tokens := systemTokens(t)

	check := func(label, source, got string) {
		t.Helper()
		if source == "" {
			return
		}
		want, ok := tokens[source]
		if !ok {
			t.Errorf("%s: source %s is not in design/system.md", label, source)
			return
		}
		want = resolve(t, tokens, want)
		if !strings.EqualFold(got, want) {
			t.Errorf("%s = %q, design/system.md %s = %q", label, got, source, want)
		}
	}

	for _, r := range Roles {
		check(r.CSS+" dark", r.Pair.DarkSource, r.Pair.Dark)
		check(r.CSS+" light", r.Pair.LightSource, r.Pair.Light)
	}
	for _, list := range [][]Scalar{Fonts, Radius, Motion, {GradientDark, GradientLight, GlowSoftDark, GlowStrongDk}} {
		for _, s := range list {
			check(s.CSS, s.Source, s.Value)
		}
	}
}

// Extensions are the roles with no design/system.md source. They are listed
// here on purpose so a new one cannot slip in unnoticed.
func TestExtensionsAreExplicit(t *testing.T) {
	want := map[string]bool{
		"accent-text:light": true, "success:light": true, "error:dark": true, "error:light": true,
	}
	got := map[string]bool{}
	for _, r := range Roles {
		if r.Pair.DarkSource == "" {
			got[r.CSS+":dark"] = true
		}
		if r.Pair.LightSource == "" {
			got[r.CSS+":light"] = true
		}
	}
	for k := range want {
		if !got[k] {
			t.Errorf("expected extension %s", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("unexpected undocumented extension %s", k)
		}
	}
}

func luminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	ch := func(s string) float64 {
		var v int
		for _, c := range s {
			v = v*16 + strings.IndexRune("0123456789abcdef", c|0x20)
		}
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(hex[0:2]) + 0.7152*ch(hex[2:4]) + 0.0722*ch(hex[4:6])
}

func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestTextRolesMeetWCAGAA(t *testing.T) {
	for _, dark := range []bool{true, false} {
		pick := func(p Pair) string {
			if dark {
				return p.Dark
			}
			return p.Light
		}
		for _, surface := range []Pair{Bg, Panel} {
			for name, fg := range map[string]Pair{
				"ink": Ink, "muted": Muted, "accent-text": AccentText,
				"success": Success, "warn": Warn, "error": Error,
			} {
				if got := contrast(pick(fg), pick(surface)); got < 4.5 {
					t.Errorf("dark=%v %s on %s = %.2f:1, want >= 4.5", dark, name, pick(surface), got)
				}
			}
		}
		if got := contrast(pick(OnAccent), pick(Blue)); got < 4.5 {
			t.Errorf("dark=%v on-accent on blue = %.2f:1, want >= 4.5", dark, got)
		}
	}
}

func TestCSSContainsEveryTokenInBothThemes(t *testing.T) {
	css := CSS()
	for _, r := range Roles {
		for _, v := range []string{r.Pair.Dark, r.Pair.Light} {
			decl := Prefix + r.CSS + ": " + v + ";"
			if !strings.Contains(css, decl) {
				t.Errorf("CSS lacks %q", decl)
			}
		}
	}
	for _, list := range [][]Scalar{Fonts, Radius, Spacing, Motion} {
		for _, s := range list {
			if !strings.Contains(css, Prefix+s.CSS+": "+s.Value+";") {
				t.Errorf("CSS lacks %s", s.CSS)
			}
		}
	}
	if !strings.Contains(css, "@media (prefers-color-scheme: light)") {
		t.Error("CSS lacks the light media block")
	}
	if strings.Contains(css, "--engram") {
		t.Error("CSS must not carry upstream engram variables")
	}
	if CSS() != css {
		t.Error("CSS output is not deterministic")
	}
}

func TestLightModeHasNoGlow(t *testing.T) {
	css := CSS()
	i := strings.Index(css, "@media (prefers-color-scheme: light)")
	if i < 0 {
		t.Fatal("no light block")
	}
	light := css[i:]
	if end := strings.Index(light, ":root[data-theme="); end > 0 {
		light = light[:end]
	}
	if strings.Contains(light, "rgba(53, 224, 255") {
		t.Error("light theme must not define a glow")
	}
}
