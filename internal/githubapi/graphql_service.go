package githubapi

import (
	"context"
	"fmt"
	"strings"
	"sync"

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
	options ...ListOptions,
) ([]StarList, error) {
	limit := limitFromOptions(options)
	lists := make([]StarList, 0, s.pageSize)
	var endCursor any

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var result listStarListsResponse
		variables := map[string]any{
			"endCursor": endCursor,
			"first":     pageFirst(s.pageSize, limit, len(lists)),
		}
		if err := s.client.DoWithContext(ctx, listStarListsQuery, variables, &result); err != nil {
			return nil, fmt.Errorf("GitHub GraphQL request failed: %w", err)
		}

		for _, node := range result.Viewer.Lists.Nodes {
			repoCount := 0
			if node.Items != nil {
				repoCount = node.Items.TotalCount
			}
			lists = append(lists, StarList{
				Name:            node.Name,
				Description:     stringValue(node.Description),
				LastAddedAt:     stringValue(node.LastAddedAt),
				IsPrivate:       node.IsPrivate,
				ID:              node.ID,
				RepoCount:       repoCount,
				URL:             listURL(s.host, node.User.Login, node.Slug),
				NormName:        strings.ToLower(strings.TrimSpace(node.Name)),
				NormDescription: strings.ToLower(strings.TrimSpace(stringValue(node.Description))),
			})
			if limitReached(limit, len(lists)) {
				return lists, nil
			}
		}
		if !result.Viewer.Lists.PageInfo.HasNextPage {
			return lists, nil
		}
		endCursor = stringValue(result.Viewer.Lists.PageInfo.EndCursor)
	}
}

func (s *graphQLService) ListRepositories(
	ctx context.Context,
	listID string,
	options ...ListOptions,
) ([]Repository, error) {
	limit := limitFromOptions(options)
	withTopics := withTopicsFromOptions(options)
	repositories := make([]Repository, 0, s.pageSize)
	var endCursor any

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var result listRepositoriesResponse
		variables := map[string]any{
			"id":         listID,
			"endCursor":  endCursor,
			"first":      pageFirst(s.pageSize, limit, len(repositories)),
			"withTopics": withTopics,
		}
		if err := s.client.DoWithContext(
			ctx,
			listRepositoriesQuery,
			variables,
			&result,
		); err != nil {
			return nil, fmt.Errorf("GitHub GraphQL request failed: %w", err)
		}
		if result.Node == nil || result.Node.Typename != "UserList" || result.Node.Items == nil {
			return nil, ErrInaccessibleList
		}

		for _, node := range result.Node.Items.Nodes {
			if node == nil || node.Typename != "Repository" || node.NameWithOwner == "" {
				continue
			}
			repositories = append(repositories, Repository{
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
			if limitReached(limit, len(repositories)) {
				return repositories, nil
			}
		}
		if !result.Node.Items.PageInfo.HasNextPage {
			return repositories, nil
		}
		endCursor = stringValue(result.Node.Items.PageInfo.EndCursor)
	}
}

func (s *graphQLService) ListStarredRepositories(
	ctx context.Context,
	options ...ListOptions,
) ([]Repository, error) {
	limit := limitFromOptions(options)
	withTopics := withTopicsFromOptions(options)
	repositories := make([]Repository, 0, s.pageSize)
	var endCursor any

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var result listStarredRepositoriesResponse
		variables := map[string]any{
			"endCursor":  endCursor,
			"first":      pageFirst(s.pageSize, limit, len(repositories)),
			"withTopics": withTopics,
		}
		if err := s.client.DoWithContext(
			ctx,
			listStarredRepositoriesQuery,
			variables,
			&result,
		); err != nil {
			return nil, fmt.Errorf("GitHub GraphQL request failed: %w", err)
		}

		for _, edge := range result.Viewer.StarredRepositories.Edges {
			repositories = append(repositories, Repository{
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
			if limitReached(limit, len(repositories)) {
				return repositories, nil
			}
		}
		if !result.Viewer.StarredRepositories.PageInfo.HasNextPage {
			return repositories, nil
		}
		endCursor = stringValue(result.Viewer.StarredRepositories.PageInfo.EndCursor)
	}
}

func (s *graphQLService) GetRepository(
	ctx context.Context,
	nameWithOwner string,
) (Repository, error) {
	if err := ctx.Err(); err != nil {
		return Repository{}, err
	}
	owner, name, err := parseRepoName(nameWithOwner)
	if err != nil {
		return Repository{}, err
	}
	var result getRepositoryResponse
	variables := map[string]any{"owner": owner, "name": name, "withTopics": true}
	if err := s.client.DoWithContext(ctx, getRepositoryQuery, variables, &result); err != nil {
		return Repository{}, fmt.Errorf("GitHub GraphQL request failed: %w", err)
	}
	if result.Repository == nil || result.Repository.ID == "" {
		return Repository{}, fmt.Errorf("repository %q not found", nameWithOwner)
	}
	return Repository{
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
		return "", fmt.Errorf("GitHub GraphQL request failed: %w", err)
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
			repos, err := s.ListRepositories(groupCtx, list.ID)
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
	input StarListInput,
) (StarList, error) {
	if err := ctx.Err(); err != nil {
		return StarList{}, err
	}
	var result createStarListResponse
	variables := map[string]any{
		"name":        input.Name,
		"description": input.Description,
		"private":     input.Private,
	}
	if err := s.client.DoWithContext(ctx, createStarListMutation, variables, &result); err != nil {
		return StarList{}, fmt.Errorf("GitHub GraphQL request failed: %w", err)
	}
	return s.starListFromNode(result.CreateUserList.List), nil
}

func (s *graphQLService) UpdateStarList(
	ctx context.Context,
	input UpdateStarListInput,
) (StarList, error) {
	if err := ctx.Err(); err != nil {
		return StarList{}, err
	}
	var result updateStarListResponse
	variables := map[string]any{
		"listID":      input.ID,
		"name":        nullableString(input.Name),
		"description": nullableString(input.Description),
		"private":     input.Private,
	}
	if err := s.client.DoWithContext(ctx, updateStarListMutation, variables, &result); err != nil {
		return StarList{}, fmt.Errorf("GitHub GraphQL request failed: %w", err)
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
		return fmt.Errorf("GitHub GraphQL request failed: %w", err)
	}
	return nil
}

func (s *graphQLService) starListFromNode(node starListNode) StarList {
	repoCount := 0
	if node.Items != nil {
		repoCount = node.Items.TotalCount
	}
	return StarList{
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
