package tui

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Back):
		return m.handleBack()

	case key.Matches(msg, keys.Left):
		if m.active == paneRepo {
			m.active = paneList
		}
		return m, nil

	case key.Matches(msg, keys.Right):
		if m.active == paneList && m.focusedList != nil {
			return m.activateRepoPane()
		}
		return m, nil

	case key.Matches(msg, keys.Up):
		return m.handleUp()

	case key.Matches(msg, keys.Down):
		return m.handleDown()

	case key.Matches(msg, keys.PgUp):
		return m.handlePageUp()

	case key.Matches(msg, keys.PgDn):
		return m.handlePageDown()

	case key.Matches(msg, keys.Home):
		return m.handleHome()

	case key.Matches(msg, keys.End):
		return m.handleEnd()

	case key.Matches(msg, keys.Enter):
		return m.handleEnter()

	case key.Matches(msg, keys.Open):
		return m.handleOpen()

	case key.Matches(msg, keys.Sort):
		return m.cycleSort()

	case key.Matches(msg, keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, keys.Help):
		m.modal = newHelpModal()
		return m, nil

	case key.Matches(msg, keys.CreateList):
		mo, focusCmd := newCreateListModal(m.ctx, m.svc)
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.EditList):
		if m.active != paneList || len(m.displayedLists) == 0 {
			return m, nil
		}
		mo, focusCmd := newEditListModal(m.ctx, m.svc, m.displayedLists[m.listCursor])
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.DeleteList):
		if m.active != paneList || len(m.displayedLists) == 0 {
			return m, nil
		}
		mo, focusCmd := newDeleteListModal(m.ctx, m.svc, m.displayedLists[m.listCursor])
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.AddRepo):
		if m.active != paneRepo || len(m.displayedRepos) == 0 || len(m.lists) == 0 {
			return m, nil
		}
		if len(m.selected) > 0 {
			m.modal = newBulkAddModal(m.ctx, m.svc, m.selectedNWOs(), m.lists)
		} else {
			m.modal = newAddRepoModal(m.ctx, m.svc, m.displayedRepos[m.repoCursor], m.lists)
		}
		return m, nil

	case key.Matches(msg, keys.RemoveRepo):
		if m.active != paneRepo || len(m.displayedRepos) == 0 || m.focusedList == nil {
			return m, nil
		}
		if len(m.selected) > 0 {
			m.modal = newBulkRemoveModal(m.ctx, m.svc, m.selectedNWOs(), m.lists, m.focusedList.ID)
		} else {
			repo := m.displayedRepos[m.repoCursor]
			m.modal = newRemoveRepoModal(m.ctx, m.svc, repo, m.focusedList.ID)
		}
		return m, nil

	case key.Matches(msg, keys.MoveRepo):
		if m.active != paneRepo || len(m.displayedRepos) == 0 ||
			m.focusedList == nil || len(m.lists) < 2 {
			return m, nil
		}
		if len(m.selected) > 0 {
			m.modal = newBulkMoveModal(m.ctx, m.svc, m.selectedNWOs(), m.lists, m.focusedList.ID)
		} else {
			repo := m.displayedRepos[m.repoCursor]
			m.modal = newMoveRepoModal(m.ctx, m.svc, repo, m.lists, m.focusedList.ID)
		}
		return m, nil

	case key.Matches(msg, keys.UnstarRepo):
		if m.active != paneRepo || len(m.displayedRepos) == 0 {
			return m, nil
		}
		repo := m.displayedRepos[m.repoCursor]
		mo, focusCmd := newUnstarRepoModal(m.ctx, m.svc, repo)
		m.modal = mo
		return m, focusCmd

	case key.Matches(msg, keys.CopyList):
		if m.active != paneList || len(m.displayedLists) == 0 || len(m.lists) < 2 {
			return m, nil
		}
		m.modal = newCopyListModal(m.ctx, m.svc, m.displayedLists[m.listCursor], m.lists)
		return m, nil

	case key.Matches(msg, keys.MergeList):
		if m.active != paneList || len(m.displayedLists) == 0 || len(m.lists) < 2 {
			return m, nil
		}
		m.modal = newMergeListModal(m.ctx, m.svc, m.displayedLists[m.listCursor], m.lists)
		return m, nil

	case key.Matches(msg, keys.Preview):
		return m.handlePreview()

	case key.Matches(msg, keys.Search):
		return m.activateSearch()

	case key.Matches(msg, keys.Select):
		return m.handleSelect()
	}

	return m, nil
}

