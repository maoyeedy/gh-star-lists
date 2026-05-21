package tui

import (
	lipgloss "charm.land/lipgloss/v2"
)

var (
	// Header / pane chrome
	styleAppTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	stylePaneTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	stylePaneSubtitle = lipgloss.NewStyle().Faint(true)
	styleSeparator    = lipgloss.NewStyle().Faint(true)

	// Repo rows
	styleRepoName         = lipgloss.NewStyle()
	styleRepoNameFocused  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleRepoNameInactive = lipgloss.NewStyle().Faint(true)
	styleRepoOwner        = lipgloss.NewStyle().Faint(true)
	styleRepoStars        = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleRepoLanguage     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleRepoURL          = lipgloss.NewStyle().Faint(true)
	styleRepoBadge        = lipgloss.NewStyle().Faint(true)

	// Search
	styleSearchPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	// Empty / loading states
	styleEmptyState = lipgloss.NewStyle().Faint(true)

	// Footer
	styleFooterKey  = lipgloss.NewStyle().Bold(true)
	styleFooterText = lipgloss.NewStyle().Faint(true)

	// Modals
	styleModalBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	styleModalTitle  = lipgloss.NewStyle().Bold(true)

	// Status
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleChecked = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))

	// Cursor rows
	styleCursorActive   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleCursorInactive = lipgloss.NewStyle().Faint(true)

	// Backward-compatible aliases (callers in model.go still use these names).
	styleFaint    = stylePaneSubtitle
	styleSelected = styleCursorActive
)
