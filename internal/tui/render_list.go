package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	lipgloss "charm.land/lipgloss/v2"
)

func (m model) renderListPane(w, h int) string {
	totalH := h
	out := make([]string, 0, totalH)

	if m.searchActive && m.active == paneList {
		// Build search bar with optional N/total count on the right.
		prefix := styleSearchPrompt.Render("/") + " "
		prefixW := lipgloss.Width(prefix)

		countStr := ""
		countW := 0
		total := len(m.lists)
		displayed := len(m.displayedLists)
		candidate := fmt.Sprintf("%d/%d", displayed, total)
		candidateW := lipgloss.Width(candidate)
		// Show count only when at least 4 columns remain for the query after prefix + count + gap.
		if prefixW+4+2+candidateW <= w {
			countStr = stylePaneSubtitle.Render(candidate)
			countW = candidateW
		}

		// Remaining width for query display.
		queryBudget := w - prefixW - countW
		if countW > 0 {
			queryBudget -= 2 // gap between query and count
		}
		if queryBudget < 0 {
			queryBudget = 0
		}

		qDisplay := m.searchQuery
		if lipgloss.Width(qDisplay) > queryBudget {
			// Truncate from left.
			tail := ""
			for _, r := range qDisplay {
				candidate := "..." + tail + string(r)
				if lipgloss.Width(candidate) <= queryBudget {
					tail += string(r)
				}
			}
			qDisplay = "..." + tail
		}

		bar := prefix + qDisplay
		if countStr != "" {
			barW := lipgloss.Width(bar)
			gap := w - barW - countW
			if gap < 1 {
				gap = 1
			}
			bar = bar + strings.Repeat(" ", gap) + countStr
		}
		out = append(out, padRight(bar, w))
		h--
	}

	if m.listsLoading && m.focusedList == nil {
		out = append(out, "  Loading "+m.spinner.View())
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	if len(m.displayedLists) == 0 {
		label := "(no lists)"
		if m.searchQuery != "" {
			q := m.searchQuery
			if utf8.RuneCountInString(q) > 20 {
				q = string([]rune(q)[:20]) + "..."
			}
			label = "(no matches for \"" + q + "\")"
		}
		out = append(out, label)
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	const cursorWidth = 2 // "> " or "  "
	start := m.listOffset
	end := min(start+h, len(m.displayedLists))
	for i := start; i < end; i++ {
		l := m.displayedLists[i]
		cursor := "  "
		isCursor := i == m.listCursor

		// Format count right-side.
		countRaw := fmt.Sprintf("%d", l.RepoCount)
		countStyled := stylePaneSubtitle.Render(countRaw)
		countW := lipgloss.Width(countRaw)

		// Available for name: total - cursor - spacer(1) - count.
		maxNameW := w - cursorWidth - 1 - countW
		if maxNameW < 1 {
			maxNameW = 1
		}

		name := l.Name
		nameW := lipgloss.Width(name)
		if nameW > maxNameW {
			// Truncate with ellipsis.
			const ellipsis = "..."
			ellipsisW := lipgloss.Width(ellipsis)
			runes := []rune(name)
			for len(runes) > 0 && lipgloss.Width(string(runes))+ellipsisW > maxNameW {
				runes = runes[:len(runes)-1]
			}
			name = string(runes) + ellipsis
		}

		if isCursor {
			cursor = "> "
			if m.active == paneList {
				name = styleCursorActive.Render(name)
			} else {
				name = styleCursorInactive.Render(name)
			}
		}

		row := cursor + name
		rowW := lipgloss.Width(row)
		space := w - rowW - countW
		if space < 1 {
			space = 1
		}
		out = append(out, row+strings.Repeat(" ", space)+countStyled)
	}
	for len(out) < totalH {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// starGlyph is the Unicode star character (U+2605) used in star count display.
