package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m model) handleListsLoaded(msg listsLoadedMsg) (tea.Model, tea.Cmd) {
	var previousFocusedID string
	if m.focusedList != nil {
		previousFocusedID = m.focusedList.ID
	}
	m.lists = msg.lists
	m.listsLoading = false
	sortStarLists(m.lists, m.sortLists)
	m = m.rebuildDisplayed()
	if m.listCursor >= len(m.displayedLists) && len(m.displayedLists) > 0 {
		m.listCursor = len(m.displayedLists) - 1
	}
	// Eager initial load: auto-focus first list and build preload queue.
	if m.focusedList == nil && len(m.lists) > 0 {
		m.focusedList = &m.lists[0]
		m.repoCursor = 0
		m.previewOffset = 0
		m.repoOffset = 0
		m.selected = nil
		// Build the preload queue from the sorted displayed list IDs.
		// Put the focused list first (it gets the first slot).
		m.preloader.queue = make([]string, 0, len(m.displayedLists))
		m.preloader.queue = append(m.preloader.queue, m.focusedList.ID)
		for _, l := range m.displayedLists {
			if l.ID != m.focusedList.ID {
				m.preloader.queue = append(m.preloader.queue, l.ID)
			}
		}
		m.preloader.inFlight = 0
		preloadCmd := m.preloader.schedulePreload(m.ctx, m.svc)
		return m, preloadCmd
	}
	if previousFocusedID != "" {
		for i, list := range m.displayedLists {
			if list.ID == previousFocusedID {
				m.listCursor = i
				cmd := (&m).focusList(i)
				return m, cmd
			}
		}
		if len(m.displayedLists) > 0 {
			m.listCursor = 0
			cmd := (&m).focusList(0)
			return m, cmd
		}
		m.focusedList = nil
		m.displayedRepos = nil
		m.repoCursor = 0
		m.previewOffset = 0
		m.repoOffset = 0
		m.selected = nil
	}
	return m, nil
}
