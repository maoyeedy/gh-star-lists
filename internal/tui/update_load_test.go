package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestListsLoadedPopulatesLists(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)

	m2 := update(m, listsLoadedMsg{lists: svc.lists})

	if len(m2.lists) != 3 {
		t.Fatalf("lists len = %d, want 3", len(m2.lists))
	}
	// Eager load: focusedList is set and anyPending is true (awaiting repos).
	if m2.focusedList == nil {
		t.Error("focusedList should be non-nil after eager initial load")
	}
	if !m2.anyPending() {
		t.Error("anyPending should be true after listsLoadedMsg (eager repo fetch in flight)")
	}
}

// TestReposLoadedPopulatesRepos verifies that receiving reposLoadedMsg
// populates the repo cache and resolves focusedList from the current lists.
// With the preloader, all three lists start loading after listsLoadedMsg, so
// we deliver all three responses to reach a fully idle state.
func TestReposLoadedPopulatesRepos(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo

	// Deliver repos for all lists so preloading completes and anyPending is false.
	m2 := update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m2 = update(m2, reposLoadedMsg{repos: svc.repos, listID: "UL_2"})
	m2 = update(m2, reposLoadedMsg{repos: svc.repos, listID: "UL_3"})

	if len(m2.currentRepos()) != 2 {
		t.Fatalf("currentRepos len = %d, want 2", len(m2.currentRepos()))
	}
	if m2.anyPending() {
		t.Error("anyPending should be false after all reposLoadedMsg delivered")
	}
	if m2.focusedList == nil || m2.focusedList.ID != "UL_1" {
		t.Errorf("focusedList = %v, want ID UL_1", m2.focusedList)
	}
}

// TestErrMsgSetsError verifies that errMsg sets the error field.
func TestErrMsgSetsError(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	sentinel := errors.New("boom")
	m2 := update(m, errMsg{err: sentinel})

	if !errors.Is(m2.err, sentinel) {
		t.Errorf("err = %v, want %v", m2.err, sentinel)
	}
	if m2.anyPending() {
		t.Error("anyPending should be false after errMsg")
	}
}

func TestWindowSizeSetsWidthHeight(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	m2 := update(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if m2.width != 120 || m2.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", m2.width, m2.height)
	}
}
