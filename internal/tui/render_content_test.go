package tui

import (
	"errors"
	"strings"
	"testing"
)

func TestHelpViewContainsAllKeys(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.showHelp = true
	m.width = 80
	m.height = 24

	view := m.renderHelp()

	for _, want := range []string{"up/k", "down/j", "enter", "esc", "o", "s", "ctrl+r", "?", "q"} {
		if !containsStr(view, want) {
			t.Errorf("help view missing key %q", want)
		}
	}
}

func TestErrorRendersInView(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.err = errors.New("api error")
	m.listsLoading = false
	m.width = 80
	m.height = 24

	view := m.renderContent()
	if !containsStr(view, "Error:") {
		t.Errorf("error view = %q, want to contain 'Error:'", view)
	}
}

// TestLoadingRendersInView verifies the loading state renders without crash.
func TestLoadingRendersInView(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.listsLoading = true
	m.width = 80
	m.height = 24

	view := m.renderContent()
	if !containsStr(view, "Loading") {
		t.Errorf("loading view = %q, want to contain 'Loading'", view)
	}
}

func TestLoadingRendersInsidePane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.focusedList = &m.lists[0]
	// Mark the focused list's cache entry as loading to simulate repo fetch in flight.
	m.repoCache[repoCacheKey{m.focusedList.ID, false}] = &repoCacheEntry{state: repoCacheLoading}
	m.width = 120
	m.height = 24

	content := m.renderContent()
	if content == "Loading..." {
		t.Error("renderContent returned bare Loading... -- pane-local spinner not active")
	}
	// The list pane content (at least one list name) should be visible.
	if !strings.Contains(content, svc.lists[0].Name) {
		t.Errorf("list pane not rendered during repo load; content:\n%s", content)
	}
}

// TestHelpOverlayContainsV12Keys verifies the rendered help overlay references
// v1.2 additions and new v1.3 Left/Right keys.
func TestHelpOverlayContainsV12Keys(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m.width = 120
	m.height = 40

	help := m.renderHelp()
	for _, want := range []string{"space", "/", "pgup", "g", "left", "right"} {
		if !strings.Contains(help, want) {
			t.Errorf("renderHelp missing %q; got:\n%s", want, help)
		}
	}
}

func TestEagerInitialLoad(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)

	next, cmd := m.Update(listsLoadedMsg{lists: svc.lists})
	m2 := next.(model)

	if m2.focusedList == nil {
		t.Error("focusedList should be non-nil after eager initial load")
	}
	if m2.repoCursor != 0 {
		t.Errorf("repoCursor = %d, want 0", m2.repoCursor)
	}
	if m2.repoOffset != 0 {
		t.Errorf("repoOffset = %d, want 0", m2.repoOffset)
	}
	if m2.selected != nil {
		t.Errorf("selected should be nil after eager load, got %v", m2.selected)
	}
	if cmd == nil {
		t.Error("returned cmd should be non-nil (repo load command expected)")
	}
}

// TestNoPressEnterHint verifies that the repo pane no longer shows a
// "(press enter to view repos)" placeholder in any state.
func TestNoPressEnterHint(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	// Do NOT load repos -- focusedList is set by eager load, loading=true.

	rendered := repoPane(m, 80, 20)

	if strings.Contains(rendered, "press enter") {
		t.Errorf("repo pane should not contain 'press enter' hint; got:\n%s", rendered)
	}
}
