package githubapi

import (
	"context"
	"fmt"
	"strings"
)

const listStarListsQuery = `query($endCursor: String, $first: Int!) {
  viewer {
    lists(first: $first, after: $endCursor) {
      nodes {
        id
        name
        slug
        description
        lastAddedAt
        items {
          totalCount
        }
        user {
          login
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`

// repositoryFieldsFragment is the shared set of repository fields fetched by all three
// repository queries. Embedded via Go const concatenation; GraphQL is whitespace-insensitive.
// repositoryTopics uses @include(if: $withTopics); callers must declare $withTopics: Boolean!.
const repositoryFieldsFragment = `
    id
    nameWithOwner
    description
    url
    isFork
    isArchived
    stargazerCount
    pushedAt
    licenseInfo {
      key
    }
    repositoryTopics(first: 20) @include(if: $withTopics) {
      nodes {
        topic {
          name
        }
      }
    }
    primaryLanguage {
      name
    }`

const listStarredRepositoriesQuery = `query($endCursor: String, $first: Int!, $withTopics: Boolean!) {
  viewer {
    starredRepositories(first: $first, after: $endCursor, orderBy: {field: STARRED_AT, direction: DESC}) {
      edges {
        starredAt
        node {` + repositoryFieldsFragment + `
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`

const getRepositoryQuery = `query($owner: String!, $name: String!, $withTopics: Boolean!) {
  repository(owner: $owner, name: $name) {` + repositoryFieldsFragment + `
  }
}`

const getRepositoryWithListsQuery = `query($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    id
    nameWithOwner
    userLists(first: 100) {
      nodes {
        id
        name
      }
    }
  }
}`

const createStarListMutation = `mutation($name: String!, $description: String, $private: Boolean!) {
  createUserList(input: {name: $name, description: $description, isPrivate: $private}) {
    list {
      id
      name
      slug
      description
      lastAddedAt
      items {
        totalCount
      }
      user {
        login
      }
    }
  }
}`

const updateStarListMutation = `mutation($listID: ID!, $name: String, $description: String, $private: Boolean) {
  updateUserList(input: {listId: $listID, name: $name, description: $description, isPrivate: $private}) {
    list {
      id
      name
      slug
      description
      lastAddedAt
      items {
        totalCount
      }
      user {
        login
      }
    }
  }
}`

const deleteStarListMutation = `mutation($listID: ID!) {
  deleteUserList(input: {listId: $listID}) {
    clientMutationId
  }
}`

const updateRepositoryListsMutation = `mutation($itemID: ID!, $listIDs: [ID!]!) {
  updateUserListsForItem(input: {itemId: $itemID, listIds: $listIDs}) {
    clientMutationId
  }
}`

const addStarMutation = `mutation($starrableID: ID!) {
  addStar(input: {starrableId: $starrableID}) {
    clientMutationId
  }
}`

const removeStarMutation = `mutation($starrableID: ID!) {
  removeStar(input: {starrableId: $starrableID}) {
    clientMutationId
  }
}`

