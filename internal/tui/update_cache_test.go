package tui

import (
	"testing"
)

func TestStaleReposLoadedMsgIgnored(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	// Bump generation so gen=0 messages are stale.
	m.preloader.generation = 1

	staleMsg := reposLoadedMsg{
		repos:  svc.repos,
		listID: m.focusedList.ID,
		gen:    0, // stale: model.generation is now 1
	}
	m2 := update(m, staleMsg)

	key := repoCacheKey{m.focusedList.ID, false}
	entry := m2.preloader.cache[key]
	if entry != nil && entry.state == repoCacheLoaded {
		t.Error("stale reposLoadedMsg should not write a repoCacheLoaded entry")
	}
}

// TestRepoCacheEntryWrittenOnLoad verifies that a fresh reposLoadedMsg writes a
// repoCacheLoaded entry and currentRepos() reflects the loaded data.
func TestRepoCacheEntryWrittenOnLoad(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	firstListID := m.lists[0].ID

	m2 := update(m, reposLoadedMsg{
		repos:      svc.repos,
		listID:     firstListID,
		withTopics: false,
		gen:        0,
	})

	key := repoCacheKey{firstListID, false}
	entry := m2.preloader.cache[key]
	if entry == nil {
		t.Fatal("repoCache entry should exist after reposLoadedMsg")
	}
	if entry.state != repoCacheLoaded {
		t.Errorf("entry.state = %d, want repoCacheLoaded", entry.state)
	}
	if len(m2.currentRepos()) != len(svc.repos) {
		t.Errorf("currentRepos len = %d, want %d", len(m2.currentRepos()), len(svc.repos))
	}
}

// TestAnyPendingDerivedFromMap verifies anyPending() returns true only when a
// repoCacheLoading entry exists in the map.
func TestAnyPendingDerivedFromMap(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	// newTestModel sets listsLoading=true; clear it so we can test repoCache alone.
	m.listsLoading = false

	if m.anyPending() {
		t.Error("anyPending should be false with empty cache and no flags set")
	}

	// Write a loading entry.
	key := repoCacheKey{listID: "UL_1", withTopics: false}
	m.preloader.cache[key] = &repoCacheEntry{state: repoCacheLoading}

	if !m.anyPending() {
		t.Error("anyPending should be true when a repoCacheLoading entry exists")
	}

	// Mark it loaded.
	m.preloader.cache[key] = &repoCacheEntry{state: repoCacheLoaded}

	if m.anyPending() {
		t.Error("anyPending should be false after entry transitions to repoCacheLoaded")
	}
}
