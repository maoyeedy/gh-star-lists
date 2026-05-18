package githubapi

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/auth"
)

var ErrInaccessibleList = errors.New("GitHub Star List is inaccessible or is not a UserList")

type Service interface {
	ListStarLists(ctx context.Context, options ...ListOptions) ([]StarList, error)
	ListRepositories(ctx context.Context, listID string, options ...ListOptions) ([]Repository, error)
	ListStarredRepositories(ctx context.Context, options ...ListOptions) ([]Repository, error)
	GetRepository(ctx context.Context, nameWithOwner string) (Repository, error)
	GetRepositoryMemberships(ctx context.Context, nameWithOwner string) (string, []string, error)
	CreateStarList(ctx context.Context, input StarListInput) (StarList, error)
	UpdateStarList(ctx context.Context, input UpdateStarListInput) (StarList, error)
	DeleteStarList(ctx context.Context, listID string) error
	UpdateRepositoryLists(ctx context.Context, repoID string, listIDs []string) error
	AddStar(ctx context.Context, repoID string) error
	RemoveStar(ctx context.Context, repoID string) error
}

type ListOptions struct {
	Limit      int
	WithTopics bool
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

type StarListInput struct {
	Name        string
	Description string
	Private     bool
}

type UpdateStarListInput struct {
	ID          string
	Name        string
	Description string
	Private     *bool
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

func (s *lazyService) ListStarLists(ctx context.Context, options ...ListOptions) ([]StarList, error) {
	service, err := s.init(ctx)
	if err != nil {
		return nil, err
	}
	return service.ListStarLists(ctx, options...)
}

func (s *lazyService) ListRepositories(
	ctx context.Context,
	listID string,
	options ...ListOptions,
) ([]Repository, error) {
	service, err := s.init(ctx)
	if err != nil {
		return nil, err
	}
	return service.ListRepositories(ctx, listID, options...)
}

func (s *lazyService) ListStarredRepositories(
	ctx context.Context,
	options ...ListOptions,
) ([]Repository, error) {
	service, err := s.init(ctx)
	if err != nil {
		return nil, err
	}
	return service.ListStarredRepositories(ctx, options...)
}

func (s *lazyService) GetRepository(ctx context.Context, nameWithOwner string) (Repository, error) {
	service, err := s.init(ctx)
	if err != nil {
		return Repository{}, err
	}
	return service.GetRepository(ctx, nameWithOwner)
}

func (s *lazyService) GetRepositoryMemberships(
	ctx context.Context,
	nameWithOwner string,
) (string, []string, error) {
	service, err := s.init(ctx)
	if err != nil {
		return "", nil, err
	}
	return service.GetRepositoryMemberships(ctx, nameWithOwner)
}

func (s *lazyService) CreateStarList(ctx context.Context, input StarListInput) (StarList, error) {
	service, err := s.init(ctx)
	if err != nil {
		return StarList{}, err
	}
	return service.CreateStarList(ctx, input)
}

func (s *lazyService) UpdateStarList(ctx context.Context, input UpdateStarListInput) (StarList, error) {
	service, err := s.init(ctx)
	if err != nil {
		return StarList{}, err
	}
	return service.UpdateStarList(ctx, input)
}

func (s *lazyService) DeleteStarList(ctx context.Context, listID string) error {
	service, err := s.init(ctx)
	if err != nil {
		return err
	}
	return service.DeleteStarList(ctx, listID)
}

func (s *lazyService) UpdateRepositoryLists(
	ctx context.Context,
	repoID string,
	listIDs []string,
) error {
	service, err := s.init(ctx)
	if err != nil {
		return err
	}
	return service.UpdateRepositoryLists(ctx, repoID, listIDs)
}

func (s *lazyService) AddStar(ctx context.Context, repoID string) error {
	service, err := s.init(ctx)
	if err != nil {
		return err
	}
	return service.AddStar(ctx, repoID)
}

func (s *lazyService) RemoveStar(ctx context.Context, repoID string) error {
	service, err := s.init(ctx)
	if err != nil {
		return err
	}
	return service.RemoveStar(ctx, repoID)
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

func NewProductionService() Service {
	return NewProductionServiceWithOptions(ProductionOptions{})
}

type ProductionOptions struct {
	Host          string
	HTTPReadCache bool
}

func NewProductionServiceWithOptions(options ProductionOptions) Service {
	return newLazyService(func() (Service, error) {
		return newGoGHGraphQLService(options)
	})
}

func newGoGHGraphQLService(options ProductionOptions) (Service, error) {
	host := options.Host
	if host == "" {
		host, _ = auth.DefaultHost()
	}
	clientOptions := api.ClientOptions{
		Host: host,
	}
	if options.HTTPReadCache {
		clientOptions.EnableCache = true
		clientOptions.CacheTTL = defaultCacheTTL
	}
	client, err := api.NewGraphQLClient(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub GraphQL client: %w", err)
	}
	return newGraphQLService(newRetryDoer(client, 3, time.Second), 100, host), nil
}

type graphQLDoer interface {
	DoWithContext(ctx context.Context, query string, variables map[string]any, response any) error
}
