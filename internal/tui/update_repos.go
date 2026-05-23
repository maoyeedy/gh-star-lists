package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m model) handleReposLoaded(msg reposLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.preloader.generation {
		return m, nil // stale: drop
	}
	listID := msg.listID
	// Cancelled loads have their cache entry removed, so this response is stale.
	if e, ok := m.preloader.cache[listID]; !ok || e.state != repoCacheLoading {
		return m, nil
	}
	delete(m.preloader.preloadCancels, listID)
	if msg.err != nil {
		m.preloader.setCacheEntry(listID, &repoCacheEntry{
			state: repoCacheError,
			err:   msg.err,
			gen:   msg.gen,
		})
	} else {
		entry := &repoCacheEntry{state: repoCacheLoaded, repos: msg.repos, gen: msg.gen}
		m.preloader.setCacheEntry(listID, entry)
		if m.focusedList != nil && listID == m.focusedList.ID {
			m.populateDisplayedRepos(entry.repos)
		}
		if m.focusedList != nil && listID == m.focusedList.ID {
			if m.repoCursor >= len(m.displayedRepos) && len(m.displayedRepos) > 0 {
				m.setRepoCursor(len(m.displayedRepos) - 1)
			}
		}
		// Refresh focusedList pointer only if this load is for the focused list.
		if m.focusedList != nil && m.focusedList.ID == listID {
			for i := range m.lists {
				if m.lists[i].ID == listID {
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
	if m.preloader.inFlight > 0 {
		m.preloader.inFlight--
	}

	cmds := []tea.Cmd{
		m.preloader.schedulePreload(m.ctx, m.svc),
	}
	return m, tea.Batch(cmds...)
}

func (m model) handleStarredAtEnriched(msg starredAtEnrichedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.preloader.generation {
		return m, nil
	}
	if msg.err != nil {
		return m, nil
	}
	// If this enrichment targeted a specific list, update its cached repos.
	if msg.listID != "" {
		e, ok := m.preloader.cache[msg.listID]
		if !ok || e.state != repoCacheLoaded {
			return m, nil
		}
		// Update the cached entry with starredAt-enriched repos.
		m.preloader.cache[msg.listID] = &repoCacheEntry{
			state: repoCacheLoaded,
			repos: msg.repos,
			gen:   msg.gen,
		}
		// Refresh displayed repos if this is the focused list.
		if m.focusedList != nil && msg.listID == m.focusedList.ID {
			m.refreshDisplayedRepos(msg.repos)
		}
	}
	return m, nil
}
