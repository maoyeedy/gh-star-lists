package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m model) handleRefresh() (tea.Model, tea.Cmd) {
	if inv, ok := m.svc.(invalidatable); ok {
		inv.Invalidate()
	}
	m.preloader.clear()
	m.listsLoading = true
	m.err = nil
	m.lists = nil
	m.displayedRepos = nil
	m.focusedList = nil
	m.active = paneList
	m.listCursor = 0
	m.listOffset = 0
	m.repoCursor = 0
	m.repoOffset = 0
	return m, loadListsCmd(m.ctx, m.svc)
}

// toastDuration returns the status-message display duration based on whether
// any items failed during a bulk operation.
func toastDuration(failedNWOs []string) time.Duration {
	if len(failedNWOs) > 0 {
		return 4 * time.Second
	}
	return 2 * time.Second
}
