package githubapi

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cli/go-gh/v2/pkg/api"
)

// ErrInaccessibleList is returned when GitHub does not return a UserList node
// for a requested Star List ID. This covers wrong IDs, inaccessible lists, and
// malformed responses that cannot be safely interpreted as repository pages.
var ErrInaccessibleList = errors.New("GitHub Star List is inaccessible or is not a UserList")

// Service is the GitHub API boundary used by command.Run.
// Implementations must use the user's existing gh authentication and must not
// store tokens.
type Service interface {
	ListStarLists(ctx context.Context) ([]StarList, error)
	ListRepositories(ctx context.Context, listID string) ([]Repository, error)
	ListStarredRepositories(ctx context.Context) ([]Repository, error)
}

type StarList struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LastAddedAt string `json:"lastAddedAt"`
	ID          string `json:"id"`
	RepoCount   int    `json:"repoCount"`
	URL         string `json:"url"`
}

type Repository struct {
	ID             string   `json:"-"`
	NameWithOwner  string   `json:"nameWithOwner"`
	Description    string   `json:"description"`
	IsFork         bool     `json:"isFork"`
	StargazerCount int      `json:"stargazerCount"`
	PushedAt       string   `json:"pushedAt"`
	URL            string   `json:"url"`
	Language       string   `json:"language"`
	StarredAt      string   `json:"starredAt,omitempty"`
	IsArchived     bool     `json:"-"`
	License        string   `json:"-"`
	Topics         []string `json:"-"`
}

type serviceConstructor func() (Service, error)

type lazyService struct {
	once        sync.Once
	constructor serviceConstructor
	service     Service
	err         error
}

func newLazyService(constructor serviceConstructor) *lazyService {
	return &lazyService{constructor: constructor}
}

func (s *lazyService) ListStarLists(ctx context.Context) ([]StarList, error) {
	service, err := s.init(ctx)
	if err != nil {
		return nil, err
	}
	return service.ListStarLists(ctx)
}

func (s *lazyService) ListRepositories(ctx context.Context, listID string) ([]Repository, error) {
	service, err := s.init(ctx)
	if err != nil {
		return nil, err
	}
	return service.ListRepositories(ctx, listID)
}

func (s *lazyService) ListStarredRepositories(ctx context.Context) ([]Repository, error) {
	service, err := s.init(ctx)
	if err != nil {
		return nil, err
	}
	return service.ListStarredRepositories(ctx)
}

func (s *lazyService) init(ctx context.Context) (Service, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.once.Do(func() {
		if s.constructor == nil {
			s.err = errors.New("GitHub service constructor is not configured")
			return
		}
		s.service, s.err = s.constructor()
		if s.err == nil && s.service == nil {
			s.err = errors.New("GitHub service constructor returned nil service")
		}
	})
	return s.service, s.err
}

// NewProductionService returns the lazily initialized service used by the
// extension entrypoint. go-gh auth/config resolution is deferred until a runtime
// command calls the service.
func NewProductionService() Service {
	return newLazyService(newGoGHGraphQLService)
}

func newGoGHGraphQLService() (Service, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub GraphQL client: %w", err)
	}
	return newGraphQLService(goGHGraphQLExecutor{client: client}, 100), nil
}

type graphQLDoer interface {
	DoWithContext(ctx context.Context, query string, variables map[string]any, response any) error
}

type goGHGraphQLExecutor struct {
	client graphQLDoer
}

func (e goGHGraphQLExecutor) Execute(
	ctx context.Context,
	query string,
	variables map[string]any,
	response any,
) error {
	return e.client.DoWithContext(ctx, query, variables, response)
}
