package githubapi

import (
	"context"
	"testing"
	"time"
)

func TestDiskCacheWarmStart(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inner1 := &fakeCacheInner{
		repos: map[string][]Repository{
			"UL_1": {{NameWithOwner: "owner/cached-repo"}},
		},
	}
	svc1 := NewDiskCacheService(inner1, DiskCacheOptions{TTL: 1 * time.Hour})
	ds1 := svc1.(*diskCacheService)
	ds1.cacheDir = t.TempDir()

	// First call populates cache.
	repos1, err := ds1.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("first ListRepositories: %v", err)
	}
	if len(repos1) != 1 || repos1[0].NameWithOwner != "owner/cached-repo" {
		t.Fatalf("first call got %+v, want owner/cached-repo", repos1)
	}

	// New service with different inner should return cached data on warm start.
	inner2 := &fakeCacheInner{
		repos: map[string][]Repository{
			"UL_1": {{NameWithOwner: "owner/new-repo"}},
		},
	}
	svc2 := NewDiskCacheService(inner2, DiskCacheOptions{TTL: 1 * time.Hour})
	ds2 := svc2.(*diskCacheService)
	ds2.cacheDir = ds1.cacheDir // same directory = warm start

	repos2, err := ds2.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("warm start ListRepositories: %v", err)
	}
	if len(repos2) != 1 || repos2[0].NameWithOwner != "owner/cached-repo" {
		t.Fatalf("warm start returned %+v, want owner/cached-repo (disk cache)", repos2)
	}
	if inner2.reposCalls["UL_1"] != 0 {
		t.Error("inner should not be called on disk cache hit")
	}
}

func TestDiskCacheInvalidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inner := &fakeCacheInner{
		lists: []StarList{
			{Name: "original", ID: "UL_1"},
		},
	}
	svc := NewDiskCacheService(inner, DiskCacheOptions{TTL: 1 * time.Hour})
	ds := svc.(*diskCacheService)
	ds.cacheDir = t.TempDir()

	// Populate disk cache.
	_, err := ds.ListStarLists(ctx)
	if err != nil {
		t.Fatalf("first ListStarLists: %v", err)
	}
	if inner.listCalls != 1 {
		t.Fatalf("inner calls after first = %d, want 1", inner.listCalls)
	}

	// Mutation invalidates disk cache entry for lists.
	_, err = ds.CreateStarList(ctx, StarListInput{Name: "new"})
	if err != nil {
		t.Fatalf("CreateStarList: %v", err)
	}

	// After invalidation, next read should call inner again.
	lists, err := ds.ListStarLists(ctx)
	if err != nil {
		t.Fatalf("second ListStarLists: %v", err)
	}
	if inner.listCalls != 2 {
		t.Fatalf("inner calls after invalidation = %d, want 2", inner.listCalls)
	}
	if len(lists) != 1 || lists[0].Name != "original" {
		t.Fatalf("lists = %+v, want original", lists)
	}
}

func TestDiskCacheTTLExpiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inner := &fakeCacheInner{
		repos: map[string][]Repository{
			"UL_1": {{NameWithOwner: "owner/cached-repo"}},
		},
	}
	svc := NewDiskCacheService(inner, DiskCacheOptions{TTL: 1 * time.Millisecond})
	ds := svc.(*diskCacheService)
	ds.cacheDir = t.TempDir()

	// First call populates disk cache.
	_, err := ds.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if inner.reposCalls["UL_1"] != 1 {
		t.Fatalf("inner calls after first = %d, want 1", inner.reposCalls["UL_1"])
	}

	// Wait for TTL to expire.
	time.Sleep(2 * time.Millisecond)

	// Second call should re-fetch from inner.
	_, err = ds.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if inner.reposCalls["UL_1"] != 2 {
		t.Fatalf("inner calls after expiry = %d, want 2", inner.reposCalls["UL_1"])
	}
}
