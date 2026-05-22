package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

func (m model) handlePreview() (tea.Model, tea.Cmd) {
	wasShowing := m.showPreview
	m.showPreview = !m.showPreview
	m.previewOffset = 0
	if wasShowing {
		m.preloader.cancelTopicsPreloads()
		return m, nil
	}
	if m.focusedList != nil {
		topicsKey := repoCacheKey{m.focusedList.ID, true}
		e := m.preloader.cache[topicsKey]
		if e == nil || e.state == repoCacheIdle {
			// Cancel non-focused topics loads to free a slot.
			if m.preloader.topicsInFlight >= maxTopicsInFlight {
				for id, cancel := range m.preloader.topicsCancels {
					if id != m.focusedList.ID {
						cancel()
						delete(m.preloader.topicsCancels, id)
						m.preloader.deleteCacheEntry(repoCacheKey{id, true})
						m.preloader.topicsInFlight--
					}
				}
			}
			if m.preloader.topicsInFlight >= maxTopicsInFlight {
				return m, nil
			}
			loadCtx, cancel := context.WithCancel(m.ctx)
			if m.preloader.topicsCancels == nil {
				m.preloader.topicsCancels = make(map[string]context.CancelFunc)
			}
			m.preloader.topicsCancels[m.focusedList.ID] = cancel
			m.preloader.setCacheEntry(topicsKey, &repoCacheEntry{
				state: repoCacheLoading,
				gen:   m.preloader.generation,
			})
			m.preloader.topicsInFlight++
			return m, loadReposCmd(
				loadCtx,
				m.svc,
				m.focusedList.ID,
				true,
				m.preloader.generation,
			)
		}
	}
	return m, nil
}
