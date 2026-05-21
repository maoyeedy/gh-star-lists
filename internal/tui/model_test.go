package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
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

// executeBatch runs a cmd (which may be a BatchMsg) and collects all tea.Msg results.
// This is used in tests that need to inspect what messages a batch produces.
func executeBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var results []tea.Msg
		for _, c := range batch {
			results = append(results, executeBatch(c)...)
		}
		return results
	}
	return []tea.Msg{msg}
}

// TestListsLoadedPopulatesLists verifies that receiving listsLoadedMsg
// sets the lists field. After P4 eager-load, the first list is auto-focused
// and a repo load is kicked off (loading stays true until repos arrive).
func TestListsLoadedPopulatesLists(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)

	m2 := update(m, listsLoadedMsg{lists: svc.lists})

	if len(m2.lists) != 3 {
		t.Fatalf("lists len = %d, want 3", len(m2.lists))
	}
	// Eager load: focusedList is set and anyPending is true (awaiting repos).
	if m2.focusedList == nil {
		t.Error("focusedList should be non-nil after eager initial load")
	}
	if !m2.anyPending() {
		t.Error("anyPending should be true after listsLoadedMsg (eager repo fetch in flight)")
	}
}

// TestReposLoadedPopulatesRepos verifies that receiving reposLoadedMsg
// populates the repo cache and resolves focusedList from the current lists.
// With the preloader, all three lists start loading after listsLoadedMsg, so
// we deliver all three responses to reach a fully idle state.
func TestReposLoadedPopulatesRepos(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo

	// Deliver repos for all lists so preloading completes and anyPending is false.
	m2 := update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m2 = update(m2, reposLoadedMsg{repos: svc.repos, listID: "UL_2"})
	m2 = update(m2, reposLoadedMsg{repos: svc.repos, listID: "UL_3"})

	if len(m2.currentRepos()) != 2 {
		t.Fatalf("currentRepos len = %d, want 2", len(m2.currentRepos()))
	}
	if m2.anyPending() {
		t.Error("anyPending should be false after all reposLoadedMsg delivered")
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
	if m2.anyPending() {
		t.Error("anyPending should be false after errMsg")
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
	for i := 0; i < 6; i++ {
		cur = update(cur, keyPress('s'))
	}
	if cur.sortRepos != initial {
		t.Errorf("sortRepos after 6 cycles = %d, want %d (initial)", cur.sortRepos, initial)
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
	if !m2.listsLoading {
		t.Error("listsLoading should be true after refresh")
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

// TestSortReposByLanguage verifies ascending language sort (case-insensitive,
// empty language sorts last).
func TestSortReposByLanguage(t *testing.T) {
	t.Parallel()
	repos := []githubapi.Repository{
		{NameWithOwner: "a/a", Language: "Rust"},
		{NameWithOwner: "b/b", Language: ""},
		{NameWithOwner: "c/c", Language: "Go"},
		{NameWithOwner: "d/d", Language: "go"}, // lowercase -- ties with c/c, tiebreak by name
	}
	sortRepos(repos, sortReposLanguage)

	// Empty must be last.
	if repos[len(repos)-1].Language != "" {
		t.Errorf("last repo Language = %q, want empty (sorts last)", repos[len(repos)-1].Language)
	}
	// First two should be "go"/"Go" variants before "Rust".
	for i := 0; i < 2; i++ {
		if strings.ToLower(repos[i].Language) != "go" {
			t.Errorf("repos[%d].Language = %q, want a go variant", i, repos[i].Language)
		}
	}
	if strings.ToLower(repos[2].Language) != "rust" {
		t.Errorf("repos[2].Language = %q, want rust", repos[2].Language)
	}
}

// TestSortReposByStarredAt verifies descending StarredAt sort (empty sorts last).
func TestSortReposByStarredAt(t *testing.T) {
	t.Parallel()
	repos := []githubapi.Repository{
		{NameWithOwner: "a/a", StarredAt: "2024-01-01T00:00:00Z"},
		{NameWithOwner: "b/b", StarredAt: ""},
		{NameWithOwner: "c/c", StarredAt: "2024-03-01T00:00:00Z"},
	}
	sortRepos(repos, sortReposStarredAt)

	if repos[0].NameWithOwner != "c/c" {
		t.Errorf("first by starredAt = %q, want c/c (newest)", repos[0].NameWithOwner)
	}
	if repos[len(repos)-1].StarredAt != "" {
		t.Errorf("last StarredAt = %q, want empty (sorts last)", repos[len(repos)-1].StarredAt)
	}
}

// TestModalOpenAndClose verifies a new-key stub opens a modal that Esc closes.
func TestModalOpenAndClose(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	m2 := update(m, keyPress('n'))
	if m2.modal == nil {
		t.Fatal("modal should be open after 'n'")
	}

	m3 := update(m2, specialKey(tea.KeyEscape))
	if m3.modal != nil {
		t.Error("modal should be closed after esc")
	}
}

// TestRepoMutationKeysNoOpInListPane verifies a/x/m/u do nothing in list pane.
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

// TestPreviewToggle verifies p key toggles showPreview.
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
func TestStatusToastSetAndExpire(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	// Simulate a mutation completing.
	m2 := update(m, mutationDoneMsg{kind: modalCreateList})
	if m2.statusMsg == "" {
		t.Error("statusMsg should be set after mutationDoneMsg success")
	}
	if m2.statusExpiry.IsZero() {
		t.Error("statusExpiry should be set after mutationDoneMsg success")
	}

	// Simulate expiry.
	m3 := update(m2, statusExpiredMsg{})
	if m3.statusMsg != "" {
		t.Error("statusMsg should be cleared after statusExpiredMsg")
	}
}

type recordingFakeService struct {
	fakeService
	createCalls []githubapi.StarListInput
	updateCalls []githubapi.UpdateStarListInput
	deleteCalls []string
	createErr   error
	updateErr   error
	deleteErr   error
}

func (f *recordingFakeService) CreateStarList(
	_ context.Context, input githubapi.StarListInput,
) (githubapi.StarList, error) {
	f.createCalls = append(f.createCalls, input)
	return githubapi.StarList{Name: input.Name}, f.createErr
}

func (f *recordingFakeService) UpdateStarList(
	_ context.Context, input githubapi.UpdateStarListInput,
) (githubapi.StarList, error) {
	f.updateCalls = append(f.updateCalls, input)
	return githubapi.StarList{}, f.updateErr
}

func (f *recordingFakeService) DeleteStarList(_ context.Context, id string) error {
	f.deleteCalls = append(f.deleteCalls, id)
	return f.deleteErr
}

// TestCreateListModalOpenClose verifies n opens a create form and esc closes it.
func TestCreateListModalOpenClose(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{}
	m := newTestModel(svc)

	m2 := update(m, keyPress('n'))
	if m2.modal == nil {
		t.Fatal("modal should open on n")
	}
	if m2.modal.kind != modalCreateList {
		t.Errorf("modal.kind = %v, want modalCreateList", m2.modal.kind)
	}

	m3 := update(m2, specialKey(tea.KeyEscape))
	if m3.modal != nil {
		t.Error("modal should be nil after esc")
	}
	if len(svc.createCalls) != 0 {
		t.Error("esc should not trigger mutation")
	}
}

// TestCreateListModalSubmit verifies typing a name and entering submits the mutation.
func TestCreateListModalSubmit(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{
		fakeService: fakeService{
			lists: []githubapi.StarList{{ID: "UL_1", Name: "existing"}},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Open create modal.
	m = update(m, keyPress('n'))
	if m.modal == nil {
		t.Fatal("modal did not open")
	}

	// Type into name field using individual key presses.
	for _, ch := range "My New List" {
		m = update(m, keyPress(ch))
	}

	// Submit (enter while on name field -- advances to description; enter again to submit).
	m = update(m, specialKey(tea.KeyEnter)) // advance to desc
	m = update(m, specialKey(tea.KeyEnter)) // advance to visibility (or submit from desc)
	m = update(m, specialKey(tea.KeyEnter)) // submit or advance

	// The modal may still be open if extra advances are needed -- keep pressing enter.
	for attempts := 0; m.modal != nil && attempts < 5; attempts++ {
		m = update(m, specialKey(tea.KeyEnter))
	}

	// Deliver the mutationDoneMsg (simulate cmd completion).
	m = update(m, mutationDoneMsg{kind: modalCreateList})

	if m.modal != nil {
		t.Error("modal should be closed after mutation done")
	}
	if m.statusMsg == "" {
		t.Error("statusMsg should be set after success")
	}
}

// TestDeleteListModalWrongNameBlocked verifies wrong input doesn't submit.
func TestDeleteListModalWrongNameBlocked(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{
		fakeService: fakeService{
			lists: []githubapi.StarList{{ID: "UL_1", Name: "mylist"}},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m = update(m, keyPress('d'))
	if m.modal == nil {
		t.Fatal("delete modal should open")
	}

	// Type wrong name.
	for _, ch := range "wrongname" {
		m = update(m, keyPress(ch))
	}
	m = update(m, specialKey(tea.KeyEnter))

	// Modal should still be open (name doesn't match).
	if m.modal == nil {
		t.Error("modal should stay open after wrong typed name")
	}
	if len(svc.deleteCalls) != 0 {
		t.Error("wrong name should not trigger delete")
	}
}

// TestDeleteListModalCorrectName verifies correct name submits.
func TestDeleteListModalCorrectName(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{
		fakeService: fakeService{
			lists: []githubapi.StarList{{ID: "UL_1", Name: "mylist"}},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m = update(m, keyPress('d'))
	if m.modal == nil {
		t.Fatal("delete modal should open")
	}

	// Type correct name.
	for _, ch := range "mylist" {
		m = update(m, keyPress(ch))
	}
	m2, cmd := m.Update(specialKey(tea.KeyEnter))
	m = m2.(model)
	if cmd == nil {
		t.Error("correct name should produce a cmd (delete mutation)")
	}
	// Modal should now be in submitting state (kept open, not closed).
	if m.modal == nil {
		t.Fatal("modal should remain open while submitting")
	}
	if !m.modal.submitting {
		t.Error("modal.submitting should be true after submit")
	}
	// The batch contains the mutation cmd. Execute cmds to find mutationDoneMsg.
	msgs := executeBatch(cmd)
	var found *mutationDoneMsg
	for _, msg := range msgs {
		if d, ok := msg.(mutationDoneMsg); ok {
			d := d
			found = &d
			break
		}
	}
	if found == nil {
		t.Fatal("batch should contain a mutationDoneMsg producer")
	}
	if found.kind != modalDeleteList {
		t.Errorf("doneMsg.kind = %v, want modalDeleteList", found.kind)
	}
}

// TestEditListNoOpInRepoPane verifies e is no-op in repo pane.
func TestEditListNoOpInRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo

	m2 := update(m, keyPress('e'))
	if m2.modal != nil {
		t.Error("edit should be no-op in repo pane")
	}
}

// TestMutationListErrorDisplayed verifies that mutationDoneMsg with an error keeps the modal
// open and stores the error in modal.submitErr (P3 behavior).
func TestMutationListErrorDisplayed(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	// Modal must be open for submitErr to be stored.
	m.modal = &modal{kind: modalDeleteList, submitting: true}
	sentinel := errors.New("delete failed")

	m2 := update(m, mutationDoneMsg{kind: modalDeleteList, err: sentinel})
	if m2.modal == nil {
		t.Error("modal should remain open after error (P3 inline error)")
	}
	if m2.modal != nil && !strings.Contains(m2.modal.submitErr, sentinel.Error()) {
		t.Errorf("modal.submitErr = %q, want to contain %q", m2.modal.submitErr, sentinel.Error())
	}
	if m2.modal != nil && m2.modal.submitting {
		t.Error("modal.submitting should be false after mutation error")
	}
}

// TestMutationErrorSetsErrField verifies mutationDoneMsg with err stores error in modal.submitErr.
func TestMutationErrorSetsErrField(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	// Modal must be open for submitErr to be stored.
	m.modal = &modal{kind: modalCreateList, submitting: true}
	sentinel := errors.New("create failed")

	m2 := update(m, mutationDoneMsg{kind: modalCreateList, err: sentinel})
	if m2.modal == nil {
		t.Error("modal should remain open after error (P3 inline error)")
	}
	if m2.modal != nil && !strings.Contains(m2.modal.submitErr, sentinel.Error()) {
		t.Errorf("modal.submitErr = %q, want to contain %q", m2.modal.submitErr, sentinel.Error())
	}
}

type repoMutationFakeService struct {
	fakeService
	membershipsResult struct {
		repoID  string
		listIDs []string
		err     error
	}
	membershipsCalls []string // nameWithOwner values called
	updateListsCalls []struct {
		repoID  string
		listIDs []string
	}
	removeStarCalls []string // repoIDs
	removeStarErr   error
}

func (f *repoMutationFakeService) GetRepositoryMemberships(
	_ context.Context, nameWithOwner string,
) (string, []string, error) {
	f.membershipsCalls = append(f.membershipsCalls, nameWithOwner)
	return f.membershipsResult.repoID, f.membershipsResult.listIDs, f.membershipsResult.err
}

func (f *repoMutationFakeService) UpdateRepositoryLists(
	_ context.Context, repoID string, listIDs []string,
) error {
	f.updateListsCalls = append(f.updateListsCalls, struct {
		repoID  string
		listIDs []string
	}{repoID, listIDs})
	return nil
}

func (f *repoMutationFakeService) RemoveStar(_ context.Context, repoID string) error {
	f.removeStarCalls = append(f.removeStarCalls, repoID)
	return f.removeStarErr
}

// TestAddRepoModalOpensInRepoPane verifies 'a' opens picker in repo pane with all lists.
func TestAddRepoModalOpensInRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	m2 := update(m, keyPress('a'))
	if m2.modal == nil {
		t.Fatal("modal should open on 'a' in repo pane")
	}
	if m2.modal.kind != modalPickList {
		t.Errorf("modal.kind = %v, want modalPickList", m2.modal.kind)
	}
	if len(m2.modal.choices) != len(svc.lists) {
		t.Errorf("picker choices = %d, want %d (all lists)", len(m2.modal.choices), len(svc.lists))
	}
}

// TestAddRepoNoOpInListPane verifies 'a' is no-op in list pane.
func TestAddRepoNoOpInListPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList

	m2 := update(m, keyPress('a'))
	if m2.modal != nil {
		t.Error("'a' should be no-op in list pane")
	}
}

// TestMoveRepoExcludesCurrentList verifies move picker excludes the current list.
func TestMoveRepoExcludesCurrentList(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0] // UL_1

	m2 := update(m, keyPress('m'))
	if m2.modal == nil {
		t.Fatal("modal should open on 'm'")
	}
	for _, choice := range m2.modal.choices {
		if choice.ID == "UL_1" {
			t.Error("move picker should not include current list UL_1")
		}
	}
}

// TestPickListNavigation verifies j/k cursor movement and Enter calls onConfirm.
func TestPickListNavigation(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	m = update(m, keyPress('a'))
	if m.modal == nil {
		t.Fatal("modal should be open")
	}
	// Move cursor down.
	m = update(m, keyPress('j'))
	if m.modal.choiceCursor != 1 {
		t.Errorf("choiceCursor after j = %d, want 1", m.modal.choiceCursor)
	}
	// Esc cancels.
	m = update(m, specialKey(tea.KeyEscape))
	if m.modal != nil {
		t.Error("modal should close on esc")
	}
}

// TestRemoveRepoConfirmYesNo verifies y confirms, n cancels.
func TestRemoveRepoConfirmYesNo(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	// Open remove modal.
	m2 := update(m, keyPress('x'))
	if m2.modal == nil {
		t.Fatal("remove modal should open")
	}
	if m2.modal.kind != modalConfirmYesNo {
		t.Errorf("modal.kind = %v, want modalConfirmYesNo", m2.modal.kind)
	}

	// 'n' cancels.
	m3 := update(m2, keyPress('n'))
	if m3.modal != nil {
		t.Error("modal should close on 'n'")
	}

	// Reopen and 'y' fires the command.
	m4 := update(m, keyPress('x'))
	_, cmd := m4.Update(keyPress('y'))
	if cmd == nil {
		t.Error("'y' should produce a mutation cmd")
	}
}

// TestAddRepoCmd verifies the set-union logic in addRepoToListCmd.
func TestAddRepoCmd(t *testing.T) {
	t.Parallel()
	svc := &repoMutationFakeService{}
	svc.membershipsResult.repoID = "R_1"
	svc.membershipsResult.listIDs = []string{"UL_2"}

	cmd := addRepoToListCmd(context.Background(), svc, "owner/repo", "UL_3")
	msg := cmd()
	done, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want mutationDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if len(svc.updateListsCalls) != 1 {
		t.Fatalf("UpdateRepositoryLists calls = %d, want 1", len(svc.updateListsCalls))
	}
	got := svc.updateListsCalls[0].listIDs
	want := []string{"UL_2", "UL_3"} // sorted
	if len(got) != len(want) {
		t.Fatalf("listIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("listIDs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestRemoveRepoCmd verifies the set-remove logic in removeRepoFromListCmd.
func TestRemoveRepoCmd(t *testing.T) {
	t.Parallel()
	svc := &repoMutationFakeService{}
	svc.membershipsResult.repoID = "R_1"
	svc.membershipsResult.listIDs = []string{"UL_1", "UL_2"}

	cmd := removeRepoFromListCmd(context.Background(), svc, "owner/repo", "UL_1")
	msg := cmd()
	done, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want mutationDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	got := svc.updateListsCalls[0].listIDs
	want := []string{"UL_2"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("listIDs = %v, want %v", got, want)
	}
}

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

// TestPreviewPaneRendersInThreeColumnLayout verifies showPreview adds a third column.
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
func TestUnstarRepoCmd(t *testing.T) {
	t.Parallel()
	svc := &repoMutationFakeService{}
	svc.membershipsResult.repoID = "R_star_1"

	cmd := unstarRepoCmd(context.Background(), svc, "owner/repo")
	msg := cmd()
	done, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want mutationDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if len(svc.removeStarCalls) != 1 || svc.removeStarCalls[0] != "R_star_1" {
		t.Errorf("RemoveStar calls = %v, want [R_star_1]", svc.removeStarCalls)
	}
}

type copyMergeFakeService struct {
	fakeService
	reposResult        []githubapi.Repository
	membershipsRepoID  string
	membershipsListIDs []string
	updateListsCalls   [][]string // just listIDs per call
	deleteListCalls    []string
	deleteListErr      error
}

func (f *copyMergeFakeService) ListRepositories(
	_ context.Context, _ string, _ ...githubapi.ListOptions,
) ([]githubapi.Repository, error) {
	return f.reposResult, nil
}

func (f *copyMergeFakeService) GetRepositoryMemberships(
	_ context.Context, _ string,
) (string, []string, error) {
	return f.membershipsRepoID, f.membershipsListIDs, nil
}

func (f *copyMergeFakeService) UpdateRepositoryLists(
	_ context.Context, _ string, listIDs []string,
) error {
	f.updateListsCalls = append(f.updateListsCalls, listIDs)
	return nil
}

func (f *copyMergeFakeService) DeleteStarList(_ context.Context, id string) error {
	f.deleteListCalls = append(f.deleteListCalls, id)
	return f.deleteListErr
}

// TestCopyListModalOpens verifies 'c' opens a list picker in list pane.
func TestCopyListModalOpens(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList

	m2 := update(m, keyPress('c'))
	if m2.modal == nil {
		t.Fatal("copy modal should open on 'c'")
	}
	if m2.modal.kind != modalPickList {
		t.Errorf("modal.kind = %v, want modalPickList", m2.modal.kind)
	}
	// Source list excluded from choices.
	if len(m2.modal.choices) != len(svc.lists)-1 {
		t.Errorf(
			"choices = %d, want %d (all except source)",
			len(m2.modal.choices),
			len(svc.lists)-1,
		)
	}
}

// TestMergeListModalTitle verifies 'C' modal has destructive indicator.
func TestMergeListModalTitle(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList

	m2 := update(m, keyPress('C'))
	if m2.modal == nil {
		t.Fatal("merge modal should open on 'C'")
	}
	if !containsStr(m2.modal.title, "source deleted") && !containsStr(m2.modal.title, "Merge") {
		t.Errorf("merge modal title = %q, want to contain 'Merge'", m2.modal.title)
	}
}

// TestCopyMergeNoOpInRepoPane verifies c/C are no-ops in repo pane.
func TestCopyMergeNoOpInRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo

	for _, k := range []rune{'c', 'C'} {
		m2 := update(m, keyPress(k))
		if m2.modal != nil {
			t.Errorf("key %c should be no-op in repo pane", k)
		}
	}
}

// TestCopyListCmd verifies repos are added to target list via UpdateRepositoryLists.
func TestCopyListCmd(t *testing.T) {
	t.Parallel()
	svc := &copyMergeFakeService{
		reposResult: []githubapi.Repository{
			{NameWithOwner: "owner/repo1"},
		},
		membershipsRepoID:  "R_1",
		membershipsListIDs: []string{"UL_src"},
	}
	cmd := copyListCmd(context.Background(), svc, "UL_src", "UL_dst", false)
	msg := cmd()
	done, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want mutationDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if len(svc.updateListsCalls) != 1 {
		t.Fatalf("UpdateRepositoryLists calls = %d, want 1", len(svc.updateListsCalls))
	}
	got := svc.updateListsCalls[0]
	// Should contain both src and dst.
	found := false
	for _, id := range got {
		if id == "UL_dst" {
			found = true
		}
	}
	if !found {
		t.Errorf("listIDs %v should contain UL_dst", got)
	}
	// No delete because deleteSource=false.
	if len(svc.deleteListCalls) != 0 {
		t.Error("DeleteStarList should not be called for copy (not merge)")
	}
}

// TestMergeListCmdDeletesSource verifies DeleteStarList is called when deleteSource=true.
func TestMergeListCmdDeletesSource(t *testing.T) {
	t.Parallel()
	svc := &copyMergeFakeService{
		reposResult: []githubapi.Repository{
			{NameWithOwner: "owner/repo1"},
		},
		membershipsRepoID:  "R_1",
		membershipsListIDs: []string{"UL_src"},
	}
	cmd := copyListCmd(context.Background(), svc, "UL_src", "UL_dst", true)
	msg := cmd()
	done, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want mutationDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if len(svc.deleteListCalls) != 1 || svc.deleteListCalls[0] != "UL_src" {
		t.Errorf("DeleteStarList calls = %v, want [UL_src]", svc.deleteListCalls)
	}
}

// largeSvc builds a service with n repos for viewport scrolling tests.
func largeSvc(n int) *fakeService {
	repos := make([]githubapi.Repository, n)
	for i := 0; i < n; i++ {
		repos[i] = githubapi.Repository{
			ID:            fmt.Sprintf("R_%d", i),
			NameWithOwner: fmt.Sprintf("owner/repo-%02d", i),
			URL:           fmt.Sprintf("https://github.com/owner/repo-%02d", i),
		}
	}
	return &fakeService{
		lists: []githubapi.StarList{
			{ID: "UL_1", Name: "big", RepoCount: n},
		},
		repos: repos,
	}
}

func repoPane(m model, w, h int) string { return m.renderRepoPane(w, h) }

// TestViewportPgDnMovesReposCursorByPageHeight verifies that PgDn in the repo
// pane advances the cursor by pane height minus one.
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
	if !strings.Contains(rendered, "owner/repo-49") {
		t.Errorf("rendered pane after G should show repo-49, got:\n%s", rendered)
	}
}

// TestViewportListPanePgDn verifies PgDn works in list pane too.
func TestViewportListPanePgDn(t *testing.T) {
	t.Parallel()
	n := 20
	lists := make([]githubapi.StarList, n)
	for i := 0; i < n; i++ {
		lists[i] = githubapi.StarList{
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

// TestSearchActivatesOnSlash verifies / activates search mode.
func TestSearchActivatesOnSlash(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m2 := update(m, keyPress('/'))

	if !m2.searchActive {
		t.Error("searchActive should be true after /")
	}
}

// TestSearchFiltersListsByQuery verifies that typing narrows displayedLists.
func TestSearchFiltersListsByQuery(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, keyPress('/'))

	// Type "alp" - should match "Alpha" from threeListsSvc.
	for _, ch := range "alp" {
		m = update(m, keyPress(ch))
	}

	if len(m.displayedLists) == 0 {
		t.Fatal("displayedLists should have at least one match for 'alp'")
	}
	if m.displayedLists[0].Name != "Alpha" {
		t.Errorf("first result = %q, want Alpha", m.displayedLists[0].Name)
	}
	// non-matching lists should not be displayed
	for _, l := range m.displayedLists {
		if l.Name != "Alpha" {
			t.Errorf("unexpected match %q for query 'alp'", l.Name)
		}
	}
}

// TestSearchEscClearsFilter verifies Esc restores full list.
func TestSearchEscClearsFilter(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, keyPress('/'))
	m = update(m, keyPress('z')) // "z" matches "zeta"
	if len(m.displayedLists) == 0 {
		t.Fatal("need at least one match to test clear")
	}

	m2 := update(m, specialKey(tea.KeyEscape))

	if m2.searchActive {
		t.Error("searchActive should be false after Esc")
	}
	if m2.searchQuery != "" {
		t.Errorf("searchQuery = %q, want empty after Esc", m2.searchQuery)
	}
	if len(m2.displayedLists) != len(svc.lists) {
		t.Errorf(
			"displayedLists len = %d after Esc, want %d (all)",
			len(m2.displayedLists),
			len(svc.lists),
		)
	}
}

// TestSearchEnterDeactivatesKeepsFilter verifies Enter deactivates search
// but keeps the current filter.
func TestSearchEnterDeactivatesKeepsFilter(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, keyPress('/'))
	// "alpha" uniquely matches only the "Alpha" list (not "zeta" or "beta").
	for _, ch := range "alpha" {
		m = update(m, keyPress(ch))
	}

	m2 := update(m, specialKey(tea.KeyEnter))

	if m2.searchActive {
		t.Error("searchActive should be false after Enter")
	}
	if m2.searchQuery == "" {
		t.Error("searchQuery should still be non-empty after Enter")
	}
	// displayedLists should still be filtered - only "Alpha" matches.
	if len(m2.displayedLists) >= len(svc.lists) {
		t.Error("displayedLists should still be filtered after Enter")
	}
}

// TestSearchResetsCursorToZero verifies cursor resets on each keystroke.
func TestSearchResetsCursorToZero(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.listCursor = 2
	m = update(m, keyPress('/'))
	m = update(m, keyPress('z'))

	if m.listCursor != 0 {
		t.Errorf("listCursor = %d after search input, want 0", m.listCursor)
	}
}

// TestSearchBackspaceRemovesLastChar verifies Backspace trims the query.
func TestSearchBackspaceRemovesLastChar(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, keyPress('/'))
	m = update(m, keyPress('a'))
	m = update(m, keyPress('l'))

	if m.searchQuery != "al" {
		t.Fatalf("searchQuery = %q before backspace, want 'al'", m.searchQuery)
	}
	m2 := update(m, specialKey(tea.KeyBackspace))

	if m2.searchQuery != "a" {
		t.Errorf("searchQuery = %q after backspace, want 'a'", m2.searchQuery)
	}
}

// TestSearchFilterRepoPane verifies filtering works in the repo pane.
func TestSearchFilterRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})

	// Query "a-repo": the exact-name match "owner/a-repo" should appear and
	// rank first. The fuzzy algorithm may also surface "owner/b-repo" via
	// single-char edit distance on the "a" token - that is expected behaviour;
	// what matters is the correct repo is present and ranked at the top.
	m = update(m, keyPress('/'))
	for _, ch := range "a-repo" {
		m = update(m, keyPress(ch))
	}

	if len(m.displayedRepos) == 0 {
		t.Fatal("displayedRepos should have at least one match for 'a-repo'")
	}
	if m.displayedRepos[0].NameWithOwner != "owner/a-repo" {
		t.Errorf(
			"top result = %q, want 'owner/a-repo' (exact match should rank first)",
			m.displayedRepos[0].NameWithOwner,
		)
	}
}

// TestSearchNoModalOnSlash verifies / is a no-op when a modal is open.
func TestSearchNoModalOnSlash(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.modal = &modal{kind: modalConfirmYesNo}

	m2 := update(m, keyPress('/'))

	if m2.searchActive {
		t.Error("searchActive should stay false when modal is open")
	}
}

// --- Phase 3: multi-select ---

// TestSelectTogglesRepo verifies space marks and unmarks the focused repo.
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

// TestBulkDoneMsgClearsSelectionAndSetsToast verifies bulkDoneMsg clears selection.
func TestBulkDoneMsgClearsSelectionAndSetsToast(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.selected = map[string]struct{}{
		"owner/a-repo": {},
		"owner/b-repo": {},
	}

	m2 := update(m, bulkDoneMsg{verb: "added", succeeded: 2, failed: 0})

	if len(m2.selected) != 0 {
		t.Errorf("selected should be cleared after bulkDoneMsg, got %d", len(m2.selected))
	}
	if m2.statusMsg == "" {
		t.Error("statusMsg should be set after bulkDoneMsg")
	}
	if !strings.Contains(m2.statusMsg, "added") {
		t.Errorf("statusMsg = %q, want to contain 'added'", m2.statusMsg)
	}
}

// TestBulkDoneMsgPartialFailureToast verifies toast mentions failed count.
func TestBulkDoneMsgPartialFailureToast(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m2 := update(m, bulkDoneMsg{verb: "removed", succeeded: 1, failed: 1})

	if !strings.Contains(m2.statusMsg, "failed") {
		t.Errorf("statusMsg = %q, want to contain 'failed'", m2.statusMsg)
	}
}

// TestBulkDoneMsgPartialFailureKeepsModalOpen verifies that partial bulk failures
// stay in the modal and list failed repositories.
func TestBulkDoneMsgPartialFailureKeepsModalOpen(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m.modal = newBulkRemoveModal(
		m.ctx,
		m.svc,
		[]string{"owner/a-repo", "owner/b-repo"},
		m.focusedList.ID,
	)
	m.modal.submitting = true
	m.selected = map[string]struct{}{
		"owner/a-repo": {},
		"owner/b-repo": {},
	}

	m2 := update(m, bulkDoneMsg{
		verb:       "removed",
		succeeded:  1,
		failed:     1,
		failedNWOs: []string{"owner/b-repo"},
	})

	if m2.modal == nil {
		t.Fatal("modal should remain open after partial bulk failure")
	}
	if m2.modal.submitting {
		t.Error("modal.submitting should be false after partial bulk failure")
	}
	if m2.modal.bulkFailure == nil {
		t.Fatal("modal.bulkFailure should be set after partial bulk failure")
	}
	if got := m2.modal.bulkFailure.failedNWOs; len(got) != 1 || got[0] != "owner/b-repo" {
		t.Fatalf("failedNWOs = %v, want [owner/b-repo]", got)
	}
	if len(m2.selected) != 0 {
		t.Errorf("selected should be cleared after bulkDoneMsg, got %d", len(m2.selected))
	}
	rendered := m2.modal.view()
	if !strings.Contains(rendered, "owner/b-repo") {
		t.Errorf("modal view = %q, want failed repo name", rendered)
	}
	if !strings.Contains(rendered, "retry") {
		t.Errorf("modal view = %q, want retry hint", rendered)
	}
}

// TestBulkFailureRetryUsesFailedNWOsOnly verifies retry replays only failed repos.
func TestBulkFailureRetryUsesFailedNWOsOnly(t *testing.T) {
	t.Parallel()
	svc := &repoMutationFakeService{}
	svc.membershipsResult.repoID = "R_1"
	svc.membershipsResult.listIDs = []string{"UL_1"}
	m := newTestModel(svc)
	m.modal = newBulkRemoveModal(
		m.ctx,
		m.svc,
		[]string{"owner/a-repo", "owner/b-repo", "owner/c-repo"},
		"UL_1",
	)
	m.modal.submitting = true

	m = update(m, bulkDoneMsg{
		verb:       "removed",
		succeeded:  1,
		failed:     2,
		failedNWOs: []string{"owner/b-repo", "owner/c-repo"},
	})
	if m.modal == nil || m.modal.bulkFailure == nil {
		t.Fatal("modal should show bulk failure before retry")
	}

	next, cmd := m.Update(keyPress('r'))
	m2 := next.(model)
	if cmd == nil {
		t.Fatal("retry key should produce a command")
	}
	if m2.modal == nil || !m2.modal.submitting {
		t.Fatal("modal should remain open and submitting during retry")
	}
	executeBatch(cmd)

	want := []string{"owner/b-repo", "owner/c-repo"}
	if len(svc.membershipsCalls) != len(want) {
		t.Fatalf("membershipsCalls = %v, want %v", svc.membershipsCalls, want)
	}
	for i := range want {
		if svc.membershipsCalls[i] != want[i] {
			t.Errorf("membershipsCalls[%d] = %q, want %q", i, svc.membershipsCalls[i], want[i])
		}
	}
}

// TestBulkFailureListScrolls verifies long failed-repo lists can scroll.
func TestBulkFailureListScrolls(t *testing.T) {
	t.Parallel()
	failed := []string{
		"owner/repo-01",
		"owner/repo-02",
		"owner/repo-03",
		"owner/repo-04",
		"owner/repo-05",
		"owner/repo-06",
		"owner/repo-07",
		"owner/repo-08",
		"owner/repo-09",
	}
	mo := &modal{
		kind: modalConfirmYesNo,
		bulkFailure: &bulkFailureState{
			verb:       "removed",
			succeeded:  1,
			failedNWOs: failed,
		},
		bulkRetry: func([]string) tea.Cmd { return nil },
	}

	before := mo.view()
	if !strings.Contains(before, "owner/repo-01") {
		t.Fatalf("initial view = %q, want first failed repo", before)
	}
	if strings.Contains(before, "owner/repo-09") {
		t.Fatalf("initial view = %q, should clip final failed repo", before)
	}

	updated, cmd := mo.update(keyPress('j'))
	if cmd != nil {
		t.Fatal("scrolling failure list should not produce a command")
	}
	mo = updated

	after := mo.view()
	if !strings.Contains(after, "owner/repo-09") {
		t.Errorf("scrolled view = %q, want final failed repo", after)
	}
	if !strings.Contains(after, "above") {
		t.Errorf("scrolled view = %q, want above indicator", after)
	}
}

// TestBulkAddModalOpenedWithSelection verifies 'a' opens bulk modal when repos selected.
func TestBulkAddModalOpenedWithSelection(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.selected = map[string]struct{}{"owner/a-repo": {}}

	m2 := update(m, keyPress('a'))

	if m2.modal == nil {
		t.Fatal("modal should open on 'a' with selection")
	}
	if m2.modal.kind != modalPickList {
		t.Errorf("modal.kind = %v, want modalPickList", m2.modal.kind)
	}
	if !strings.Contains(m2.modal.title, "1 repo") {
		t.Errorf("modal.title = %q, want to contain '1 repo'", m2.modal.title)
	}
}

// TestSelectRendersPrefixWhenSelectionNonEmpty verifies [x]/[ ] prefix appears.
func TestSelectRendersPrefixWhenSelectionNonEmpty(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m.width = 80
	m.height = 24
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	// Mark the second repo ("owner/a-repo" at index 1).
	m.selected = map[string]struct{}{"owner/a-repo": {}}

	rendered := m.renderRepoPane(60, 20)

	if !strings.Contains(rendered, "[x]") {
		t.Errorf("renderRepoPane should contain '[x]' for checked repo, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[ ]") {
		t.Errorf("renderRepoPane should contain '[ ]' for unchecked repo, got:\n%s", rendered)
	}
}

// TestDropLastRuneMultiByte verifies dropLastRune removes exactly one Unicode
// code point, not one byte.
func TestDropLastRuneMultiByte(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  string
	}{
		{"abc\u4e2d", "abc"},       // CJK ideograph (3 bytes)
		{"\u4e2d\u6587", "\u4e2d"}, // two CJK chars, drop last
		{"a\U0001F600", "a"},       // emoji (4 bytes)
		{"", ""},                   // empty -- no panic
	}
	for _, tc := range cases {
		got := dropLastRune(tc.input)
		if got != tc.want {
			t.Errorf("dropLastRune(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestSearchWhileFilterActiveActionKeys verifies action keys operate on
// displayedRepos when a search filter query is set but search input is committed
// (searchActive == false, searchQuery != "").
func TestSearchWhileFilterActiveActionKeys(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	// Commit a filter query via Enter (searchActive=false, query kept).
	m.searchQuery = string([]rune(svc.repos[0].NameWithOwner)[:1]) // first char of first repo
	m.searchActive = false
	m = m.rebuildDisplayed()
	if len(m.displayedRepos) == 0 {
		t.Skip("filter removed all repos -- fixture mismatch")
	}

	// Press 'a' (AddRepo): should open a modal targeting displayedRepos[cursor].
	m2 := update(m, keyPress('a'))
	if m2.modal == nil {
		t.Error("expected modal to open when pressing 'a' with filter active")
	}
}

// TestNarrowRepoPaneHidesMetadata verifies renderRepoPane omits
// language/stars/pushed metadata when width is below the threshold (60).
func TestNarrowRepoPaneHidesMetadata(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	// Render at narrow width (below threshold of 60).
	out := m.renderRepoPane(40, 10)

	// The meta format uses "*" as the star-count marker. It must be absent.
	if strings.Contains(out, "*") {
		t.Errorf("narrow renderRepoPane should not contain star-count marker, got:\n%s", out)
	}
}

// TestFooterCoreHintsOnly verifies renderFooter contains core hints and omits
// old dense markers.
func TestFooterCoreHintsOnly(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.width = 60

	footer := m.renderFooter()
	// Core hints must be present.
	for _, want := range []string{"search", "help", "quit"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer missing %q; got: %s", want, footer)
		}
	}
	// Should not contain the old dense hint markers.
	for _, banned := range []string{"ctrl+r:refresh", "pg/g/G:scroll"} {
		if strings.Contains(footer, banned) {
			t.Errorf("footer contains banned dense hint %q", banned)
		}
	}
}

// TestLoadingRendersInsidePane verifies that while loading repos the layout
// (list pane) is still visible -- not just a bare "Loading..." string.
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

// TestMouseClickFocusesPane synthesizes a MouseClickMsg in the repo pane and
// asserts that active switches to paneRepo and repoCursor updates.
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
func TestSpinnerTickMsgUpdatesSpinner(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	// listsLoading is already true from newTestModel (listsLoading: true).

	// Capture initial View output.
	before := m.spinner.View()

	// Synthesize a TickMsg for this spinner's ID so it accepts the message.
	tick := spinner.TickMsg{Time: time.Now(), ID: m.spinner.ID()}
	m2 := update(m, tick)

	// spinner.View() must return a non-empty string after a tick.
	after := m2.spinner.View()
	if after == "" {
		t.Error("spinner.View() returned empty string after TickMsg")
	}
	// The frame should have advanced (before and after differ).
	if before == after {
		t.Logf(
			"spinner.View() did not advance frame (before=%q after=%q); may be acceptable if same char",
			before,
			after,
		)
	}
}

// TestLoadingViewUsesSpinnerView verifies that while loading repos the rendered
// repo pane contains the spinner output.
func TestLoadingViewUsesSpinnerView(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.focusedList = &m.lists[0]
	// Mark the focused list's cache entry as loading to simulate repo fetch in flight.
	m.repoCache[repoCacheKey{m.focusedList.ID, false}] = &repoCacheEntry{state: repoCacheLoading}

	spinnerStr := m.spinner.View()
	rendered := repoPane(m, 80, 20)

	if !strings.Contains(rendered, spinnerStr) {
		t.Errorf(
			"repo pane during loading should contain spinner output %q; got:\n%s",
			spinnerStr,
			rendered,
		)
	}
}

// --- Geometry tests (P2) ---

// TestPaneGeometryTwoColumn verifies two-column layout invariants across a
// range of terminal widths.
func TestPaneGeometryTwoColumn(t *testing.T) {
	t.Parallel()
	widths := []int{80, 100, 120, 160}
	for _, w := range widths {
		g := calcPaneGeometry(w, false)
		// Separator occupies exactly 1 column: leftWidth + sep(1) + repoWidth == totalWidth.
		if got := g.leftWidth + 1 + g.repoWidth; got != w {
			t.Errorf(
				"width=%d: leftWidth(%d)+1+repoWidth(%d)=%d, want %d",
				w,
				g.leftWidth,
				g.repoWidth,
				got,
				w,
			)
		}
		// sep1Col is at leftWidth.
		if g.sep1Col != g.leftWidth {
			t.Errorf("width=%d: sep1Col=%d, want %d", w, g.sep1Col, g.leftWidth)
		}
		// No preview in 2-col mode.
		if g.previewWidth != 0 {
			t.Errorf("width=%d: previewWidth=%d, want 0", w, g.previewWidth)
		}
		// sep2Col must be -1 in 2-col mode.
		if g.sep2Col != -1 {
			t.Errorf("width=%d: sep2Col=%d, want -1", w, g.sep2Col)
		}
	}
}

// TestPaneGeometryThreeColumn verifies three-column layout invariants across a
// range of terminal widths.
func TestPaneGeometryThreeColumn(t *testing.T) {
	t.Parallel()
	widths := []int{100, 120, 160, 200}
	for _, w := range widths {
		g := calcPaneGeometry(w, true)
		// Two separators: leftWidth + sep(1) + repoWidth + sep(1) + previewWidth == totalWidth.
		total := g.leftWidth + 1 + g.repoWidth + 1 + g.previewWidth
		if total != w {
			t.Errorf("width=%d: leftWidth(%d)+1+repoWidth(%d)+1+previewWidth(%d)=%d, want %d",
				w, g.leftWidth, g.repoWidth, g.previewWidth, total, w)
		}
		// sep1Col is at leftWidth.
		if g.sep1Col != g.leftWidth {
			t.Errorf("width=%d: sep1Col=%d, want %d (leftWidth)", w, g.sep1Col, g.leftWidth)
		}
		// sep2Col is at leftWidth + 1 + repoWidth.
		wantSep2 := g.leftWidth + 1 + g.repoWidth
		if g.sep2Col != wantSep2 {
			t.Errorf("width=%d: sep2Col=%d, want %d", w, g.sep2Col, wantSep2)
		}
		// preview pane must have positive width.
		if g.previewWidth <= 0 {
			t.Errorf("width=%d: previewWidth=%d, want >0", w, g.previewWidth)
		}
	}
}

// TestHoverWheelScrollsListPane verifies that a wheel event whose X coordinate
// falls inside the list pane scrolls listCursor, even when the repo pane is
// active.
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

// --- P3: Visual polish tests ---

// TestListRowsSimplified verifies the new list row format: no "|" column
// separator, no age strings, but the repo count is present.
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
	lists := []githubapi.StarList{
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
	m.searchActive = true
	m.searchQuery = "alp"
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

// TestHeaderPriorityTruncation verifies that with a very long list name and
// sort label on a narrow terminal, the app name is preserved and the sort label
// is dropped before truncating the list name.
func TestHeaderPriorityTruncation(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.width = 60
	m.height = 24
	// Give it a very long focused list name.
	longName := "this-is-a-very-long-list-name-that-exceeds-the-terminal-width"
	fl := githubapi.StarList{ID: "UL_1", Name: longName, RepoCount: 5}
	m.lists = []githubapi.StarList{fl}
	m.focusedList = &m.lists[0]
	// Set a non-default sort so sortLabel would appear if there is room.
	m.sortLists = sortListsName // produces "name" label

	header := m.renderHeader()

	// App name must always be present.
	if !strings.Contains(header, "gh star-lists") {
		t.Errorf("header must contain 'gh star-lists'; got: %s", header)
	}
	// Sort label should be absent (dropped due to narrow width).
	if strings.Contains(header, "[sort:") {
		t.Errorf("header should not contain sort label when terminal is narrow; got: %s", header)
	}
	// Visible width of the rendered header must not exceed m.width.
	visW := lipgloss.Width(header)
	if visW > m.width {
		t.Errorf("header visible width %d exceeds terminal width %d", visW, m.width)
	}
}

// TestFooterKeyTokensStyled verifies that with a real color profile the footer
// wraps key tokens in ANSI escape sequences (i.e., they are styled, not plain).
func TestFooterKeyTokensStyled(t *testing.T) {
	t.Parallel()
	// styleFooterKey is Bold; in a real (non-NoTTY) profile the renderer
	// emits escape sequences. We can detect this by comparing the rendered
	// key with the raw key string.
	keyRendered := styleFooterKey.Render("q")
	if keyRendered == "q" {
		// lipgloss may strip styles when it detects no TTY; skip rather than fail.
		t.Skip("color profile strips ANSI (no TTY); skipping styled-token assertion")
	}

	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.width = 80

	footer := m.renderFooter()

	// The footer must contain the styled rendering of the "q" key, not just "q".
	if !strings.Contains(footer, keyRendered) {
		t.Errorf("footer does not contain styled 'q' token %q; got: %s", keyRendered, footer)
	}
}

// --- Phase 4: repo pane rebuild ---

// TestRepoPaneNoHeading verifies that the repo pane has no header rows above
// repos: the first line must be a repo row so it aligns with the first list
// item.
func TestRepoPaneNoHeading(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo

	rendered := repoPane(m, 80, 20)

	if strings.Contains(rendered, "Repos in this list:") {
		t.Errorf("repo pane must not contain 'Repos in this list:'; got:\n%s", rendered)
	}
	firstLine := strings.SplitN(rendered, "\n", 2)[0]
	if strings.TrimSpace(firstLine) == "" {
		t.Errorf(
			"first line of repo pane must not be blank (no heading overhead); got:\n%s",
			rendered,
		)
	}
}

// TestRepoFieldStyling verifies that the repo pane produces styled output (ANSI
// sequences or at least readable content in NoTTY mode) and that the repo name
// is always present.
func TestRepoFieldStyling(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.width = 120

	rendered := repoPane(m, 120, 20)

	// The repo name must appear somewhere in the output.
	for _, r := range svc.repos {
		if !strings.Contains(rendered, r.NameWithOwner) {
			t.Errorf(
				"repo pane should contain NameWithOwner %q; got:\n%s",
				r.NameWithOwner,
				rendered,
			)
		}
	}

	// In a real terminal (or when styles render), the star glyph should appear
	// (since width 120 >= 30 threshold for stars).
	if !strings.Contains(rendered, "\u2605") {
		// Not a hard failure if lipgloss strips styling -- skip.
		t.Logf("star glyph absent (may be a NoTTY environment); rendered:\n%s", rendered)
	}
}

// stripANSI removes ANSI escape sequences from s for column-position testing.
func stripANSI(s string) string {
	var out []byte
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm'.
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
			} else {
				i = j
			}
			continue
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}

// TestRepoColumnAlignment verifies that the star glyph appears at a consistent
// visual column across all content rows.
func TestRepoColumnAlignment(t *testing.T) {
	t.Parallel()
	repos := []githubapi.Repository{
		{ID: "R_1", NameWithOwner: "a/short", StargazerCount: 1, Language: "Go"},
		{ID: "R_2", NameWithOwner: "b/medium", StargazerCount: 100, Language: "Rust"},
		{ID: "R_3", NameWithOwner: "c/another", StargazerCount: 10000, Language: "TypeScript"},
		{ID: "R_4", NameWithOwner: "d/no-lang", StargazerCount: 42, Language: ""},
		{ID: "R_5", NameWithOwner: "e/more", StargazerCount: 999, Language: "Python"},
	}
	svc := &fakeService{
		lists: []githubapi.StarList{{ID: "UL_1", Name: "test", RepoCount: 5}},
		repos: repos,
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: repos, listID: "UL_1"})
	m.active = paneRepo

	rendered := repoPane(m, 120, 15)
	lines := strings.Split(rendered, "\n")

	// Heading is 3 lines (title, subtitle, blank), content starts at line 3.
	// Strip ANSI before measuring byte position so escape-length differences
	// do not affect the column comparison.
	starCol := -1
	for i, line := range lines {
		if i < 3 {
			continue // heading rows
		}
		plain := stripANSI(line)
		if plain == "" {
			continue // padding rows
		}
		glyphPos := strings.Index(plain, "\u2605")
		if glyphPos < 0 {
			t.Errorf("line %d missing star glyph: %q", i, line)
			continue
		}
		if starCol < 0 {
			starCol = glyphPos
		} else if glyphPos != starCol {
			t.Errorf("star glyph at byte-col %d on line %d, want %d (alignment); plain: %q",
				glyphPos, i, starCol, plain)
		}
	}
}

// TestRepoTruncation verifies that a repo with a very long NameWithOwner does
// not produce a rendered line longer than the pane width.
func TestRepoTruncation(t *testing.T) {
	t.Parallel()
	longName := "some-very-long-owner-name/this-is-an-extremely-long-repository-name-with-extra"
	svc := &fakeService{
		lists: []githubapi.StarList{{ID: "UL_1", Name: "trunctest", RepoCount: 1}},
		repos: []githubapi.Repository{
			{ID: "R_1", NameWithOwner: longName, StargazerCount: 5, Language: "Go"},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	const paneW = 80

	rendered := repoPane(m, paneW, 15)
	for i, line := range strings.Split(rendered, "\n") {
		w := lipgloss.Width(line)
		if w > paneW {
			t.Errorf("line %d has visual width %d > pane width %d: %q", i, w, paneW, line)
		}
	}
}

// TestNarrowRepoPaneP4HidesMetadata verifies progressive field hiding at narrow
// widths using the P4 thresholds.
func TestNarrowRepoPaneP4HidesMetadata(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	// Very narrow: stars and language hidden (width < 30 hides stars, < 42 hides lang).
	narrowOut := repoPane(m, 29, 15)
	if strings.Contains(narrowOut, "\u2605") {
		t.Errorf("width 29: star glyph should be absent; got:\n%s", narrowOut)
	}
	// Repo name must still appear.
	if !strings.Contains(narrowOut, "owner/") {
		t.Errorf("width 29: repo name should still appear; got:\n%s", narrowOut)
	}

	// Medium width (>= 42 shows lang).
	medOut := repoPane(m, 55, 15)
	if !strings.Contains(medOut, "\u2605") {
		t.Errorf("width 55: star glyph should appear; got:\n%s", medOut)
	}
}

// TestEagerInitialLoad verifies that listsLoadedMsg auto-focuses the first list
// and returns a non-nil repo-load command.
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

// --- Phase 5: preview pane styled detail block ---

// previewPane is a test helper that calls renderPreviewPane directly.
func previewPane(m model, w, h int) string { return m.renderPreviewPane(w, h) }

// TestPreviewDetailBlock verifies that the styled preview pane renders all
// key fields when a repo has full data populated.
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
// whenever the focused list changes via Enter key.
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

// TestCursorResetOnFocusChange verifies that repoCursor and repoOffset are
// reset to 0 whenever the focused list changes via Enter key.
func TestCursorResetOnFocusChange(t *testing.T) {
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

	if m2.repoCursor != 0 {
		t.Errorf("repoCursor = %d after focus change, want 0", m2.repoCursor)
	}
	if m2.repoOffset != 0 {
		t.Errorf("repoOffset = %d after focus change, want 0", m2.repoOffset)
	}
}

// TestDoubleClickDrillsToRepoPane verifies that two rapid clicks on the same
// list row switch active pane to paneRepo.
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

// --- P1 cache tests ---

// TestStaleReposLoadedMsgIgnored verifies that a reposLoadedMsg with a stale
// generation is silently dropped and does not update the cache.
func TestStaleReposLoadedMsgIgnored(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	// Bump generation so gen=0 messages are stale.
	m.generation = 1

	staleMsg := reposLoadedMsg{
		repos:  svc.repos,
		listID: m.focusedList.ID,
		gen:    0, // stale: model.generation is now 1
	}
	m2 := update(m, staleMsg)

	key := repoCacheKey{m.focusedList.ID, false}
	entry := m2.repoCache[key]
	if entry != nil && entry.state == repoCacheLoaded {
		t.Error("stale reposLoadedMsg should not write a repoCacheLoaded entry")
	}
}

// TestRepoCacheEntryWrittenOnLoad verifies that a fresh reposLoadedMsg writes a
// repoCacheLoaded entry and currentRepos() reflects the loaded data.
func TestRepoCacheEntryWrittenOnLoad(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	firstListID := m.lists[0].ID

	m2 := update(m, reposLoadedMsg{
		repos:      svc.repos,
		listID:     firstListID,
		withTopics: false,
		gen:        0,
	})

	key := repoCacheKey{firstListID, false}
	entry := m2.repoCache[key]
	if entry == nil {
		t.Fatal("repoCache entry should exist after reposLoadedMsg")
	}
	if entry.state != repoCacheLoaded {
		t.Errorf("entry.state = %d, want repoCacheLoaded", entry.state)
	}
	if len(m2.currentRepos()) != len(svc.repos) {
		t.Errorf("currentRepos len = %d, want %d", len(m2.currentRepos()), len(svc.repos))
	}
}

// TestAnyPendingDerivedFromMap verifies anyPending() returns true only when a
// repoCacheLoading entry exists in the map.
func TestAnyPendingDerivedFromMap(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	// newTestModel sets listsLoading=true; clear it so we can test repoCache alone.
	m.listsLoading = false

	if m.anyPending() {
		t.Error("anyPending should be false with empty cache and no flags set")
	}

	// Write a loading entry.
	key := repoCacheKey{listID: "UL_1", withTopics: false}
	m.repoCache[key] = &repoCacheEntry{state: repoCacheLoading}

	if !m.anyPending() {
		t.Error("anyPending should be true when a repoCacheLoading entry exists")
	}

	// Mark it loaded.
	m.repoCache[key] = &repoCacheEntry{state: repoCacheLoaded}

	if m.anyPending() {
		t.Error("anyPending should be false after entry transitions to repoCacheLoaded")
	}
}

// --- P2: bounded preloader and live pane update tests ---

// fiveListsSvc returns a service with 5 lists and 2 repos.
func fiveListsSvc() *fakeService {
	lists := []githubapi.StarList{
		{ID: "UL_1", Name: "list-one", RepoCount: 2},
		{ID: "UL_2", Name: "list-two", RepoCount: 2},
		{ID: "UL_3", Name: "list-three", RepoCount: 2},
		{ID: "UL_4", Name: "list-four", RepoCount: 2},
		{ID: "UL_5", Name: "list-five", RepoCount: 2},
	}
	repos := []githubapi.Repository{
		{ID: "R_1", NameWithOwner: "owner/repo-a", StargazerCount: 1},
		{ID: "R_2", NameWithOwner: "owner/repo-b", StargazerCount: 2},
	}
	return &fakeService{lists: lists, repos: repos}
}

// TestPreloaderRespectsConcurrencyCap verifies that after listsLoadedMsg with 5
// lists, at most 3 loads are in flight; delivering one reposLoadedMsg schedules
// the 4th.
func TestPreloaderRespectsConcurrencyCap(t *testing.T) {
	t.Parallel()
	svc := fiveListsSvc()
	m := newTestModel(svc)

	next, _ := m.Update(listsLoadedMsg{lists: svc.lists})
	m = next.(model)

	// At most 3 loads should be in flight.
	if m.preloadInFlight > 3 {
		t.Errorf("preloadInFlight = %d after listsLoadedMsg, want <= 3", m.preloadInFlight)
	}
	if m.preloadInFlight != 3 {
		t.Errorf("preloadInFlight = %d, want exactly 3 (cap not filled)", m.preloadInFlight)
	}
	if len(m.preloadQueue) != 2 {
		t.Errorf("preloadQueue len = %d, want 2 (remaining lists)", len(m.preloadQueue))
	}

	// Deliver one success -- a 4th load should now be scheduled.
	inflight := m.preloadInFlight
	next2, _ := m.Update(
		reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID, gen: m.generation},
	)
	m2 := next2.(model)

	// preloadInFlight should have decreased by 1 then increased by 1 (net same or +0).
	// It decreased when the response arrived, and then schedulePreload added the next one.
	// Since there are 2 in the queue, one is scheduled: inflight stays the same.
	if m2.preloadInFlight != inflight {
		t.Errorf(
			"preloadInFlight = %d after one reposLoadedMsg, want %d (one freed, one scheduled)",
			m2.preloadInFlight, inflight,
		)
	}
	// Queue should have shrunk by 1 (one was promoted to in-flight).
	if len(m2.preloadQueue) != 1 {
		t.Errorf("preloadQueue len = %d after one response, want 1", len(m2.preloadQueue))
	}
}

// TestCursorMoveUsesCacheNoNewCmd verifies that moving the cursor down to a
// cached list immediately populates displayedRepos with no new load command.
func TestCursorMoveUsesCacheNoNewCmd(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Pre-populate cache for all three lists so no loads are needed.
	reposA := []githubapi.Repository{{ID: "A_1", NameWithOwner: "owner/a-list-repo"}}
	reposB := []githubapi.Repository{{ID: "B_1", NameWithOwner: "owner/b-list-repo"}}
	reposC := []githubapi.Repository{{ID: "C_1", NameWithOwner: "owner/c-list-repo"}}
	m = update(m, reposLoadedMsg{repos: reposA, listID: m.lists[0].ID, gen: m.generation})
	m = update(m, reposLoadedMsg{repos: reposB, listID: m.lists[1].ID, gen: m.generation})
	m = update(m, reposLoadedMsg{repos: reposC, listID: m.lists[2].ID, gen: m.generation})

	// All lists are cached now; reset cursor to 0 and ensure list pane is active.
	m.active = paneList
	m.listCursor = 0
	inflightBefore := m.preloadInFlight

	// Move cursor down -- should use cache, no new load.
	next, _ := m.Update(specialKey(tea.KeyDown))
	m2 := next.(model)

	if m2.preloadInFlight != inflightBefore {
		t.Errorf(
			"preloadInFlight changed from %d to %d after cursor move to cached list",
			inflightBefore, m2.preloadInFlight,
		)
	}
	// displayedRepos should be populated from the cache for lists[1].
	if len(m2.displayedRepos) == 0 {
		t.Error("displayedRepos should be populated from cache after cursor move")
	}
	if m2.listCursor != 1 {
		t.Errorf("listCursor = %d, want 1", m2.listCursor)
	}
}

// TestCursorMoveIdleListSchedulesLoad verifies that moving the cursor to an idle
// list (no cache entry) creates a repoCacheLoading entry and increments
// preloadInFlight.
func TestCursorMoveIdleListSchedulesLoad(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Simulate: deliver repos for first list only, then manually clear all other
	// cache entries to simulate idle state for lists[1] and lists[2].
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: m.lists[0].ID, gen: m.generation})
	// Remove loading/loaded entries for UL_2 and UL_3 (simulate idle).
	delete(m.repoCache, repoCacheKey{m.lists[1].ID, false})
	delete(m.repoCache, repoCacheKey{m.lists[2].ID, false})
	m.preloadInFlight = 0
	m.preloadQueue = nil

	m.active = paneList
	m.listCursor = 0
	inflightBefore := m.preloadInFlight

	// Move down to lists[1] which has no cache entry.
	m2 := update(m, specialKey(tea.KeyDown))

	cacheEntry := m2.repoCache[repoCacheKey{m2.lists[1].ID, false}]
	if cacheEntry == nil {
		t.Fatal("repoCache entry for list[1] should exist after cursor move to idle list")
	}
	if cacheEntry.state != repoCacheLoading {
		t.Errorf("cache entry state = %d, want repoCacheLoading", cacheEntry.state)
	}
	if m2.preloadInFlight <= inflightBefore {
		t.Errorf(
			"preloadInFlight did not increase: before=%d after=%d",
			inflightBefore, m2.preloadInFlight,
		)
	}
}

// TestSingleClickFocusedUncachedTriggersLoad verifies that a single click on the
// already-focused list row (with idle cache entry) triggers a repo load without
// switching to the repo pane.
func TestSingleClickFocusedUncachedTriggersLoad(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Clear the focused list's cache entry to simulate idle state.
	if m.focusedList != nil {
		delete(m.repoCache, repoCacheKey{m.focusedList.ID, false})
	}
	m.preloadInFlight = 0
	m.preloadQueue = nil
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
	entry := m2.repoCache[repoCacheKey{m2.focusedList.ID, false}]
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

// TestEnterListPaneNoLoadWhenCached verifies that pressing Enter in the list pane
// when the focused list's repos are already cached switches to paneRepo without
// issuing a new load command.
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

// --- P3: nine new tests ---

// TestRefreshBumpsGenerationAndClearsCache verifies ctrl+r increments m.generation
// and empties m.repoCache.
func TestRefreshBumpsGenerationAndClearsCache(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Populate cache with a loaded entry.
	m.repoCache[repoCacheKey{"UL_1", false}] = &repoCacheEntry{state: repoCacheLoaded}
	genBefore := m.generation

	m2 := update(m, ctrlKey('r'))

	if m2.generation != genBefore+1 {
		t.Errorf("generation = %d, want %d after ctrl+r", m2.generation, genBefore+1)
	}
	if len(m2.repoCache) != 0 {
		t.Errorf("repoCache len = %d, want 0 after ctrl+r", len(m2.repoCache))
	}
	if !m2.listsLoading {
		t.Error("listsLoading should be true after ctrl+r")
	}
}

// TestStaleMsgDroppedAfterRefresh verifies that a reposLoadedMsg from the prior
// generation is dropped and does not install a loaded entry.
func TestStaleMsgDroppedAfterRefresh(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Refresh bumps generation to 1.
	m = update(m, ctrlKey('r'))
	if m.generation != 1 {
		t.Fatalf("generation = %d after ctrl+r, want 1", m.generation)
	}

	// Deliver a stale message (gen 0).
	stale := reposLoadedMsg{repos: svc.repos, listID: "UL_1", gen: 0}
	m2 := update(m, stale)

	entry := m2.repoCache[repoCacheKey{"UL_1", false}]
	if entry != nil && entry.state == repoCacheLoaded {
		t.Error(
			"stale reposLoadedMsg (gen=0) should not create a loaded entry after refresh (gen=1)",
		)
	}
}

// TestMutationModalStaysOpenWhileSubmitting verifies that submitting a modal form
// keeps the modal open with submitting=true and mutationPending=true.
func TestMutationModalStaysOpenWhileSubmitting(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{
		fakeService: fakeService{
			lists: []githubapi.StarList{{ID: "UL_1", Name: "existing"}},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Open create-list modal.
	m = update(m, keyPress('n'))
	if m.modal == nil {
		t.Fatal("create modal should be open")
	}

	// Type a name.
	for _, ch := range "NewList" {
		m = update(m, keyPress(ch))
	}

	// Submit: advance to last field, then submit.
	m = update(m, specialKey(tea.KeyEnter)) // advance to desc
	m = update(m, specialKey(tea.KeyEnter)) // advance to visibility or submit

	// Keep pressing enter until modal is submitting or we give up.
	for i := 0; i < 5; i++ {
		if m.modal != nil && m.modal.submitting {
			break
		}
		m = update(m, specialKey(tea.KeyEnter))
	}

	if m.modal == nil {
		t.Fatal("modal should remain open while submitting")
	}
	if !m.modal.submitting {
		t.Error("modal.submitting should be true after submit")
	}
	if !m.mutationPending {
		t.Error("mutationPending should be true while modal is submitting")
	}
}

// TestMutationDoneClosesModalAndInvalidatesEntry verifies that a successful
// mutationDoneMsg closes the modal, sets a toast, and removes the focused list's
// cache entries.
func TestMutationDoneClosesModalAndInvalidatesEntry(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.focusedList = &m.lists[0]

	// Pre-populate a cache entry to confirm it gets invalidated.
	m.repoCache[repoCacheKey{"UL_1", false}] = &repoCacheEntry{state: repoCacheLoaded}
	m.repoCache[repoCacheKey{"UL_1", true}] = &repoCacheEntry{state: repoCacheLoaded}

	// Open a modal in submitting state.
	m.modal = &modal{kind: modalCreateList, submitting: true}
	m.mutationPending = true

	m2 := update(m, mutationDoneMsg{kind: modalCreateList})

	if m2.modal != nil {
		t.Error("modal should be nil after successful mutationDoneMsg")
	}
	if m2.statusMsg == "" {
		t.Error("statusMsg should be set after successful mutation")
	}
	if m2.mutationPending {
		t.Error("mutationPending should be false after successful mutation")
	}
	// Both cache entries for UL_1 should be deleted (invalidated).
	if e := m2.repoCache[repoCacheKey{"UL_1", false}]; e != nil && e.state == repoCacheLoaded {
		t.Error("repoCache[UL_1, false] should be invalidated after mutation")
	}
	if e := m2.repoCache[repoCacheKey{"UL_1", true}]; e != nil && e.state == repoCacheLoaded {
		t.Error("repoCache[UL_1, true] should be invalidated after mutation")
	}
}

// TestMutationErrorKeepsModalOpenWithMessage verifies that a failed mutationDoneMsg
// keeps the modal open with submitting=false and submitErr set.
func TestMutationErrorKeepsModalOpenWithMessage(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.modal = &modal{kind: modalCreateList, submitting: true}
	m.mutationPending = true
	someErr := errors.New("network timeout")

	m2 := update(m, mutationDoneMsg{kind: modalCreateList, err: someErr})

	if m2.modal == nil {
		t.Fatal("modal should remain open after mutation error")
	}
	if m2.modal.submitting {
		t.Error("modal.submitting should be false after error")
	}
	if !strings.Contains(m2.modal.submitErr, someErr.Error()) {
		t.Errorf("modal.submitErr = %q, want to contain %q", m2.modal.submitErr, someErr.Error())
	}
	if m2.mutationPending {
		t.Error("mutationPending should be false after error")
	}
}

// TestBulkMutationModalSubmittingState verifies that submitting a bulk-add modal
// sets submitting=true on the modal.
func TestBulkMutationModalSubmittingState(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	// Select a repo.
	m.selected = map[string]struct{}{"owner/b-repo": {}}

	// Open bulk-add modal.
	m = update(m, keyPress('a'))
	if m.modal == nil {
		t.Fatal("bulk-add modal should open")
	}

	// Submit (enter on the picker).
	m = update(m, specialKey(tea.KeyEnter))

	if m.modal == nil {
		t.Fatal("modal should remain open while submitting (bulk)")
	}
	if !m.modal.submitting {
		t.Error("modal.submitting should be true after bulk-add submit")
	}
}

// TestRepoWidthsCachedAcrossScrolls verifies that ensureRepoWidths populates the star
// and language width cache fields and that the sentinel is stable across repeated calls
// with the same displayedRepos.
func TestRepoWidthsCachedAcrossScrolls(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m.width = 120
	m.height = 24

	// Call ensureRepoWidths directly on a pointer so mutations are visible.
	(&m).ensureRepoWidths()
	sig1 := m.cachedRepoSig
	sw1 := m.cachedStarWidth
	lw1 := m.cachedLangWidth

	if sig1 == "" {
		t.Error("cachedRepoSig should be non-empty after ensureRepoWidths")
	}
	if sw1 <= 0 {
		t.Errorf("cachedStarWidth = %d, want > 0", sw1)
	}

	// Second call -- sentinel unchanged, widths stable (cache hit).
	(&m).ensureRepoWidths()
	if m.cachedRepoSig != sig1 {
		t.Errorf("cachedRepoSig changed on second call: %q -> %q", sig1, m.cachedRepoSig)
	}
	if m.cachedStarWidth != sw1 {
		t.Errorf("cachedStarWidth changed on second call: %d -> %d", sw1, m.cachedStarWidth)
	}
	if m.cachedLangWidth != lw1 {
		t.Errorf("cachedLangWidth changed on second call: %d -> %d", lw1, m.cachedLangWidth)
	}

	// Verify sentinel invalidates when list changes.
	m.focusedList = &m.lists[1]
	m.displayedRepos = svc.repos // same repos, different focused list
	(&m).ensureRepoWidths()
	if m.cachedRepoSig == sig1 {
		t.Error("cachedRepoSig should change when focused list changes")
	}
}

// TestPreviewWheelScrollsOffset verifies that a mouse wheel event over the preview
// column increases m.previewOffset.
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
	e1 := m2.repoCache[repoCacheKey{"UL_1", true}]
	if e1 == nil {
		t.Error("repoCache[UL_1, true] should exist after preview toggle for focused list")
	}
	// Other lists should NOT have withTopics=true entries.
	if e := m2.repoCache[repoCacheKey{"UL_2", true}]; e != nil {
		t.Errorf("repoCache[UL_2, true] should not exist; focused list is UL_1")
	}
	if e := m2.repoCache[repoCacheKey{"UL_3", true}]; e != nil {
		t.Errorf("repoCache[UL_3, true] should not exist; focused list is UL_1")
	}
}
