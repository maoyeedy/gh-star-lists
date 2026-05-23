package tui

import (
	"context"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func TestRefreshCallsInvalidate(t *testing.T) {
	t.Parallel()
	svc := &fakeInvalidatableService{
		fakeService: fakeService{
			lists: []domain.StarList{{ID: "UL_1", Name: "Go Tools"}},
		},
	}
	m := newModel(context.Background(), svc, Options{})
	m = update(m, listsLoadedMsg{lists: svc.lists})

	update(m, ctrlKey('r'))

	if svc.invalidateCalls != 1 {
		t.Errorf("Invalidate calls = %d, want 1", svc.invalidateCalls)
	}
}

// TestRefreshNoInvalidateOnPlainService verifies ctrl+r is safe on a
// service without Invalidate (no panic, just reload).
func TestRefreshNoInvalidateOnPlainService(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Must not panic
	m2 := update(m, ctrlKey('r'))
	if !m2.listsLoading {
		t.Error("listsLoading should be true after refresh")
	}
}

func TestRefreshBumpsGenerationAndClearsCache(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Populate cache with a loaded entry.
	m.preloader.cache["UL_1"] = &repoCacheEntry{state: repoCacheLoaded}
	genBefore := m.preloader.generation

	m2 := update(m, ctrlKey('r'))

	if m2.preloader.generation != genBefore+1 {
		t.Errorf("generation = %d, want %d after ctrl+r", m2.preloader.generation, genBefore+1)
	}
	if len(m2.preloader.cache) != 0 {
		t.Errorf("repoCache len = %d, want 0 after ctrl+r", len(m2.preloader.cache))
	}
	if !m2.listsLoading {
		t.Error("listsLoading should be true after ctrl+r")
	}
}

// TestStaleMsgDroppedAfterRefresh verifies that a reposLoadedMsg from the prior
// generation is dropped and does not install a loaded entry.
func TestStaleMsgDroppedAfterRefresh(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Refresh bumps generation to 1.
	m = update(m, ctrlKey('r'))
	if m.preloader.generation != 1 {
		t.Fatalf("generation = %d after ctrl+r, want 1", m.preloader.generation)
	}

	// Deliver a stale message (gen 0).
	stale := reposLoadedMsg{repos: svc.repos, listID: "UL_1", gen: 0}
	m2 := update(m, stale)

	entry := m2.preloader.cache["UL_1"]
	if entry != nil && entry.state == repoCacheLoaded {
		t.Error(
			"stale reposLoadedMsg (gen=0) should not create a loaded entry after refresh (gen=1)",
		)
	}
}
