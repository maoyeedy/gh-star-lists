package tui

import (
	lipgloss "charm.land/lipgloss/v2"
)

var (
	stylePaneTitle   = lipgloss.NewStyle().Bold(true).Underline(true)
	styleFaint       = lipgloss.NewStyle().Faint(true)
	styleSelected    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleFooter      = lipgloss.NewStyle().Faint(true)
	styleError       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleSuccess     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleChecked     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	styleModalBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	styleModalTitle  = lipgloss.NewStyle().Bold(true)

	// Cursor row styles: active pane gets bold+colored, inactive pane gets faint ghost cursor.
	styleCursorActive   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleCursorInactive = lipgloss.NewStyle().Faint(true)
)
