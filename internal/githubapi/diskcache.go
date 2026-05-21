package githubapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	diskCacheVersion    = 1
	defaultDiskCacheTTL = 5 * time.Minute
	diskCacheDirName    = "gh-star-lists"
)

// DiskCacheOptions configures the disk-backed cache service.
type DiskCacheOptions struct {
	TTL  time.Duration
	Host string
}

type diskCacheEntry struct {
	Version int          `json:"version"`
	Expiry  time.Time    `json:"expiry"`
	Lists   []StarList   `json:"lists,omitempty"`
	Repos   []Repository `json:"repos,omitempty"`
	Repo    *Repository  `json:"repo,omitempty"`
}

type diskCacheService struct {
	inner    Service
	ttl      time.Duration
	host     string
	cacheDir string
	mu       sync.Mutex
}

// NewDiskCacheService wraps inner with an opt-in disk read cache.
// Cache entries use the host, method type, list ID, and withTopics in the
// cache key. Entries expire after TTL (default 5 min). Mutations and
// Invalidate() clear affected disk entries.
func NewDiskCacheService(inner Service, opts DiskCacheOptions) Service {
	ttl := defaultDiskCacheTTL
	if opts.TTL > 0 {
		ttl = opts.TTL
	}
	host := opts.Host
	if host == "" {
		host = "default"
	}
	cacheDir := ""
	if userCacheDir, err := os.UserCacheDir(); err == nil {
		cacheDir = filepath.Join(userCacheDir, diskCacheDirName)
	}
	return &diskCacheService{
		inner:    inner,
		ttl:      ttl,
		host:     host,
		cacheDir: cacheDir,
	}
}

func (s *diskCacheService) cachePath(key string) string {
	if s.cacheDir == "" {
		return ""
	}
	h := sha256.Sum256([]byte(key))
	return filepath.Join(s.cacheDir, hex.EncodeToString(h[:16]))
}

func (s *diskCacheService) canonicalKey(method string, parts ...string) string {
	key := fmt.Sprintf("v%d/%s/%s", diskCacheVersion, s.host, method)
	for _, p := range parts {
		key += "/" + p
	}
	return key
}

func (s *diskCacheService) readFromDisk(key string) *diskCacheEntry {
	path := s.cachePath(key)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entry diskCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	if entry.Version != diskCacheVersion {
		return nil
	}
	if time.Now().After(entry.Expiry) {
		return nil
	}
	return &entry
}

func (s *diskCacheService) writeToDisk(key string, entry *diskCacheEntry) {
	path := s.cachePath(key)
	if path == "" {
		return
	}
	entry.Version = diskCacheVersion
	entry.Expiry = time.Now().Add(s.ttl)
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmpPath, path)
}

// Read methods

func (s *diskCacheService) ListStarLists(
	ctx context.Context,
	options ...ListOptions,
) ([]StarList, error) {
	key := s.canonicalKey("lists")
	if entry := s.readFromDisk(key); entry != nil && entry.Lists != nil {
		return applyLimit(entry.Lists, limitFromOptions(options)), nil
	}
	lists, err := s.inner.ListStarLists(ctx, options...)
	if err != nil {
		return nil, err
	}
	s.writeToDisk(key, &diskCacheEntry{Lists: lists})
	return applyLimit(lists, limitFromOptions(options)), nil
}

func (s *diskCacheService) ListRepositories(
	ctx context.Context,
	listID string,
	options ...ListOptions,
) ([]Repository, error) {
	withTopics := withTopicsFromOptions(options)
	key := s.canonicalKey("repos", listID, fmt.Sprintf("topics:%t", withTopics))
	if entry := s.readFromDisk(key); entry != nil && entry.Repos != nil {
		return applyLimit(entry.Repos, limitFromOptions(options)), nil
	}
	repos, err := s.inner.ListRepositories(ctx, listID, options...)
	if err != nil {
		return nil, err
	}
	s.writeToDisk(key, &diskCacheEntry{Repos: repos})
	return applyLimit(repos, limitFromOptions(options)), nil
}

