package command

import (
	"context"
	"maps"
	"strings"
	"sync"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"golang.org/x/sync/errgroup"
)

type repoMembership struct {
	repoID string
	lists  map[string]struct{}
}

type membershipIndex struct {
	byRepo      map[string]repoMembership
	reposByList map[string][]githubapi.Repository
}

func loadMembershipIndex(
	ctx context.Context,
	service githubapi.Service,
	lists []githubapi.StarList,
) (membershipIndex, error) {
	index := membershipIndex{
		byRepo:      make(map[string]repoMembership),
		reposByList: make(map[string][]githubapi.Repository),
	}
	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(5)
	for _, list := range lists {
		list := list
		group.Go(func() error {
			repos, err := service.ListRepositories(groupCtx, list.ID)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			index.reposByList[list.ID] = repos
			for _, repo := range repos {
				key := strings.ToLower(repo.NameWithOwner)
				entry := index.byRepo[key]
				if entry.lists == nil {
					entry.lists = make(map[string]struct{})
				}
				entry.lists[list.ID] = struct{}{}
				if repo.ID != "" {
					entry.repoID = repo.ID
				}
				index.byRepo[key] = entry
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return membershipIndex{}, err
	}
	return index, nil
}

func (i membershipIndex) repositoryMemberships(
	ctx context.Context,
	service githubapi.Service,
	repoName string,
) (string, map[string]struct{}, error) {
	entry := i.byRepo[strings.ToLower(repoName)]
	memberships := maps.Clone(entry.lists)
	if memberships == nil {
		memberships = make(map[string]struct{})
	}
	repoID := entry.repoID
	if repoID == "" {
		repo, err := service.GetRepository(ctx, repoName)
		if err != nil {
			return "", nil, err
		}
		repoID = repo.ID
	}
	return repoID, memberships, nil
}
