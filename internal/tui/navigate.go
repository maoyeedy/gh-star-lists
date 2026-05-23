package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m model) handleUp() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		newIdx := clampInt(m.listCursor-1, 0, len(m.displayedLists)-1)
		cmd := (&m).focusList(newIdx)
		m = m.slideListOffset()
		return m, cmd
	}
	(&m).setRepoCursor(clampInt(m.repoCursor-1, 0, len(m.displayedRepos)-1))
	m = m.slideRepoOffset()
	return m, nil
}

func (m model) handleDown() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		newIdx := clampInt(m.listCursor+1, 0, len(m.displayedLists)-1)
		cmd := (&m).focusList(newIdx)
		m = m.slideListOffset()
		return m, cmd
	}
	(&m).setRepoCursor(clampInt(m.repoCursor+1, 0, len(m.displayedRepos)-1))
	m = m.slideRepoOffset()
	return m, nil
}

func (m model) handlePageUp() (tea.Model, tea.Cmd) {
	paneH := m.repoPaneH()
	if m.active == paneList {
		newIdx := clampInt(m.listCursor-(paneH-1), 0, len(m.displayedLists)-1)
		cmd := (&m).focusList(newIdx)
		m = m.slideListOffset()
		return m, cmd
	}
	(&m).setRepoCursor(clampInt(m.repoCursor-(paneH-1), 0, len(m.displayedRepos)-1))
	m = m.slideRepoOffset()
	return m, nil
}

func (m model) handlePageDown() (tea.Model, tea.Cmd) {
	paneH := m.repoPaneH()
	if m.active == paneList {
		newIdx := clampInt(m.listCursor+(paneH-1), 0, len(m.displayedLists)-1)
		cmd := (&m).focusList(newIdx)
		m = m.slideListOffset()
		return m, cmd
	}
	(&m).setRepoCursor(clampInt(m.repoCursor+(paneH-1), 0, len(m.displayedRepos)-1))
	m = m.slideRepoOffset()
	return m, nil
}

func (m model) handleHome() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		cmd := (&m).focusList(0)
		m.listOffset = 0
		return m, cmd
	}
	(&m).setRepoCursor(0)
	m.repoOffset = 0
	return m, nil
}

func (m model) handleEnd() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		newIdx := max(0, len(m.displayedLists)-1)
		cmd := (&m).focusList(newIdx)
		m = m.slideListOffset()
		return m, cmd
	}
	(&m).setRepoCursor(max(0, len(m.displayedRepos)-1))
	m = m.slideRepoOffset()
	return m, nil
}

func (m model) handleBack() (tea.Model, tea.Cmd) {
	// Clear selection first if any; second Esc then navigates back / quits.
	if len(m.selected) > 0 {
		m.selected = nil
		return m, nil
	}
	if m.active == paneRepo {
		m.active = paneList
		return m, nil
	}
	return m, tea.Quit
}

// activateRepoPane switches focus to the repo pane, triggering a load if
// the focused list's repos aren't cached. Does not modify repoCursor or repoOffset.
func (m model) activateRepoPane() (model, tea.Cmd) {
	if len(m.displayedLists) == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	e := m.preloader.cache[m.displayedLists[m.listCursor].ID]
	if e == nil || e.state == repoCacheIdle {
		// Not cached / idle: focusList triggers load. Its idle/default branch
		// does NOT touch repoCursor/repoOffset.
		cmd = (&m).focusList(m.listCursor)
	}
	// Cache-loaded branch: skip focusList -- displayedRepos is current and we
	// want to preserve repoCursor.
	m.active = paneRepo
	return m, cmd
}
