package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func TestViewportPgDnMovesReposCursorByPageHeight(t *testing.T) {
	t.Parallel()
	svc := largeSvc(50)
	m := newTestModel(svc)
	m.height = 25 // contentH = 23, paneH = 23 -> step = 22
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})

	paneH := m.height - 2
	m2 := update(m, specialKey(tea.KeyPgDown))

	want := clampInt(paneH-1, 0, 49)
	if m2.repoCursor != want {
		t.Errorf("repoCursor = %d, want %d after PgDn", m2.repoCursor, want)
	}
}

// TestViewportGJumpsToTop verifies g/home resets cursor and offset to 0.
func TestViewportGJumpsToTop(t *testing.T) {
	t.Parallel()
	svc := largeSvc(50)
	m := newTestModel(svc)
	m.height = 25
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	// move to bottom first
	m = update(m, keyPress('G'))
	if m.repoCursor != 49 {
		t.Fatalf("G: repoCursor = %d, want 49", m.repoCursor)
	}
	// now go back to top
	m2 := update(m, keyPress('g'))
	if m2.repoCursor != 0 {
		t.Errorf("g: repoCursor = %d, want 0", m2.repoCursor)
	}
	if m2.repoOffset != 0 {
		t.Errorf("g: repoOffset = %d, want 0", m2.repoOffset)
	}
}

// TestViewportGCapitalJumpsToBottom verifies G jumps cursor to last item and
// slides offset so the cursor is visible.
func TestViewportGCapitalJumpsToBottom(t *testing.T) {
	t.Parallel()
	svc := largeSvc(50)
	m := newTestModel(svc)
	m.height = 25
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})

	m2 := update(m, keyPress('G'))

	if m2.repoCursor != 49 {
		t.Errorf("G: repoCursor = %d, want 49", m2.repoCursor)
	}
	// repoPaneH == full pane content height; no heading overhead.
	effectivePaneH := m2.repoPaneH()
	if m2.repoOffset != max(0, 50-effectivePaneH) {
		t.Errorf("G: repoOffset = %d, want %d", m2.repoOffset, max(0, 50-effectivePaneH))
	}
}

// TestViewportOffsetSlidesToKeepCursorVisible verifies that moving the cursor
// down past the visible window advances the offset.
func TestViewportOffsetSlidesToKeepCursorVisible(t *testing.T) {
	t.Parallel()
	svc := largeSvc(50)
	m := newTestModel(svc)
	m.height = 10 // contentH = 8
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})

	// move cursor to row 8 (one past visible window of 8)
	for range 8 {
		m = update(m, specialKey(tea.KeyDown))
	}

	if m.repoCursor != 8 {
		t.Fatalf("repoCursor = %d, want 8", m.repoCursor)
	}
	if m.repoOffset == 0 {
		t.Errorf("repoOffset should have slid, still 0")
	}
	// cursor must be within [offset, offset+paneH-1]
	paneH := m.height - 2
	if m.repoCursor < m.repoOffset || m.repoCursor >= m.repoOffset+paneH {
		t.Errorf(
			"cursor %d not in visible window [%d, %d)",
			m.repoCursor, m.repoOffset, m.repoOffset+paneH,
		)
	}
}

// TestViewportCursorVisibleInRender verifies that after G the rendered pane
// shows the last repo name.
func TestViewportCursorVisibleInRender(t *testing.T) {
	t.Parallel()
	svc := largeSvc(50)
	m := newTestModel(svc)
	m.height = 25
	m.width = 100
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m = update(m, keyPress('G'))

	paneH := m.height - 2
	rendered := repoPane(m, 50, paneH)
	if !strings.Contains(stripANSI(rendered), "owner/repo-49") {
		t.Errorf("rendered pane after G should show repo-49, got:\n%s", rendered)
	}
}

// TestViewportListPanePgDn verifies PgDn works in list pane too.
func TestViewportListPanePgDn(t *testing.T) {
	t.Parallel()
	n := 20
	lists := make([]domain.StarList, n)
	for i := 0; i < n; i++ {
		lists[i] = domain.StarList{
			ID:   fmt.Sprintf("UL_%d", i),
			Name: fmt.Sprintf("list-%02d", i),
		}
	}
	svc := &fakeService{lists: lists}
	m := newTestModel(svc)
	m.height = 12 // paneH = 10, step = 9
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m2 := update(m, specialKey(tea.KeyPgDown))

	paneH := m.height - 2
	want := clampInt(paneH-1, 0, n-1)
	if m2.listCursor != want {
		t.Errorf("listCursor = %d after PgDn, want %d", m2.listCursor, want)
	}
	if m2.listOffset == 0 && m2.listCursor >= paneH {
		t.Errorf("listOffset should have slid, still 0 with cursor %d", m2.listCursor)
	}
}