const listRepositoriesQuery = `query($id: ID!, $endCursor: String, $first: Int!, $withTopics: Boolean!) {
  node(id: $id) {
    __typename
    ... on UserList {
      name
      items(first: $first, after: $endCursor) {
        nodes {
          __typename
          ... on Repository {` + repositoryFieldsFragment + `
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }
}`

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
		variables := map[string]any{"endCursor": endCursor, "first": pageFirst(s.pageSize, limit, len(lists))}
		if err := s.client.DoWithContext(ctx, listStarListsQuery, variables, &result); err != nil {
			return nil, fmt.Errorf("GitHub GraphQL request failed: %w", err)
		}

		for _, node := range result.Viewer.Lists.Nodes {
			repoCount := 0
			if node.Items != nil {
				repoCount = node.Items.TotalCount
			}
			lists = append(lists, StarList{
				Name:        node.Name,
				Description: stringValue(node.Description),
				LastAddedAt: stringValue(node.LastAddedAt),
				ID:          node.ID,
				RepoCount:   repoCount,
				URL:         listURL(s.host, node.User.Login, node.Slug),
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
		if err := s.client.DoWithContext(ctx, listRepositoriesQuery, variables, &result); err != nil {
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
				ID:             node.ID,
				NameWithOwner:  node.NameWithOwner,
				Description:    stringValue(node.Description),
				IsFork:         node.IsFork,
				IsArchived:     node.IsArchived,
				StargazerCount: node.StargazerCount,
				PushedAt:       stringValue(node.PushedAt),
				URL:            node.URL,
				Language:       node.PrimaryLanguage.OrEmpty(),
				License:        node.LicenseInfo.OrEmpty(),
				Topics:         node.RepositoryTopics.Names(),
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

type listStarListsResponse struct {
	Viewer struct {
		Lists struct {
			Nodes    []starListNode `json:"nodes"`
			PageInfo pageInfo       `json:"pageInfo"`
		} `json:"lists"`
	} `json:"viewer"`
}

type starListItemsConnection struct {
	TotalCount int `json:"totalCount"`
}

type starListNode struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Slug        string                   `json:"slug"`
	Description *string                  `json:"description"`
	LastAddedAt *string                  `json:"lastAddedAt"`
	Items       *starListItemsConnection `json:"items"`
	User        userNode                 `json:"user"`
}

type userNode struct {
	Login string `json:"login"`
}

type pageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}

type listRepositoriesResponse struct {
	Node *userListNode `json:"node"`
}

type userListNode struct {
	Typename string                     `json:"__typename"`
	Items    *repositoryItemsConnection `json:"items"`
}

type repositoryItemsConnection struct {
	Nodes    []*repositoryItemNode `json:"nodes"`
	PageInfo pageInfo              `json:"pageInfo"`
}

type languageNode struct {
	Name string `json:"name"`
}

func (l *languageNode) OrEmpty() string {
	if l == nil {
		return ""
	}
	return l.Name
}

type licenseNode struct {
	Key string `json:"key"`
}

func (l *licenseNode) OrEmpty() string {
	if l == nil {
		return ""
	}
	return l.Key
}

type repositoryTopicConnection struct {
	Nodes []repositoryTopicNode `json:"nodes"`
}

func (c repositoryTopicConnection) Names() []string {
	if len(c.Nodes) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.Nodes))
	for _, node := range c.Nodes {
		if node.Topic.Name != "" {
			names = append(names, node.Topic.Name)
		}
	}
	return names
}

type repositoryTopicNode struct {
	Topic topicNode `json:"topic"`
}

type topicNode struct {
	Name string `json:"name"`
}

type repositoryItemNode struct {
	Typename         string                    `json:"__typename"`
	ID               string                    `json:"id"`
	NameWithOwner    string                    `json:"nameWithOwner"`
	Description      *string                   `json:"description"`
	URL              string                    `json:"url"`
	IsFork           bool                      `json:"isFork"`
	IsArchived       bool                      `json:"isArchived"`
	StargazerCount   int                       `json:"stargazerCount"`
	PushedAt         *string                   `json:"pushedAt"`
	LicenseInfo      *licenseNode              `json:"licenseInfo"`
	RepositoryTopics repositoryTopicConnection `json:"repositoryTopics"`
	PrimaryLanguage  *languageNode             `json:"primaryLanguage"`
}

type listStarredRepositoriesResponse struct {
	Viewer struct {
		StarredRepositories struct {
			Edges    []starredRepositoryEdge `json:"edges"`
			PageInfo pageInfo                `json:"pageInfo"`
		} `json:"starredRepositories"`
	} `json:"viewer"`
}

type starredRepositoryEdge struct {
	StarredAt string              `json:"starredAt"`
	Node      starredRepoItemNode `json:"node"`
}

