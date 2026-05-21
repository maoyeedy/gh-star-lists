package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNavigateCursorDown(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m2 := update(m, specialKey(tea.KeyDown))

	if m2.listCursor != 1 {
		t.Errorf("listCursor = %d, want 1", m2.listCursor)
	}
}

// TestCursorClampedAtBottom verifies cursor doesn't exceed list length.
func TestCursorClampedAtBottom(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.listCursor = 2 // already at last item

	m2 := update(m, specialKey(tea.KeyDown))

	if m2.listCursor != 2 {
		t.Errorf("listCursor = %d, want 2 (clamped)", m2.listCursor)
	}
}

// TestCursorClampedAtTop verifies cursor doesn't go below 0.
func TestCursorClampedAtTop(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.listCursor = 0

	m2 := update(m, specialKey(tea.KeyUp))

	if m2.listCursor != 0 {
		t.Errorf("listCursor = %d, want 0 (clamped)", m2.listCursor)
	}
}

// TestVimKeys verifies j/k as cursor aliases.
func TestVimKeys(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m2 := update(m, keyPress('j'))
	if m2.listCursor != 1 {
		t.Errorf("j: listCursor = %d, want 1", m2.listCursor)
	}
	m3 := update(m2, keyPress('k'))
	if m3.listCursor != 0 {
		t.Errorf("k: listCursor = %d, want 0", m3.listCursor)
	}
}

// TestDrillIntoListOnEnter verifies that Enter in list pane switches to
// repo pane and sets loading.
func TestDrillIntoListOnEnter(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m2 := update(m, specialKey(tea.KeyEnter))

	if m2.active != paneRepo {
		t.Errorf("active = %v, want paneRepo", m2.active)
	}
	if !m2.anyPending() {
		t.Error("anyPending should be true after drilling into list")
	}
}

// TestBackFromRepoPane verifies Esc in repo pane returns focus to list pane
// while preserving the loaded repos and focusedList (v1.3 behaviour).
func TestBackFromRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	// Populate cache for the focused list.
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo // reposLoadedMsg may not change active pane

	m2 := update(m, specialKey(tea.KeyEscape))

	if m2.active != paneList {
		t.Errorf("active = %v after esc from repo pane, want paneList", m2.active)
	}
	if m2.focusedList == nil {
		t.Error("focusedList should be preserved after back")
	}
	if len(m2.currentRepos()) == 0 {
		t.Error("repos should be preserved after back")
	}
}

// TestEscFromListPaneQuits verifies Esc in list pane sends quit.
func TestEscFromListPaneQuits(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.active = paneList

	_, cmd := m.Update(specialKey(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("expected QuitMsg from esc in list pane")
	}
}

// TestQuitKeySendsQuit verifies q key sends quit.
func TestQuitKeySendsQuit(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	_, cmd := m.Update(keyPress('q'))
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("expected QuitMsg from q key")
	}
}

// TestHelpToggle verifies ? key toggles showHelp.
func TestHelpToggle(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	m2 := update(m, keyPress('?'))
	if !m2.showHelp {
		t.Error("showHelp should be true after first ?")
	}
	m3 := update(m2, keyPress('?'))
	if m3.showHelp {
		t.Error("showHelp should be false after second ?")
	}
}

// TestSortCycleListsPane verifies s key cycles sortLists through all modes.

func TestRepoMutationKeysNoOpInListPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList

	for _, k := range []rune{'a', 'x', 'm', 'u'} {
		m2 := update(m, keyPress(k))
		if m2.modal != nil {
			t.Errorf("key %c should be no-op in list pane, got modal", k)
		}
	}
}

func TestSelectionClearedOnFocusChange(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneList
	m.listCursor = 0
	// Populate a selection.
	m.selected = map[string]struct{}{"owner/a-repo": {}}

	// Enter in list pane drills into the list and should clear selection.
	m2 := update(m, specialKey(tea.KeyEnter))

	if len(m2.selected) != 0 {
		t.Errorf("selected should be nil/empty after focus change via Enter, got %v", m2.selected)
	}
}

// TestCursorResetOnFocusChange verifies that repoCursor and repoOffset are
// reset to 0 whenever the focused list changes via Enter key.
func TestCursorResetOnFocusChange(t *testing.T) {
	t.Parallel()
	svc := largeSvc(30)
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.repoCursor = 15
	m.repoOffset = 10
	// Switch back to list pane, then Enter to drill into same list again.
	m.active = paneList
	m.listCursor = 0

	m2 := update(m, specialKey(tea.KeyEnter))

	if m2.repoCursor != 0 {
		t.Errorf("repoCursor = %d after focus change, want 0", m2.repoCursor)
	}
	if m2.repoOffset != 0 {
		t.Errorf("repoOffset = %d after focus change, want 0", m2.repoOffset)
	}
}

func TestEnterListPaneNoLoadWhenCached(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Pre-populate cache for the focused list.
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: m.lists[0].ID, gen: m.generation})
	m.active = paneList
	m.listCursor = 0
	inflightBefore := m.preloadInFlight

	// Press Enter -- should switch to paneRepo with no new load.
	next, cmd := m.Update(specialKey(tea.KeyEnter))
	m2 := next.(model)

	if m2.active != paneRepo {
		t.Errorf("active = %v after Enter with cached list, want paneRepo", m2.active)
	}
	if cmd != nil {
		// Allow nil cmd (no load) -- if cmd is non-nil, check that inflight didn't increase.
		if m2.preloadInFlight > inflightBefore {
			t.Errorf(
				"preloadInFlight increased from %d to %d after Enter with cached list (should not start new load)",
				inflightBefore,
				m2.preloadInFlight,
			)
		}
	}
}
