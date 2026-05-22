package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func TestPreviewToggle(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	m2 := update(m, keyPress('p'))
	if !m2.showPreview {
		t.Error("showPreview should be true after p")
	}
	m3 := update(m2, keyPress('p'))
	if m3.showPreview {
		t.Error("showPreview should be false after second p")
	}
}

// TestStatusToastSetAndExpire verifies mutationDoneMsg sets toast and
// statusExpiredMsg clears it.

type topicTrackingService struct {
	fakeService
	withTopicsReceived bool
}

func (f *topicTrackingService) ListRepositories(
	_ context.Context,
	_ string,
	opts ...githubapi.ListOptions,
) ([]githubapi.Repository, error) {
	for _, opt := range opts {
		if opt.WithTopics {
			f.withTopicsReceived = true
		}
	}
	return f.repos, f.reposErr
}

// TestPreviewToggleLoadsTopics verifies 'p' in repo pane dispatches loadReposCmd
// with WithTopics=true.
func TestPreviewToggleLoadsTopics(t *testing.T) {
	t.Parallel()
	inner := threeListsSvc()
	svc := &topicTrackingService{fakeService: *inner}
	m := newModel(context.Background(), svc, Options{
		OpenBrowser: func(_ string) error { return nil },
	})
	m = update(m, listsLoadedMsg{lists: inner.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	// Populate repo cache for focused list (without topics).
	m = update(m, reposLoadedMsg{repos: inner.repos, listID: inner.lists[0].ID})
	m.active = paneRepo // restore pane after update

	// Toggle preview on.
	_, cmd := m.Update(keyPress('p'))
	if cmd == nil {
		t.Fatal("p in repo pane should dispatch a loadReposCmd")
	}
	// Execute the cmd to trigger the ListRepositories call.
	cmd()

	if !svc.withTopicsReceived {
		t.Error("WithTopics should be true when preview is toggled on")
	}
}

// TestPreviewNoReloadInListPane verifies 'p' in list pane only toggles without fetching.
func TestPreviewNoReloadInListPane(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.active = paneList

	_, cmd := m.Update(keyPress('p'))
	if cmd != nil {
		t.Error("p in list pane (no focused list) should not dispatch a cmd")
	}
}

func TestPreviewPaneRendersInThreeColumnLayout(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m.showPreview = true
	m.width = 120
	m.height = 24

	layout := m.renderLayout()
	// Three-column has two separators on each row.
	rows := strings.Split(layout, "\n")
	// Find a row that has exactly 2 "|" separators (content rows, not header/footer).
	found := false
	for _, row := range rows[1 : len(rows)-1] { // skip header + footer
		if strings.Count(row, "|") >= 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("three-column layout should have rows with at least 2 '|' separators")
	}
}

// TestUnstarRepoCmd verifies GetRepositoryMemberships is called and RemoveStar is invoked.

func TestPreviewDetailBlock(t *testing.T) {
	t.Parallel()
	repo := githubapi.Repository{
		ID:             "R_full",
		NameWithOwner:  "owner/full-repo",
		Description:    "A fully populated repository",
		URL:            "https://github.com/owner/full-repo",
		StargazerCount: 1234,
		Language:       "Go",
		License:        "MIT",
		IsFork:         false,
		IsArchived:     false,
		PushedAt:       "2024-01-15T00:00:00Z",
		StarredAt:      "2024-03-01T00:00:00Z",
		Topics:         []string{"cli", "github"},
	}
	svc := &fakeService{
		lists: []githubapi.StarList{{ID: "UL_1", Name: "list1", RepoCount: 1}},
		repos: []githubapi.Repository{repo},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m.showPreview = true
	m.repoCursor = 0

	rendered := previewPane(m, 50, 20)

	for _, want := range []string{
		"owner/full-repo",
		"https://github.com/owner/full-repo",
		"\u2605", // star glyph
		"Go",
		"Description",
		"Topics",
		"cli",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("preview pane missing %q; got:\n%s", want, rendered)
		}
	}
}

// TestPreviewFallbacks verifies that empty fields render the appropriate
// fallback text in the styled preview pane.
func TestPreviewFallbacks(t *testing.T) {
	t.Parallel()
	repo := githubapi.Repository{
		ID:            "R_empty",
		NameWithOwner: "owner/sparse-repo",
		URL:           "https://github.com/owner/sparse-repo",
		// Description, Language, License, Topics all zero/nil.
	}
	svc := &fakeService{
		lists: []githubapi.StarList{{ID: "UL_1", Name: "list1", RepoCount: 1}},
		repos: []githubapi.Repository{repo},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m.showPreview = true
	m.repoCursor = 0

	rendered := previewPane(m, 50, 20)

	if !strings.Contains(rendered, "(no description)") {
		t.Errorf(
			"preview pane should contain '(no description)' for empty description; got:\n%s",
			rendered,
		)
	}
	// "-" must appear for at least one empty field (language, license, topics).
	if !strings.Contains(rendered, "-") {
		t.Errorf("preview pane should contain '-' for empty fields; got:\n%s", rendered)
	}
}

// TestSelectionClearedOnFocusChange verifies that m.selected is set to nil

func TestPreviewWheelScrollsOffset(t *testing.T) {
	t.Parallel()
	// Use a repo with enough data that the preview pane has more than viewH lines.
	repo := githubapi.Repository{
		ID:             "R_1",
		NameWithOwner:  "owner/repo",
		Description:    "A test repo with topics to make preview content long",
		URL:            "https://github.com/owner/repo",
		StargazerCount: 42,
		Language:       "Go",
		License:        "MIT",
		PushedAt:       "2024-01-01T00:00:00Z",
		StarredAt:      "2024-03-01T00:00:00Z",
		Topics:         []string{"topic1", "topic2", "topic3"},
	}
	svc := &fakeService{
		lists: []githubapi.StarList{{ID: "UL_1", Name: "list", RepoCount: 1}},
		repos: []githubapi.Repository{repo},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m.showPreview = true
	m.width = 160
	m.height = 8 // small height so content overflows

	// sep2Col at width=160, showPreview=true, totalWidth>120:
	//   leftW = 160*22/100 = 35, midW = 160*28/100 = 44
	//   sep2Col = 35 + 1 + 44 = 80
	// So X=100 is in the preview pane.
	g := calcPaneGeometry(m.width, m.showPreview)
	previewX := g.sep2Col + 2 // safely inside preview pane

	before := m.previewOffset
	wheel := tea.MouseWheelMsg{X: previewX, Y: 3, Button: tea.MouseWheelDown}
	m2 := update(m, wheel)

	if m2.previewOffset <= before {
		t.Errorf(
			"previewOffset = %d after wheel-down over preview, want > %d",
			m2.previewOffset,
			before,
		)
	}
}

func TestPreviewOffsetResetsOnRepoCursorChange(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.focusedList = &m.lists[0]
	m.active = paneRepo
	m.showPreview = true
	m.previewOffset = 4

	m2 := update(m, specialKey(tea.KeyDown))

	if m2.repoCursor != 1 {
		t.Fatalf("repoCursor = %d after down, want 1", m2.repoCursor)
	}
	if m2.previewOffset != 0 {
		t.Errorf("previewOffset = %d after repo cursor change, want 0", m2.previewOffset)
	}
}

// TestPreviewToggleSchedulesTopicsLoadForFocusedListOnly verifies that toggling
// showPreview on only creates a withTopics=true loading entry for the focused list,
// not for other lists.
func TestPreviewToggleSchedulesTopicsLoadForFocusedListOnly(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	// Focused list is lists[0] = UL_1.
	m.focusedList = &m.lists[0]
	m.active = paneRepo

	// Toggle preview on.
	m2 := update(m, keyPress('p'))

	// Only UL_1 should have a withTopics=true entry.
	e1 := m2.preloader.cache[repoCacheKey{"UL_1", true}]
	if e1 == nil {
		t.Error("repoCache[UL_1, true] should exist after preview toggle for focused list")
	}
	// Other lists should NOT have withTopics=true entries.
	if e := m2.preloader.cache[repoCacheKey{"UL_2", true}]; e != nil {
		t.Errorf("repoCache[UL_2, true] should not exist; focused list is UL_1")
	}
	if e := m2.preloader.cache[repoCacheKey{"UL_3", true}]; e != nil {
		t.Errorf("repoCache[UL_3, true] should not exist; focused list is UL_1")
	}
}

func TestPreviewUsesTopicsRepoByIDAfterSort(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.focusedList = &m.lists[0]
	m.active = paneRepo
	m.showPreview = true
	m.displayedRepos = []githubapi.Repository{
		{ID: "R_1", NameWithOwner: "owner/z", StargazerCount: 1},
		{ID: "R_2", NameWithOwner: "owner/a", StargazerCount: 100},
	}
	m.repoCursor = 1
	m.preloader.setCacheEntry(repoCacheKey{m.focusedList.ID, true}, &repoCacheEntry{
		state: repoCacheLoaded,
		repos: []githubapi.Repository{
			{ID: "R_1", NameWithOwner: "owner/z", Topics: []string{"wrong"}},
			{ID: "R_2", NameWithOwner: "owner/a", Topics: []string{"right"}},
		},
		gen: m.preloader.generation,
	})

	lines := strings.Join(m.previewContentLines(80), "\n")
	if !strings.Contains(lines, "right") {
		t.Fatalf("preview lines = %q, want topics for repo R_2", lines)
	}
	if strings.Contains(lines, "wrong") {
		t.Fatalf("preview lines = %q, should not use cache index topic", lines)
	}
}

func TestPreviewUsesTopicsRepoByNameAfterSearch(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.focusedList = &m.lists[0]
	m.active = paneRepo
	m.showPreview = true
	m.displayedRepos = []githubapi.Repository{{NameWithOwner: "owner/match"}}
	m.repoCursor = 0
	m.preloader.setCacheEntry(repoCacheKey{m.focusedList.ID, true}, &repoCacheEntry{
		state: repoCacheLoaded,
		repos: []githubapi.Repository{
			{NameWithOwner: "owner/other", Topics: []string{"wrong"}},
			{NameWithOwner: "owner/match", Topics: []string{"right"}},
		},
		gen: m.preloader.generation,
	})

	lines := strings.Join(m.previewContentLines(80), "\n")
	if !strings.Contains(lines, "right") {
		t.Fatalf("preview lines = %q, want topics for owner/match", lines)
	}
	if strings.Contains(lines, "wrong") {
		t.Fatalf("preview lines = %q, should not use cache index topic", lines)
	}
}
