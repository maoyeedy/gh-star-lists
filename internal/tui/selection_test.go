package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSelectTogglesRepo(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.repoCursor = 0 // "owner/b-repo"

	m2 := update(m, keyPress(' '))

	nwo := m2.displayedRepos[0].NameWithOwner
	if _, ok := m2.selected[nwo]; !ok {
		t.Errorf("repo %q should be selected after space", nwo)
	}

	// Second space unmarks.
	m3 := update(m2, keyPress(' '))
	if _, ok := m3.selected[nwo]; ok {
		t.Errorf("repo %q should be unselected after second space", nwo)
	}
}

// TestSelectNoOpInListPane verifies space is a no-op in the list pane.
func TestSelectNoOpInListPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	// active is paneList by default

	m2 := update(m, keyPress(' '))

	if len(m2.selected) != 0 {
		t.Errorf("selected should be empty in list pane, got %d", len(m2.selected))
	}
}

// TestEscClearsSelectionFirst verifies Esc clears selection before navigating back.
func TestEscClearsSelectionFirst(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m = update(m, keyPress(' ')) // mark one repo

	if len(m.selected) == 0 {
		t.Fatal("precondition: selected should be non-empty")
	}

	m2 := update(m, specialKey(tea.KeyEsc))

	if len(m2.selected) != 0 {
		t.Errorf("selected should be cleared after Esc, got %d", len(m2.selected))
	}
	// Should stay in repo pane (not navigate back).
	if m2.active != paneRepo {
		t.Errorf("active = %v, want paneRepo (Esc should clear selection, not navigate)", m2.active)
	}
}
