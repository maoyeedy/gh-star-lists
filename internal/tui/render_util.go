package tui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

func truncateToWidth(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	const ellipsis = "..."
	ellipsisW := lipgloss.Width(ellipsis)
	if maxW < ellipsisW {
		return strings.Repeat(".", maxW)
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+ellipsisW > maxW {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

func formatStars(n int) string {
	switch {
	case n >= 1_000_000:
		m := n / 1_000_000
		if m >= 10 {
			return fmt.Sprintf("%dm", m)
		}
		d := (n % 1_000_000) / 100_000
		return fmt.Sprintf("%d.%dm", m, d)
	case n >= 1_000:
		k := n / 1_000
		if k >= 10 {
			return fmt.Sprintf("%dk", k)
		}
		d := (n % 1_000) / 100
		return fmt.Sprintf("%d.%dk", k, d)
	default:
		return fmt.Sprintf("%d", n)
	}
}
