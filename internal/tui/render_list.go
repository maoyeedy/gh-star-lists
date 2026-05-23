package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func (m model) renderListPane(w, h int) string {
	if h <= 0 {
		return ""
	}
	totalH := h
	out := make([]string, 0, totalH)

	out = append(out, paneTitle("Lists", len(m.displayedLists), w))
	h--
	if h <= 0 {
		return strings.Join(out, "\n")
	}

	if m.listSearchActive && m.active == paneList {
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

		qDisplay := m.listSearchQuery
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
		if m.listSearchQuery != "" {
			q := m.listSearchQuery
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
	if m.listCursor < start {
		start = m.listCursor
	} else if m.listCursor >= start+h {
		start = m.listCursor - h + 1
	}
	start = clampInt(start, 0, max(0, len(m.displayedLists)-h))
	end := min(start+h, len(m.displayedLists))

	// Convert to ListRow for rendering with pre-computed fields.
	listRows := make([]domain.ListRow, end-start)
	for j := 0; j < end-start; j++ {
		listRows[j] = listToRow(m.displayedLists[start+j])
	}

	for i, row := range listRows {
		cursor := "  "
		isCursor := start+i == m.listCursor

		countRaw := row.RepoCountStr
		countStyled := stylePaneSubtitle.Render(countRaw)
		countW := lipgloss.Width(countRaw)

		// Available for name: total - cursor - spacer(1) - count.
		maxNameW := w - cursorWidth - 1 - countW
		if maxNameW < 1 {
			maxNameW = 1
		}

		name := row.Name
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

		rendered := cursor + name
		rowW := lipgloss.Width(rendered)
		space := w - rowW - countW
		if space < 1 {
			space = 1
		}
		out = append(out, rendered+strings.Repeat(" ", space)+countStyled)
	}
	for len(out) < totalH {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func paneTitle(label string, count, w int) string {
	title := fmt.Sprintf("%s (%d)", label, count)
	return padRight(stylePaneTitle.Render(truncateToWidth(title, w)), w)
}

// listToRow converts a domain.StarList to a domain.ListRow for rendering.
func listToRow(list domain.StarList) domain.ListRow {
	return domain.ListRow{
		RepoCountStr: fmt.Sprintf("%d", list.RepoCount),
		Name:         list.Name,
		ID:           list.ID,
		RepoCount:    list.RepoCount,
		URL:          list.URL,
	}
}

// starGlyph is the Unicode star character (U+2605) used in star count display.
