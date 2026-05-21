package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestMouseClickFocusesPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.focusedList = &m.lists[0]
	m.width = 120
	m.height = 24
	m.active = paneList // start in list pane

	// leftW at width 120: 120*30/100 = 36. Repo pane starts at x=38.
	// Click row 2 (Y=2 => contentRow=1 => repoIdx = 1+repoOffset = 1).
	click := tea.MouseClickMsg{X: 50, Y: 2, Button: tea.MouseLeft}
	m2 := update(m, click)

	if m2.active != paneRepo {
		t.Errorf("active = %v after click in repo pane, want paneRepo", m2.active)
	}
	if m2.repoCursor != 1 {
		t.Errorf("repoCursor = %d after clicking row 2, want 1", m2.repoCursor)
	}
}

// --- Spinner migration tests ---

// TestSpinnerTickMsgUpdatesSpinner verifies that a spinner.TickMsg advances the
// spinner state when the model is in loading mode.

func TestHoverWheelScrollsListPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.focusedList = &m.lists[0]
	m.width = 120
	m.height = 24
	// Start with repo pane active and cursor in the middle so scrolling is possible.
	m.active = paneRepo
	m.listCursor = 1 // cursor at middle list item

	// g.sep1Col at width=120, showPreview=false: leftW = 120*30/100 = 36.
	// X=5 is within the list pane.
	before := m.listCursor
	wheel := tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelUp}
	m2 := update(m, wheel)

	if m2.listCursor == before {
		t.Errorf("listCursor unchanged after wheel-up over list pane: got %d", m2.listCursor)
	}
	if m2.active != paneRepo {
		t.Errorf("active pane changed by hover-wheel: got %v, want paneRepo", m2.active)
	}
}

// TestHoverWheelScrollsRepoPane verifies that a wheel event whose X coordinate
// falls inside the repo pane scrolls repoCursor, even when the list pane is
// active.
func TestHoverWheelScrollsRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.focusedList = &m.lists[0]
	m.width = 120
	m.height = 24
	// Start with list pane active.
	m.active = paneList
	m.repoCursor = 0

	// g.sep1Col at width=120, showPreview=false: leftW = 36, sep1Col = 36.
	// X=50 is in the repo pane (> sep1Col=36).
	before := m.repoCursor
	wheel := tea.MouseWheelMsg{X: 50, Y: 5, Button: tea.MouseWheelDown}
	m2 := update(m, wheel)

	if m2.repoCursor == before {
		t.Errorf("repoCursor unchanged after wheel-down over repo pane: got %d", m2.repoCursor)
	}
	if m2.active != paneList {
		t.Errorf("active pane changed by hover-wheel: got %v, want paneList", m2.active)
	}
}

func TestDoubleClickDrillsToRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.width = 120
	m.height = 24
	m.active = paneList

	// g.sep1Col at width=120 showPreview=false: leftW=36, sep1Col=36.
	// X=5 is in the list pane. Y=1 => contentRow=0 => idx=0.
	click := tea.MouseClickMsg{X: 5, Y: 1, Button: tea.MouseLeft}

	// First click: single click, stays in list pane.
	m2 := update(m, click)
	if m2.active != paneList {
		t.Errorf("active after single click = %v, want paneList", m2.active)
	}

	// Immediately inject a second click at the same location while
	// lastClickTime is still recent (we can't control real time in tests, so
	// we manipulate the tracker directly after the first click).
	m2.lastClickTime = time.Now() // ensure within 300ms window
	m3 := update(m2, click)

	if m3.active != paneRepo {
		t.Errorf("active after double-click = %v, want paneRepo", m3.active)
	}
	if m3.repoCursor != 0 {
		t.Errorf("repoCursor after double-click = %d, want 0", m3.repoCursor)
	}
	if m3.repoOffset != 0 {
		t.Errorf("repoOffset after double-click = %d, want 0", m3.repoOffset)
	}
}

