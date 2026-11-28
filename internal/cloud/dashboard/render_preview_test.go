package dashboard

import (
	"context"
	"errors"
	"html"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/qbitdevz/qbit-jay/internal/cloud/cloudstore"
	"github.com/qbitdevz/qbit-jay/internal/qbit/theme"
)

// TestRenderPreviewPages writes static HTML of the main dashboard screens, with
// the real CSS, theme and fonts copied next to them, so a browser can
// screenshot the identity without running a server. It only runs when
// QJ_PREVIEW_DIR names an output directory:
//
//	QJ_PREVIEW_DIR=/tmp/qj-dash go test ./internal/cloud/dashboard -run TestRenderPreviewPages
func TestRenderPreviewPages(t *testing.T) {
	out := os.Getenv("QJ_PREVIEW_DIR")
	if out == "" {
		t.Skip("set QJ_PREVIEW_DIR to render preview pages")
	}

	if err := fs.WalkDir(StaticFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(out, path)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := fs.ReadFile(StaticFS, path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "static", "theme.css"), []byte(theme.CSS()), 0o644); err != nil {
		t.Fatal(err)
	}

	proj := cloudstore.DashboardProjectRow{Project: "qbit-jay", Chunks: 42, Sessions: 12, Observations: 318, Prompts: 96}
	sess := cloudstore.DashboardSessionRow{Project: "qbit-jay", SessionID: "s-1", StartedAt: "2026-11-27T09:00:00Z", Summary: "Rename and visual identity"}
	obs := []cloudstore.DashboardObservationRow{
		{Project: "qbit-jay", SessionID: "s-1", SyncID: "o-1", Type: "decision", Title: "Direction 03: hexagon-facet jay", Content: "Chosen identity.", CreatedAt: "2026-11-27T10:00:00Z"},
		{Project: "qbit-jay", SessionID: "s-1", SyncID: "o-2", Type: "bugfix", Title: "Update check pointed at upstream", Content: "Fixed.", CreatedAt: "2026-11-27T11:00:00Z"},
		{Project: "qbit-jay", SessionID: "s-1", SyncID: "o-3", Type: "discovery", Title: "Light cyan is lines only", Content: "2.7:1.", CreatedAt: "2026-11-27T12:00:00Z"},
	}
	full := parityStoreStub{
		projects:     []cloudstore.DashboardProjectRow{proj, {Project: "acorn-lab", Chunks: 7, Sessions: 3, Observations: 51, Prompts: 12}},
		contributors: []cloudstore.DashboardContributorRow{{CreatedBy: "leandro@example.com", Chunks: 30, Projects: 2, LastChunkAt: "2026-11-27T12:00:00Z"}},
		sessions:     []cloudstore.DashboardSessionRow{sess},
		observations: obs,
		prompts:      []cloudstore.DashboardPromptRow{{Project: "qbit-jay", SessionID: "s-1", SyncID: "p-1", Content: "Design the theme package", CreatedAt: "2026-11-27T09:30:00Z"}},
		projectDetail: cloudstore.DashboardProjectDetail{
			Project: "qbit-jay", Stats: proj, Sessions: []cloudstore.DashboardSessionRow{sess}, Observations: obs,
			Contributors: []cloudstore.DashboardContributorRow{{CreatedBy: "leandro@example.com", Chunks: 30, Projects: 2, LastChunkAt: "2026-11-27T12:00:00Z"}},
		},
		distinctTypes: []string{"decision", "bugfix", "discovery"},
		systemHealth:  cloudstore.DashboardSystemHealth{DBConnected: true, Projects: 2, Contributors: 1},
	}
	broken := parityStoreStub{errListProjects: errors.New("cloud store unreachable"), errListContributors: errors.New("cloud store unreachable"), errListRecentObservations: errors.New("cloud store unreachable")}

	mux := func(store parityStoreStub, admin bool) *http.ServeMux {
		m := http.NewServeMux()
		Mount(m, MountConfig{
			RequireSession: func(*http.Request) error { return nil },
			IsAdmin:        func(*http.Request) bool { return admin },
			Store:          store,
			GetDisplayName: func(*http.Request) string { return "leandro" },
			Version:        "v0.1.0",
		})
		return m
	}

	pages := []struct {
		name, path string
		m          *http.ServeMux
	}{
		{"home", "/dashboard/", mux(full, true)},
		{"browser", "/dashboard/browser", mux(full, true)},
		{"projects", "/dashboard/projects", mux(full, true)},
		{"project-detail", "/dashboard/projects/qbit-jay", mux(full, true)},
		{"contributors", "/dashboard/contributors", mux(full, true)},
		{"empty", "/dashboard/", mux(parityStoreStub{}, false)},
		{"empty-projects", "/dashboard/projects", mux(parityStoreStub{}, false)},
		{"error", "/dashboard/projects", mux(broken, false)},
		{"error-browser", "/dashboard/browser", mux(broken, false)},
	}
	for _, p := range pages {
		rec := httptest.NewRecorder()
		p.m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p.path, nil))
		write(t, out, p.name, inlineLoadPartials(p.m, rec.Body.String()))
	}

	for name, errMsg := range map[string]string{"login": "", "login-error": "That token was not accepted."} {
		var b strings.Builder
		if err := LoginPage(errMsg, "", "v0.1.0").Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		write(t, out, name, b.String())
	}
}

var loadBlock = regexp.MustCompile(`(?s)(<div[^>]*hx-get="([^"]+)"[^>]*hx-trigger="load[^"]*"[^>]*>)(.*?)(</div>)`)

// inlineLoadPartials replaces every hx-trigger="load" placeholder with the
// partial it would fetch, so the static page shows what a browser shows after
// htmx has run.
func inlineLoadPartials(m *http.ServeMux, page string) string {
	return loadBlock.ReplaceAllStringFunc(page, func(block string) string {
		sub := loadBlock.FindStringSubmatch(block)
		req := httptest.NewRequest(http.MethodGet, html.UnescapeString(sub[2]), nil)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, req)
		return sub[1] + rec.Body.String() + sub[4]
	})
}

func write(t *testing.T, dir, name, html string) {
	t.Helper()
	html = strings.ReplaceAll(html, "/dashboard/static/", "static/")
	if err := os.WriteFile(filepath.Join(dir, name+".html"), []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
}
