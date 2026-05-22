package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
)

// anyEnabled returns true if at least one binding in the slice is enabled.
func anyEnabled(bindings []key.Binding) bool {
	for _, b := range bindings {
		if b.Enabled() {
			return true
		}
	}
	return false
}

// renderHelpLines returns the help content as a slice of lines,
// derived from the key bindings defined in keys.go.
// width controls layout: narrow (<50) single-column, wide two-column.
func renderHelpLines(width int) []string {
	groups := keys.helpGroups()
	narrow := width > 0 && width < 50

	if narrow {
		var lines []string
		for _, g := range groups {
			if !anyEnabled(g.bindings) {
				continue
			}
			for _, b := range g.bindings {
				if !b.Enabled() {
					continue
				}
				h := b.Help()
				lines = append(lines, fmt.Sprintf("  %-12s %s", h.Key, h.Desc))
			}
		}
		return lines
	}

	// Two-column table: Navigation group on the left, all others on the right.
	leftBindings := groups[0].bindings
	var rightBindings []key.Binding
	for i := 1; i < len(groups); i++ {
		rightBindings = append(rightBindings, groups[i].bindings...)
	}

	var left, right []string
	for _, b := range leftBindings {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		left = append(left, fmt.Sprintf("%-9s %s", h.Key, h.Desc))
	}
	for _, b := range rightBindings {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		right = append(right, fmt.Sprintf("%-9s %s", h.Key, h.Desc))
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
	lines = append(lines, fmt.Sprintf("  %-22s  %s", "----------", "-------"))
	for i := range left {
		lines = append(lines, fmt.Sprintf("  %-22s  %s", left[i], right[i]))
	}
	return lines
}
