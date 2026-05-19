package githubapi

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"strings"
	"testing"
)

type graphQLCall struct {
	query     string
	variables map[string]any
}

type fakeGraphQLExecutor struct {
	responses []string
	errors    []error
	calls     []graphQLCall
}

func (f *fakeGraphQLExecutor) DoWithContext(
	_ context.Context,
	query string,
	variables map[string]any,
	response any,
) error {
	f.calls = append(f.calls, graphQLCall{query: query, variables: maps.Clone(variables)})
	if len(f.errors) > 0 {
		err := f.errors[0]
		f.errors = f.errors[1:]
		if err != nil {
			return err
		}
	}
	if len(f.responses) == 0 {
		return nil
	}
	body := f.responses[0]
	f.responses = f.responses[1:]
	return json.Unmarshal([]byte(body), response)
}

func TestGraphQLServiceListStarListsReturnsEmptyResults(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"viewer": {
			"lists": {
				"nodes": [],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	lists, err := service.ListStarLists(context.Background())
	if err != nil {
		t.Fatalf("ListStarLists returned error: %v", err)
	}
	if len(lists) != 0 {
		t.Fatalf("ListStarLists() = %#v, want empty", lists)
	}
}

func TestGraphQLServiceListStarListsNormalizesNullNullableFields(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"viewer": {
			"lists": {
				"nodes": [
					{"id": "UL_1", "name": "Tools", "slug": "tools", "description": null, "lastAddedAt": null, "user": {"login": "testuser"}}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	lists, err := service.ListStarLists(context.Background())
	if err != nil {
		t.Fatalf("ListStarLists returned error: %v", err)
	}
	want := StarList{
		Name:        "Tools",
		Description: "",
		LastAddedAt: "",
		ID:          "UL_1",
		URL:         "https://github.com/stars/testuser/lists/tools",
	}
	if len(lists) != 1 || lists[0] != want {
		t.Fatalf("ListStarLists() = %#v, want %#v", lists, []StarList{want})
	}
}

func TestGraphQLServiceListStarListsWrapsLaterPageErrorAndReturnsNoPartialResults(t *testing.T) {
	t.Parallel()

	boom := errors.New("rate limited")
	executor := &fakeGraphQLExecutor{
		responses: []string{`{
			"viewer": {
				"lists": {
					"nodes": [
						{"id": "UL_1", "name": "Tools", "slug": "tools", "description": "Useful CLIs", "lastAddedAt": "2025-01-02T03:04:05Z", "user": {"login": "testuser"}}
					],
					"pageInfo": {"hasNextPage": true, "endCursor": "cursor-1"}
				}
			}
		}`},
		errors: []error{nil, boom},
	}
	service := newGraphQLService(executor, 100)

	lists, err := service.ListStarLists(context.Background())
	if err == nil {
		t.Fatal("ListStarLists error = nil, want wrapped executor error")
	}
	if !strings.HasPrefix(err.Error(), "GitHub GraphQL request failed: ") {
		t.Fatalf("error = %q, want stable GraphQL prefix", err.Error())
	}
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want to wrap executor error %v", err, boom)
	}
	if lists != nil {
		t.Fatalf("ListStarLists partial results = %#v, want nil", lists)
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
}

func TestGraphQLServiceListStarListsStopsBeforeRequestWhenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := &fakeGraphQLExecutor{}
	service := newGraphQLService(executor, 100)

	lists, err := service.ListStarLists(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListStarLists error = %v, want context.Canceled", err)
	}
	if lists != nil {
		t.Fatalf("ListStarLists results = %#v, want nil", lists)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %d, want 0", len(executor.calls))
	}
}

func TestGraphQLServiceListStarListsFetchesMultiplePages(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"viewer": {
			"lists": {
				"nodes": [
					{"id": "UL_1", "name": "Tools", "slug": "tools", "description": "Useful CLIs", "lastAddedAt": "2025-01-02T03:04:05Z", "user": {"login": "testuser"}}
				],
				"pageInfo": {"hasNextPage": true, "endCursor": "cursor-1"}
			}
		}
	}`, `{
		"viewer": {
			"lists": {
				"nodes": [
					{"id": "UL_2", "name": "Libraries", "slug": "libraries", "description": "Packages", "lastAddedAt": "2025-02-03T04:05:06Z", "user": {"login": "testuser"}}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	lists, err := service.ListStarLists(context.Background())
	if err != nil {
		t.Fatalf("ListStarLists returned error: %v", err)
	}

	want := []StarList{
		{
			Name:        "Tools",
			Description: "Useful CLIs",
			LastAddedAt: "2025-01-02T03:04:05Z",
			ID:          "UL_1",
			URL:         "https://github.com/stars/testuser/lists/tools",
		},
		{
			Name:        "Libraries",
			Description: "Packages",
			LastAddedAt: "2025-02-03T04:05:06Z",
			ID:          "UL_2",
			URL:         "https://github.com/stars/testuser/lists/libraries",
		},
	}
	if len(lists) != len(want) || lists[0] != want[0] || lists[1] != want[1] {
		t.Fatalf("ListStarLists() = %#v, want %#v", lists, want)
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
	if got := executor.calls[0].variables["endCursor"]; got != nil {
		t.Fatalf("first page endCursor = %#v, want nil", got)
	}
	if got := executor.calls[1].variables["endCursor"]; got != "cursor-1" {
		t.Fatalf("second page endCursor = %#v, want cursor-1", got)
	}
}

func TestGraphQLServiceListStarListsMapsSinglePage(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"viewer": {
			"lists": {
				"nodes": [
					{"id": "UL_1", "name": "Tools", "slug": "tools", "description": "Useful CLIs", "lastAddedAt": "2025-01-02T03:04:05Z", "items": {"totalCount": 5}, "user": {"login": "testuser"}}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	lists, err := service.ListStarLists(context.Background())
	if err != nil {
		t.Fatalf("ListStarLists returned error: %v", err)
	}

	want := []StarList{
		{
			Name:        "Tools",
			Description: "Useful CLIs",
			LastAddedAt: "2025-01-02T03:04:05Z",
			ID:          "UL_1",
			RepoCount:   5,
			URL:         "https://github.com/stars/testuser/lists/tools",
		},
	}
	if len(lists) != len(want) || lists[0] != want[0] {
		t.Fatalf("ListStarLists() = %#v, want %#v", lists, want)
	}
	if len(executor.calls) != 1 {
		t.Fatalf("executor calls = %d, want 1", len(executor.calls))
	}
	if got := executor.calls[0].variables["endCursor"]; got != nil {
		t.Fatalf("first page endCursor = %#v, want nil", got)
	}
	if !strings.Contains(executor.calls[0].query, "lists(first: $first, after: $endCursor)") {
		t.Fatalf("query = %q, want paginated Star Lists query", executor.calls[0].query)
	}
}

func TestGraphQLServiceListRepositoriesMapsMultiplePages(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"node": {
			"__typename": "UserList",
			"name": "Tools",
			"items": {
				"nodes": [
					{"__typename": "Repository", "nameWithOwner": "cli/cli", "description": "GitHub CLI", "url": "https://github.com/cli/cli", "isFork": false, "stargazerCount": 39000, "pushedAt": "2026-01-02T03:04:05Z", "primaryLanguage": {"name": "Go"}}
				],
				"pageInfo": {"hasNextPage": true, "endCursor": "repo-cursor-1"}
			}
		}
	}`, `{
		"node": {
			"__typename": "UserList",
			"name": "Tools",
			"items": {
				"nodes": [
					{"__typename": "Repository", "nameWithOwner": "cli/go-gh", "description": "Go helpers", "url": "https://github.com/cli/go-gh", "isFork": false, "stargazerCount": 700, "pushedAt": "2026-02-03T04:05:06Z", "primaryLanguage": null}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	repositories, err := service.ListRepositories(context.Background(), "UL_1")
	if err != nil {
		t.Fatalf("ListRepositories returned error: %v", err)
	}

	want := []Repository{
		{
			NameWithOwner:  "cli/cli",
			Description:    "GitHub CLI",
			URL:            "https://github.com/cli/cli",
			IsFork:         false,
			StargazerCount: 39000,
			PushedAt:       "2026-01-02T03:04:05Z",
			Language:       "Go",
		},
		{
			NameWithOwner:  "cli/go-gh",
			Description:    "Go helpers",
			URL:            "https://github.com/cli/go-gh",
			IsFork:         false,
			StargazerCount: 700,
			PushedAt:       "2026-02-03T04:05:06Z",
			Language:       "",
		},
	}
	if !reflect.DeepEqual(repositories, want) {
		t.Fatalf("ListRepositories() = %#v, want %#v", repositories, want)
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
	if got := executor.calls[0].variables["id"]; got != "UL_1" {
		t.Fatalf("first page id = %#v, want UL_1", got)
	}
	if got := executor.calls[0].variables["endCursor"]; got != nil {
		t.Fatalf("first page endCursor = %#v, want nil", got)
	}
	if got := executor.calls[1].variables["endCursor"]; got != "repo-cursor-1" {
		t.Fatalf("second page endCursor = %#v, want repo-cursor-1", got)
	}
	if !strings.Contains(executor.calls[0].query, "items(first: $first, after: $endCursor)") {
		t.Fatalf("query = %q, want paginated UserList items query", executor.calls[0].query)
	}
}

func TestGraphQLServiceListRepositoriesNormalizesNullNullableFields(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"node": {
			"__typename": "UserList",
			"items": {
				"nodes": [
					{"__typename": "Repository", "nameWithOwner": "owner/repo", "description": null, "url": "https://github.com/owner/repo", "isFork": true, "stargazerCount": 12, "pushedAt": null, "primaryLanguage": null}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	repositories, err := service.ListRepositories(context.Background(), "UL_1")
	if err != nil {
		t.Fatalf("ListRepositories returned error: %v", err)
	}
	want := Repository{
		NameWithOwner:  "owner/repo",
		Description:    "",
		URL:            "https://github.com/owner/repo",
		IsFork:         true,
		StargazerCount: 12,
		PushedAt:       "",
		Language:       "",
	}
	if len(repositories) != 1 || !reflect.DeepEqual(repositories[0], want) {
		t.Fatalf("ListRepositories() = %#v, want %#v", repositories, []Repository{want})
	}
}

func TestGraphQLServiceListRepositoriesReturnsEmptyResults(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"node": {
			"__typename": "UserList",
			"items": {
				"nodes": [],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	repositories, err := service.ListRepositories(context.Background(), "UL_1")
	if err != nil {
		t.Fatalf("ListRepositories returned error: %v", err)
	}
	if len(repositories) != 0 {
		t.Fatalf("ListRepositories() = %#v, want empty", repositories)
	}
}

func TestGraphQLServiceListRepositoriesReturnsErrInaccessibleListForWrongNodeType(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{"node": {"__typename": "Repository"}}`}}
	service := newGraphQLService(executor, 100)

	repositories, err := service.ListRepositories(context.Background(), "R_1")
	if !errors.Is(err, ErrInaccessibleList) {
		t.Fatalf("ListRepositories error = %v, want ErrInaccessibleList", err)
	}
	if repositories != nil {
		t.Fatalf("ListRepositories results = %#v, want nil", repositories)
	}
}

func TestGraphQLServiceListRepositoriesReturnsErrInaccessibleListForNilNode(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{"node": null}`}}
	service := newGraphQLService(executor, 100)

	repositories, err := service.ListRepositories(context.Background(), "UL_missing")
	if !errors.Is(err, ErrInaccessibleList) {
		t.Fatalf("ListRepositories error = %v, want ErrInaccessibleList", err)
	}
	if repositories != nil {
		t.Fatalf("ListRepositories results = %#v, want nil", repositories)
	}
}

func TestGraphQLServiceListRepositoriesReturnsErrInaccessibleListForMissingItemsConnection(
	t *testing.T,
) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{
		responses: []string{`{"node": {"__typename": "UserList", "items": null}}`},
	}
	service := newGraphQLService(executor, 100)

	repositories, err := service.ListRepositories(context.Background(), "UL_1")
	if !errors.Is(err, ErrInaccessibleList) {
		t.Fatalf("ListRepositories error = %v, want ErrInaccessibleList", err)
	}
	if repositories != nil {
		t.Fatalf("ListRepositories results = %#v, want nil", repositories)
	}
}

func TestGraphQLServiceListRepositoriesSkipsNonRepositoryAndMissingNameItems(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"node": {
			"__typename": "UserList",
			"items": {
				"nodes": [
					null,
					{"__typename": "Issue"},
					{"__typename": "Repository", "nameWithOwner": "", "description": "missing name", "url": "https://github.com/missing/name", "isFork": false, "stargazerCount": 1, "pushedAt": "2026-01-01T00:00:00Z"},
					{"__typename": "Repository", "nameWithOwner": "owner/repo", "description": "kept", "url": "https://github.com/owner/repo", "isFork": false, "stargazerCount": 2, "pushedAt": "2026-01-02T00:00:00Z"}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	repositories, err := service.ListRepositories(context.Background(), "UL_1")
	if err != nil {
		t.Fatalf("ListRepositories returned error: %v", err)
	}
	want := Repository{
		NameWithOwner:  "owner/repo",
		Description:    "kept",
		URL:            "https://github.com/owner/repo",
		IsFork:         false,
		StargazerCount: 2,
		PushedAt:       "2026-01-02T00:00:00Z",
	}
	if len(repositories) != 1 || !reflect.DeepEqual(repositories[0], want) {
		t.Fatalf("ListRepositories() = %#v, want %#v", repositories, []Repository{want})
	}
}

func TestGraphQLServiceListRepositoriesWrapsLaterPageErrorAndReturnsNoPartialResults(t *testing.T) {
	t.Parallel()

	boom := errors.New("secondary rate limit")
	executor := &fakeGraphQLExecutor{
		responses: []string{`{
			"node": {
				"__typename": "UserList",
				"items": {
					"nodes": [
						{"__typename": "Repository", "nameWithOwner": "owner/repo", "description": "first", "url": "https://github.com/owner/repo", "isFork": false, "stargazerCount": 1, "pushedAt": "2026-01-01T00:00:00Z"}
					],
					"pageInfo": {"hasNextPage": true, "endCursor": "repo-cursor-1"}
				}
			}
		}`},
		errors: []error{nil, boom},
	}
	service := newGraphQLService(executor, 100)

	repositories, err := service.ListRepositories(context.Background(), "UL_1")
	if err == nil {
		t.Fatal("ListRepositories error = nil, want wrapped executor error")
	}
	if !strings.HasPrefix(err.Error(), "GitHub GraphQL request failed: ") {
		t.Fatalf("error = %q, want stable GraphQL prefix", err.Error())
	}
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want to wrap executor error %v", err, boom)
	}
	if repositories != nil {
		t.Fatalf("ListRepositories partial results = %#v, want nil", repositories)
	}
}

func TestGraphQLServiceListRepositoriesStopsBeforeRequestWhenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := &fakeGraphQLExecutor{}
	service := newGraphQLService(executor, 100)

	repositories, err := service.ListRepositories(ctx, "UL_1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListRepositories error = %v, want context.Canceled", err)
	}
	if repositories != nil {
		t.Fatalf("ListRepositories results = %#v, want nil", repositories)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %d, want 0", len(executor.calls))
	}
}

func TestGraphQLServiceSmallPageSize(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"viewer": {
			"lists": {
				"nodes": [
					{"id": "UL_1", "name": "A", "slug": "a", "description": "first", "lastAddedAt": "2025-01-01T00:00:00Z", "user": {"login": "testuser"}},
					{"id": "UL_2", "name": "B", "slug": "b", "description": "second", "lastAddedAt": "2025-01-02T00:00:00Z", "user": {"login": "testuser"}}
				],
				"pageInfo": {"hasNextPage": true, "endCursor": "cursor-1"}
			}
		}
	}`, `{
		"viewer": {
			"lists": {
				"nodes": [
					{"id": "UL_3", "name": "C", "slug": "c", "description": "third", "lastAddedAt": "2025-01-03T00:00:00Z", "user": {"login": "testuser"}}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 2)

	lists, err := service.ListStarLists(context.Background())
	if err != nil {
		t.Fatalf("ListStarLists error: %v", err)
	}
	if len(lists) != 3 {
		t.Fatalf("ListStarLists returned %d items, want 3", len(lists))
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
	if got, want := executor.calls[0].variables["first"].(int), 2; got != want {
		t.Fatalf("pageSize variable = %d, want %d", got, want)
	}
}

func TestGraphQLServiceReposSmallPageSize(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"node": {
			"__typename": "UserList",
			"items": {
				"nodes": [
					{"__typename": "Repository", "nameWithOwner": "owner/A", "description": "first", "url": "https://github.com/owner/A", "isFork": false, "stargazerCount": 1, "pushedAt": "2025-01-01T00:00:00Z"},
					{"__typename": "Repository", "nameWithOwner": "owner/B", "description": "second", "url": "https://github.com/owner/B", "isFork": false, "stargazerCount": 2, "pushedAt": "2025-01-02T00:00:00Z"}
				],
				"pageInfo": {"hasNextPage": true, "endCursor": "repo-cursor-1"}
			}
		}
	}`, `{
		"node": {
			"__typename": "UserList",
			"items": {
				"nodes": [
					{"__typename": "Repository", "nameWithOwner": "owner/C", "description": "third", "url": "https://github.com/owner/C", "isFork": false, "stargazerCount": 3, "pushedAt": "2025-01-03T00:00:00Z"}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 2)

	repos, err := service.ListRepositories(context.Background(), "UL_1")
	if err != nil {
		t.Fatalf("ListRepositories error: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("ListRepositories returned %d items, want 3", len(repos))
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
	if got, want := executor.calls[0].variables["first"].(int), 2; got != want {
		t.Fatalf("pageSize variable = %d, want %d", got, want)
	}
}

func TestGraphQLServicePaginationWithSkippedItems(t *testing.T) {
	t.Parallel()

	// pageSize=4, 3 repos + 1 non-Repository + 1 null = 5 items on 1 page
	executor := &fakeGraphQLExecutor{responses: []string{`{
		"node": {
			"__typename": "UserList",
			"items": {
				"nodes": [
					{"__typename": "Repository", "nameWithOwner": "owner/repo1", "description": "first", "url": "https://github.com/owner/repo1", "isFork": false, "stargazerCount": 1, "pushedAt": "2025-01-01T00:00:00Z"},
					null,
					{"__typename": "Issue"},
					{"__typename": "Repository", "nameWithOwner": "owner/repo2", "description": "second", "url": "https://github.com/owner/repo2", "isFork": true, "stargazerCount": 2, "pushedAt": "2025-01-02T00:00:00Z"},
					{"__typename": "Repository", "nameWithOwner": "", "description": "missing name", "url": "https://github.com/missing/name", "isFork": false, "stargazerCount": 3, "pushedAt": "2025-01-03T00:00:00Z"}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 4)

	repos, err := service.ListRepositories(context.Background(), "UL_1")
	if err != nil {
		t.Fatalf("ListRepositories error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("ListRepositories returned %d items, want 2 (only valid repos kept)", len(repos))
	}
	if repos[0].NameWithOwner != "owner/repo1" || repos[1].NameWithOwner != "owner/repo2" {
		t.Fatalf("ListRepositories = %#v, want [owner/repo1 owner/repo2]", repos)
	}
}

func TestGraphQLServiceListStarredRepositoriesMapsFields(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"viewer": {
			"starredRepositories": {
				"edges": [
					{
						"starredAt": "2026-03-01T00:00:00Z",
						"node": {
							"nameWithOwner": "owner/go-tool",
							"description": "A Go tool",
							"url": "https://github.com/owner/go-tool",
							"isFork": false,
							"stargazerCount": 500,
							"pushedAt": "2026-02-01T00:00:00Z",
							"primaryLanguage": {"name": "Go"}
						}
					},
					{
						"starredAt": "2026-02-01T00:00:00Z",
						"node": {
							"nameWithOwner": "owner/rust-lib",
							"description": null,
							"url": "https://github.com/owner/rust-lib",
							"isFork": true,
							"stargazerCount": 100,
							"pushedAt": null,
							"primaryLanguage": null
						}
					}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	repos, err := service.ListStarredRepositories(context.Background())
	if err != nil {
		t.Fatalf("ListStarredRepositories returned error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("ListStarredRepositories returned %d repos, want 2", len(repos))
	}
	want0 := Repository{
		NameWithOwner:  "owner/go-tool",
		Description:    "A Go tool",
		URL:            "https://github.com/owner/go-tool",
		IsFork:         false,
		StargazerCount: 500,
		PushedAt:       "2026-02-01T00:00:00Z",
		Language:       "Go",
		StarredAt:      "2026-03-01T00:00:00Z",
	}
	want1 := Repository{
		NameWithOwner:  "owner/rust-lib",
		Description:    "",
		URL:            "https://github.com/owner/rust-lib",
		IsFork:         true,
		StargazerCount: 100,
		PushedAt:       "",
		Language:       "",
		StarredAt:      "2026-02-01T00:00:00Z",
	}
	if !reflect.DeepEqual(repos[0], want0) {
		t.Fatalf("repos[0] = %#v, want %#v", repos[0], want0)
	}
	if !reflect.DeepEqual(repos[1], want1) {
		t.Fatalf("repos[1] = %#v, want %#v", repos[1], want1)
	}
	if !strings.Contains(executor.calls[0].query, "starredRepositories") {
		t.Fatalf("query = %q, want starredRepositories query", executor.calls[0].query)
	}
}

func TestGraphQLServiceListStarredRepositoriesPaginates(t *testing.T) {
	t.Parallel()

	executor := &fakeGraphQLExecutor{responses: []string{`{
		"viewer": {
			"starredRepositories": {
				"edges": [
					{"starredAt": "2026-03-01T00:00:00Z", "node": {"nameWithOwner": "owner/a", "url": "https://github.com/owner/a"}}
				],
				"pageInfo": {"hasNextPage": true, "endCursor": "star-cursor-1"}
			}
		}
	}`, `{
		"viewer": {
			"starredRepositories": {
				"edges": [
					{"starredAt": "2026-02-01T00:00:00Z", "node": {"nameWithOwner": "owner/b", "url": "https://github.com/owner/b"}}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 100)

	repos, err := service.ListStarredRepositories(context.Background())
	if err != nil {
		t.Fatalf("ListStarredRepositories returned error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("ListStarredRepositories returned %d repos, want 2", len(repos))
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
	if got := executor.calls[1].variables["endCursor"]; got != "star-cursor-1" {
		t.Fatalf("second page endCursor = %#v, want star-cursor-1", got)
	}
}

func TestGraphQLServicePaginationExactMultiple(t *testing.T) {
	t.Parallel()

	// pageSize=4, 2 pages = 8 items exactly
	executor := &fakeGraphQLExecutor{responses: []string{`{
		"viewer": {
			"lists": {
				"nodes": [
					{"id": "UL_1", "name": "A", "slug": "a", "description": "", "lastAddedAt": "2025-01-01T00:00:00Z", "user": {"login": "testuser"}},
					{"id": "UL_2", "name": "B", "slug": "b", "description": "", "lastAddedAt": "2025-01-02T00:00:00Z", "user": {"login": "testuser"}},
					{"id": "UL_3", "name": "C", "slug": "c", "description": "", "lastAddedAt": "2025-01-03T00:00:00Z", "user": {"login": "testuser"}},
					{"id": "UL_4", "name": "D", "slug": "d", "description": "", "lastAddedAt": "2025-01-04T00:00:00Z", "user": {"login": "testuser"}}
				],
				"pageInfo": {"hasNextPage": true, "endCursor": "cursor-1"}
			}
		}
	}`, `{
		"viewer": {
			"lists": {
				"nodes": [
					{"id": "UL_5", "name": "E", "slug": "e", "description": "", "lastAddedAt": "2025-01-05T00:00:00Z", "user": {"login": "testuser"}},
					{"id": "UL_6", "name": "F", "slug": "f", "description": "", "lastAddedAt": "2025-01-06T00:00:00Z", "user": {"login": "testuser"}},
					{"id": "UL_7", "name": "G", "slug": "g", "description": "", "lastAddedAt": "2025-01-07T00:00:00Z", "user": {"login": "testuser"}},
					{"id": "UL_8", "name": "H", "slug": "h", "description": "", "lastAddedAt": "2025-01-08T00:00:00Z", "user": {"login": "testuser"}}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}
		}
	}`}}
	service := newGraphQLService(executor, 4)

	lists, err := service.ListStarLists(context.Background())
	if err != nil {
		t.Fatalf("ListStarLists error: %v", err)
	}
	if len(lists) != 8 {
		t.Fatalf("ListStarLists returned %d items, want 8", len(lists))
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
}
