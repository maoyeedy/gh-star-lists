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

const listStarredRepositoriesQuery = `query($endCursor: String, $first: Int!) {
  viewer {
    starredRepositories(first: $first, after: $endCursor, orderBy: {field: STARRED_AT, direction: DESC}) {
      edges {
        starredAt
        node {
          nameWithOwner
          description
          url
          isFork
          stargazerCount
          pushedAt
          primaryLanguage {
            name
          }
        }
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
            primaryLanguage {
              name
            }
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
				URL:         listURL(node.User.Login, node.Slug),
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
				Language:       node.PrimaryLanguage.OrEmpty(),
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

type repositoryItemNode struct {
	Typename        string        `json:"__typename"`
	NameWithOwner   string        `json:"nameWithOwner"`
	Description     *string       `json:"description"`
	URL             string        `json:"url"`
	IsFork          bool          `json:"isFork"`
	StargazerCount  int           `json:"stargazerCount"`
	PushedAt        *string       `json:"pushedAt"`
	PrimaryLanguage *languageNode `json:"primaryLanguage"`
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
	NameWithOwner   string        `json:"nameWithOwner"`
	Description     *string       `json:"description"`
	URL             string        `json:"url"`
	IsFork          bool          `json:"isFork"`
	StargazerCount  int           `json:"stargazerCount"`
	PushedAt        *string       `json:"pushedAt"`
	PrimaryLanguage *languageNode `json:"primaryLanguage"`
}

func (s *graphQLService) ListStarredRepositories(ctx context.Context) ([]Repository, error) {
	repositories := make([]Repository, 0, s.pageSize)
	var endCursor any

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var result listStarredRepositoriesResponse
		variables := map[string]any{"endCursor": endCursor, "first": s.pageSize}
		if err := s.executor.Execute(
			ctx,
			listStarredRepositoriesQuery,
			variables,
			&result,
		); err != nil {
			return nil, fmt.Errorf("GitHub GraphQL request failed: %w", err)
		}

		for _, edge := range result.Viewer.StarredRepositories.Edges {
			repositories = append(repositories, Repository{
				NameWithOwner:  edge.Node.NameWithOwner,
				Description:    stringValue(edge.Node.Description),
				IsFork:         edge.Node.IsFork,
				StargazerCount: edge.Node.StargazerCount,
				PushedAt:       stringValue(edge.Node.PushedAt),
				URL:            edge.Node.URL,
				Language:       edge.Node.PrimaryLanguage.OrEmpty(),
				StarredAt:      edge.StarredAt,
			})
		}
		if !result.Viewer.StarredRepositories.PageInfo.HasNextPage {
			return repositories, nil
		}
		endCursor = stringValue(result.Viewer.StarredRepositories.PageInfo.EndCursor)
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func listURL(login, slug string) string {
	var b strings.Builder
	b.Grow(len("https://github.com/stars/") + len(login) + len("/lists/") + len(slug))
	b.WriteString("https://github.com/stars/")
	b.WriteString(login)
	b.WriteString("/lists/")
	b.WriteString(slug)
	return b.String()
}
