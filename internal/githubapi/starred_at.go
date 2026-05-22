package githubapi

import (
	"context"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

// WithStarredAt returns repositories with StarredAt copied from the viewer's
// starred repositories when GitHub exposes a matching starred edge.
func WithStarredAt(
	ctx context.Context,
	service Service,
	repos []domain.Repository,
) ([]domain.Repository, error) {
	if len(repos) == 0 || allHaveStarredAt(repos) {
		return repos, nil
	}
	starred, err := service.ListStarredRepositories(ctx)
	if err != nil {
		return nil, err
	}
	return MergeStarredAt(repos, starred), nil
}

func MergeStarredAt(repos, starred []domain.Repository) []domain.Repository {
	if len(repos) == 0 || len(starred) == 0 {
		return repos
	}
	byID := make(map[string]string, len(starred))
	byName := make(map[string]string, len(starred))
	for _, repo := range starred {
		if repo.StarredAt == "" {
			continue
		}
		if repo.ID != "" {
			byID[repo.ID] = repo.StarredAt
		}
		if repo.NameWithOwner != "" {
			byName[strings.ToLower(repo.NameWithOwner)] = repo.StarredAt
		}
	}
	if len(byID) == 0 && len(byName) == 0 {
		return repos
	}
	out := make([]domain.Repository, len(repos))
	copy(out, repos)
	for i := range out {
		if out[i].StarredAt != "" {
			continue
		}
		if out[i].ID != "" {
			if starredAt := byID[out[i].ID]; starredAt != "" {
				out[i].StarredAt = starredAt
				continue
			}
		}
		if out[i].NameWithOwner != "" {
			out[i].StarredAt = byName[strings.ToLower(out[i].NameWithOwner)]
		}
	}
	return out
}

func allHaveStarredAt(repos []domain.Repository) bool {
	for _, repo := range repos {
		if repo.StarredAt == "" {
			return false
		}
	}
	return true
}
