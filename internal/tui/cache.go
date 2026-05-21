package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type repoCacheKey struct {
	listID     string
	withTopics bool
}

type repoCacheState int

const (
	repoCacheIdle repoCacheState = iota
	repoCacheLoading
	repoCacheLoaded
	repoCacheError
)

type repoCacheEntry struct {
	state repoCacheState
	repos []githubapi.Repository
	err   error
	gen   uint64
}

func (m *model) schedulePreload() tea.Cmd {
	const maxInFlight = 3
	var cmds []tea.Cmd
	for m.preloadInFlight < maxInFlight && len(m.preloadQueue) > 0 {
		listID := m.preloadQueue[0]
		m.preloadQueue = m.preloadQueue[1:]
		key := repoCacheKey{listID, false}
		if e := m.repoCache[key]; e != nil && e.state != repoCacheIdle {
			continue // already loading or loaded: skip
		}
		m.repoCache[key] = &repoCacheEntry{state: repoCacheLoading, gen: m.generation}
		m.preloadInFlight++
		capturedID := listID // capture for closure
		cmds = append(cmds, loadReposCmd(m.ctx, m.svc, capturedID, false, m.generation))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// focusList sets the list cursor to idx, resolves focusedList, and updates the
// repo pane immediately from the cache. Returns a cmd if a load must be started.
func (m *model) focusList(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.displayedLists) {
		return nil
	}
	m.previewOffset = 0
	m.listCursor = idx
	list := m.displayedLists[idx]
	// Find the pointer in m.lists to keep focusedList pointing at the canonical slice.
	for i := range m.lists {
		if m.lists[i].ID == list.ID {
			m.focusedList = &m.lists[i]
			break
		}
	}
	key := repoCacheKey{list.ID, false}
	e := m.repoCache[key]
	switch {
	case e != nil && e.state == repoCacheLoaded:
		// Populate display slice immediately from cache.
		sorted := make([]githubapi.Repository, len(e.repos))
		copy(sorted, e.repos)
		sortRepos(sorted, m.sortRepos)
		m.displayedRepos = sorted
		if m.searchActive && m.searchQuery != "" {
			*m = m.rebuildDisplayed()
		}
		m.repoCursor = 0
		m.repoOffset = 0
		return nil
	case e != nil && e.state == repoCacheLoading:
		m.displayedRepos = nil
		return nil
	case e != nil && e.state == repoCacheError:
		m.displayedRepos = nil
		return nil
	default: // repoCacheIdle or absent: promote and start
		// Remove from queue if present, prepend.
		newQueue := make([]string, 0, len(m.preloadQueue)+1)
		newQueue = append(newQueue, list.ID)
		for _, id := range m.preloadQueue {
			if id != list.ID {
				newQueue = append(newQueue, id)
			}
		}
		m.preloadQueue = newQueue
		m.displayedRepos = nil
		return m.schedulePreload()
	}
}

func (m *model) setRepoCursor(idx int) {
	if m.repoCursor != idx {
		m.previewOffset = 0
	}
	m.repoCursor = idx
}

// currentRepos returns the repos for the focused list (using current withTopics state).
// Returns nil when loading, idle, or no list is focused.
func (m model) currentRepos() []githubapi.Repository {
	if m.focusedList == nil {
		return nil
	}
	e := m.repoCache[repoCacheKey{m.focusedList.ID, m.showPreview}]
	if e == nil || e.state != repoCacheLoaded {
		return nil
	}
	return e.repos
}

// anyPending reports whether any async work is in progress.
func (m model) anyPending() bool {
	if m.listsLoading || m.mutationPending {
		return true
	}
	for _, e := range m.repoCache {
		if e.state == repoCacheLoading {
			return true
		}
	}
	return false
}

// invalidatable is satisfied by cacheService.
type invalidatable interface {
	Invalidate()
}
