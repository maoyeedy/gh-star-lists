package tui

import (
	"testing"

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

	// g.sep1Col at width=120: leftW = 120*30/100 = 36.
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

	// g.sep1Col at width=120: leftW = 36, sep1Col = 36.
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

func TestSingleClickFocusedUncachedTriggersLoad(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Clear the focused list's cache entry to simulate idle state.
	if m.focusedList != nil {
		delete(m.preloader.cache, m.focusedList.ID)
	}
	m.preloader.inFlight = 0
	m.preloader.queue = nil
	m.active = paneList
	m.listCursor = 0
	m.width = 120
	m.height = 24

	// Single click on the already-focused row (idx 0, which is listCursor).
	// g.sep1Col at width=120: leftW=36, sep1Col=36.
	// Y=1 => contentRow=0 => idx=0 (matches listCursor=0).
	click := tea.MouseClickMsg{X: 5, Y: 1, Button: tea.MouseLeft}

	m2 := update(m, click)

	// Should still be in list pane.
	if m2.active != paneList {
		t.Errorf("active = %v after single click on focused row, want paneList", m2.active)
	}
	// Load should be triggered.
	if m2.focusedList == nil {
		t.Fatal("focusedList is nil")
	}
	entry := m2.preloader.cache[m2.focusedList.ID]
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