type starredRepoItemNode struct {
	ID               string                    `json:"id"`
	NameWithOwner    string                    `json:"nameWithOwner"`
	Description      *string                   `json:"description"`
	URL              string                    `json:"url"`
	IsFork           bool                      `json:"isFork"`
	IsArchived       bool                      `json:"isArchived"`
	StargazerCount   int                       `json:"stargazerCount"`
	PushedAt         *string                   `json:"pushedAt"`
	LicenseInfo      *licenseNode              `json:"licenseInfo"`
	RepositoryTopics repositoryTopicConnection `json:"repositoryTopics"`
	PrimaryLanguage  *languageNode             `json:"primaryLanguage"`
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
				ID:             edge.Node.ID,
				NameWithOwner:  edge.Node.NameWithOwner,
				Description:    stringValue(edge.Node.Description),
				IsFork:         edge.Node.IsFork,
				IsArchived:     edge.Node.IsArchived,
				StargazerCount: edge.Node.StargazerCount,
				PushedAt:       stringValue(edge.Node.PushedAt),
				URL:            edge.Node.URL,
				Language:       edge.Node.PrimaryLanguage.OrEmpty(),
				StarredAt:      edge.StarredAt,
				License:        edge.Node.LicenseInfo.OrEmpty(),
				Topics:         edge.Node.RepositoryTopics.Names(),
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

type getRepositoryResponse struct {
	Repository *repositoryNode `json:"repository"`
}

type repositoryNode struct {
	ID               string                    `json:"id"`
	NameWithOwner    string                    `json:"nameWithOwner"`
	Description      *string                   `json:"description"`
	URL              string                    `json:"url"`
	IsFork           bool                      `json:"isFork"`
	IsArchived       bool                      `json:"isArchived"`
	StargazerCount   int                       `json:"stargazerCount"`
	PushedAt         *string                   `json:"pushedAt"`
	LicenseInfo      *licenseNode              `json:"licenseInfo"`
	RepositoryTopics repositoryTopicConnection `json:"repositoryTopics"`
	PrimaryLanguage  *languageNode             `json:"primaryLanguage"`
}

type repositoryMembershipsResponse struct {
	Repository *repositoryMembershipsNode `json:"repository"`
}

type repositoryMembershipsNode struct {
	ID            string              `json:"id"`
	NameWithOwner string              `json:"nameWithOwner"`
	UserLists     repositoryUserLists `json:"userLists"`
}

type repositoryUserLists struct {
	Nodes []userListIDNode `json:"nodes"`
}

type userListIDNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type starListMutationResponse struct {
	List starListNode `json:"list"`
}

type createStarListResponse struct {
	CreateUserList starListMutationResponse `json:"createUserList"`
}

type updateStarListResponse struct {
	UpdateUserList starListMutationResponse `json:"updateUserList"`
}

func (s *graphQLService) GetRepository(
	ctx context.Context,
	nameWithOwner string,
) (Repository, error) {
	if err := ctx.Err(); err != nil {
		return Repository{}, err
	}
	owner, name, ok := strings.Cut(nameWithOwner, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return Repository{}, fmt.Errorf("invalid repository %q: expected owner/name", nameWithOwner)
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
		ID:             result.Repository.ID,
		NameWithOwner:  result.Repository.NameWithOwner,
		Description:    stringValue(result.Repository.Description),
		IsFork:         result.Repository.IsFork,
		IsArchived:     result.Repository.IsArchived,
		StargazerCount: result.Repository.StargazerCount,
		PushedAt:       stringValue(result.Repository.PushedAt),
		URL:            result.Repository.URL,
		Language:       result.Repository.PrimaryLanguage.OrEmpty(),
		License:        result.Repository.LicenseInfo.OrEmpty(),
		Topics:         result.Repository.RepositoryTopics.Names(),
	}, nil
}

func (s *graphQLService) GetRepositoryMemberships(
	ctx context.Context,
	nameWithOwner string,
) (string, []string, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	owner, name, ok := strings.Cut(nameWithOwner, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", nil, fmt.Errorf("invalid repository %q: expected owner/name", nameWithOwner)
	}
	var result repositoryMembershipsResponse
	variables := map[string]any{"owner": owner, "name": name}
	if err := s.client.DoWithContext(ctx, getRepositoryWithListsQuery, variables, &result); err != nil {
		return "", nil, fmt.Errorf("GitHub GraphQL request failed: %w", err)
	}
	if result.Repository == nil || result.Repository.ID == "" {
		return "", nil, fmt.Errorf("repository %q not found", nameWithOwner)
	}
	listIDs := make([]string, 0, len(result.Repository.UserLists.Nodes))
	for _, node := range result.Repository.UserLists.Nodes {
		if node.ID == "" {
			continue
		}
		listIDs = append(listIDs, node.ID)
	}
	return result.Repository.ID, listIDs, nil
}

func (s *graphQLService) CreateStarList(ctx context.Context, input StarListInput) (StarList, error) {
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
		Name:        node.Name,
		Description: stringValue(node.Description),
		LastAddedAt: stringValue(node.LastAddedAt),
		ID:          node.ID,
		RepoCount:   repoCount,
		URL:         listURL(s.host, node.User.Login, node.Slug),
	}
}

func limitFromOptions(options []ListOptions) int {
	if len(options) == 0 {
		return 0
	}
	return options[0].Limit
}

func withTopicsFromOptions(options []ListOptions) bool {
	if len(options) == 0 {
		return false
	}
	return options[0].WithTopics
}

func pageFirst(pageSize, limit, current int) int {
	if limit <= 0 {
		return pageSize
	}
	remaining := limit - current
	if remaining <= 0 {
		return 1
	}
	if remaining < pageSize {
		return remaining
	}
	return pageSize
}

func limitReached(limit, count int) bool {
	return limit > 0 && count >= limit
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func listURL(host, login, slug string) string {
	var b strings.Builder
	b.Grow(len("https://") + len(host) + len("/stars/") + len(login) + len("/lists/") + len(slug))
	b.WriteString("https://")
	b.WriteString(host)
	b.WriteString("/stars/")
	b.WriteString(login)
	b.WriteString("/lists/")
	b.WriteString(slug)
	return b.String()
}
