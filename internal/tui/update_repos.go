package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m model) handleReposLoaded(msg reposLoadedMsg) (tea.Model, tea.Cmd) {
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
		m.preloader.setCacheEntry(key, &repoCacheEntry{
			state: repoCacheError,
			err:   msg.err,
			gen:   msg.gen,
		})
	} else {
		entry := &repoCacheEntry{state: repoCacheLoaded, repos: msg.repos, gen: msg.gen}
		m.preloader.setCacheEntry(key, entry)
		// Update displayed slice when this load is the active detail level, or
		// when preview is waiting on topics and basic repos are the best data.
		if m.focusedList != nil && msg.listID == m.focusedList.ID &&
			msg.withTopics == m.showPreview {
			if msg.withTopics {
				m.refreshDisplayedRepos(entry.repos)
			} else {
				m.populateDisplayedRepos(entry.repos)
			}
		} else if m.focusedList != nil && msg.listID == m.focusedList.ID &&
			m.showPreview &&
			!msg.withTopics {
			displayEntry := m.repoPaneCacheEntry()
			if displayEntry == entry {
				m.populateDisplayedRepos(entry.repos)
			}
		}
		if m.focusedList != nil && msg.listID == m.focusedList.ID {
			if m.repoCursor >= len(m.displayedRepos) && len(m.displayedRepos) > 0 {
				m.setRepoCursor(len(m.displayedRepos) - 1)
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

	cmds := []tea.Cmd{
		m.preloader.schedulePreload(m.ctx, m.svc),
	}
	if m.showPreview {
		cmds = append(cmds, m.preloader.scheduleTopicsPreload(
			m.ctx, m.svc, m.focusedList, m.displayedLists,
		))
	}
	// After a successful withTopics load, asynchronously enrich with starredAt
	// so that topics appear immediately while starredAt fills in later.
	if msg.err == nil && msg.withTopics {
		cmds = append(cmds, enrichStarredAtCmd(
			m.ctx, m.svc, m.preloader, msg.listID, msg.repos, msg.gen,
		))
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
	key := repoCacheKey{msg.listID, true}
	e, ok := m.preloader.cache[key]
	if !ok || e.state != repoCacheLoaded {
		return m, nil
	}
	// Update the cached entry with starredAt-enriched repos.
	m.preloader.cache[key] = &repoCacheEntry{
		state: repoCacheLoaded,
		repos: msg.repos,
		gen:   msg.gen,
	}
	// Refresh displayed repos if this is the focused list and preview is showing.
	if m.focusedList != nil && msg.listID == m.focusedList.ID && m.showPreview {
		m.refreshDisplayedRepos(msg.repos)
	}
	return m, nil
}
