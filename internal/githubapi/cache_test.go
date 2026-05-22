package githubapi

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeCacheInner struct {
	listCalls    int
	reposCalls   map[string]int
	starredCalls int
	getRepoCalls int
	lists        []StarList
	repos        map[string][]Repository
	starred      []Repository
	gotRepo      Repository
	listErr      error
	reposErr     error
}

func (f *fakeCacheInner) ListStarLists(context.Context, ...ListOptions) ([]StarList, error) {
	f.listCalls++
	return f.lists, f.listErr
}

func (f *fakeCacheInner) ListRepositories(
	_ context.Context,
	listID string,
	_ ...ListOptions,
) ([]Repository, error) {
	if f.reposCalls == nil {
		f.reposCalls = make(map[string]int)
	}
	f.reposCalls[listID]++
	if f.reposErr != nil {
		return nil, f.reposErr
	}
	return f.repos[listID], nil
}

func (f *fakeCacheInner) ListStarredRepositories(
	_ context.Context,
	_ ...ListOptions,
) ([]Repository, error) {
	f.starredCalls++
	return f.starred, nil
}

func (f *fakeCacheInner) GetRepository(
	_ context.Context,
	nameWithOwner string,
) (Repository, error) {
	f.getRepoCalls++
	if f.gotRepo.NameWithOwner != "" {
		return f.gotRepo, nil
	}
	return Repository{ID: "R_1", NameWithOwner: nameWithOwner}, nil
}

func (f *fakeCacheInner) GetRepositoryMemberships(
	context.Context,
	string,
) (string, []string, error) {
	return "R_1", nil, nil
}

func (f *fakeCacheInner) CreateStarList(_ context.Context, input StarListInput) (StarList, error) {
	return StarList{Name: input.Name, ID: "UL_new"}, nil
}

func (f *fakeCacheInner) UpdateStarList(
	_ context.Context,
	input UpdateStarListInput,
) (StarList, error) {
	return StarList{Name: input.Name, ID: input.ID}, nil
}

func (f *fakeCacheInner) DeleteStarList(context.Context, string) error {
	return nil
}

func (f *fakeCacheInner) UpdateRepositoryLists(context.Context, string, []string) error {
	return nil
}

func (f *fakeCacheInner) AddStar(context.Context, string) error {
	return nil
}

func (f *fakeCacheInner) RemoveStar(context.Context, string) error {
	return nil
}

func TestCacheServiceHits(t *testing.T) {
	inner := &fakeCacheInner{
		lists: []StarList{
			{Name: "test", ID: "UL_1", URL: "https://github.com/stars/testuser/lists/test"},
		},
		repos: map[string][]Repository{
			"UL_1": {{NameWithOwner: "owner/repo"}},
		},
	}
	svc := newCacheService(inner)
	// Use a long TTL so cache doesn't expire during test.
	svc.ttl = 10 * time.Minute

	ctx := context.Background()

	// First call - should hit inner.
	lists1, err := svc.ListStarLists(ctx)
	if err != nil {
		t.Fatalf("first ListStarLists: %v", err)
	}
	if len(lists1) != 1 {
		t.Fatalf("first call got %d lists, want 1", len(lists1))
	}
	if inner.listCalls != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.listCalls)
	}

	// Second call - should be cached.
	lists2, err := svc.ListStarLists(ctx)
	if err != nil {
		t.Fatalf("second ListStarLists: %v", err)
	}
	if len(lists2) != 1 {
		t.Fatalf("second call got %d lists, want 1", len(lists2))
	}
	if inner.listCalls != 1 {
		t.Fatalf("inner calls after cache = %d, want 1 (should not increase)", inner.listCalls)
	}

	// Repos: first call hits inner.
	repos1, err := svc.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("first ListRepositories: %v", err)
	}
	if len(repos1) != 1 {
		t.Fatalf("first repos call got %d, want 1", len(repos1))
	}
	if inner.reposCalls["UL_1"] != 1 {
		t.Fatalf("inner repos calls = %d, want 1", inner.reposCalls["UL_1"])
	}

	// Second repos call - should be cached.
	repos2, err := svc.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("second ListRepositories: %v", err)
	}
	if len(repos2) != 1 {
		t.Fatalf("second repos call got %d, want 1", len(repos2))
	}
	if inner.reposCalls["UL_1"] != 1 {
		t.Fatalf(
			"inner repos calls after cache = %d, want 1 (should not increase)",
			inner.reposCalls["UL_1"],
		)
	}
}

