package tui

import (
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func TestLoadingCountO1(t *testing.T) {
	t.Parallel()
	p := newPreloader()
	key1 := repoCacheKey{listID: "UL_1"}
	key2 := repoCacheKey{listID: "UL_2"}
	key3 := repoCacheKey{listID: "UL_3"}
	key4 := repoCacheKey{listID: "UL_4"}

	p.setCacheEntry(key1, &repoCacheEntry{state: repoCacheLoading})
	p.setCacheEntry(key2, &repoCacheEntry{state: repoCacheLoading})
	if p.loadingCount != 2 {
		t.Fatalf("loadingCount after two loads = %d, want 2", p.loadingCount)
	}
	if !p.anyPendingInCache() {
		t.Fatal("anyPendingInCache should be true while entries are loading")
	}

	p.deleteCacheEntry(key1)
	if p.loadingCount != 1 {
		t.Fatalf("loadingCount after cancel/delete = %d, want 1", p.loadingCount)
	}

	p.setCacheEntry(key2, &repoCacheEntry{state: repoCacheError})
	if p.loadingCount != 0 {
		t.Fatalf("loadingCount after error transition = %d, want 0", p.loadingCount)
	}

	p.setCacheEntry(key3, &repoCacheEntry{state: repoCacheLoading})
	p.setCacheEntry(key3, &repoCacheEntry{state: repoCacheLoaded})
	if p.loadingCount != 0 {
		t.Fatalf("loadingCount after loaded transition = %d, want 0", p.loadingCount)
	}

	p.setCacheEntry(key4, &repoCacheEntry{state: repoCacheLoading})
	p.clear()
	if p.loadingCount != 0 {
		t.Fatalf("loadingCount after clear = %d, want 0", p.loadingCount)
	}
	if p.anyPendingInCache() {
		t.Fatal("anyPendingInCache should be false after clear")
	}
}

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

func TestAnyPendingUsesLoadingCount(t *testing.T) {
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
	m.preloader.setCacheEntry(key, &repoCacheEntry{state: repoCacheLoading})

	if !m.anyPending() {
		t.Error("anyPending should be true when a repoCacheLoading entry exists")
	}

	// Mark it loaded.
	m.preloader.setCacheEntry(key, &repoCacheEntry{state: repoCacheLoaded})

	if m.anyPending() {
		t.Error("anyPending should be false after entry transitions to repoCacheLoaded")
	}
}

func TestStaleLoadDropped(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	initialRepos := []githubapi.Repository{{ID: "R_init", NameWithOwner: "owner/init"}}
	m.displayedRepos = initialRepos

	// Test 1: stale generation.
	staleMsg := reposLoadedMsg{
		repos:  svc.repos,
		listID: m.focusedList.ID,
		gen:    999,
	}
	m2 := update(m, staleMsg)

	key := repoCacheKey{m.focusedList.ID, false}
	if e := m2.preloader.cache[key]; e != nil && e.state == repoCacheLoaded {
		t.Error("stale reposLoadedMsg (gen=999) should not create a loaded cache entry")
	}
	if len(m2.displayedRepos) != 1 || m2.displayedRepos[0].NameWithOwner != "owner/init" {
		t.Error("displayedRepos should be unchanged after stale reposLoadedMsg")
	}

	// Test 2: cancelled load - cache entry removed, valid gen.
	delete(m2.preloader.cache, key)
	cancelledMsg := reposLoadedMsg{
		repos:  svc.repos,
		listID: m.focusedList.ID,
		gen:    m2.preloader.generation,
	}
	m3 := update(m2, cancelledMsg)

	if e := m3.preloader.cache[key]; e != nil && e.state == repoCacheLoaded {
		t.Error("reposLoadedMsg for cancelled load (removed cache entry) should be dropped")
	}
	if len(m3.displayedRepos) != 1 || m3.displayedRepos[0].NameWithOwner != "owner/init" {
		t.Error("displayedRepos should be unchanged after cancelled-load reposLoadedMsg")
	}
}
