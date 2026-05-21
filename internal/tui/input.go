package tui

import (
	"maps"
	"slices"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Back):
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
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
		if m.active == paneList {
			newIdx := clampInt(m.listCursor-1, 0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(clampInt(m.repoCursor-1, 0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.Down):
		if m.active == paneList {
			newIdx := clampInt(m.listCursor+1, 0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(clampInt(m.repoCursor+1, 0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.PgUp):
		paneH := max(1, m.height-2)
		if m.active == paneList {
			newIdx := clampInt(m.listCursor-(paneH-1), 0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(clampInt(m.repoCursor-(paneH-1), 0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.PgDn):
		paneH := max(1, m.height-2)
		if m.active == paneList {
			newIdx := clampInt(m.listCursor+(paneH-1), 0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(clampInt(m.repoCursor+(paneH-1), 0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.Home):
		if m.active == paneList {
			cmd := (&m).focusList(0)
			m.listOffset = 0
			return m, cmd
		}
		(&m).setRepoCursor(0)
		m.repoOffset = 0
		return m, nil

	case key.Matches(msg, keys.End):
		if m.active == paneList {
			newIdx := max(0, len(m.displayedLists)-1)
			cmd := (&m).focusList(newIdx)
			m = m.slideListOffset()
			return m, cmd
		}
		(&m).setRepoCursor(max(0, len(m.displayedRepos)-1))
		m = m.slideRepoOffset()
		return m, nil

	case key.Matches(msg, keys.Enter):
		return m.handleEnter()

	case key.Matches(msg, keys.Open):
		return m.handleOpen()

	case key.Matches(msg, keys.Sort):
		m = m.cycleSort()
		return m, nil

	case key.Matches(msg, keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, keys.Help):
		m.showHelp = !m.showHelp
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
			m.modal = newBulkRemoveModal(m.ctx, m.svc, m.selectedNWOs(), m.focusedList.ID)
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
		m.showPreview = !m.showPreview
		m.previewOffset = 0
		if m.showPreview && m.focusedList != nil {
			// Only schedule a withTopics=true load for the focused list if not already loaded/loading.
			topicsKey := repoCacheKey{m.focusedList.ID, true}
			e := m.repoCache[topicsKey]
			if e == nil || e.state == repoCacheIdle {
				m.repoCache[topicsKey] = &repoCacheEntry{state: repoCacheLoading, gen: m.generation}
				return m, loadReposCmd(m.ctx, m.svc, m.focusedList.ID, true, m.generation)
			}
		}
		return m, nil

	case key.Matches(msg, keys.Search):
		m.searchActive = true
		m.searchQuery = ""
		m.listCursor = 0
		m.repoCursor = 0
		m.previewOffset = 0
		m.listOffset = 0
		m.repoOffset = 0
		m = m.rebuildDisplayed()
		return m, nil

	case key.Matches(msg, keys.Select):
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

	return m, nil
}

// selectedNWOs returns sorted NameWithOwner strings from the selection set.
func (m model) selectedNWOs() []string {
	return slices.Sorted(maps.Keys(m.selected))
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
			_ = (&m).focusList(idx)
			m.active = paneRepo
			m.repoCursor = 0
			m.previewOffset = 0
			m.repoOffset = 0
			m.selected = nil
			// Reset tracker.
			m.lastClickTime = time.Time{}
			return m, nil
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
			e := m.repoCache[cacheKey]
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

// activateRepoPane switches focus to the repo pane, triggering a load if
// the focused list's repos aren't cached. Does not modify repoCursor or repoOffset.
func (m model) activateRepoPane() (model, tea.Cmd) {
	if len(m.displayedLists) == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	key := repoCacheKey{m.displayedLists[m.listCursor].ID, m.showPreview}
	e := m.repoCache[key]
	if e == nil || e.state == repoCacheIdle {
		// Not cached / idle: focusList triggers load. Its idle/default branch
		// does NOT touch repoCursor/repoOffset.
		cmd = (&m).focusList(m.listCursor)
	}
	// Cache-loaded branch: skip focusList entirely — displayedRepos is current
	// and we want to preserve repoCursor.
	m.active = paneRepo
	return m, cmd
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
