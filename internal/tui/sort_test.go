package tui

import (
	"strings"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

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
