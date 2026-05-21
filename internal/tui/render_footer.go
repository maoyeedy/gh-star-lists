package tui

import (
	"fmt"
	"strings"
	"time"
)

func renderHint(k, desc string) string {
	return styleFooterKey.Render(k) + " " + styleFooterText.Render(desc)
}

// joinHints joins rendered hint pairs with two spaces between them.
func joinHints(hints []string) string {
	return strings.Join(hints, "  ")
}

func (m model) renderFooter() string {
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		return styleSuccess.Render(m.statusMsg)
	}
	if m.listSearchActive || m.repoSearchActive {
		return joinHints([]string{
			renderHint("/", "search"),
			renderHint("esc", "clear"),
			renderHint("enter", "done"),
			renderHint("up/down", "navigate"),
		})
	}
	if m.active == paneRepo {
		hints := []string{
			renderHint("/", "search"),
			renderHint("space", "select"),
		}
		if len(m.selected) > 0 {
			hints = append(
				hints,
				styleFooterText.Render(fmt.Sprintf("[%d selected]", len(m.selected))),
			)
		}
		hints = append(hints,
			renderHint("o", "browser"),
			renderHint("?", "help"),
			renderHint("q", "quit"),
		)
		return joinHints(hints)
	}
	return joinHints([]string{
		renderHint("/", "search"),
		renderHint("enter", "open"),
		renderHint("s", "sort"),
		renderHint("?", "help"),
		renderHint("q", "quit"),
	})
}
