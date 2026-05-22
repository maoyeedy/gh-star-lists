package githubapi

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type diskCacheService struct {
	inner    Service
	ttl      time.Duration
	host     string
	cacheDir string
	mu       sync.Mutex
	fills    map[string]*diskCacheFill
	gen      int64
	maxFiles int

	beforeDiskWrite func()
}

// Read methods

func (s *diskCacheService) ListStarLists(
	ctx context.Context,
	options ...ListOptions,
) ([]StarList, error) {
	key := s.canonicalKey("lists")
	limit := limitFromOptions(options)
	for {
		if entry := s.readFromDisk(key); entry != nil && entry.Lists != nil {
			return applyLimit(entry.Lists, limit), nil
		}
		fill, gen, wait := s.startFill(key)
		if !wait {
			lists, err := s.inner.ListStarLists(ctx, options...)
			if err != nil {
				s.finishFill(key, fill, nil, err)
				return nil, err
			}
			entry := &diskCacheEntry{Lists: lists}
			s.finishFill(key, fill, entry, nil)
			s.writeToDisk(key, entry, fill, gen)
			return applyLimit(lists, limit), nil
		}
		entry, err := s.waitForFill(ctx, fill)
		if err != nil {
			return nil, err
		}
		if entry != nil && entry.Lists != nil {
			return applyLimit(entry.Lists, limit), nil
		}
	}
}

func (s *diskCacheService) ListRepositories(
	ctx context.Context,
	listID string,
	options ...ListOptions,
) ([]Repository, error) {
	withTopics := withTopicsFromOptions(options)
	key := s.canonicalKey("repos", listID, fmt.Sprintf("topics:%t", withTopics))
	limit := limitFromOptions(options)
	for {
		if entry := s.readFromDisk(key); entry != nil && entry.Repos != nil {
			return applyLimit(entry.Repos, limit), nil
		}
		fill, gen, wait := s.startFill(key)
		if !wait {
			repos, err := s.inner.ListRepositories(ctx, listID, options...)
			if err != nil {
				s.finishFill(key, fill, nil, err)
				return nil, err
			}
			entry := &diskCacheEntry{Repos: repos}
			s.finishFill(key, fill, entry, nil)
			s.writeToDisk(key, entry, fill, gen)
			return applyLimit(repos, limit), nil
		}
		entry, err := s.waitForFill(ctx, fill)
		if err != nil {
			return nil, err
		}
		if entry != nil && entry.Repos != nil {
			return applyLimit(entry.Repos, limit), nil
		}
	}
}

func (s *diskCacheService) ListStarredRepositories(
	ctx context.Context,
	options ...ListOptions,
) ([]Repository, error) {
	withTopics := withTopicsFromOptions(options)
	key := s.canonicalKey("starred", fmt.Sprintf("topics:%t", withTopics))
	limit := limitFromOptions(options)
	for {
		if entry := s.readFromDisk(key); entry != nil && entry.Repos != nil {
			return applyLimit(entry.Repos, limit), nil
		}
		fill, gen, wait := s.startFill(key)
		if !wait {
			repos, err := s.inner.ListStarredRepositories(ctx, options...)
			if err != nil {
				s.finishFill(key, fill, nil, err)
				return nil, err
			}
			entry := &diskCacheEntry{Repos: repos}
			s.finishFill(key, fill, entry, nil)
			s.writeToDisk(key, entry, fill, gen)
			return applyLimit(repos, limit), nil
		}
		entry, err := s.waitForFill(ctx, fill)
		if err != nil {
			return nil, err
		}
		if entry != nil && entry.Repos != nil {
			return applyLimit(entry.Repos, limit), nil
		}
	}
}

func (s *diskCacheService) GetRepository(
	ctx context.Context,
	nameWithOwner string,
) (Repository, error) {
	key := s.canonicalKey("repo", nameWithOwner)
	for {
		if entry := s.readFromDisk(key); entry != nil && entry.Repo != nil {
			return *entry.Repo, nil
		}
		fill, gen, wait := s.startFill(key)
		if !wait {
			repo, err := s.inner.GetRepository(ctx, nameWithOwner)
			if err != nil {
				s.finishFill(key, fill, nil, err)
				return Repository{}, err
			}
			entry := &diskCacheEntry{Repo: &repo}
			s.finishFill(key, fill, entry, nil)
			s.writeToDisk(key, entry, fill, gen)
			return repo, nil
		}
		entry, err := s.waitForFill(ctx, fill)
		if err != nil {
			return Repository{}, err
		}
		if entry != nil && entry.Repo != nil {
			return *entry.Repo, nil
		}
	}
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