func TestCacheServiceRemoveStarInvalidatesListRepositories(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	inner := &fakeCacheInner{
		repos: map[string][]Repository{
			"UL_1": {{ID: "R_1", NameWithOwner: "owner/stale"}},
		},
		starred: []Repository{{ID: "R_1", NameWithOwner: "owner/stale"}},
	}
	svc := newCacheService(inner)
	svc.ttl = 10 * time.Minute

	if _, err := svc.ListRepositories(ctx, "UL_1"); err != nil {
		t.Fatalf("first ListRepositories: %v", err)
	}
	if _, err := svc.ListStarredRepositories(ctx); err != nil {
		t.Fatalf("first ListStarredRepositories: %v", err)
	}
	inner.repos["UL_1"] = []Repository{{ID: "R_2", NameWithOwner: "owner/fresh"}}
	inner.starred = []Repository{{ID: "R_2", NameWithOwner: "owner/fresh"}}

	if err := svc.RemoveStar(ctx, "R_1"); err != nil {
		t.Fatalf("RemoveStar: %v", err)
	}
	repos, err := svc.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("second ListRepositories: %v", err)
	}
	if len(repos) != 1 || repos[0].NameWithOwner != "owner/fresh" {
		t.Fatalf("ListRepositories after RemoveStar = %+v, want owner/fresh", repos)
	}
	if inner.reposCalls["UL_1"] != 2 {
		t.Fatalf("repos calls = %d, want 2", inner.reposCalls["UL_1"])
	}
	if _, err := svc.ListStarredRepositories(ctx); err != nil {
		t.Fatalf("second ListStarredRepositories: %v", err)
	}
	if inner.starredCalls != 2 {
		t.Fatalf("starred calls = %d, want 2", inner.starredCalls)
	}
}

func TestCacheServiceMisses(t *testing.T) {
	inner := &fakeCacheInner{
		lists: []StarList{
			{Name: "test", ID: "UL_1", URL: "https://github.com/stars/testuser/lists/test"},
		},
	}
	svc := newCacheService(inner)
	svc.ttl = 1 * time.Millisecond

	ctx := context.Background()

	// First call fills cache.
	_, err := svc.ListStarLists(ctx)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Wait for TTL to expire.
	time.Sleep(2 * time.Millisecond)

	// Second call should re-fetch.
	_, err = svc.ListStarLists(ctx)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if inner.listCalls != 2 {
		t.Fatalf("inner calls after expired TTL = %d, want 2", inner.listCalls)
	}
}

func TestCacheServicePerListID(t *testing.T) {
	inner := &fakeCacheInner{
		repos: map[string][]Repository{
			"UL_1": {{NameWithOwner: "owner/one"}},
			"UL_2": {{NameWithOwner: "owner/two"}},
		},
	}
	svc := newCacheService(inner)
	svc.ttl = 10 * time.Minute

	ctx := context.Background()

	// Fetch UL_1.
	_, err := svc.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("UL_1 fetch: %v", err)
	}

	// Fetch UL_2 - should miss cache (different key).
	_, err = svc.ListRepositories(ctx, "UL_2")
	if err != nil {
		t.Fatalf("UL_2 fetch: %v", err)
	}

	if inner.reposCalls["UL_1"] != 1 {
		t.Fatalf("UL_1 calls = %d, want 1", inner.reposCalls["UL_1"])
	}
	if inner.reposCalls["UL_2"] != 1 {
		t.Fatalf("UL_2 calls = %d, want 1", inner.reposCalls["UL_2"])
	}

	// Fetch UL_1 again - should be cached.
	_, err = svc.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("UL_1 re-fetch: %v", err)
	}
	if inner.reposCalls["UL_1"] != 1 {
		t.Fatalf(
			"UL_1 calls after cache = %d, want 1 (should not increase)",
			inner.reposCalls["UL_1"],
		)
	}
}

