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
	// Eager initial load: preserve the previous first-real-list focus when
	// real lists exist; otherwise focus the virtual Unlisted entry.
	if m.focusedList == nil && len(m.displayedLists) > 0 {
		focusIdx := 0
		if len(m.lists) > 0 {
			for i, list := range m.displayedLists {
				if list.ID == m.lists[0].ID {
					focusIdx = i
					break
				}
			}
		}
		m.repoCursor = 0
		m.repoOffset = 0
		m.selected = nil
		// Build the preload queue from real list IDs. The virtual list is loaded
		// through the same repo cache path when it is selected.
		// Put the focused list first (it gets the first slot).
		m.preloader.queue = make([]string, 0, len(m.lists))
		m.preloader.queue = append(m.preloader.queue, m.displayedLists[focusIdx].ID)
		for _, l := range m.lists {
			if l.ID != m.displayedLists[focusIdx].ID {
				m.preloader.queue = append(m.preloader.queue, l.ID)
			}
		}
		m.preloader.inFlight = 0
		preloadCmd := (&m).focusList(focusIdx)
		enrichCmd := enrichStarredAtCmd(m.ctx, m.svc, m.preloader, "", nil, m.preloader.generation)
		return m, tea.Batch(preloadCmd, enrichCmd)
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
		m.repoOffset = 0
		m.selected = nil
	}
	return m, nil
}
