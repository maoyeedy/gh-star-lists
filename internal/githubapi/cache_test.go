package githubapi

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeCacheInner struct {
	listCalls  int
	reposCalls map[string]int
	lists      []StarList
	repos      map[string][]Repository
	listErr    error
	reposErr   error
}

func (f *fakeCacheInner) ListStarLists(context.Context) ([]StarList, error) {
	f.listCalls++
	return f.lists, f.listErr
}

func (f *fakeCacheInner) ListRepositories(_ context.Context, listID string) ([]Repository, error) {
	if f.reposCalls == nil {
		f.reposCalls = make(map[string]int)
	}
	f.reposCalls[listID]++
	if f.reposErr != nil {
		return nil, f.reposErr
	}
	return f.repos[listID], nil
}

func TestCacheServiceHits(t *testing.T) {
	inner := &fakeCacheInner{
		lists: []StarList{{Name: "test", ID: "UL_1", URL: "https://github.com/stars/testuser/lists/test"}},
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

func TestCacheServiceMisses(t *testing.T) {
	inner := &fakeCacheInner{
		lists: []StarList{{Name: "test", ID: "UL_1", URL: "https://github.com/stars/testuser/lists/test"}},
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