// TestDoubleClickDispatchesLoadCmd verifies that a double-click returns the
// focusList load cmd instead of discarding it. The first click on an already-focused
// row with cached repos is a no-op; the second click within 300ms becomes a
// double-click that starts a load.
func TestDoubleClickDispatchesLoadCmd(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.width = 120
	m.height = 24
	m.active = paneList

	// Pre-populate cache so the first click does not start a load.
	if m.focusedList != nil {
		m.preloader.cache[repoCacheKey{m.focusedList.ID, false}] = &repoCacheEntry{
			state: repoCacheLoaded, repos: svc.repos, gen: m.preloader.generation,
		}
	}

	// First click on already-focused row -- triggers no load.
	click := tea.MouseClickMsg{X: 5, Y: 1, Button: tea.MouseLeft}
	m2 := update(m, click)

	// Set tracker time and delete cache so double-click starts a fresh load.
	m2.lastClickTime = time.Now()
	if m2.focusedList != nil {
		delete(m2.preloader.cache, repoCacheKey{m2.focusedList.ID, false})
	}
	m2.preloader.inFlight = 0
	m2.preloader.queue = nil

	// Second click -- use m.Update directly to capture cmd.
	next, cmd := m2.Update(click)
	m3 := next.(model)

	if m3.active != paneRepo {
		t.Errorf("active after double-click = %v, want paneRepo", m3.active)
	}
	if cmd == nil {
		t.Error("cmd should be non-nil after double-click (repo load command expected)")
	}
	// Verify the cmd produces a reposLoadedMsg for the focused list.
	msgs := executeBatch(cmd)
	if len(msgs) == 0 {
		t.Error("cmd produced no messages")
	} else {
		_, ok := msgs[0].(reposLoadedMsg)
		if !ok {
			t.Errorf("cmd produced %T, want reposLoadedMsg", msgs[0])
		}
	}
}

// TestDoubleClickDifferentRowNoSwitch verifies that two rapid clicks on
// different list rows do NOT switch pane.
func TestDoubleClickDifferentRowNoSwitch(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.width = 120
	m.height = 24
	m.active = paneList

	// First click: row 0 (Y=1).
	click1 := tea.MouseClickMsg{X: 5, Y: 1, Button: tea.MouseLeft}
	m2 := update(m, click1)
	m2.lastClickTime = time.Now() // ensure within 300ms window

	// Second click: row 1 (Y=2) -- different row.
	click2 := tea.MouseClickMsg{X: 5, Y: 2, Button: tea.MouseLeft}
	m3 := update(m2, click2)

	if m3.active != paneList {
		t.Errorf(
			"active after clicks on different rows = %v, want paneList (no double-click switch)",
			m3.active,
		)
	}
}

func TestSingleClickFocusedUncachedTriggersLoad(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Clear the focused list's cache entry to simulate idle state.
	if m.focusedList != nil {
		delete(m.preloader.cache, repoCacheKey{m.focusedList.ID, false})
	}
	m.preloader.inFlight = 0
	m.preloader.queue = nil
	m.active = paneList
	m.listCursor = 0
	m.width = 120
	m.height = 24

	// Single click on the already-focused row (idx 0, which is listCursor).
	// g.sep1Col at width=120 showPreview=false: leftW=36, sep1Col=36.
	// Y=1 => contentRow=0 => idx=0 (matches listCursor=0).
	click := tea.MouseClickMsg{X: 5, Y: 1, Button: tea.MouseLeft}
	// Record as if we already clicked (so next click is treated as first click on same row).
	m.lastClickPane = int(paneList)
	m.lastClickIndex = 0
	m.lastClickTime = time.Now().Add(-400 * time.Millisecond) // older than 300ms window

	m2 := update(m, click)

	// Should still be in list pane.
	if m2.active != paneList {
		t.Errorf("active = %v after single click on focused row, want paneList", m2.active)
	}
	// Load should be triggered.
	if m2.focusedList == nil {
		t.Fatal("focusedList is nil")
	}
	entry := m2.preloader.cache[repoCacheKey{m2.focusedList.ID, false}]
	if entry == nil || entry.state != repoCacheLoading {
		var state repoCacheState = -1
		if entry != nil {
			state = entry.state
		}
		t.Errorf(
			"cache entry state = %d after single click on idle focused row, want repoCacheLoading",
			state,
		)
	}
}
