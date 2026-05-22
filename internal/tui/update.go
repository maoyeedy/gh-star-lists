package tui

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case listsLoadedMsg:
		return m.handleListsLoaded(msg)

	case reposLoadedMsg:
		return m.handleReposLoaded(msg)

	case starredAtEnrichedMsg:
		return m.handleStarredAtEnriched(msg)

	case errMsg:
		m.err = msg.err
		m.listsLoading = false
		for k, e := range m.preloader.cache {
			if e.state == repoCacheLoading {
				m.preloader.setCacheEntry(k, &repoCacheEntry{
					state: repoCacheError,
					err:   msg.err,
					gen:   e.gen,
				})
			}
		}
		return m, nil

	case mutationDoneMsg:
		return m.handleMutationDone(msg)

	case bulkDoneMsg:
		return m.handleBulkDone(msg)

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
