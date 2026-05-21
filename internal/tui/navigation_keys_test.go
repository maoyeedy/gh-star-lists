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

// TestHelpOverlayScrollsWithUpDown verifies that Up/Down scroll the help
// overlay when showHelp is true.
func TestHelpOverlayScrollsWithUpDown(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.showHelp = true
	m.width = 80
	m.height = 5 // short terminal to force scrolling

	if m.helpViewportOffset != 0 {
		t.Errorf("initial helpViewportOffset = %d, want 0", m.helpViewportOffset)
	}

	// Down scrolls down by 1.
	m2 := update(m, specialKey(tea.KeyDown))
	if m2.helpViewportOffset != 1 {
		t.Errorf("after Down: helpViewportOffset = %d, want 1", m2.helpViewportOffset)
	}

	// Up scrolls back up by 1.
	m3 := update(m2, specialKey(tea.KeyUp))
	if m3.helpViewportOffset != 0 {
		t.Errorf("after Up: helpViewportOffset = %d, want 0", m3.helpViewportOffset)
	}

	// Up at 0 stays at 0.
	m4 := update(m3, specialKey(tea.KeyUp))
	if m4.helpViewportOffset != 0 {
		t.Errorf("after Up at 0: helpViewportOffset = %d, want 0", m4.helpViewportOffset)
	}
}

// TestHelpOverlayPgDnPgUpScrolls verifies that PgUp/PgDn scroll the help
// overlay by a full page.
func TestHelpOverlayPgDnPgUpScrolls(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.showHelp = true
	m.width = 80
	m.height = 5

	// PgDn scrolls by m.height.
	m2 := update(m, specialKey(tea.KeyPgDown))
	if m2.helpViewportOffset != 5 {
		t.Errorf("after PgDn: helpViewportOffset = %d, want 5", m2.helpViewportOffset)
	}

	// PgUp scrolls back up.
	m3 := update(m2, specialKey(tea.KeyPgUp))
	if m3.helpViewportOffset != 0 {
		t.Errorf("after PgUp: helpViewportOffset = %d, want 0", m3.helpViewportOffset)
	}
}

// TestHelpOverlayScrollingDoesNotAffectNormalNav verifies that help overlay
// key handling does not interfere with normal navigation when help is not shown.
func TestHelpOverlayScrollingDoesNotAffectNormalNav(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.width = 120
	m.height = 24
	// showHelp is false by default.

	m2 := update(m, specialKey(tea.KeyDown))
	if m2.listCursor != 1 {
		t.Errorf(
			"Down when help not shown should navigate normally, got listCursor = %d",
			m2.listCursor,
		)
	}
}

// TestHelpOverlayEscResetsOffset verifies that pressing Esc closes help and
// resets the scroll offset.
func TestHelpOverlayEscResetsOffset(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.showHelp = true
	m.helpViewportOffset = 3

	m2 := update(m, specialKey(tea.KeyEscape))
	if m2.showHelp {
		t.Error("showHelp should be false after Esc")
	}
	if m2.helpViewportOffset != 0 {
		t.Errorf("helpViewportOffset = %d after Esc, want 0", m2.helpViewportOffset)
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

// TestCursorPreservedOnEnter verifies that repoCursor and repoOffset are
// preserved when entering the repo pane via Enter (same behavior as Right arrow).
func TestCursorPreservedOnEnter(t *testing.T) {
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

	if m2.repoCursor != 15 {
		t.Errorf("repoCursor = %d after Enter, want 15 (preserved)", m2.repoCursor)
	}
	if m2.repoOffset != 10 {
		t.Errorf("repoOffset = %d after Enter, want 10 (preserved)", m2.repoOffset)
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
