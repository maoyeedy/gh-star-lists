package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

func (m model) View() tea.View {
	content := m.renderContent()
	v := tea.NewView(content)
	v.AltScreen = true
	if m.mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func (m model) renderContent() string {
	if m.err != nil {
		return styleError.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\nPress q to quit."
	}
	base := m.renderLayout()
	if m.modal != nil {
		// RoundedBorder (1 col/side) + Padding(1,2) (2 cols/side) = 6 overhead.
		const modalBorderOverhead = 6
		const modalHMargin = 8 // at least 4 cols margin on each side
		maxOuter := m.width - modalHMargin
		if maxOuter < modalBorderOverhead+4 {
			maxOuter = modalBorderOverhead + 4
		}
		m.modal.width = maxOuter - modalBorderOverhead
		box := styleModalBorder.Width(m.modal.width).Render(m.modal.view())
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return base
}

func (m model) renderLayout() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	contentH := m.height - 2

	g := calcPaneGeometry(m.width)

	leftPane := m.renderListPane(g.leftWidth, contentH)
	rightPane := m.renderRepoPane(g.repoWidth, contentH)

	separator := "|"
	rows := make([]string, contentH)
	leftLines := strings.Split(leftPane, "\n")
	rightLines := strings.Split(rightPane, "\n")

	for i := 0; i < contentH; i++ {
		l := ""
		r := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		l = padRight(l, g.leftWidth)
		rows[i] = l + separator + r
	}

	header := m.renderHeader()
	footer := renderFooter(
		m.active,
		m.listSearchActive,
		m.repoSearchActive,
		m.selected,
		m.statusMsg,
		m.statusExpiry,
	)
	body := strings.Join(rows, "\n")
	return header + "\n" + body + "\n" + footer
}

func padRight(s string, width int) string {
	vis := lipgloss.Width(s)
	if vis > width {
		s = lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(s)
		vis = lipgloss.Width(s)
	}
	if vis == width {
		return s
	}
	return s + strings.Repeat(" ", width-vis)
}

func padLeft(s string, width int) string {
	vis := lipgloss.Width(s)
	if vis >= width {
		return s
	}
	return strings.Repeat(" ", width-vis) + s
}
