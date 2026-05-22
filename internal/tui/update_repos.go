package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
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

	cmds := []tea.Cmd{
		m.preloader.schedulePreload(m.ctx, m.svc),
	}
	if m.showPreview {
		cmds = append(cmds, m.preloader.scheduleTopicsPreload(
			m.ctx, m.svc, m.focusedList, m.displayedLists,
		))
	}
	return m, tea.Batch(cmds...)
}
