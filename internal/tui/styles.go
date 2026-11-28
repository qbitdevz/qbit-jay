package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/qbitdevz/qbit-jay/internal/qbit/theme"
)

// ─── Colors (qbit-jay theme) ──────────────────────────────────────────────────
//
// Every colour comes from internal/qbit/theme (S14) as an adaptive pair: the
// dark tokens on dark terminals, the light tokens on light ones. The variable
// names are upstream's, kept so view.go and update.go stay close to upstream;
// read each one by its role in the comment, not by its old hue.

var (
	colorBase     = theme.Bg.Adaptive()         // App background
	colorOverlay  = borderColor                 // Borders and connectors
	colorText     = theme.Ink.Adaptive()        // Primary text
	colorSubtext  = theme.Muted.Adaptive()      // Secondary text
	colorLavender = theme.AccentText.Adaptive() // Primary accent: headers, selection, focus
	colorGreen    = theme.Success.Adaptive()    // Success and stat numbers
	colorPeach    = theme.Warn.Adaptive()       // Acorn: type badges
	colorRed      = theme.Error.Adaptive()      // Errors
	colorBlue     = theme.Sky.Adaptive()        // Identifiers
	colorMauve    = sectionColor                // Section titles
	colorYellow   = theme.Warn.Adaptive()       // Acorn: warnings, projects, update banner
)

var (
	// Borders: --jay-beak-hi on dark (the brand line colour is 1.2:1 on the
	// deep background, too faint for a terminal frame); the muted ink on light.
	borderColor = lipgloss.AdaptiveColor{Dark: theme.JayBeakHi.Dark, Light: theme.Muted.Light}
	// Section titles: teal on dark, the accent blue on light (teal is lines
	// only there).
	sectionColor = lipgloss.AdaptiveColor{Dark: theme.Teal.Dark, Light: theme.AccentText.Light}
)

// ─── Layout Styles ───────────────────────────────────────────────────────────

var (
	// App frame
	appStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorBase).
			Padding(1, 2)

	// Header bar
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorLavender).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorOverlay).
			PaddingBottom(1).
			MarginBottom(1)

	// Footer / help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			MarginTop(1)

	// Error message
	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			Padding(0, 1)

	// Clipboard copy feedback ("✓ Copied!")
	copyFeedbackStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true).
				Padding(0, 1)

	// Update available banner
	updateBannerStyle = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true).
				Padding(0, 1)
)

// ─── Dashboard Styles ────────────────────────────────────────────────────────

var (
	// Big stat number
	statNumberStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGreen).
			Width(8).
			Align(lipgloss.Right)

	// Stat label
	statLabelStyle = lipgloss.NewStyle().
			Foreground(colorText).
			PaddingLeft(2)

	// Stat card container
	statCardStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorOverlay).
			Padding(1, 2).
			MarginBottom(1)

	// Menu item (normal)
	menuItemStyle = lipgloss.NewStyle().
			Foreground(colorText).
			PaddingLeft(2)

	// Menu item (selected)
	menuSelectedStyle = lipgloss.NewStyle().
				Foreground(colorLavender).
				Bold(true).
				PaddingLeft(1)

	// Dashboard title
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMauve).
			MarginBottom(1)
)

// ─── List Styles ─────────────────────────────────────────────────────────────

var (
	// List item (normal)
	listItemStyle = lipgloss.NewStyle().
			Foreground(colorText).
			PaddingLeft(2)

	// List item (selected/cursor)
	listSelectedStyle = lipgloss.NewStyle().
				Foreground(colorLavender).
				Bold(true).
				PaddingLeft(1)

	// Observation type badge
	typeBadgeStyle = lipgloss.NewStyle().
			Foreground(colorPeach).
			Bold(true)

	// Observation lifecycle warning badge
	stateWarningBadgeStyle = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true)

	// Observation ID
	idStyle = lipgloss.NewStyle().
		Foreground(colorBlue)

	// Timestamp
	timestampStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Italic(true)

	// Project name
	projectStyle = lipgloss.NewStyle().
			Foreground(colorMauve)

	// Content preview
	contentPreviewStyle = lipgloss.NewStyle().
				Foreground(colorSubtext).
				PaddingLeft(4)
)

// ─── Detail View Styles ──────────────────────────────────────────────────────

var (
	// Section heading in detail views
	sectionHeadingStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorMauve).
				MarginTop(1).
				MarginBottom(1)

	// Detail content
	detailContentStyle = lipgloss.NewStyle().
				Foreground(colorText).
				PaddingLeft(2)

	// Detail label
	detailLabelStyle = lipgloss.NewStyle().
				Foreground(colorSubtext).
				Width(14).
				Align(lipgloss.Right).
				PaddingRight(1)

	// Detail value
	detailValueStyle = lipgloss.NewStyle().
				Foreground(colorText)
)

// ─── Timeline Styles ─────────────────────────────────────────────────────────

var (
	// Timeline focus observation
	timelineFocusStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorLavender).
				Padding(0, 1)

	// Timeline before/after items
	timelineItemStyle = lipgloss.NewStyle().
				Foreground(colorSubtext).
				PaddingLeft(2)

	// Timeline arrow connector
	timelineConnectorStyle = lipgloss.NewStyle().
				Foreground(colorOverlay)
)

// ─── Search Styles ───────────────────────────────────────────────────────────

var (
	searchInputStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorLavender).
				Foreground(colorText).
				Padding(0, 1).
				MarginBottom(1)

	searchHighlightStyle = lipgloss.NewStyle().
				Foreground(colorPeach).
				Bold(true)

	noResultsStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Italic(true).
			PaddingLeft(2).
			MarginTop(1)
)
