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
		box := styleModalBorder.Render(m.modal.view())
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return base
}

func (m model) renderLayout() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	contentH := m.height - 2

	g := calcPaneGeometry(m.width, m.showPreview)

	if m.showPreview {
		// Three-column layout: lists | repos | preview
		leftPane := m.renderListPane(g.leftWidth, contentH)
		midPane := m.renderRepoPane(g.repoWidth, contentH)
		previewPane := m.renderPreviewPane(g.previewWidth, contentH)

		sep := "|"
		rows := make([]string, contentH)
		leftLines := strings.Split(leftPane, "\n")
		midLines := strings.Split(midPane, "\n")
		previewLines := strings.Split(previewPane, "\n")

		for i := 0; i < contentH; i++ {
			l, mid, r := "", "", ""
			if i < len(leftLines) {
				l = leftLines[i]
			}
			if i < len(midLines) {
				mid = midLines[i]
			}
			if i < len(previewLines) {
				r = previewLines[i]
			}
			l = padRight(l, g.leftWidth)
			mid = padRight(mid, g.repoWidth)
			rows[i] = l + sep + mid + sep + r
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
		return header + "\n" + strings.Join(rows, "\n") + "\n" + footer
	}

	// Two-column layout.
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
