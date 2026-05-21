package tui

import (
	"context"
	"fmt"

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

func previewPane(m model, w, h int) string { return m.renderPreviewPane(w, h) }

// TestPreviewDetailBlock verifies that the styled preview pane renders all

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
