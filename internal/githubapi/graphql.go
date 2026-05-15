package githubapi

import (
	"context"
	"fmt"
)

const listStarListsQuery = `query($endCursor: String, $first: Int!) {
  viewer {
    lists(first: $first, after: $endCursor) {
      nodes {
        id
        name
        description
        lastAddedAt
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}`

const listRepositoriesQuery = `query($id: ID!, $endCursor: String, $first: Int!) {
  node(id: $id) {
    __typename
    ... on UserList {
      name
      items(first: $first, after: $endCursor) {
        nodes {
          __typename
          ... on Repository {
            nameWithOwner
            description
            url
            isFork
            stargazerCount
            pushedAt
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

type graphQLExecutor interface {
	Execute(ctx context.Context, query string, variables map[string]any, response any) error
}

type graphQLService struct {
	executor graphQLExecutor
	pageSize int
}

func newGraphQLService(executor graphQLExecutor, pageSize int) *graphQLService {
	return &graphQLService{executor: executor, pageSize: pageSize}
}

func (s *graphQLService) ListStarLists(ctx context.Context) ([]StarList, error) {
	lists := make([]StarList, 0, s.pageSize)
	var endCursor any

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var result listStarListsResponse
		variables := map[string]any{"endCursor": endCursor, "first": s.pageSize}
		if err := s.executor.Execute(ctx, listStarListsQuery, variables, &result); err != nil {
			return nil, fmt.Errorf("GitHub GraphQL request failed: %w", err)
		}

		for _, node := range result.Viewer.Lists.Nodes {
			lists = append(lists, StarList{
				Name:        node.Name,
				Description: stringValue(node.Description),
				LastAddedAt: stringValue(node.LastAddedAt),
				ID:          node.ID,
			})
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
) ([]Repository, error) {
	repositories := make([]Repository, 0, s.pageSize)
	var endCursor any

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var result listRepositoriesResponse
		variables := map[string]any{"id": listID, "endCursor": endCursor, "first": s.pageSize}
		if err := s.executor.Execute(ctx, listRepositoriesQuery, variables, &result); err != nil {
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
				NameWithOwner:  node.NameWithOwner,
				Description:    stringValue(node.Description),
				IsFork:         node.IsFork,
				StargazerCount: node.StargazerCount,
				PushedAt:       stringValue(node.PushedAt),
				URL:            node.URL,
			})
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

type starListNode struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	LastAddedAt *string `json:"lastAddedAt"`
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

type repositoryItemNode struct {
	Typename       string  `json:"__typename"`
	NameWithOwner  string  `json:"nameWithOwner"`
	Description    *string `json:"description"`
	URL            string  `json:"url"`
	IsFork         bool    `json:"isFork"`
	StargazerCount int     `json:"stargazerCount"`
	PushedAt       *string `json:"pushedAt"`
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
