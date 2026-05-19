package tui

import (
	lipgloss "charm.land/lipgloss/v2"
)

var (
	stylePaneTitle = lipgloss.NewStyle().Bold(true).Underline(true)
	styleFaint     = lipgloss.NewStyle().Faint(true)
	styleSelected  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleFooter    = lipgloss.NewStyle().Faint(true)
	styleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)
