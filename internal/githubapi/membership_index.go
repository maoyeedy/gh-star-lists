package githubapi

import (
	"context"
	"maps"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

type repositoryMembership struct {
	repoID string
	lists  map[string]struct{}
}

// MembershipIndex is a read-side index of repository Star List memberships
// built by scanning lists. GitHub's GraphQL schema does not expose direct
// repo-to-Star-Lists reads, so callers that need many memberships should build
// this once and reuse it.
type MembershipIndex struct {
	byRepo      map[string]repositoryMembership
	reposByList map[string][]Repository
}

// LoadMembershipIndex scans all provided lists with bounded concurrency and
// returns an index keyed by lower-cased nameWithOwner.
func LoadMembershipIndex(
	ctx context.Context,
	service Service,
	lists []StarList,
) (MembershipIndex, error) {
	index := MembershipIndex{
		byRepo:      make(map[string]repositoryMembership),
		reposByList: make(map[string][]Repository),
	}
	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(5)
	for _, list := range lists {
		list := list
		group.Go(func() error {
			if err := groupCtx.Err(); err != nil {
				return err
			}
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
		return MembershipIndex{}, err
	}
	return index, nil
}

// RepositoryMemberships returns the repository ID and current Star List
// membership set. If the repository was not found while scanning lists, it
// falls back to GetRepository to resolve the ID.
func (i MembershipIndex) RepositoryMemberships(
	ctx context.Context,
	service Service,
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

// ContainsRepository reports whether repoName was found in any indexed list.
func (i MembershipIndex) ContainsRepository(repoName string) bool {
	_, ok := i.byRepo[strings.ToLower(repoName)]
	return ok
}

// RepositoriesForList returns the indexed repositories for listID.
func (i MembershipIndex) RepositoriesForList(listID string) []Repository {
	return i.reposByList[listID]
}