func (m model) handleMouseClick(msg tea.MouseClickMsg) (model, tea.Cmd) {
	contentRow := msg.Y - 1 // row 0 is the header
	if contentRow < 0 {
		return m, nil
	}
	g := calcPaneGeometry(m.width, m.showPreview)
	if msg.X < g.sep1Col {
		// List pane click.
		m.active = paneList
		idx := contentRow + m.listOffset
		if idx < 0 || idx >= len(m.displayedLists) {
			return m, nil
		}

		// Double-click detection: two clicks on same pane+row within 300ms drills to repo pane.
		now := time.Now()
		if m.lastClickPane == int(paneList) && m.lastClickIndex == idx &&
			!m.lastClickTime.IsZero() && now.Sub(m.lastClickTime) < 300*time.Millisecond {
			// Double-click: drill into the list, ensure load starts if idle.
			cmd := (&m).focusList(idx)
			m.active = paneRepo
			m.repoCursor = 0
			m.previewOffset = 0
			m.repoOffset = 0
			m.selected = nil
			// Reset tracker.
			m.lastClickTime = time.Time{}
			return m, cmd
		}
		// Single click.
		if idx != m.listCursor {
			cmd := (&m).focusList(idx)
			m.lastClickPane = int(paneList)
			m.lastClickIndex = idx
			m.lastClickTime = now
			return m, cmd
		}
		// Already focused: if idle, trigger load without switching pane.
		m.lastClickPane = int(paneList)
		m.lastClickIndex = idx
		m.lastClickTime = now
		if m.focusedList != nil {
			cacheKey := repoCacheKey{m.focusedList.ID, false}
			e := m.preloader.cache[cacheKey]
			if e == nil || e.state == repoCacheIdle {
				cmd := (&m).focusList(idx)
				return m, cmd
			}
		}
		return m, nil
	} else if msg.X > g.sep1Col && (g.sep2Col < 0 || msg.X < g.sep2Col) {
		// Repo pane click.
		if m.focusedList != nil && len(m.displayedRepos) > 0 {
			m.active = paneRepo
			idx := contentRow + m.repoOffset
			if idx >= 0 && idx < len(m.displayedRepos) {
				(&m).setRepoCursor(idx)
			}
		}
	}
	// Clicks in the preview pane (msg.X > g.sep2Col when showPreview) are no-ops for now.
	return m, nil
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		if len(m.displayedLists) == 0 {
			return m, nil
		}
		mo, cmd := m.activateRepoPane()
		mo.selected = nil
		return mo, cmd
	}
	// paneRepo: open in browser
	return m.openFocusedRepoURL()
}

func (m model) handleOpen() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		if len(m.displayedLists) == 0 {
			return m, nil
		}
		url := m.displayedLists[m.listCursor].URL
		if url != "" && m.openBrowser != nil {
			_ = m.openBrowser(url)
		}
		return m, nil
	}
	return m.openFocusedRepoURL()
}

func (m model) openFocusedRepoURL() (tea.Model, tea.Cmd) {
	if len(m.displayedRepos) == 0 {
		return m, nil
	}
	url := m.displayedRepos[m.repoCursor].URL
	if url != "" && m.openBrowser != nil {
		_ = m.openBrowser(url)
	}
	return m, nil
}
