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

func NewCacheService(inner Service) Service {
	return newCacheService(inner)
}

func (s *cacheService) ListStarLists(ctx context.Context, options ...ListOptions) ([]StarList, error) {
	limit := limitFromOptions(options)
	s.mu.RLock()
	if s.listsEntry != nil && time.Now().Before(s.listsEntry.expiry) {
		data := s.listsEntry.data
		s.mu.RUnlock()
		return applyLimit(data, limit), nil
	}
	s.mu.RUnlock()

	lists, err := s.inner.ListStarLists(ctx, options...)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.listsEntry = &cacheEntry[StarList]{
		data:   lists,
		expiry: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return applyLimit(lists, limit), nil
}

func (s *cacheService) ListRepositories(
	ctx context.Context,
	listID string,
	options ...ListOptions,
) ([]Repository, error) {
	limit := limitFromOptions(options)
	s.mu.RLock()
	if entry, ok := s.reposEntry[listID]; ok && time.Now().Before(entry.expiry) {
		data := entry.data
		s.mu.RUnlock()
		return applyLimit(data, limit), nil
	}
	s.mu.RUnlock()

	repos, err := s.inner.ListRepositories(ctx, listID, options...)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.reposEntry[listID] = &cacheEntry[Repository]{
		data:   repos,
		expiry: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return applyLimit(repos, limit), nil
}

func (s *cacheService) ListStarredRepositories(
	ctx context.Context,
	options ...ListOptions,
) ([]Repository, error) {
	limit := limitFromOptions(options)
	s.mu.RLock()
	if s.starredEntry != nil && time.Now().Before(s.starredEntry.expiry) {
		data := s.starredEntry.data
		s.mu.RUnlock()
		return applyLimit(data, limit), nil
	}
	s.mu.RUnlock()

	repos, err := s.inner.ListStarredRepositories(ctx, options...)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.starredEntry = &cacheEntry[Repository]{
		data:   repos,
		expiry: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return applyLimit(repos, limit), nil
}

func (s *cacheService) GetRepository(ctx context.Context, nameWithOwner string) (Repository, error) {
	return s.inner.GetRepository(ctx, nameWithOwner)
}

func (s *cacheService) CreateStarList(ctx context.Context, input StarListInput) (StarList, error) {
	list, err := s.inner.CreateStarList(ctx, input)
	if err != nil {
		return StarList{}, err
	}
	s.invalidateLists()
	return list, nil
}

func (s *cacheService) UpdateStarList(ctx context.Context, input UpdateStarListInput) (StarList, error) {
	list, err := s.inner.UpdateStarList(ctx, input)
	if err != nil {
		return StarList{}, err
	}
	s.invalidateLists()
	return list, nil
}

func (s *cacheService) DeleteStarList(ctx context.Context, listID string) error {
	if err := s.inner.DeleteStarList(ctx, listID); err != nil {
		return err
	}
	s.mu.Lock()
	s.listsEntry = nil
	delete(s.reposEntry, listID)
	s.mu.Unlock()
	return nil
}

func (s *cacheService) UpdateRepositoryLists(
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

func (s *cacheService) AddStar(ctx context.Context, repoID string) error {
	if err := s.inner.AddStar(ctx, repoID); err != nil {
		return err
	}
	s.invalidateStarred()
	return nil
}

func (s *cacheService) RemoveStar(ctx context.Context, repoID string) error {
	if err := s.inner.RemoveStar(ctx, repoID); err != nil {
		return err
	}
	s.invalidateStarred()
	return nil
}

func (s *cacheService) invalidateLists() {
	s.mu.Lock()
	s.listsEntry = nil
	s.mu.Unlock()
}

func (s *cacheService) invalidateStarred() {
	s.mu.Lock()
	s.starredEntry = nil
	s.mu.Unlock()
}

func (s *cacheService) invalidateAll() {
	s.mu.Lock()
	s.listsEntry = nil
	s.reposEntry = make(map[string]*cacheEntry[Repository])
	s.starredEntry = nil
	s.mu.Unlock()
}

func applyLimit[T any](data []T, limit int) []T {
	if limit > 0 && len(data) > limit {
		return data[:limit]
	}
	return data
}