func (s *diskCacheService) ListStarredRepositories(
	ctx context.Context,
	options ...ListOptions,
) ([]Repository, error) {
	withTopics := withTopicsFromOptions(options)
	key := s.canonicalKey("starred", fmt.Sprintf("topics:%t", withTopics))
	if entry := s.readFromDisk(key); entry != nil && entry.Repos != nil {
		return applyLimit(entry.Repos, limitFromOptions(options)), nil
	}
	repos, err := s.inner.ListStarredRepositories(ctx, options...)
	if err != nil {
		return nil, err
	}
	s.writeToDisk(key, &diskCacheEntry{Repos: repos})
	return applyLimit(repos, limitFromOptions(options)), nil
}

func (s *diskCacheService) GetRepository(
	ctx context.Context,
	nameWithOwner string,
) (Repository, error) {
	key := s.canonicalKey("repo", nameWithOwner)
	if entry := s.readFromDisk(key); entry != nil && entry.Repo != nil {
		return *entry.Repo, nil
	}
	repo, err := s.inner.GetRepository(ctx, nameWithOwner)
	if err != nil {
		return Repository{}, err
	}
	s.writeToDisk(key, &diskCacheEntry{Repo: &repo})
	return repo, nil
}

// Pass-through methods (no disk caching)

func (s *diskCacheService) GetRepositoryMemberships(
	ctx context.Context,
	nameWithOwner string,
) (string, []string, error) {
	return s.inner.GetRepositoryMemberships(ctx, nameWithOwner)
}

// Mutation methods with disk cache invalidation

func (s *diskCacheService) CreateStarList(
	ctx context.Context,
	input StarListInput,
) (StarList, error) {
	list, err := s.inner.CreateStarList(ctx, input)
	if err != nil {
		return StarList{}, err
	}
	s.invalidateLists()
	return list, nil
}

func (s *diskCacheService) UpdateStarList(
	ctx context.Context,
	input UpdateStarListInput,
) (StarList, error) {
	list, err := s.inner.UpdateStarList(ctx, input)
	if err != nil {
		return StarList{}, err
	}
	s.invalidateLists()
	return list, nil
}

func (s *diskCacheService) DeleteStarList(ctx context.Context, listID string) error {
	if err := s.inner.DeleteStarList(ctx, listID); err != nil {
		return err
	}
	s.invalidateLists()
	s.invalidateRepos(listID)
	return nil
}

func (s *diskCacheService) UpdateRepositoryLists(
	ctx context.Context,
	repoID string,
	listIDs []string,
) error {
	if err := s.inner.UpdateRepositoryLists(ctx, repoID, listIDs); err != nil {
		return err
	}
	s.invalidateAll()
	return nil
}

func (s *diskCacheService) AddStar(ctx context.Context, repoID string) error {
	if err := s.inner.AddStar(ctx, repoID); err != nil {
		return err
	}
	s.invalidateStarred()
	return nil
}

func (s *diskCacheService) RemoveStar(ctx context.Context, repoID string) error {
	if err := s.inner.RemoveStar(ctx, repoID); err != nil {
		return err
	}
	s.invalidateStarred()
	return nil
}

// Invalidate removes all disk cache entries. Used by the TUI for manual refresh.
func (s *diskCacheService) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheDir == "" {
		return
	}
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			_ = os.Remove(filepath.Join(s.cacheDir, entry.Name()))
		}
	}
}

func (s *diskCacheService) invalidateLists() {
	s.removeFile(s.canonicalKey("lists"))
}

func (s *diskCacheService) invalidateRepos(listID string) {
	s.removeFile(s.canonicalKey("repos", listID, "topics:true"))
	s.removeFile(s.canonicalKey("repos", listID, "topics:false"))
}

func (s *diskCacheService) invalidateStarred() {
	s.removeFile(s.canonicalKey("starred", "topics:true"))
	s.removeFile(s.canonicalKey("starred", "topics:false"))
}

func (s *diskCacheService) invalidateAll() {
	s.Invalidate()
}

func (s *diskCacheService) removeFile(key string) {
	path := s.cachePath(key)
	if path == "" {
		return
	}
	_ = os.Remove(path)
}
