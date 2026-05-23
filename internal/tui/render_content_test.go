package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func TestHelpViewContainsAllKeys(t *testing.T) {
	t.Parallel()
	mo := newHelpModal()
	mo.scrollOffset = 0

	view := mo.view()

	for _, want := range []string{"up/k", "down/j", "enter", "esc", "o", "s", "ctrl+r", "q"} {
		if !containsStr(view, want) {
			t.Errorf("help modal view missing key %q; got:\n%s", want, view)
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
	m.preloader.cache[m.focusedList.ID] = &repoCacheEntry{
		state: repoCacheLoading,
	}
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

// TestHelpModalContainsV12Keys verifies the rendered help modal references
// v1.2 additions and new v1.3 Left/Right keys.
func TestHelpModalContainsV12Keys(t *testing.T) {
	t.Parallel()
	mo := newHelpModal()
	mo.scrollOffset = 0

	view := mo.view()
	for _, want := range []string{"space", "/", "pgup", "g", "left", "right"} {
		if !strings.Contains(view, want) {
			t.Errorf("help modal missing %q; got:\n%s", want, view)
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

func TestHalfWidthLayoutPrioritizesRepoNames(t *testing.T) {
	t.Parallel()
	repos := []domain.Repository{
		{
			ID:             "R_1",
			NameWithOwner:  "solidjs/solid-start",
			StargazerCount: 2900,
			Language:       "HTML",
		},
	}
	svc := &fakeService{
		lists: []domain.StarList{{ID: "UL_1", Name: "Webdev", RepoCount: 30}},
		repos: repos,
	}
	m := newTestModel(svc)
	m.width = 80
	m.height = 12
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	rendered := m.renderContent()
	plain := stripANSI(rendered)

	if !strings.Contains(plain, "solidjs/solid-start") {
		t.Fatalf("half-width layout should keep repo name visible; got:\n%s", rendered)
	}
	if strings.Contains(rendered, starGlyph) {
		t.Fatalf("half-width repo pane should hide stars; got:\n%s", rendered)
	}
	if strings.Contains(plain, "HTML") {
		t.Fatalf("half-width repo pane should hide language; got:\n%s", rendered)
	}
	if g := calcPaneGeometry(80); g.leftWidth != 24 || g.repoWidth != 55 {
		t.Fatalf("width 80 geometry = %+v, want left 24 repo 55", g)
	}
}

func TestNarrowLayoutRendersOnlyActivePane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m.width = 71
	m.height = 12
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	repoView := stripANSI(m.renderContent())
	if !strings.Contains(repoView, "Repos (2)") {
		t.Fatalf("repo-only narrow view missing repo pane; got:\n%s", repoView)
	}
	if strings.Contains(repoView, "Lists (") {
		t.Fatalf("repo-only narrow view should not render list pane; got:\n%s", repoView)
	}
	if strings.Contains(repoView, "|") {
		t.Fatalf("single-pane narrow view should not render separator; got:\n%s", repoView)
	}

	m.active = paneList
	listView := stripANSI(m.renderContent())
	if !strings.Contains(listView, "Lists (4)") {
		t.Fatalf("list-only narrow view missing list pane; got:\n%s", listView)
	}
	if strings.Contains(listView, "Repos (") {
		t.Fatalf("list-only narrow view should not render repo pane; got:\n%s", listView)
	}
}
