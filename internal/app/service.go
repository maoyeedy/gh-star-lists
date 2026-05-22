package app

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"github.com/maoyeedy/gh-star-lists/internal/search"
	"golang.org/x/sync/errgroup"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type StarListService struct {
	svc   githubapi.Service
	clock Clock
}

func NewStarListService(svc githubapi.Service) *StarListService {
	return &StarListService{svc: svc, clock: realClock{}}
}

func (s *StarListService) Service() githubapi.Service {
	return s.svc
}

func (s *StarListService) ListLists(
	ctx context.Context,
	opts ListListsOptions,
) ([]domain.StarList, error) {
	var listOpts []domain.ListOptions
	if opts.Limit > 0 && len(opts.Filters) == 0 && len(opts.SortKeys) == 0 {
		listOpts = append(listOpts, domain.ListOptions{Limit: opts.Limit})
	}
	lists, err := s.svc.ListStarLists(ctx, listOpts...)
	if err != nil {
		return nil, err
	}
	lists = filterStarLists(lists, opts.Filters)
	SortStarLists(lists, opts.SortKeys, opts.SortTerms, opts.SortDesc)
	if opts.Limit > 0 && len(lists) > opts.Limit {
		lists = lists[:opts.Limit]
	}
	return lists, nil
}

func (s *StarListService) ListRepos(
	ctx context.Context,
	listID string,
	opts ListReposOptions,
) ([]domain.Repository, error) {
	var repos []domain.Repository
	var err error

	switch {
	case opts.Unlisted:
		repos, err = s.listUnlistedRepos(ctx)
	case opts.All:
		listOpts := domain.ListOptions{WithTopics: opts.Topics}
		if opts.Limit > 0 && len(opts.Filters) == 0 && opts.Search == "" &&
			len(opts.SortKeys) == 0 {
			listOpts.Limit = opts.Limit
		}
		repos, err = s.svc.ListStarredRepositories(ctx, listOpts)
	default:
		resolvedID, err2 := s.resolveListID(ctx, listID)
		if err2 != nil {
			return nil, err2
		}
		listOpts := domain.ListOptions{WithTopics: opts.Topics}
		if opts.Limit > 0 && len(opts.Filters) == 0 && opts.Search == "" &&
			len(opts.SortKeys) == 0 {
			listOpts.Limit = opts.Limit
		}
		repos, err = s.svc.ListRepositories(ctx, resolvedID, listOpts)
		if err != nil {
			return nil, err
		}
		if sortNeedsStarredAt(opts) {
			repos, err = githubapi.WithStarredAt(ctx, s.svc, repos)
		}
	}
	if err != nil {
		return nil, err
	}

	repos = filterRepositories(repos, opts.Filters)
	repos = search.FilterRepositories(repos, opts.Search)
	SortRepositories(repos, opts.SortKeys, opts.SortTerms, opts.SortDesc)
	if opts.Limit > 0 && len(repos) > opts.Limit {
		repos = repos[:opts.Limit]
	}
	return repos, nil
}

func (s *StarListService) listUnlistedRepos(ctx context.Context) ([]domain.Repository, error) {
	lists, err := s.svc.ListStarLists(ctx)
	if err != nil {
		return nil, err
	}
	index, err := githubapi.LoadMembershipIndex(ctx, s.svc, lists)
	if err != nil {
		return nil, err
	}
	starred, err := s.svc.ListStarredRepositories(ctx)
	if err != nil {
		return nil, err
	}
	unlisted := make([]domain.Repository, 0, len(starred))
	for _, r := range starred {
		if !index.ContainsRepository(r.NameWithOwner) {
			unlisted = append(unlisted, r)
		}
	}
	return unlisted, nil
}

func (s *StarListService) resolveListID(ctx context.Context, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	lists, err := s.svc.ListStarLists(ctx)
	if err != nil {
		return "", err
	}
	for _, l := range lists {
		if strings.EqualFold(l.Name, raw) || l.ID == raw {
			return l.ID, nil
		}
	}
	return raw, nil
}

func sortNeedsStarredAt(opts ListReposOptions) bool {
	for _, key := range opts.SortKeys {
		if key == SortKeyStarred {
			return true
		}
	}
	return false
}

func (s *StarListService) CreateList(
	ctx context.Context,
	input domain.StarListInput,
) (domain.StarList, error) {
	return s.svc.CreateStarList(ctx, input)
}

func (s *StarListService) UpdateList(
	ctx context.Context,
	input domain.UpdateStarListInput,
) (domain.StarList, error) {
	return s.svc.UpdateStarList(ctx, input)
}

func (s *StarListService) DeleteList(ctx context.Context, listID string) error {
	return s.svc.DeleteStarList(ctx, listID)
}

func (s *StarListService) GetRepository(
	ctx context.Context,
	nameWithOwner string,
) (domain.Repository, error) {
	return s.svc.GetRepository(ctx, nameWithOwner)
}

func (s *StarListService) GetRepositoryMemberships(
	ctx context.Context,
	nameWithOwner string,
) (string, []string, error) {
	return s.svc.GetRepositoryMemberships(ctx, nameWithOwner)
}

func (s *StarListService) UpdateRepositoryLists(
	ctx context.Context,
	repoID string,
	listIDs []string,
) error {
	return s.svc.UpdateRepositoryLists(ctx, repoID, listIDs)
}

func (s *StarListService) RemoveStar(ctx context.Context, repoID string) error {
	return s.svc.RemoveStar(ctx, repoID)
}

func (s *StarListService) CopyList(
	ctx context.Context,
	fromListID, toListID string,
) (changed, total int, err error) {
	lists, err := s.svc.ListStarLists(ctx)
	if err != nil {
		return 0, 0, err
	}
	index, err := githubapi.LoadMembershipIndex(ctx, s.svc, lists)
	if err != nil {
		return 0, 0, err
	}
	repos := index.RepositoriesForList(fromListID)
	if len(repos) == 0 {
		// Source list not in user's own lists; fetch repos directly.
		directRepos, err := s.svc.ListRepositories(ctx, fromListID)
		if err != nil {
			return 0, 0, err
		}
		repos = directRepos
	}
	total = len(repos)

	var changedAtomic atomic.Int64
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(5)
	for _, repo := range repos {
		repo := repo
		group.Go(func() error {
			repoID, memberships, err := index.RepositoryMemberships(
				groupCtx,
				s.svc,
				repo.NameWithOwner,
			)
			if err != nil {
				return fmt.Errorf("%s: %w", repo.NameWithOwner, err)
			}
			if _, ok := memberships[toListID]; ok {
				return nil
			}
			memberships[toListID] = struct{}{}
			if err := s.svc.UpdateRepositoryLists(
				groupCtx,
				repoID,
				slices.Sorted(maps.Keys(memberships)),
			); err != nil {
				return fmt.Errorf("%s: %w", repo.NameWithOwner, err)
			}
			changedAtomic.Add(1)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return 0, 0, err
	}
	return int(changedAtomic.Load()), total, nil
}
