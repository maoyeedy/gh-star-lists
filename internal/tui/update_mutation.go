package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m model) handleMutationDone(msg mutationDoneMsg) (tea.Model, tea.Cmd) {
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
		m.preloader.deleteCacheEntry(m.focusedList.ID)
	}
	// For repo-pane mutations, trigger re-fetch of the focused list.
	if m.active == paneRepo && m.focusedList != nil {
		m.preloader.setCacheEntry(m.focusedList.ID, &repoCacheEntry{
			state: repoCacheLoading,
			gen:   m.preloader.generation,
		})
		cmds = append(
			cmds,
			loadReposCmd(m.ctx, m.svc, m.focusedList.ID, m.preloader.generation),
		)
	}
	return m, tea.Batch(cmds...)
}

func (m model) handleBulkDone(msg bulkDoneMsg) (tea.Model, tea.Cmd) {
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
		m.preloader.deleteCacheEntry(m.focusedList.ID)
	}
	if m.active == paneRepo && m.focusedList != nil {
		m.preloader.setCacheEntry(m.focusedList.ID, &repoCacheEntry{
			state: repoCacheLoading,
			gen:   m.preloader.generation,
		})
		cmds = append(
			cmds,
			loadReposCmd(m.ctx, m.svc, m.focusedList.ID, m.preloader.generation),
		)
	}
	return m, tea.Batch(cmds...)
}
