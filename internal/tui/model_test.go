package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

// fakeService implements githubapi.Service with canned responses.
type fakeService struct {
	lists    []githubapi.StarList
	repos    []githubapi.Repository
	listErr  error
	reposErr error
}

func (f *fakeService) ListStarLists(
	_ context.Context,
	_ ...githubapi.ListOptions,
) ([]githubapi.StarList, error) {
	return f.lists, f.listErr
}

func (f *fakeService) ListRepositories(
	_ context.Context,
	_ string,
	_ ...githubapi.ListOptions,
) ([]githubapi.Repository, error) {
	return f.repos, f.reposErr
}

func (f *fakeService) ListStarredRepositories(
	_ context.Context,
	_ ...githubapi.ListOptions,
) ([]githubapi.Repository, error) {
	return nil, nil
}

func (f *fakeService) GetRepository(_ context.Context, _ string) (githubapi.Repository, error) {
	return githubapi.Repository{}, nil
}

func (f *fakeService) GetRepositoryMemberships(
	_ context.Context,
	_ string,
) (string, []string, error) {
	return "", nil, nil
}

func (f *fakeService) CreateStarList(
	_ context.Context,
	_ githubapi.StarListInput,
) (githubapi.StarList, error) {
	return githubapi.StarList{}, nil
}

func (f *fakeService) UpdateStarList(
	_ context.Context,
	_ githubapi.UpdateStarListInput,
) (githubapi.StarList, error) {
	return githubapi.StarList{}, nil
}
func (f *fakeService) DeleteStarList(_ context.Context, _ string) error { return nil }
func (f *fakeService) UpdateRepositoryLists(_ context.Context, _ string, _ []string) error {
	return nil
}
func (f *fakeService) AddStar(_ context.Context, _ string) error    { return nil }
func (f *fakeService) RemoveStar(_ context.Context, _ string) error { return nil }

type fakeInvalidatableService struct {
	fakeService
	invalidateCalls int
}

func (f *fakeInvalidatableService) Invalidate() { f.invalidateCalls++ }

func threeListsSvc() *fakeService {
	return &fakeService{
		lists: []githubapi.StarList{
			{
				ID:          "UL_1",
				Name:        "zeta",
				RepoCount:   1,
				LastAddedAt: "2024-05-03T00:00:00Z",
				URL:         "https://example.com/1",
			},
			{
				ID:          "UL_2",
				Name:        "Alpha",
				RepoCount:   5,
				LastAddedAt: "2024-05-02T00:00:00Z",
				URL:         "https://example.com/2",
			},
			{
				ID:          "UL_3",
				Name:        "beta",
				RepoCount:   3,
				LastAddedAt: "2024-05-01T00:00:00Z",
				URL:         "https://example.com/3",
			},
		},
		repos: []githubapi.Repository{
			{
				ID:             "R_1",
				NameWithOwner:  "owner/b-repo",
				StargazerCount: 10,
				PushedAt:       "2024-05-02T00:00:00Z",
				URL:            "https://github.com/owner/b-repo",
			},
			{
				ID:             "R_2",
				NameWithOwner:  "owner/a-repo",
				StargazerCount: 50,
				PushedAt:       "2024-05-03T00:00:00Z",
				URL:            "https://github.com/owner/a-repo",
			},
		},
	}
}

func keyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func ctrlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}

func newTestModel(svc githubapi.Service) model {
	return newModel(context.Background(), svc, Options{
		OpenBrowser: func(_ string) error { return nil },
	})
}

func update(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}

// TestListsLoadedPopulatesLists verifies that receiving listsLoadedMsg
// sets the lists field and clears loading state.
func TestListsLoadedPopulatesLists(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)

	m2 := update(m, listsLoadedMsg{lists: svc.lists})

	if len(m2.lists) != 3 {
		t.Fatalf("lists len = %d, want 3", len(m2.lists))
	}
	if m2.loading {
		t.Error("loading should be false after listsLoadedMsg")
	}
}

// TestReposLoadedPopulatesRepos verifies that receiving reposLoadedMsg
// sets the repos field and resolves focusedList from the current lists.
func TestReposLoadedPopulatesRepos(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo

	m2 := update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})

	if len(m2.repos) != 2 {
		t.Fatalf("repos len = %d, want 2", len(m2.repos))
	}
	if m2.loading {
		t.Error("loading should be false after reposLoadedMsg")
	}
	if m2.focusedList == nil || m2.focusedList.ID != "UL_1" {
		t.Errorf("focusedList = %v, want ID UL_1", m2.focusedList)
	}
}

