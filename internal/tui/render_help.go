package tui

import (
	"fmt"
)

// renderHelpLines returns the help content as a slice of lines.
// width controls layout: narrow (<50) single-column, wide two-column.
func renderHelpLines(width int) []string {
	// Narrow terminal fallback: single-column list.
	if width > 0 && width < 50 {
		lines := []string{
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
		}
		return lines
	}

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

	lines := make([]string, 0, len(left)+3)
	lines = append(lines, fmt.Sprintf("  %-22s  %s", "Navigation", "Actions"))
	lines = append(lines, fmt.Sprintf("  %-22s  %s", "----------", "----------"))
	for i := range left {
		lines = append(lines, fmt.Sprintf("  %-22s  %s", left[i], right[i]))
	}
	return lines
}