func TestCacheServiceListStarredRepositoriesCaches(t *testing.T) {
	inner := &fakeCacheInner{
		starred: []Repository{{NameWithOwner: "owner/starred"}},
	}
	svc := newCacheService(inner)
	svc.ttl = 10 * time.Minute

	ctx := context.Background()

	// First call hits inner.
	repos1, err := svc.ListStarredRepositories(ctx)
	if err != nil {
		t.Fatalf("first ListStarredRepositories: %v", err)
	}
	if len(repos1) != 1 {
		t.Fatalf("first call got %d repos, want 1", len(repos1))
	}
	if inner.starredCalls != 1 {
		t.Fatalf("inner starred calls = %d, want 1", inner.starredCalls)
	}

	// Second call is cached.
	repos2, err := svc.ListStarredRepositories(ctx)
	if err != nil {
		t.Fatalf("second ListStarredRepositories: %v", err)
	}
	if len(repos2) != 1 {
		t.Fatalf("second call got %d repos, want 1", len(repos2))
	}
	if inner.starredCalls != 1 {
		t.Fatalf(
			"inner starred calls after cache = %d, want 1 (should not increase)",
			inner.starredCalls,
		)
	}
}

func TestCacheServiceErrorPropagation(t *testing.T) {
	inner := &fakeCacheInner{
		listErr: errors.New("network error"),
	}
	svc := newCacheService(inner)

	_, err := svc.ListStarLists(context.Background())
	if err == nil || err.Error() != "network error" {
		t.Fatalf("got error %v, want 'network error'", err)
	}
	// Error should NOT be cached.
	_, err = svc.ListStarLists(context.Background())
	if err == nil || err.Error() != "network error" {
		t.Fatalf("second call got error %v, want 'network error'", err)
	}
	if inner.listCalls != 2 {
		t.Fatalf(
			"inner calls after error = %d, want 2 (errors should not be cached)",
			inner.listCalls,
		)
	}
}

func TestCacheServiceGetRepositoryCaches(t *testing.T) {
	inner := &fakeCacheInner{
		gotRepo: Repository{ID: "R_1", NameWithOwner: "owner/repo", Language: "Go"},
	}
	svc := newCacheService(inner)
	svc.ttl = 10 * time.Minute

	ctx := context.Background()

	repo1, err := svc.GetRepository(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("first GetRepository: %v", err)
	}
	if repo1.Language != "Go" {
		t.Fatalf("got language %q, want Go", repo1.Language)
	}
	if inner.getRepoCalls != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.getRepoCalls)
	}

	repo2, err := svc.GetRepository(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("second GetRepository: %v", err)
	}
	if repo2.Language != "Go" {
		t.Fatalf("cached: got language %q, want Go", repo2.Language)
	}
	if inner.getRepoCalls != 1 {
		t.Fatalf("inner calls after cache = %d, want 1 (should not increase)", inner.getRepoCalls)
	}

	_, err = svc.GetRepository(ctx, "owner/other")
	if err != nil {
		t.Fatalf("different repo: %v", err)
	}
	if inner.getRepoCalls != 2 {
		t.Fatalf(
			"inner calls for different key = %d, want 2 (different key must miss cache)",
			inner.getRepoCalls,
		)
	}
}

func TestCacheServiceGetRepositoryExpiry(t *testing.T) {
	inner := &fakeCacheInner{
		gotRepo: Repository{ID: "R_1", NameWithOwner: "owner/repo"},
	}
	svc := newCacheService(inner)
	svc.ttl = 1 * time.Millisecond

	ctx := context.Background()

	_, err := svc.GetRepository(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	_, err = svc.GetRepository(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if inner.getRepoCalls != 2 {
		t.Fatalf("inner calls after expiry = %d, want 2", inner.getRepoCalls)
	}
}

func TestCacheServiceGetRepositoryInvalidatedByUpdateRepositoryLists(t *testing.T) {
	inner := &fakeCacheInner{
		gotRepo: Repository{ID: "R_1", NameWithOwner: "owner/repo"},
	}
	svc := newCacheService(inner)
	svc.ttl = 10 * time.Minute

	ctx := context.Background()

	_, err := svc.GetRepository(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if inner.getRepoCalls != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.getRepoCalls)
	}

	_ = svc.UpdateRepositoryLists(ctx, "R_1", nil)

	_, err = svc.GetRepository(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("post-invalidate call: %v", err)
	}
	if inner.getRepoCalls != 2 {
		t.Fatalf("inner calls after invalidation = %d, want 2", inner.getRepoCalls)
	}
}