// TestErrMsgSetsError verifies that errMsg sets the error field.
func TestErrMsgSetsError(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	sentinel := errors.New("boom")
	m2 := update(m, errMsg{err: sentinel})

	if !errors.Is(m2.err, sentinel) {
		t.Errorf("err = %v, want %v", m2.err, sentinel)
	}
	if m2.loading {
		t.Error("loading should be false after errMsg")
	}
}

// TestNavigateCursorDown verifies cursor advances on down key.
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
	if !m2.loading {
		t.Error("loading should be true after drilling into list")
	}
}

// TestBackFromRepoPane verifies Esc in repo pane goes back to list pane.
func TestBackFromRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.repos = svc.repos
	m.focusedList = &m.lists[0]

	m2 := update(m, specialKey(tea.KeyEscape))

	if m2.active != paneList {
		t.Errorf("active = %v after esc from repo pane, want paneList", m2.active)
	}
	if m2.focusedList != nil {
		t.Error("focusedList should be nil after back")
	}
	if len(m2.repos) != 0 {
		t.Error("repos should be cleared after back")
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
func TestSortCycleListsPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList
	initial := m.sortLists

	m2 := update(m, keyPress('s'))
	if m2.sortLists == initial {
		t.Error("sortLists should change after s key")
	}
	// Cycle 4 times should return to start
	cur := m
	for i := 0; i < 4; i++ {
		cur = update(cur, keyPress('s'))
	}
	if cur.sortLists != initial {
		t.Errorf("sortLists after 4 cycles = %d, want %d (initial)", cur.sortLists, initial)
	}
}

// TestSortCycleReposPane verifies s key cycles sortRepos in repo pane.
func TestSortCycleReposPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	initial := m.sortRepos

	m2 := update(m, keyPress('s'))
	if m2.sortRepos == initial {
		t.Error("sortRepos should change after s key")
	}
	cur := m
	for i := 0; i < 4; i++ {
		cur = update(cur, keyPress('s'))
	}
	if cur.sortRepos != initial {
		t.Errorf("sortRepos after 4 cycles = %d, want %d (initial)", cur.sortRepos, initial)
	}
}

// TestRefreshCallsInvalidate verifies ctrl+r calls Invalidate on a cache-like service.
func TestRefreshCallsInvalidate(t *testing.T) {
	t.Parallel()
	svc := &fakeInvalidatableService{
		fakeService: fakeService{
			lists: []githubapi.StarList{{ID: "UL_1", Name: "Go Tools"}},
		},
	}
	m := newModel(context.Background(), svc, Options{})
	m = update(m, listsLoadedMsg{lists: svc.lists})

	update(m, ctrlKey('r'))

	if svc.invalidateCalls != 1 {
		t.Errorf("Invalidate calls = %d, want 1", svc.invalidateCalls)
	}
}

// TestRefreshNoInvalidateOnPlainService verifies ctrl+r is safe on a
// service without Invalidate (no panic, just reload).
func TestRefreshNoInvalidateOnPlainService(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Must not panic
	m2 := update(m, ctrlKey('r'))
	if !m2.loading {
		t.Error("loading should be true after refresh")
	}
}

// TestOpenInBrowserListPane verifies o key in list pane calls openBrowser
// with the focused list URL.
func TestOpenInBrowserListPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	var captured string
	m := newModel(context.Background(), svc, Options{
		OpenBrowser: func(url string) error { captured = url; return nil },
	})
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList
	m.listCursor = 0

	update(m, keyPress('o'))

	want := svc.lists[0].URL
	if captured != want {
		t.Errorf("opened URL = %q, want %q", captured, want)
	}
}

// TestOpenInBrowserRepoPane verifies Enter in repo pane calls openBrowser
// with the focused repo URL.
func TestOpenInBrowserRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	var captured string
	m := newModel(context.Background(), svc, Options{
		OpenBrowser: func(url string) error { captured = url; return nil },
	})
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.repoCursor = 0

	update(m, specialKey(tea.KeyEnter))

	want := svc.repos[0].URL
	if captured != want {
		t.Errorf("opened URL = %q, want %q", captured, want)
	}
}

