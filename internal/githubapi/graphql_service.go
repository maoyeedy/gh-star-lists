package githubapi

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/domain"

	"golang.org/x/sync/errgroup"
)

type graphQLService struct {
	client   graphQLDoer
	pageSize int
	host     string
}

func newGraphQLService(client graphQLDoer, pageSize int, host ...string) *graphQLService {
	service := &graphQLService{client: client, pageSize: pageSize, host: "github.com"}
	if len(host) > 0 && host[0] != "" {
		service.host = host[0]
	}
	return service
}

func (s *graphQLService) ListStarLists(
	ctx context.Context,
	options ...domain.ListOptions,
) ([]domain.StarList, error) {
	limit := limitFromOptions(options)

	nodes, err := Pager(ctx, s.pageSize, limit,
		func(endCursor any, first int) ([]starListNode, pageInfo, error) {
			var result listStarListsResponse
			variables := map[string]any{
				"endCursor": endCursor,
				"first":     first,
			}
			if err := s.client.DoWithContext(
				ctx,
				listStarListsQuery,
				variables,
				&result,
			); err != nil {
				return nil, pageInfo{}, fmt.Errorf(
					"GitHub GraphQL request failed: %w",
					normalizeError(err),
				)
			}
			pi := result.Viewer.Lists.PageInfo
			return result.Viewer.Lists.Nodes, pi, nil
		},
	)
	if err != nil {
		return nil, err
	}

	lists := make([]domain.StarList, 0, len(nodes))
	for _, node := range nodes {
		lists = append(lists, s.starListFromNode(node))
	}
	return lists, nil
}

func (s *graphQLService) ListRepositories(
	ctx context.Context,
	listID string,
	options ...domain.ListOptions,
) ([]domain.Repository, error) {
	limit := limitFromOptions(options)
	withTopics := withTopicsFromOptions(options)

	nodes, err := Pager(ctx, s.pageSize, limit,
		func(endCursor any, first int) ([]*repositoryItemNode, pageInfo, error) {
			var result listRepositoriesResponse
			variables := map[string]any{
				"id":         listID,
				"endCursor":  endCursor,
				"first":      first,
				"withTopics": withTopics,
			}
			if err := s.client.DoWithContext(
				ctx,
				listRepositoriesQuery,
				variables,
				&result,
			); err != nil {
				return nil, pageInfo{}, fmt.Errorf(
					"GitHub GraphQL request failed: %w",
					normalizeError(err),
				)
			}
			if result.Node == nil || result.Node.Typename != "UserList" ||
				result.Node.Items == nil {
				if endCursor == nil {
					return nil, pageInfo{}, ErrInaccessibleList
				}
				// Later page with nil Node/Items: treat as end of pagination.
				return nil, pageInfo{HasNextPage: false}, nil
			}

			valid := make([]*repositoryItemNode, 0, len(result.Node.Items.Nodes))
			for _, node := range result.Node.Items.Nodes {
				if node != nil && node.Typename == "Repository" && node.NameWithOwner != "" {
					valid = append(valid, node)
				}
			}

			pi := result.Node.Items.PageInfo
			return valid, pi, nil
		},
	)
	if err != nil {
		return nil, err
	}

	repositories := make([]domain.Repository, 0, len(nodes))
	for _, node := range nodes {
		repositories = append(repositories, domain.Repository{
			ID:                node.ID,
			NameWithOwner:     node.NameWithOwner,
			Description:       stringValue(node.Description),
			IsFork:            node.IsFork,
			IsArchived:        node.IsArchived,
			StargazerCount:    node.StargazerCount,
			PushedAt:          stringValue(node.PushedAt),
			URL:               node.URL,
			Language:          node.PrimaryLanguage.OrEmpty(),
			License:           node.LicenseInfo.OrEmpty(),
			Topics:            node.RepositoryTopics.Names(),
			NormNameWithOwner: strings.ToLower(strings.TrimSpace(node.NameWithOwner)),
			NormDescription: strings.ToLower(
				strings.TrimSpace(stringValue(node.Description)),
			),
			NormLanguage: strings.ToLower(
				strings.TrimSpace(node.PrimaryLanguage.OrEmpty()),
			),
		})
	}
	return repositories, nil
}

