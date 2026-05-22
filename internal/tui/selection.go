package tui

import (
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"
)

func (m model) handleSelect() (tea.Model, tea.Cmd) {
	if m.active != paneRepo || len(m.displayedRepos) == 0 {
		return m, nil
	}
	nwo := m.displayedRepos[m.repoCursor].NameWithOwner
	if m.selected == nil {
		m.selected = make(map[string]struct{})
	}
	if _, ok := m.selected[nwo]; ok {
		delete(m.selected, nwo)
	} else {
		m.selected[nwo] = struct{}{}
	}
	return m, nil
}

// selectedNWOs returns sorted NameWithOwner strings from the selection set.
func (m model) selectedNWOs() []string {
	return slices.Sorted(maps.Keys(m.selected))
}
