package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
)

// renderHint renders a styled key+description pair.
func renderHint(k, desc string) string {
	return styleFooterKey.Render(k) + " " + styleFooterText.Render(desc)
}

// renderHintFromBinding renders a key binding as a styled key+description pair.
func renderHintFromBinding(b key.Binding) string {
	h := b.Help()
	return styleFooterKey.Render(h.Key) + " " + styleFooterText.Render(h.Desc)
}

// joinHints joins rendered hint pairs with two spaces between them.
func joinHints(hints []string) string {
	return strings.Join(hints, "  ")
}

func renderFooter(
	active pane,
	listSearchActive, repoSearchActive bool,
	selected map[string]struct{},
	statusMsg string,
	statusExpiry time.Time,
) string {
	if statusMsg != "" && time.Now().Before(statusExpiry) {
		return styleSuccess.Render(statusMsg)
	}
	if listSearchActive || repoSearchActive {
		return joinHints([]string{
			renderHint(keys.Search.Help().Key, "search"),
			renderHint(keys.Back.Help().Key, "clear"),
			renderHint(keys.Enter.Help().Key, "done"),
			renderHint("up/down", "navigate"),
		})
	}
	bindings := keys.footerBindings(active, len(selected) > 0)
	hints := make([]string, 0, len(bindings)+1)
	for _, b := range bindings {
		hints = append(hints, renderHintFromBinding(b))
	}
	if active == paneRepo && len(selected) > 0 {
		hints = append(
			hints,
			styleFooterText.Render(fmt.Sprintf("[%d selected]", len(selected))),
		)
	}
	return joinHints(hints)
}