func (s *graphQLService) ListStarredRepositories(
	ctx context.Context,
	options ...domain.ListOptions,
) ([]domain.Repository, error) {
	limit := limitFromOptions(options)
	withTopics := withTopicsFromOptions(options)

	edges, err := Pager(ctx, s.pageSize, limit,
		func(endCursor any, first int) ([]starredRepositoryEdge, pageInfo, error) {
			var result listStarredRepositoriesResponse
			variables := map[string]any{
				"endCursor":  endCursor,
				"first":      first,
				"withTopics": withTopics,
			}
			if err := s.client.DoWithContext(
				ctx,
				listStarredRepositoriesQuery,
				variables,
				&result,
			); err != nil {
				return nil, pageInfo{}, fmt.Errorf(
					"GitHub GraphQL request failed: %w",
					normalizeError(err),
				)
			}

			pi := result.Viewer.StarredRepositories.PageInfo
			return result.Viewer.StarredRepositories.Edges, pi, nil
		},
	)
	if err != nil {
		return nil, err
	}

	repositories := make([]domain.Repository, 0, len(edges))
	for _, edge := range edges {
		repositories = append(repositories, domain.Repository{
			ID:                edge.Node.ID,
			NameWithOwner:     edge.Node.NameWithOwner,
			Description:       stringValue(edge.Node.Description),
			IsFork:            edge.Node.IsFork,
			IsArchived:        edge.Node.IsArchived,
			StargazerCount:    edge.Node.StargazerCount,
			PushedAt:          stringValue(edge.Node.PushedAt),
			URL:               edge.Node.URL,
			Language:          edge.Node.PrimaryLanguage.OrEmpty(),
			StarredAt:         edge.StarredAt,
			License:           edge.Node.LicenseInfo.OrEmpty(),
			Topics:            edge.Node.RepositoryTopics.Names(),
			NormNameWithOwner: strings.ToLower(strings.TrimSpace(edge.Node.NameWithOwner)),
			NormDescription: strings.ToLower(
				strings.TrimSpace(stringValue(edge.Node.Description)),
			),
			NormLanguage: strings.ToLower(
				strings.TrimSpace(edge.Node.PrimaryLanguage.OrEmpty()),
			),
		})
	}
	return repositories, nil
}

func (s *graphQLService) GetRepository(
	ctx context.Context,
	nameWithOwner string,
) (domain.Repository, error) {
	if err := ctx.Err(); err != nil {
		return domain.Repository{}, err
	}
	owner, name, err := parseRepoName(nameWithOwner)
	if err != nil {
		return domain.Repository{}, err
	}
	var result getRepositoryResponse
	variables := map[string]any{"owner": owner, "name": name, "withTopics": true}
	if err := s.client.DoWithContext(ctx, getRepositoryQuery, variables, &result); err != nil {
		return domain.Repository{}, fmt.Errorf(
			"GitHub GraphQL request failed: %w",
			normalizeError(err),
		)
	}
	if result.Repository == nil || result.Repository.ID == "" {
		return domain.Repository{}, fmt.Errorf("repository %q not found", nameWithOwner)
	}
	return domain.Repository{
		ID:                result.Repository.ID,
		NameWithOwner:     result.Repository.NameWithOwner,
		Description:       stringValue(result.Repository.Description),
		IsFork:            result.Repository.IsFork,
		IsArchived:        result.Repository.IsArchived,
		StargazerCount:    result.Repository.StargazerCount,
		PushedAt:          stringValue(result.Repository.PushedAt),
		URL:               result.Repository.URL,
		Language:          result.Repository.PrimaryLanguage.OrEmpty(),
		License:           result.Repository.LicenseInfo.OrEmpty(),
		Topics:            result.Repository.RepositoryTopics.Names(),
		NormNameWithOwner: strings.ToLower(strings.TrimSpace(result.Repository.NameWithOwner)),
		NormDescription: strings.ToLower(
			strings.TrimSpace(stringValue(result.Repository.Description)),
		),
		NormLanguage: strings.ToLower(
			strings.TrimSpace(result.Repository.PrimaryLanguage.OrEmpty()),
		),
	}, nil
}

func (s *graphQLService) GetRepositoryID(
	ctx context.Context,
	nameWithOwner string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	owner, name, err := parseRepoName(nameWithOwner)
	if err != nil {
		return "", err
	}
	var result getRepositoryIDResponse
	variables := map[string]any{"owner": owner, "name": name}
	if err := s.client.DoWithContext(ctx, getRepositoryIDQuery, variables, &result); err != nil {
		return "", fmt.Errorf("GitHub GraphQL request failed: %w", normalizeError(err))
	}
	if result.Repository == nil || result.Repository.ID == "" {
		return "", fmt.Errorf("repository %q not found", nameWithOwner)
	}
	return result.Repository.ID, nil
}

