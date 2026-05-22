package githubapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func TestDiskCacheWarmStart(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inner1 := &fakeCacheInner{
		repos: map[string][]domain.Repository{
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
	waitForDiskCacheEntry(t, ds1, ds1.canonicalKey("repos", "UL_1", "topics:false"))

	// New service with different inner should return cached data on warm start.
	inner2 := &fakeCacheInner{
		repos: map[string][]domain.Repository{
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

func TestDiskCacheWarmStartPreservesRepositoryDetails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inner1 := &fakeCacheInner{
		repos: map[string][]domain.Repository{
			"UL_1": {{
				ID:                "R_1",
				NameWithOwner:     "owner/cached-repo",
				IsArchived:        true,
				License:           "MIT",
				Topics:            []string{"cli", "github"},
				NormNameWithOwner: "owner/cached-repo",
				NormDescription:   "cached description",
				NormLanguage:      "go",
			}},
		},
	}
	svc1 := NewDiskCacheService(inner1, DiskCacheOptions{TTL: 1 * time.Hour})
	ds1 := svc1.(*diskCacheService)
	ds1.cacheDir = t.TempDir()

	repos1, err := ds1.ListRepositories(ctx, "UL_1", domain.ListOptions{WithTopics: true})
	if err != nil {
		t.Fatalf("first ListRepositories: %v", err)
	}
	if len(repos1) != 1 || repos1[0].License != "MIT" || len(repos1[0].Topics) != 2 {
		t.Fatalf("first call got %+v, want detailed repo", repos1)
	}
	waitForDiskCacheEntry(t, ds1, ds1.canonicalKey("repos", "UL_1", "topics:true"))

	inner2 := &fakeCacheInner{
		repos: map[string][]domain.Repository{
			"UL_1": {{NameWithOwner: "owner/new-repo"}},
		},
	}
	svc2 := NewDiskCacheService(inner2, DiskCacheOptions{TTL: 1 * time.Hour})
	ds2 := svc2.(*diskCacheService)
	ds2.cacheDir = ds1.cacheDir

	repos2, err := ds2.ListRepositories(ctx, "UL_1", domain.ListOptions{WithTopics: true})
	if err != nil {
		t.Fatalf("warm start ListRepositories: %v", err)
	}
	if inner2.reposCalls["UL_1"] != 0 {
		t.Error("inner should not be called on disk cache hit")
	}
	if len(repos2) != 1 {
		t.Fatalf("warm start returned %d repos, want 1", len(repos2))
	}
	repo := repos2[0]
	if repo.ID != "R_1" ||
		repo.NameWithOwner != "owner/cached-repo" ||
		!repo.IsArchived ||
		repo.License != "MIT" ||
		repo.NormNameWithOwner != "owner/cached-repo" ||
		repo.NormDescription != "cached description" ||
		repo.NormLanguage != "go" {
		t.Fatalf("warm start repo = %+v, want cached details preserved", repo)
	}
	if got, want := repo.Topics, []string{"cli", "github"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("warm start topics = %v, want %v", got, want)
	}
}

func TestDiskCacheInvalidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inner := &fakeCacheInner{
		lists: []domain.StarList{
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
	waitForDiskCacheEntry(t, ds, ds.canonicalKey("lists"))

	// Mutation invalidates disk cache entry for lists.
	_, err = ds.CreateStarList(ctx, domain.StarListInput{Name: "new"})
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
	waitForDiskCacheIdle(t, ds)
}

func TestDiskCacheRemoveStarInvalidatesListRepositories(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	inner := &fakeCacheInner{
		repos: map[string][]domain.Repository{
			"UL_1": {{ID: "R_1", NameWithOwner: "owner/stale"}},
		},
	}
	svc := NewDiskCacheService(inner, DiskCacheOptions{TTL: 1 * time.Hour})
	ds := svc.(*diskCacheService)
	ds.cacheDir = t.TempDir()

	if _, err := ds.ListRepositories(ctx, "UL_1"); err != nil {
		t.Fatalf("first ListRepositories: %v", err)
	}
	waitForDiskCacheEntry(t, ds, ds.canonicalKey("repos", "UL_1", "topics:false"))
	inner.repos["UL_1"] = []domain.Repository{{ID: "R_2", NameWithOwner: "owner/fresh"}}

	if err := ds.RemoveStar(ctx, "R_1"); err != nil {
		t.Fatalf("RemoveStar: %v", err)
	}
	repos, err := ds.ListRepositories(ctx, "UL_1")
	if err != nil {
		t.Fatalf("second ListRepositories: %v", err)
	}
	if len(repos) != 1 || repos[0].NameWithOwner != "owner/fresh" {
		t.Fatalf("ListRepositories after RemoveStar = %+v, want owner/fresh", repos)
	}
	if inner.reposCalls["UL_1"] != 2 {
		t.Fatalf("repos calls = %d, want 2", inner.reposCalls["UL_1"])
	}
	waitForDiskCacheIdle(t, ds)
}

func TestDiskCacheTTLExpiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inner := &fakeCacheInner{
		repos: map[string][]domain.Repository{
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
	waitForDiskCacheIdle(t, ds)

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
	waitForDiskCacheIdle(t, ds)
}

func TestDiskCacheEviction(t *testing.T) {
	t.Parallel()
	ds := NewDiskCacheService(&fakeCacheInner{}, DiskCacheOptions{TTL: time.Hour}).(*diskCacheService)
	ds.cacheDir = t.TempDir()
	ds.maxFiles = 3
	base := time.Now().Add(-time.Hour)
	keys := make([]string, 5)

	for i := range keys {
		keys[i] = ds.canonicalKey("repos", fmt.Sprintf("UL_%d", i), "topics:false")
		ds.writeToDiskSync(keys[i], &diskCacheEntry{
			Repos: []domain.Repository{{NameWithOwner: fmt.Sprintf("owner/repo-%d", i)}},
		}, 0)
		path := ds.cachePath(keys[i])
		if err := os.Chtimes(
			path,
			base.Add(time.Duration(i)*time.Second),
			base.Add(time.Duration(i)*time.Second),
		); err != nil {
			t.Fatalf("Chtimes cache file %d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(ds.cacheDir)
	if err != nil {
		t.Fatalf("ReadDir cacheDir: %v", err)
	}
	if len(entries) != ds.maxFiles {
		t.Fatalf("cache file count = %d, want %d", len(entries), ds.maxFiles)
	}
	for _, key := range keys[:2] {
		if _, err := os.Stat(ds.cachePath(key)); !os.IsNotExist(err) {
			t.Fatalf("oldest cache file %q still exists or stat failed: %v", key, err)
		}
	}
	for _, key := range keys[2:] {
		if _, err := os.Stat(ds.cachePath(key)); err != nil {
			t.Fatalf("newer cache file %q missing: %v", key, err)
		}
	}
}

func TestConcurrentDiskCacheFillDeduplication(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	inner := &blockingRepoInner{
		repos:   []domain.Repository{{NameWithOwner: "owner/repo"}},
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	ds := NewDiskCacheService(inner, DiskCacheOptions{TTL: time.Hour}).(*diskCacheService)
	ds.cacheDir = t.TempDir()
	writeRelease := make(chan struct{})
	ds.beforeDiskWrite = func() {
		<-writeRelease
	}

	const callers = 8
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	start := make(chan struct{})
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			repos, err := ds.ListRepositories(ctx, "UL_1")
			if err != nil {
				errs <- err
				return
			}
			if len(repos) != 1 || repos[0].NameWithOwner != "owner/repo" {
				errs <- fmt.Errorf("repos = %+v, want owner/repo", repos)
			}
		}()
	}

	close(start)
	<-inner.entered
	waitForDiskCacheFill(t, ds, ds.canonicalKey("repos", "UL_1", "topics:false"))
	close(inner.release)
	wg.Wait()
	close(writeRelease)
	waitForDiskCacheIdle(t, ds)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls := atomic.LoadInt32(&inner.reposCalls); calls != 1 {
		t.Fatalf("inner ListRepositories calls = %d, want 1", calls)
	}
}

func TestDiskCacheWaitForFillRespectsContextCancellation(t *testing.T) {
	t.Parallel()
	inner := &blockingRepoInner{
		repos:   []domain.Repository{{NameWithOwner: "owner/repo"}},
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	ds := NewDiskCacheService(inner, DiskCacheOptions{TTL: time.Hour}).(*diskCacheService)
	ds.cacheDir = t.TempDir()

	firstDone := make(chan error, 1)
	go func() {
		_, err := ds.ListRepositories(context.Background(), "UL_1")
		firstDone <- err
	}()

	<-inner.entered
	waitForDiskCacheFill(t, ds, ds.canonicalKey("repos", "UL_1", "topics:false"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ds.ListRepositories(ctx, "UL_1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting ListRepositories error = %v, want context.Canceled", err)
	}
	if calls := atomic.LoadInt32(&inner.reposCalls); calls != 1 {
		t.Fatalf("inner ListRepositories calls = %d, want 1", calls)
	}

	close(inner.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first ListRepositories: %v", err)
	}
	waitForDiskCacheIdle(t, ds)
}

func TestConcurrentDiskCacheFillSharesResultBeforeDiskWriteCompletes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	inner := &blockingRepoInner{
		repos:   []domain.Repository{{NameWithOwner: "owner/repo"}},
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	ds := NewDiskCacheService(inner, DiskCacheOptions{TTL: time.Hour}).(*diskCacheService)
	ds.cacheDir = t.TempDir()
	writeStarted := make(chan struct{})
	writeRelease := make(chan struct{})
	var writeOnce sync.Once
	ds.beforeDiskWrite = func() {
		writeOnce.Do(func() { close(writeStarted) })
		<-writeRelease
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := ds.ListRepositories(ctx, "UL_1")
		firstDone <- err
	}()

	<-inner.entered
	waitForDiskCacheFill(t, ds, ds.canonicalKey("repos", "UL_1", "topics:false"))

	waiterDone := make(chan error, 1)
	go func() {
		repos, err := ds.ListRepositories(ctx, "UL_1")
		if err != nil {
			waiterDone <- err
			return
		}
		if len(repos) != 1 || repos[0].NameWithOwner != "owner/repo" {
			waiterDone <- fmt.Errorf("repos = %+v, want owner/repo", repos)
			return
		}
		waiterDone <- nil
	}()

	close(inner.release)
	<-writeStarted

	if err := <-firstDone; err != nil {
		t.Fatalf("first ListRepositories: %v", err)
	}
	select {
	case err := <-waiterDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("coalesced caller blocked while disk write was still running")
	}

	lateDone := make(chan error, 1)
	go func() {
		repos, err := ds.ListRepositories(ctx, "UL_1")
		if err != nil {
			lateDone <- err
			return
		}
		if len(repos) != 1 || repos[0].NameWithOwner != "owner/repo" {
			lateDone <- fmt.Errorf("repos = %+v, want owner/repo", repos)
			return
		}
		lateDone <- nil
	}()
	select {
	case err := <-lateDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("late coalesced caller blocked while disk write was still running")
	}

	close(writeRelease)
	waitForDiskCacheIdle(t, ds)
	if calls := atomic.LoadInt32(&inner.reposCalls); calls != 1 {
		t.Fatalf("inner ListRepositories calls = %d, want 1", calls)
	}
}

func waitForDiskCacheEntry(t *testing.T, ds *diskCacheService, key string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ds.readFromDisk(key) != nil {
			waitForDiskCacheIdle(t, ds)
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for disk cache entry %q", key)
}

func waitForDiskCacheIdle(t *testing.T, ds *diskCacheService) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ds.mu.Lock()
		idle := len(ds.fills) == 0
		ds.mu.Unlock()
		if idle {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for disk cache writes to finish")
}

func waitForDiskCacheFill(t *testing.T, ds *diskCacheService, key string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ds.mu.Lock()
		_, filling := ds.fills[key]
		ds.mu.Unlock()
		if filling {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for disk cache fill %q", key)
}

type blockingRepoInner struct {
	repos      []domain.Repository
	reposCalls int32
	entered    chan struct{}
	release    chan struct{}
	once       sync.Once
}

func (b *blockingRepoInner) ListStarLists(
	context.Context,
	...domain.ListOptions,
) ([]domain.StarList, error) {
	return nil, nil
}

func (b *blockingRepoInner) ListRepositories(
	context.Context,
	string,
	...domain.ListOptions,
) ([]domain.Repository, error) {
	atomic.AddInt32(&b.reposCalls, 1)
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return b.repos, nil
}

func (b *blockingRepoInner) ListStarredRepositories(
	context.Context,
	...domain.ListOptions,
) ([]domain.Repository, error) {
	return nil, nil
}

func (b *blockingRepoInner) GetRepository(context.Context, string) (domain.Repository, error) {
	return domain.Repository{}, nil
}

func (b *blockingRepoInner) GetRepositoryMemberships(
	context.Context,
	string,
) (string, []string, error) {
	return "", nil, nil
}

func (b *blockingRepoInner) CreateStarList(
	context.Context,
	domain.StarListInput,
) (domain.StarList, error) {
	return domain.StarList{}, nil
}

func (b *blockingRepoInner) UpdateStarList(
	context.Context,
	domain.UpdateStarListInput,
) (domain.StarList, error) {
	return domain.StarList{}, nil
}

func (b *blockingRepoInner) DeleteStarList(context.Context, string) error {
	return nil
}

func (b *blockingRepoInner) UpdateRepositoryLists(context.Context, string, []string) error {
	return nil
}

func (b *blockingRepoInner) AddStar(context.Context, string) error {
	return nil
}

func (b *blockingRepoInner) RemoveStar(context.Context, string) error {
	return nil
}
