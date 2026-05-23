package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type repoCacheState int

const (
	repoCacheIdle repoCacheState = iota
	repoCacheLoading
	repoCacheLoaded
	repoCacheError
)

type repoCacheEntry struct {
	state repoCacheState
	repos []domain.Repository
	err   error
	gen   uint64
}

// preloader manages the async repo cache and preload queue.
type preloader struct {
	cache          map[string]*repoCacheEntry
	generation     uint64
	loadingCount   int
	queue          []string
	inFlight       int
	preloadCancels map[string]context.CancelFunc
	// starredRepos caches the full starred-repo list so getStarredAt
	// fetches at most once per generation.
	starredRepos []domain.Repository
}

func newPreloader() *preloader {
	return &preloader{
		cache: make(map[string]*repoCacheEntry),
	}
}

func (p *preloader) schedulePreload(ctx context.Context, svc githubapi.Service) tea.Cmd {
	const maxInFlight = 3
	var cmds []tea.Cmd
	for p.inFlight < maxInFlight && len(p.queue) > 0 {
		listID := p.queue[0]
		p.queue = p.queue[1:]
		if e := p.cache[listID]; e != nil && e.state != repoCacheIdle {
			continue
		}
		loadCtx, cancel := context.WithCancel(ctx)
		if p.preloadCancels == nil {
			p.preloadCancels = make(map[string]context.CancelFunc)
		}
		p.preloadCancels[listID] = cancel
		p.setCacheEntry(listID, &repoCacheEntry{state: repoCacheLoading, gen: p.generation})
		p.inFlight++
		capturedID := listID
		cmds = append(cmds, loadReposCmd(loadCtx, svc, capturedID, p.generation))
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
// in-flight preload requests.
func (p *preloader) clear() {
	p.generation++
	p.starredRepos = nil
	p.cache = make(map[string]*repoCacheEntry)
	p.loadingCount = 0
	for _, cancel := range p.preloadCancels {
		cancel()
	}
	p.preloadCancels = nil
	p.queue = nil
	p.inFlight = 0
}

func (p *preloader) setCacheEntry(listID string, entry *repoCacheEntry) {
	if existing := p.cache[listID]; existing != nil && existing.state == repoCacheLoading {
		if p.loadingCount > 0 {
			p.loadingCount--
		}
	}
	if entry != nil && entry.state == repoCacheLoading {
		p.loadingCount++
	}
	if entry == nil {
		delete(p.cache, listID)
		return
	}
	p.cache[listID] = entry
}

func (p *preloader) deleteCacheEntry(listID string) {
	p.setCacheEntry(listID, nil)
}

// anyPendingInCache reports whether any repo cache entry is loading.
func (p *preloader) anyPendingInCache() bool {
	return p.loadingCount > 0
}

// getStarredAt enriches repos with StarredAt timestamps, using a generation-keyed
// cache so that the full starred-repo list is fetched at most once per refresh.
func (p *preloader) getStarredAt(
	ctx context.Context, svc githubapi.Service, repos []domain.Repository,
) ([]domain.Repository, error) {
	if p.starredRepos == nil {
		starred, err := svc.ListStarredRepositories(ctx)
		if err != nil {
			return nil, err
		}
		p.starredRepos = starred
	}
	return githubapi.MergeStarredAt(repos, p.starredRepos), nil
}

// focusList sets the list cursor to idx, resolves focusedList, and updates the
// repo pane immediately from the cache. Returns a cmd if a load must be started.
func (m *model) focusList(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.displayedLists) {
		return nil
	}
	m.listCursor = idx
	list := m.displayedLists[idx]
	// Find the pointer in m.lists to keep focusedList pointing at the canonical slice.
	for i := range m.lists {
		if m.lists[i].ID == list.ID {
			m.focusedList = &m.lists[i]
			break
		}
	}
	e := m.preloader.cache[list.ID]
	switch {
	case e != nil && e.state == repoCacheLoaded:
		m.populateDisplayedRepos(e.repos)
		return nil
	case e != nil && e.state == repoCacheLoading:
		m.displayedRepos = nil
		return nil
	case e != nil && e.state == repoCacheError:
		m.displayedRepos = nil
		return nil
	default: // repoCacheIdle or absent: promote and start
		// Cancel in-flight loads for non-focused lists to free concurrency slots.
		for id, cancel := range m.preloader.preloadCancels {
			if id != list.ID {
				cancel()
				delete(m.preloader.preloadCancels, id)
				if m.preloader.inFlight > 0 {
					m.preloader.inFlight--
				}
				m.preloader.deleteCacheEntry(id)
			}
		}
		m.preloader.enqueueFront(list.ID)
		m.displayedRepos = nil
		return m.preloader.schedulePreload(m.ctx, m.svc)
	}
}

func (m *model) populateDisplayedRepos(repos []domain.Repository) {
	m.populateDisplayedReposWithCursor(repos, false)
}

func (m *model) refreshDisplayedRepos(repos []domain.Repository) {
	m.populateDisplayedReposWithCursor(repos, true)
}

func (m *model) populateDisplayedReposWithCursor(repos []domain.Repository, preserveCursor bool) {
	var previousRepoID string
	previousCursor := m.repoCursor
	previousOffset := m.repoOffset
	if preserveCursor && m.repoCursor >= 0 && m.repoCursor < len(m.displayedRepos) {
		previousRepoID = repoIdentity(m.displayedRepos[m.repoCursor])
	}

	sorted := make([]domain.Repository, len(repos))
	copy(sorted, repos)
	sortRepos(sorted, m.sortRepos)
	m.displayedRepos = sorted
	if m.repoSearchActive && m.repoSearchQuery != "" {
		*m = m.rebuildDisplayed()
	}
	if !preserveCursor {
		m.repoCursor = 0
		m.repoOffset = 0
		return
	}

	m.repoCursor = clampInt(previousCursor, 0, len(m.displayedRepos)-1)
	if previousRepoID != "" {
		for i, repo := range m.displayedRepos {
			if repoIdentity(repo) == previousRepoID {
				m.repoCursor = i
				break
			}
		}
	}
	m.repoOffset = clampInt(previousOffset, 0, max(0, len(m.displayedRepos)-m.repoPaneH()))
	slidden := m.slideRepoOffset()
	*m = slidden
}

func repoIdentity(repo domain.Repository) string {
	if repo.ID != "" {
		return repo.ID
	}
	return repo.NameWithOwner
}

func (m model) repoPaneCacheEntry() *repoCacheEntry {
	if m.focusedList == nil {
		return nil
	}
	return m.preloader.cache[m.focusedList.ID]
}

func (m *model) setRepoCursor(idx int) {
	m.repoCursor = idx
}

// currentRepos returns the repos currently backing the repo pane.
// Returns nil when loading, idle, or no list is focused.
func (m model) currentRepos() []domain.Repository {
	e := m.repoPaneCacheEntry()
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
