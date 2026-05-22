package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func TestListRowsSimplified(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.width = 80
	m.height = 24

	rendered := m.renderListPane(30, 10)

	// Internal "|" column separator must be gone.
	// (Layout separators between panes are in renderLayout, not renderListPane.)
	if strings.Contains(rendered, " | ") {
		t.Errorf("renderListPane should not contain ' | ' column separator; got:\n%s", rendered)
	}
	// Age-like strings (e.g. "3d ago", "1w", "2mo") must be absent.
	for _, ageToken := range []string{"d ago", "h ago", "mo ago", "y ago", "now"} {
		if strings.Contains(rendered, ageToken) {
			t.Errorf("renderListPane should not contain age token %q; got:\n%s", ageToken, rendered)
		}
	}
	// Repo count for one of the test lists (e.g. "5" for "Alpha") must appear.
	found := false
	for _, l := range svc.lists {
		if strings.Contains(rendered, fmt.Sprintf("%d", l.RepoCount)) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("renderListPane should contain at least one repo count; got:\n%s", rendered)
	}
}

// TestSearchCountIndicator verifies the N/total indicator in the search bar.
func TestSearchCountIndicator(t *testing.T) {
	t.Parallel()
	// Build a service with 5 lists, 2 of which match "alp".
	lists := []domain.StarList{
		{ID: "1", Name: "Alpha", RepoCount: 1},
		{ID: "2", Name: "Alpine", RepoCount: 2},
		{ID: "3", Name: "beta", RepoCount: 3},
		{ID: "4", Name: "gamma", RepoCount: 0},
		{ID: "5", Name: "delta", RepoCount: 7},
	}
	svc := &fakeService{lists: lists}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: lists})
	m.active = paneList
	m.listSearchActive = true
	m.listSearchQuery = "alp"
	m = m.rebuildDisplayed()
	// Should have 2 matches ("Alpha", "Alpine").
	if len(m.displayedLists) != 2 {
		t.Fatalf("expected 2 matches for 'alp', got %d", len(m.displayedLists))
	}

	// Wide enough: count should appear.
	wideRendered := m.renderListPane(80, 10)
	if !strings.Contains(wideRendered, "2/5") {
		t.Errorf("wide renderListPane should contain '2/5' search count; got:\n%s", wideRendered)
	}

	// Narrow: count should be dropped.
	// At 8 cols: prefixW(2) + min_query(4) + gap(2) + countW(3) = 11 > 8, so count is dropped.
	narrowRendered := m.renderListPane(8, 10)
	if strings.Contains(narrowRendered, "2/5") {
		t.Errorf(
			"narrow renderListPane should NOT contain '2/5' search count; got:\n%s",
			narrowRendered,
		)
	}
}