func (s *graphQLService) GetRepositoryMemberships(
	ctx context.Context,
	nameWithOwner string,
) (string, []string, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	lists, err := s.ListStarLists(ctx)
	if err != nil {
		return "", nil, err
	}
	key := strings.ToLower(nameWithOwner)
	var mu sync.Mutex
	var repoID string
	memberListIDs := make([]string, 0)
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(5)
	for _, list := range lists {
		list := list
		group.Go(func() error {
			var repos []domain.Repository
			var err error
			for attempt := 0; attempt < 3; attempt++ {
				repos, err = s.ListRepositories(groupCtx, list.ID)
				if err == nil || !isTransientGraphQLError(err) {
					break
				}
				if attempt < 2 {
					delay := retryAfterDelay(err)
					if delay == 0 {
						delay = backoffWithJitter(time.Second, attempt)
					}
					if sleepErr := sleepWithContext(groupCtx, delay); sleepErr != nil {
						return sleepErr
					}
				}
			}
			if err != nil {
				return err
			}
			for _, repo := range repos {
				if strings.ToLower(repo.NameWithOwner) == key {
					mu.Lock()
					if repo.ID != "" {
						repoID = repo.ID
					}
					memberListIDs = append(memberListIDs, list.ID)
					mu.Unlock()
					break
				}
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return "", nil, err
	}
	if repoID == "" {
		repoID, err = s.GetRepositoryID(ctx, nameWithOwner)
		if err != nil {
			return "", nil, err
		}
	}
	return repoID, memberListIDs, nil
}

func (s *graphQLService) CreateStarList(
	ctx context.Context,
	input domain.StarListInput,
) (domain.StarList, error) {
	if err := ctx.Err(); err != nil {
		return domain.StarList{}, err
	}
	var result createStarListResponse
	variables := map[string]any{
		"name":        input.Name,
		"description": input.Description,
		"private":     input.Private,
	}
	if err := s.client.DoWithContext(ctx, createStarListMutation, variables, &result); err != nil {
		return domain.StarList{}, fmt.Errorf(
			"GitHub GraphQL request failed: %w",
			normalizeError(err),
		)
	}
	return s.starListFromNode(result.CreateUserList.List), nil
}

func (s *graphQLService) UpdateStarList(
	ctx context.Context,
	input domain.UpdateStarListInput,
) (domain.StarList, error) {
	if err := ctx.Err(); err != nil {
		return domain.StarList{}, err
	}
	var result updateStarListResponse
	variables := map[string]any{
		"listID":      input.ID,
		"name":        nullableString(input.Name),
		"description": nullableString(input.Description),
		"private":     input.Private,
	}
	if err := s.client.DoWithContext(ctx, updateStarListMutation, variables, &result); err != nil {
		return domain.StarList{}, fmt.Errorf(
			"GitHub GraphQL request failed: %w",
			normalizeError(err),
		)
	}
	return s.starListFromNode(result.UpdateUserList.List), nil
}

func (s *graphQLService) DeleteStarList(ctx context.Context, listID string) error {
	return s.execMutation(ctx, deleteStarListMutation, map[string]any{"listID": listID})
}

func (s *graphQLService) UpdateRepositoryLists(
	ctx context.Context,
	repoID string,
	listIDs []string,
) error {
	if listIDs == nil {
		listIDs = []string{}
	}
	return s.execMutation(
		ctx,
		updateRepositoryListsMutation,
		map[string]any{"itemID": repoID, "listIDs": listIDs},
	)
}

func (s *graphQLService) AddStar(ctx context.Context, repoID string) error {
	return s.execMutation(ctx, addStarMutation, map[string]any{"starrableID": repoID})
}

func (s *graphQLService) RemoveStar(ctx context.Context, repoID string) error {
	return s.execMutation(ctx, removeStarMutation, map[string]any{"starrableID": repoID})
}

func (s *graphQLService) execMutation(
	ctx context.Context,
	query string,
	variables map[string]any,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var result map[string]any
	if err := s.client.DoWithContext(ctx, query, variables, &result); err != nil {
		return fmt.Errorf("GitHub GraphQL request failed: %w", normalizeError(err))
	}
	return nil
}

func (s *graphQLService) starListFromNode(node starListNode) domain.StarList {
	repoCount := 0
	if node.Items != nil {
		repoCount = node.Items.TotalCount
	}
	return domain.StarList{
		Name:            node.Name,
		Description:     stringValue(node.Description),
		LastAddedAt:     stringValue(node.LastAddedAt),
		IsPrivate:       node.IsPrivate,
		ID:              node.ID,
		RepoCount:       repoCount,
		URL:             listURL(s.host, node.User.Login, node.Slug),
		NormName:        strings.ToLower(strings.TrimSpace(node.Name)),
		NormDescription: strings.ToLower(strings.TrimSpace(stringValue(node.Description))),
	}
}
