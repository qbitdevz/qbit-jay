package dashboard

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/qbitdevz/qbit-jay/internal/qbit/theme"
)

func brandMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	Mount(mux, MountConfig{
		RequireSession: func(r *http.Request) error {
			if r.URL.Query().Get("auth") == "ok" {
				return nil
			}
			return errUnauthorized
		},
		IsAdmin:        func(_ *http.Request) bool { return false },
		Store:          parityStoreStub{},
		GetDisplayName: func(_ *http.Request) string { return "operator" },
		Version:        "v0.1.0",
	})
	return mux
}

func get(t *testing.T, mux *http.ServeMux, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, body=%q", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// The pages the dashboard serves must carry the qbit-jay identity (S6) and
// none of Engram's.
func TestPagesCarryQbitJayIdentity(t *testing.T) {
	mux := brandMux(t)
	for _, path := range []string{"/dashboard/login", "/dashboard/?auth=ok"} {
		body := get(t, mux, path)
		for _, want := range []string{
			"qbit-jay Cloud",
			`class="brand-mark`,
			"qj-iso-head",
			`<meta name="color-scheme" content="light dark"`,
			"/dashboard/static/theme.css",
			"/dashboard/static/styles.css",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", path, want)
			}
		}
		for _, banned := range []string{"Engram", "ENGRAM", "elephant", `data-theme="dark"`} {
			if strings.Contains(body, banned) {
				t.Errorf("%s still carries %q", path, banned)
			}
		}
		if strings.Index(body, "theme.css") > strings.Index(body, "styles.css") {
			t.Errorf("%s must load theme.css before styles.css", path)
		}
	}
}

// The tokens the browser gets are the Go package's, byte for byte (S14).
func TestThemeCSSServedFromThemePackage(t *testing.T) {
	mux := brandMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dashboard/static/theme.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("theme.css status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("theme.css content type = %q", ct)
	}
	if got := rec.Body.String(); got != theme.CSS() {
		t.Fatal("served theme.css differs from theme.CSS()")
	}
	for _, r := range theme.Roles {
		for _, v := range []string{r.Pair.Dark, r.Pair.Light} {
			if !strings.Contains(rec.Body.String(), theme.Prefix+r.CSS+": "+v+";") {
				t.Errorf("theme.css lacks %s%s: %s", theme.Prefix, r.CSS, v)
			}
		}
	}
}

// styles.css only consumes tokens: it defines none of --qj-*, it uses only
// ones the theme package emits, and it carries no upstream variables.
func TestStylesOnlyConsumeThemeTokens(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)

	if def := regexp.MustCompile(`(?m)^\s*--qj-[a-z0-9-]+\s*:`).FindString(css); def != "" {
		t.Errorf("styles.css defines a theme token (%q); tokens belong to internal/qbit/theme", strings.TrimSpace(def))
	}
	if strings.Contains(css, "--engram") || strings.Contains(strings.ToLower(css), "engram") {
		t.Error("styles.css still mentions engram")
	}

	emitted := map[string]bool{}
	for _, m := range regexp.MustCompile(`(--qj-[a-z0-9-]+):`).FindAllStringSubmatch(theme.CSS(), -1) {
		emitted[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\((--qj-[a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		if !emitted[m[1]] {
			t.Errorf("styles.css uses %s, which the theme package does not define", m[1])
		}
	}
}

// Fonts: every @font-face file exists, each licence is shipped next to them
// and the total stays small.
func TestEmbeddedFontsShipWithLicencesAndStaySmall(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	urls := regexp.MustCompile(`url\("(fonts/[^"]+\.woff2)"\)`).FindAllStringSubmatch(string(raw), -1)
	if len(urls) < 5 {
		t.Fatalf("expected the brand font files in @font-face rules, found %d", len(urls))
	}
	for _, u := range urls {
		if _, err := fs.Stat(StaticFS, "static/"+u[1]); err != nil {
			t.Errorf("font %s is referenced but not embedded: %v", u[1], err)
		}
	}
	for _, lic := range []string{"outfit", "inter", "jetbrains-mono"} {
		if _, err := fs.Stat(StaticFS, "static/fonts/LICENSE-"+lic+".txt"); err != nil {
			t.Errorf("missing licence for %s: %v", lic, err)
		}
	}
	var total int64
	_ = fs.WalkDir(StaticFS, "static/fonts", func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				total += info.Size()
			}
		}
		return nil
	})
	if total > 300*1024 {
		t.Errorf("embedded fonts weigh %d bytes, limit is 300 KB", total)
	}
}

// The inline isotype and the favicon are the brand asset, not a redraw.
func TestBrandMarkMatchesAsset(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	asset := filepath.Join(filepath.Dir(file), "..", "..", "..", "assets", "branding", "qbit-jay-isotype-dark.svg")
	raw, err := os.ReadFile(asset)
	if err != nil {
		t.Fatal(err)
	}
	poly := regexp.MustCompile(`<polygon points="[^"]+"`)
	want := poly.FindAllString(string(raw), -1)
	if len(want) < 15 {
		t.Fatalf("asset parsing found %d polygons", len(want))
	}
	body := get(t, brandMux(t), "/dashboard/login")
	for _, p := range want {
		if !strings.Contains(body, p) {
			t.Errorf("inline isotype lacks %s", p)
		}
	}

	fav, err := fs.ReadFile(StaticFS, "static/qbit-jay-isotype.svg")
	if err != nil {
		t.Fatal(err)
	}
	if string(fav) != string(raw) {
		t.Error("static/qbit-jay-isotype.svg differs from assets/branding/qbit-jay-isotype-dark.svg")
	}
	if !strings.Contains(body, `rel="icon"`) {
		t.Error("pages must link the isotype as favicon")
	}
}
