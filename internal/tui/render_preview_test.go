package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/qbitdevz/qbit-jay/internal/store"
	"github.com/qbitdevz/qbit-jay/internal/version"
)

// TestRenderPreviewScreens writes the View() output of a few screens, with
// ANSI colour, for the dark and the light terminal theme. It only runs when
// QJ_TUI_PREVIEW_DIR names an output directory:
//
//	QJ_TUI_PREVIEW_DIR=/tmp/qj-tui go test ./internal/tui -run TestRenderPreviewScreens
func TestRenderPreviewScreens(t *testing.T) {
	out := os.Getenv("QJ_TUI_PREVIEW_DIR")
	if out == "" {
		t.Skip("set QJ_TUI_PREVIEW_DIR to render preview screens")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	prevProfile := lipgloss.ColorProfile()
	prevDark := lipgloss.HasDarkBackground()
	t.Cleanup(func() {
		lipgloss.SetColorProfile(prevProfile)
		lipgloss.SetHasDarkBackground(prevDark)
	})
	// termenv.TrueColor is Profile zero; subtracting avoids a direct termenv import.
	lipgloss.SetColorProfile(prevProfile - prevProfile)

	ptr := func(v string) *string { return &v }
	obs := []store.Observation{
		{ID: 41, Type: "decision", Title: "Direction 03: hexagon-facet jay", Content: "Chosen identity for qbit-jay.", Project: ptr("qbit-jay"), CreatedAt: "2026-11-27 10:00"},
		{ID: 40, Type: "bugfix", Title: "Update check pointed at upstream", Content: "It now reads qbitdevz/qbit-jay.", Project: ptr("qbit-jay"), CreatedAt: "2026-11-27 09:12"},
		{ID: 39, Type: "discovery", Title: "Light cyan is lines only", Content: "2.7:1 on white.", Project: ptr("qbit-jay"), CreatedAt: "2026-11-26 18:40"},
		{ID: 38, Type: "pattern", Title: "Tokens come from one Go package", Content: "TUI and web read the same values.", Project: ptr("acorn-lab"), CreatedAt: "2026-11-26 16:05"},
	}
	screens := map[string]func() Model{
		"dashboard": func() Model {
			m := New(nil, "v0.1.0")
			m.Width, m.Height = 100, 40
			m.Stats = &store.Stats{TotalSessions: 12, TotalObservations: 318, TotalPrompts: 96, Projects: []string{"qbit-jay", "acorn-lab", "dotfiles"}}
			return m
		},
		"dashboard-update": func() Model {
			m := New(nil, "v0.1.0")
			m.Width, m.Height = 100, 40
			m.Stats = &store.Stats{TotalSessions: 12, TotalObservations: 318, TotalPrompts: 96, Projects: []string{"qbit-jay"}}
			m.UpdateStatus = version.StatusUpdateAvailable
			m.UpdateMsg = "Update available: 0.1.0 -> 0.2.0\nTo update:\n  go install github.com/qbitdevz/qbit-jay/cmd/qbit-jay@latest"
			return m
		},
		"recent": func() Model {
			m := New(nil, "v0.1.0")
			m.Width, m.Height = 100, 30
			m.Screen = ScreenRecent
			m.RecentObservations = obs
			return m
		},
		"detail": func() Model {
			m := New(nil, "v0.1.0")
			m.Width, m.Height = 100, 30
			m.Screen = ScreenObservationDetail
			o := obs[0]
			m.SelectedObservation = &o
			return m
		},
		"error": func() Model {
			m := New(nil, "v0.1.0")
			m.Width, m.Height = 100, 24
			m.Screen = ScreenSearchResults
			m.SearchQuery = "needle"
			m.ErrorMsg = "database is locked"
			return m
		},
	}

	for _, dark := range []bool{true, false} {
		lipgloss.SetHasDarkBackground(dark)
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		for name, build := range screens {
			m := build()
			if err := os.WriteFile(filepath.Join(out, name+"-"+suffix+".ansi"), []byte(m.View()), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