// TestHelpViewContainsAllKeys verifies help overlay mentions every key
// binding to catch accidental drift.
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

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestSortStarListsByName verifies that sortStarLists with sortListsName
// sorts case-insensitively ascending.
func TestSortStarListsByName(t *testing.T) {
	t.Parallel()
	lists := []githubapi.StarList{
		{ID: "3", Name: "zeta"},
		{ID: "1", Name: "Alpha"},
		{ID: "2", Name: "beta"},
	}
	sortStarLists(lists, sortListsName)
	names := []string{lists[0].Name, lists[1].Name, lists[2].Name}
	want := []string{"Alpha", "beta", "zeta"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("lists[%d].Name = %q, want %q", i, n, want[i])
		}
	}
}

// TestSortStarListsByRepoCount verifies descending repo count sort.
func TestSortStarListsByRepoCount(t *testing.T) {
	t.Parallel()
	lists := []githubapi.StarList{
		{ID: "1", Name: "a", RepoCount: 3},
		{ID: "2", Name: "b", RepoCount: 1},
		{ID: "3", Name: "c", RepoCount: 5},
	}
	sortStarLists(lists, sortListsRepos)
	if lists[0].RepoCount != 5 || lists[1].RepoCount != 3 || lists[2].RepoCount != 1 {
		t.Errorf(
			"unexpected order: %d %d %d",
			lists[0].RepoCount,
			lists[1].RepoCount,
			lists[2].RepoCount,
		)
	}
}

// TestSortReposByStars verifies descending star sort.
func TestSortReposByStars(t *testing.T) {
	t.Parallel()
	repos := []githubapi.Repository{
		{NameWithOwner: "a/a", StargazerCount: 10},
		{NameWithOwner: "b/b", StargazerCount: 50},
		{NameWithOwner: "c/c", StargazerCount: 1},
	}
	sortRepos(repos, sortReposStars)
	if repos[0].StargazerCount != 50 || repos[1].StargazerCount != 10 ||
		repos[2].StargazerCount != 1 {
		t.Errorf(
			"unexpected star order: %d %d %d",
			repos[0].StargazerCount,
			repos[1].StargazerCount,
			repos[2].StargazerCount,
		)
	}
}

// TestSortReposByPushed verifies descending pushed-at sort.
func TestSortReposByPushed(t *testing.T) {
	t.Parallel()
	repos := []githubapi.Repository{
		{NameWithOwner: "a/a", PushedAt: "2024-01-01T00:00:00Z"},
		{NameWithOwner: "b/b", PushedAt: "2024-03-01T00:00:00Z"},
		{NameWithOwner: "c/c", PushedAt: "2024-02-01T00:00:00Z"},
	}
	sortRepos(repos, sortReposPushed)
	if repos[0].NameWithOwner != "b/b" {
		t.Errorf("first by pushed = %q, want b/b", repos[0].NameWithOwner)
	}
}

// TestShortAge verifies the human-readable age formatter.
func TestShortAge(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		ts   string
		want string
	}{
		{"", "-"},
		{now.Add(-30 * time.Second).Format(time.RFC3339), "now"},
		{now.Add(-5 * time.Minute).Format(time.RFC3339), "5m ago"},
		{now.Add(-3 * time.Hour).Format(time.RFC3339), "3h ago"},
		{now.Add(-2 * 24 * time.Hour).Format(time.RFC3339), "2d ago"},
		{now.Add(-45 * 24 * time.Hour).Format(time.RFC3339), "1mo ago"},
		{now.Add(-400 * 24 * time.Hour).Format(time.RFC3339), "1y ago"},
	}
	for _, c := range cases {
		got := shortAge(c.ts, now)
		if got != c.want {
			t.Errorf("shortAge(%q) = %q, want %q", c.ts, got, c.want)
		}
	}
}

// TestWindowSizeSetsWidthHeight verifies WindowSizeMsg is handled.
func TestWindowSizeSetsWidthHeight(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	m2 := update(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if m2.width != 120 || m2.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", m2.width, m2.height)
	}
}

// TestErrorRendersInView verifies that when err is set, the view contains
// "Error:".
func TestErrorRendersInView(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.err = errors.New("api error")
	m.loading = false
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
	m.loading = true
	m.width = 80
	m.height = 24

	view := m.renderContent()
	if !containsStr(view, "Loading") {
		t.Errorf("loading view = %q, want to contain 'Loading'", view)
	}
}
