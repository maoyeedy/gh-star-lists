package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func TestCursorMoveUsesCacheNoNewCmd(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Pre-populate cache for all three lists so no loads are needed.
	reposA := []githubapi.Repository{{ID: "A_1", NameWithOwner: "owner/a-list-repo"}}
	reposB := []githubapi.Repository{{ID: "B_1", NameWithOwner: "owner/b-list-repo"}}
	reposC := []githubapi.Repository{{ID: "C_1", NameWithOwner: "owner/c-list-repo"}}
	m = update(m, reposLoadedMsg{repos: reposA, listID: m.lists[0].ID, gen: m.preloader.generation})
	m = update(m, reposLoadedMsg{repos: reposB, listID: m.lists[1].ID, gen: m.preloader.generation})
	m = update(m, reposLoadedMsg{repos: reposC, listID: m.lists[2].ID, gen: m.preloader.generation})

	// All lists are cached now; reset cursor to 0 and ensure list pane is active.
	m.active = paneList
	m.listCursor = 0
	inflightBefore := m.preloader.inFlight

	// Move cursor down -- should use cache, no new load.
	next, _ := m.Update(specialKey(tea.KeyDown))
	m2 := next.(model)

	if m2.preloader.inFlight != inflightBefore {
		t.Errorf(
			"preloadInFlight changed from %d to %d after cursor move to cached list",
			inflightBefore, m2.preloader.inFlight,
		)
	}
	// displayedRepos should be populated from the cache for lists[1].
	if len(m2.displayedRepos) == 0 {
		t.Error("displayedRepos should be populated from cache after cursor move")
	}
	if m2.listCursor != 1 {
		t.Errorf("listCursor = %d, want 1", m2.listCursor)
	}
}

// TestCursorMoveIdleListSchedulesLoad verifies that moving the cursor to an idle
// list (no cache entry) creates a repoCacheLoading entry and increments
// preloadInFlight.
func TestCursorMoveIdleListSchedulesLoad(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Simulate: deliver repos for first list only, then manually clear all other
	// cache entries to simulate idle state for lists[1] and lists[2].
	m = update(
		m,
		reposLoadedMsg{repos: svc.repos, listID: m.lists[0].ID, gen: m.preloader.generation},
	)
	// Remove loading/loaded entries for UL_2 and UL_3 (simulate idle).
	delete(m.preloader.cache, repoCacheKey{m.lists[1].ID, false})
	delete(m.preloader.cache, repoCacheKey{m.lists[2].ID, false})
	m.preloader.inFlight = 0
	m.preloader.queue = nil

	m.active = paneList
	m.listCursor = 0
	inflightBefore := m.preloader.inFlight

	// Move down to lists[1] which has no cache entry.
	m2 := update(m, specialKey(tea.KeyDown))

	cacheEntry := m2.preloader.cache[repoCacheKey{m2.lists[1].ID, false}]
	if cacheEntry == nil {
		t.Fatal("repoCache entry for list[1] should exist after cursor move to idle list")
	}
	if cacheEntry.state != repoCacheLoading {
		t.Errorf("cache entry state = %d, want repoCacheLoading", cacheEntry.state)
	}
	if m2.preloader.inFlight <= inflightBefore {
		t.Errorf(
			"preloadInFlight did not increase: before=%d after=%d",
			inflightBefore, m2.preloader.inFlight,
		)
	}
}
