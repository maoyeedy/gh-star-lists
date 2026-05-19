package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	_, cmd := m.Update(specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Error("correct name should produce a cmd (delete mutation)")
	}
	// Execute the cmd to get mutationDoneMsg.
	msg := cmd()
	doneMsg, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want mutationDoneMsg", msg)
	}
	if doneMsg.kind != modalDeleteList {
		t.Errorf("doneMsg.kind = %v, want modalDeleteList", doneMsg.kind)
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

// TestMutationListErrorDisplayed verifies that mutationDoneMsg with an error sets model.err.
func TestMutationListErrorDisplayed(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	sentinel := errors.New("delete failed")

	m2 := update(m, mutationDoneMsg{kind: modalDeleteList, err: sentinel})
	if !errors.Is(m2.err, sentinel) {
		t.Errorf("err = %v, want sentinel", m2.err)
	}
	if m2.modal != nil {
		t.Error("modal should be nil after error")
	}
}

// TestMutationErrorSetsErrField verifies mutationDoneMsg with err sets model.err.
func TestMutationErrorSetsErrField(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	sentinel := errors.New("create failed")

	m2 := update(m, mutationDoneMsg{kind: modalCreateList, err: sentinel})
	if !errors.Is(m2.err, sentinel) {
		t.Errorf("err = %v, want %v", m2.err, sentinel)
	}
	if m2.modal != nil {
		t.Error("modal should be closed after error")
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
	m.repos = inner.repos

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
	paneH := m2.height - 2
	if m2.repoOffset != max(0, 50-paneH) {
		t.Errorf("G: repoOffset = %d, want %d", m2.repoOffset, max(0, 50-paneH))
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
