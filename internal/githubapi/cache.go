package githubapi

import (
	"context"
	"sync"
	"time"
)

const defaultCacheTTL = 5 * time.Minute

type cacheEntry[T any] struct {
	data   []T
	expiry time.Time
}

type cacheService struct {
	inner Service

	mu           sync.RWMutex
	listsEntry   *cacheEntry[StarList]
	reposEntry   map[string]*cacheEntry[Repository]
	starredEntry *cacheEntry[Repository]
	ttl          time.Duration
}

func newCacheService(inner Service) *cacheService {
	return &cacheService{
		inner:      inner,
		reposEntry: make(map[string]*cacheEntry[Repository]),
		ttl:        defaultCacheTTL,
	}
}

// NewCacheService wraps a Service with an in-memory TTL cache.
func NewCacheService(inner Service) Service {
	return newCacheService(inner)
}

func (s *cacheService) ListStarLists(ctx context.Context) ([]StarList, error) {
	s.mu.RLock()
	if s.listsEntry != nil && time.Now().Before(s.listsEntry.expiry) {
		data := s.listsEntry.data
		s.mu.RUnlock()
		return data, nil
	}
	s.mu.RUnlock()

	lists, err := s.inner.ListStarLists(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.listsEntry = &cacheEntry[StarList]{
		data:   lists,
		expiry: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return lists, nil
}

func (s *cacheService) ListRepositories(ctx context.Context, listID string) ([]Repository, error) {
	s.mu.RLock()
	if entry, ok := s.reposEntry[listID]; ok && time.Now().Before(entry.expiry) {
		data := entry.data
		s.mu.RUnlock()
		return data, nil
	}
	s.mu.RUnlock()

	repos, err := s.inner.ListRepositories(ctx, listID)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.reposEntry[listID] = &cacheEntry[Repository]{
		data:   repos,
		expiry: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return repos, nil
}

func (s *cacheService) ListStarredRepositories(ctx context.Context) ([]Repository, error) {
	s.mu.RLock()
	if s.starredEntry != nil && time.Now().Before(s.starredEntry.expiry) {
		data := s.starredEntry.data
		s.mu.RUnlock()
		return data, nil
	}
	s.mu.RUnlock()

	repos, err := s.inner.ListStarredRepositories(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.starredEntry = &cacheEntry[Repository]{
		data:   repos,
		expiry: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return repos, nil
}
