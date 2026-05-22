package tui

import (
	"context"

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

const maxTopicsInFlight = 2

type repoCacheEntry struct {
	state repoCacheState
	repos []githubapi.Repository
	err   error
	gen   uint64
}

// preloader manages the async repo cache and preload queue.
type preloader struct {
	cache          map[repoCacheKey]*repoCacheEntry
	generation     uint64
	loadingCount   int
	queue          []string
	inFlight       int
	preloadCancels map[string]context.CancelFunc
	topicsInFlight int
	topicsCancels  map[string]context.CancelFunc
}

func newPreloader() *preloader {
	return &preloader{
		cache: make(map[repoCacheKey]*repoCacheEntry),
	}
}

func (p *preloader) schedulePreload(ctx context.Context, svc githubapi.Service) tea.Cmd {
	const maxInFlight = 3
	var cmds []tea.Cmd
	for p.inFlight < maxInFlight && len(p.queue) > 0 {
		listID := p.queue[0]
		p.queue = p.queue[1:]
		key := repoCacheKey{listID, false}
		if e := p.cache[key]; e != nil && e.state != repoCacheIdle {
			continue
		}
		loadCtx, cancel := context.WithCancel(ctx)
		if p.preloadCancels == nil {
			p.preloadCancels = make(map[string]context.CancelFunc)
		}
		p.preloadCancels[listID] = cancel
		p.setCacheEntry(key, &repoCacheEntry{state: repoCacheLoading, gen: p.generation})
		p.inFlight++
		capturedID := listID
		cmds = append(cmds, loadReposCmd(loadCtx, svc, capturedID, false, p.generation))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// enqueueFront inserts listID at the front of the queue, removing any duplicate.
func (p *preloader) enqueueFront(listID string) {
	newQueue := make([]string, 0, len(p.queue)+1)
	newQueue = append(newQueue, listID)
	for _, id := range p.queue {
		if id != listID {
			newQueue = append(newQueue, id)
		}
	}
	p.queue = newQueue
}

// clear resets the cache, bumps generation, drains the queue, and cancels any
// in-flight preload and topics requests.
func (p *preloader) clear() {
	p.generation++
	p.cancelTopicsPreloads()
	p.cache = make(map[repoCacheKey]*repoCacheEntry)
	p.loadingCount = 0
	for _, cancel := range p.preloadCancels {
		cancel()
	}
	p.preloadCancels = nil
	p.queue = nil
	p.inFlight = 0
}

func (p *preloader) setCacheEntry(key repoCacheKey, entry *repoCacheEntry) {
	if existing := p.cache[key]; existing != nil && existing.state == repoCacheLoading {
		if p.loadingCount > 0 {
			p.loadingCount--
		}
	}
	if entry != nil && entry.state == repoCacheLoading {
		p.loadingCount++
	}
	if entry == nil {
		delete(p.cache, key)
		return
	}
	p.cache[key] = entry
}

func (p *preloader) deleteCacheEntry(key repoCacheKey) {
	p.setCacheEntry(key, nil)
}

// anyPendingInCache reports whether any repo cache entry is loading.
func (p *preloader) anyPendingInCache() bool {
	return p.loadingCount > 0
}

// cancelTopicsPreloads cancels all in-flight topics preloads and cleans up.
func (p *preloader) cancelTopicsPreloads() {
	for id, cancel := range p.topicsCancels {
		cancel()
		p.deleteCacheEntry(repoCacheKey{id, true})
	}
	p.topicsCancels = nil
	p.topicsInFlight = 0
}

// scheduleTopicsPreload starts withTopics=true loads for lists whose basic repos
// are already cached. The focused list is prioritized. Max 2 concurrent topics loads.
func (p *preloader) scheduleTopicsPreload(ctx context.Context, svc githubapi.Service,
	focusedList *githubapi.StarList, displayedLists []githubapi.StarList,
) tea.Cmd {
	if p.topicsInFlight >= maxTopicsInFlight {
		return nil
	}

	// Build candidates: focused list first, then all displayed lists.
	candidates := make([]string, 0, len(displayedLists))
	if focusedList != nil {
		candidates = append(candidates, focusedList.ID)
	}
	for _, l := range displayedLists {
		if focusedList == nil || l.ID != focusedList.ID {
			candidates = append(candidates, l.ID)
		}
	}

	var cmds []tea.Cmd
	for _, listID := range candidates {
		if p.topicsInFlight >= maxTopicsInFlight {
			break
		}
		// Only schedule topics when basic repos are cached.
		basicKey := repoCacheKey{listID, false}
		if e := p.cache[basicKey]; e == nil || e.state != repoCacheLoaded {
			continue
		}
		topicsKey := repoCacheKey{listID, true}
		if e := p.cache[topicsKey]; e != nil && e.state != repoCacheIdle {
			continue
		}
		loadCtx, cancel := context.WithCancel(ctx)
		if p.topicsCancels == nil {
			p.topicsCancels = make(map[string]context.CancelFunc)
		}
		p.topicsCancels[listID] = cancel
		p.setCacheEntry(topicsKey, &repoCacheEntry{state: repoCacheLoading, gen: p.generation})
		p.topicsInFlight++
		cmds = append(cmds, loadReposCmd(loadCtx, svc, listID, true, p.generation))
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
	key := repoCacheKey{list.ID, m.showPreview}
	e := m.preloader.cache[key]
	switch {
	case e != nil && e.state == repoCacheLoaded:
		// Populate display slice immediately from cache.
		sorted := make([]githubapi.Repository, len(e.repos))
		copy(sorted, e.repos)
		sortRepos(sorted, m.sortRepos)
		m.displayedRepos = sorted
		if m.repoSearchActive && m.repoSearchQuery != "" {
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
	case m.showPreview:
		m.displayedRepos = nil
		return m.startRepoLoad(list.ID, true)
	default: // repoCacheIdle or absent: promote and start
		// Cancel in-flight loads for non-focused lists to free concurrency slots.
		for id, cancel := range m.preloader.preloadCancels {
			if id != list.ID {
				cancel()
				delete(m.preloader.preloadCancels, id)
				if m.preloader.inFlight > 0 {
					m.preloader.inFlight--
				}
				m.preloader.deleteCacheEntry(repoCacheKey{id, false})
			}
		}
		m.preloader.enqueueFront(list.ID)
		m.displayedRepos = nil
		return m.preloader.schedulePreload(m.ctx, m.svc)
	}
}

func (m *model) startRepoLoad(listID string, withTopics bool) tea.Cmd {
	loadCtx, cancel := context.WithCancel(m.ctx)
	key := repoCacheKey{listID, withTopics}
	if withTopics {
		if m.preloader.topicsCancels == nil {
			m.preloader.topicsCancels = make(map[string]context.CancelFunc)
		}
		m.preloader.topicsCancels[listID] = cancel
		m.preloader.topicsInFlight++
	} else {
		if m.preloader.preloadCancels == nil {
			m.preloader.preloadCancels = make(map[string]context.CancelFunc)
		}
		m.preloader.preloadCancels[listID] = cancel
		m.preloader.inFlight++
	}
	m.preloader.setCacheEntry(key, &repoCacheEntry{
		state: repoCacheLoading,
		gen:   m.preloader.generation,
	})
	return loadReposCmd(loadCtx, m.svc, listID, withTopics, m.preloader.generation)
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
	e := m.preloader.cache[repoCacheKey{m.focusedList.ID, m.showPreview}]
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
	return m.preloader.anyPendingInCache()
}

type invalidatable interface {
	Invalidate()
}
