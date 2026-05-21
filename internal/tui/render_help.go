package tui

import (
	"fmt"
	"strings"
)

func (m model) renderHelp() string {
	var result string
	// Narrow terminal fallback: single-column list.
	if m.width > 0 && m.width < 50 {
		lines := []string{
			stylePaneTitle.Render("Key Bindings"),
			"",
			fmt.Sprintf("  %-16s %s", "up/k", "move up"),
			fmt.Sprintf("  %-16s %s", "down/j", "move down"),
			fmt.Sprintf("  %-16s %s", "pgup", "page up"),
			fmt.Sprintf("  %-16s %s", "pgdn", "page down"),
			fmt.Sprintf("  %-16s %s", "g", "top"),
			fmt.Sprintf("  %-16s %s", "G", "bottom"),
			fmt.Sprintf("  %-16s %s", "left", "focus lists"),
			fmt.Sprintf("  %-16s %s", "right", "focus repos"),
			fmt.Sprintf("  %-16s %s", "enter", "open/select"),
			fmt.Sprintf("  %-16s %s", "esc", "back/quit"),
			fmt.Sprintf("  %-16s %s", "/", "search"),
			fmt.Sprintf("  %-16s %s", "space", "select"),
			fmt.Sprintf("  %-16s %s", "a", "add repo"),
			fmt.Sprintf("  %-16s %s", "x", "remove repo"),
			fmt.Sprintf("  %-16s %s", "m", "move repo"),
			fmt.Sprintf("  %-16s %s", "u", "unstar repo"),
			fmt.Sprintf("  %-16s %s", "p", "preview"),
			fmt.Sprintf("  %-16s %s", "o", "open browser"),
			fmt.Sprintf("  %-16s %s", "n/e/d", "list CRUD"),
			fmt.Sprintf("  %-16s %s", "c/C", "copy/merge"),
			fmt.Sprintf("  %-16s %s", "ctrl+r", "refresh"),
			fmt.Sprintf("  %-16s %s", "?", "toggle help"),
			fmt.Sprintf("  %-16s %s", "q", "quit"),
			"",
			stylePaneSubtitle.Render("Press ? to close"),
		}
		result = strings.Join(lines, "\n")
	} else {
		// Two-column table: Navigation | Actions.
		left := []string{
			"up/k   move up",
			"down/j move down",
			"pgup   page up",
			"pgdn   page down",
			"g      top",
			"G      bottom",
			"left   focus lists",
			"right  focus repos",
			"enter  open/select",
			"esc    back/quit",
			"?      toggle help",
		}
		right := []string{
			"/      search",
			"space  select",
			"a      add repo",
			"x      remove repo",
			"m      move repo",
			"u      unstar repo",
			"p      preview",
			"o      open browser",
			"n/e/d  list CRUD",
			"c/C    copy/merge",
			"ctrl+r refresh",
			"q      quit",
		}

		// Pad the shorter column with empty strings.
		for len(left) < len(right) {
			left = append(left, "")
		}
		for len(right) < len(left) {
			right = append(right, "")
		}

		lines := []string{
			stylePaneTitle.Render("Key Bindings"),
			"",
			fmt.Sprintf("  %-22s  %s", "Navigation", "Actions"),
			fmt.Sprintf("  %-22s  %s", "----------", "----------"),
		}
		for i := range left {
			lines = append(lines, fmt.Sprintf("  %-22s  %s", left[i], right[i]))
		}
		lines = append(lines, "")
		lines = append(lines, stylePaneSubtitle.Render("Press ? to close"))
		result = strings.Join(lines, "\n")
	}

	// Apply viewport offset for help overlay scrolling.
	if m.helpViewportOffset > 0 && m.height > 0 {
		allLines := strings.Split(result, "\n")
		contentH := len(allLines)
		viewH := m.height
		maxOffset := max(0, contentH-viewH)
		offset := m.helpViewportOffset
		if offset > maxOffset {
			offset = maxOffset
		}
		if offset > 0 {
			end := offset + viewH
			if end > contentH {
				end = contentH
			}
			result = strings.Join(allLines[offset:end], "\n")
		}
	}
	return result
}
