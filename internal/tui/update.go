package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case listsLoadedMsg:
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
		return m, nil

	case reposLoadedMsg:
		if msg.gen != m.preloader.generation {
			return m, nil // stale: drop
		}
		key := repoCacheKey{msg.listID, msg.withTopics}
		// Cancelled loads have their cache entry removed, so this response is stale.
		if e, ok := m.preloader.cache[key]; !ok || e.state != repoCacheLoading {
			return m, nil
		}
		delete(m.preloader.preloadCancels, msg.listID)
		if msg.withTopics && m.preloader.topicsCancels != nil {
			delete(m.preloader.topicsCancels, msg.listID)
		}
		if msg.err != nil {
			m.preloader.cache[key] = &repoCacheEntry{
				state: repoCacheError,
				err:   msg.err,
				gen:   msg.gen,
			}
		} else {
			entry := &repoCacheEntry{state: repoCacheLoaded, repos: msg.repos, gen: msg.gen}
			m.preloader.cache[key] = entry
			// Update displayed slice only if this is the focused list and matches current withTopics.
			if m.focusedList != nil && msg.listID == m.focusedList.ID &&
				msg.withTopics == m.showPreview {
				sorted := make([]githubapi.Repository, len(entry.repos))
				copy(sorted, entry.repos)
				sortRepos(sorted, m.sortRepos)
				m.displayedRepos = sorted
				if m.repoSearchActive && m.repoSearchQuery != "" {
					m = m.rebuildDisplayed()
				}
				if m.repoCursor >= len(m.displayedRepos) && len(m.displayedRepos) > 0 {
					(&m).setRepoCursor(len(m.displayedRepos) - 1)
				}
			}
			// Refresh focusedList pointer only if this load is for the focused list.
			if m.focusedList != nil && m.focusedList.ID == msg.listID {
				for i := range m.lists {
					if m.lists[i].ID == msg.listID {
						m.focusedList = &m.lists[i]
						break
					}
				}
			}
			// Drop selected keys that no longer exist in the refreshed repo list.
			if len(m.selected) > 0 {
				repos := m.currentRepos()
				existing := make(map[string]struct{}, len(repos))
				for _, r := range repos {
					existing[r.NameWithOwner] = struct{}{}
				}
				for nwo := range m.selected {
					if _, ok := existing[nwo]; !ok {
						delete(m.selected, nwo)
					}
				}
			}
		}
		if !msg.withTopics {
			if m.preloader.inFlight > 0 {
				m.preloader.inFlight--
			}
		} else if m.preloader.topicsInFlight > 0 {
			m.preloader.topicsInFlight--
		}

		preloadCmd := m.preloader.schedulePreload(m.ctx, m.svc)
		if preloadCmd == nil && m.showPreview {
			preloadCmd = m.preloader.scheduleTopicsPreload(
				m.ctx, m.svc, m.focusedList, m.displayedLists,
			)
		}
		return m, preloadCmd

	case errMsg:
		m.err = msg.err
		m.listsLoading = false
		for k, e := range m.preloader.cache {
			if e.state == repoCacheLoading {
				m.preloader.cache[k] = &repoCacheEntry{
					state: repoCacheError,
					err:   msg.err,
					gen:   e.gen,
				}
			}
		}
		return m, nil

	case mutationDoneMsg:
		m.mutationPending = false
		if msg.err != nil {
			// Keep modal open, show error inline.
			if m.modal != nil {
				m.modal.submitting = false
				m.modal.submitErr = msg.err.Error()
			}
			return m, nil
		}
		m.modal = nil
		m.statusMsg = "Done."
		m.statusExpiry = time.Now().Add(2 * time.Second)
		m.listsLoading = true
		cmds := []tea.Cmd{loadListsCmd(m.ctx, m.svc), statusClearCmd(m.statusExpiry)}
		// Invalidate repo cache for focused list.
		if m.focusedList != nil {
			delete(m.preloader.cache, repoCacheKey{m.focusedList.ID, false})
			delete(m.preloader.cache, repoCacheKey{m.focusedList.ID, true})
		}
		// For repo-pane mutations, trigger re-fetch of the focused list.
		if m.active == paneRepo && m.focusedList != nil {
			key := repoCacheKey{m.focusedList.ID, m.showPreview}
			m.preloader.cache[key] = &repoCacheEntry{
				state: repoCacheLoading,
				gen:   m.preloader.generation,
			}
			cmds = append(
				cmds,
				loadReposCmd(m.ctx, m.svc, m.focusedList.ID, m.showPreview, m.preloader.generation),
			)
		}
		return m, tea.Batch(cmds...)

	case bulkDoneMsg:
		m.mutationPending = false
		m.selected = nil
		switch {
		case msg.failed > 0 && m.modal != nil:
			m.statusMsg = ""
			m.modal.submitting = false
			m.modal.submitErr = ""
			m.modal.bulkFailure = &bulkFailureState{
				verb:       msg.verb,
				succeeded:  msg.succeeded,
				failedNWOs: append([]string(nil), msg.failedNWOs...),
			}
			if msg.succeeded == 0 {
				return m, nil
			}
		case msg.failed > 0:
			m.modal = nil
			names := msg.failedNWOs
			var failDetail string
			switch {
			case len(names) == 0:
				failDetail = ""
			case len(names) <= 3:
				failDetail = " (" + strings.Join(names, ", ") + ")"
			default:
				failDetail = " (" + strings.Join(
					names[:3],
					", ",
				) + fmt.Sprintf(
					", +%d more)",
					len(names)-3,
				)
			}
			m.statusMsg = fmt.Sprintf(
				"%d %s, %d failed%s",
				msg.succeeded,
				msg.verb,
				msg.failed,
				failDetail,
			)
		default:
			m.modal = nil
			m.statusMsg = fmt.Sprintf("%d repos %s.", msg.succeeded, msg.verb)
		}
		var cmds []tea.Cmd
		if m.statusMsg != "" {
			m.statusExpiry = time.Now().Add(toastDuration(msg.failedNWOs))
			cmds = append(cmds, statusClearCmd(m.statusExpiry))
		}
		m.listsLoading = true
		cmds = append(cmds, loadListsCmd(m.ctx, m.svc))
		// Invalidate repo cache for focused list.
		if m.focusedList != nil {
			delete(m.preloader.cache, repoCacheKey{m.focusedList.ID, false})
			delete(m.preloader.cache, repoCacheKey{m.focusedList.ID, true})
		}
		if m.active == paneRepo && m.focusedList != nil {
			key := repoCacheKey{m.focusedList.ID, m.showPreview}
			m.preloader.cache[key] = &repoCacheEntry{
				state: repoCacheLoading,
				gen:   m.preloader.generation,
			}
			cmds = append(
				cmds,
				loadReposCmd(m.ctx, m.svc, m.focusedList.ID, m.showPreview, m.preloader.generation),
			)
		}
		return m, tea.Batch(cmds...)

	case statusExpiredMsg:
		m.statusMsg = ""
		return m, nil

	case tea.MouseWheelMsg:
		if m.modal != nil || m.listSearchActive || m.repoSearchActive {
			return m, nil
		}
		var delta int
		switch msg.Button {
		case tea.MouseWheelUp:
			delta = -1
		case tea.MouseWheelDown:
			delta = 1
		default:
			return m, nil
		}
		// Scroll the pane under the pointer, regardless of which pane is active.
		g := calcPaneGeometry(m.width, m.showPreview)
		switch {
		case msg.X < g.sep1Col:
			// List pane.
			m.listCursor = clampInt(m.listCursor+delta, 0, len(m.displayedLists)-1)
			m = m.slideListOffset()
		case m.showPreview && g.sep2Col >= 0 && msg.X > g.sep2Col:
			// Preview pane wheel scroll.
			contentH := m.height - 2
			viewH := contentH
			content := countPreviewLines(m, g.previewWidth, contentH)
			previewDelta := 3
			if delta < 0 {
				previewDelta = -3
			}
			m.previewOffset = slidePreviewOffset(m.previewOffset, previewDelta, content, viewH)
		default:
			// Repo pane (between sep1Col and sep2Col, or sep2Col < 0).
			(&m).setRepoCursor(clampInt(m.repoCursor+delta, 0, len(m.displayedRepos)-1))
			m = m.slideRepoOffset()
		}
		return m, nil

	case tea.MouseClickMsg:
		if m.modal != nil || m.listSearchActive || m.repoSearchActive {
			return m, nil
		}
		var cmd tea.Cmd
		m, cmd = m.handleMouseClick(msg)
		return m, cmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.anyPending() {
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.modal != nil {
			updated, cmd := m.modal.update(msg)
			if updated == nil && cmd != nil {
				// Modal signalled submit: keep it open in submitting state.
				m.modal.submitting = true
				m.modal.submitErr = ""
				m.modal.bulkFailure = nil
				m.mutationPending = true
				return m, tea.Batch(cmd, m.spinner.Tick)
			}
			m.modal = updated
			return m, cmd
		}
		if m.listSearchActive || m.repoSearchActive {
			return m.handleSearchKey(msg)
		}
		return m.handleKey(msg)
	}

	return m, nil
}

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
	m.previewOffset = 0
	m.repoOffset = 0
	m.cachedRepoSig = ""
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
