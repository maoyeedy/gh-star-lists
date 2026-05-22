package githubapi

import (
	"context"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

// RetryService wraps a Service and adds retry logic for transient errors on
// read-only methods. Mutation methods pass through without retry since they
// are not idempotent.
type RetryService struct {
	inner       Service
	maxAttempts int
	baseDelay   time.Duration
}

// NewRetryService returns a RetryService that retries read methods up to
// maxAttempts times with exponential backoff starting at baseDelay.
func NewRetryService(inner Service, maxAttempts int, baseDelay time.Duration) *RetryService {
	return &RetryService{
		inner:       inner,
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
	}
}

func (s *RetryService) ListStarLists(
	ctx context.Context,
	options ...domain.ListOptions,
) ([]domain.StarList, error) {
	var lastErr error
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lists, err := s.inner.ListStarLists(ctx, options...)
		if err == nil {
			return lists, nil
		}
		lastErr = err
		if !isTransientGraphQLError(err) {
			return nil, err
		}
		if attempt == s.maxAttempts-1 {
			break
		}
		delay := retryAfterDelay(err)
		if delay == 0 {
			delay = backoffWithJitter(s.baseDelay, attempt)
		}
		if err := sleepWithContext(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (s *RetryService) ListRepositories(
	ctx context.Context,
	listID string,
	options ...domain.ListOptions,
) ([]domain.Repository, error) {
	var lastErr error
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		repos, err := s.inner.ListRepositories(ctx, listID, options...)
		if err == nil {
			return repos, nil
		}
		lastErr = err
		if !isTransientGraphQLError(err) {
			return nil, err
		}
		if attempt == s.maxAttempts-1 {
			break
		}
		delay := retryAfterDelay(err)
		if delay == 0 {
			delay = backoffWithJitter(s.baseDelay, attempt)
		}
		if err := sleepWithContext(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (s *RetryService) ListStarredRepositories(
	ctx context.Context,
	options ...domain.ListOptions,
) ([]domain.Repository, error) {
	var lastErr error
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		repos, err := s.inner.ListStarredRepositories(ctx, options...)
		if err == nil {
			return repos, nil
		}
		lastErr = err
		if !isTransientGraphQLError(err) {
			return nil, err
		}
		if attempt == s.maxAttempts-1 {
			break
		}
		delay := retryAfterDelay(err)
		if delay == 0 {
			delay = backoffWithJitter(s.baseDelay, attempt)
		}
		if err := sleepWithContext(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (s *RetryService) GetRepository(
	ctx context.Context,
	nameWithOwner string,
) (domain.Repository, error) {
	var lastErr error
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return domain.Repository{}, err
		}
		repo, err := s.inner.GetRepository(ctx, nameWithOwner)
		if err == nil {
			return repo, nil
		}
		lastErr = err
		if !isTransientGraphQLError(err) {
			return domain.Repository{}, err
		}
		if attempt == s.maxAttempts-1 {
			break
		}
		delay := retryAfterDelay(err)
		if delay == 0 {
			delay = backoffWithJitter(s.baseDelay, attempt)
		}
		if err := sleepWithContext(ctx, delay); err != nil {
			return domain.Repository{}, err
		}
	}
	return domain.Repository{}, lastErr
}

func (s *RetryService) GetRepositoryMemberships(
	ctx context.Context,
	nameWithOwner string,
) (string, []string, error) {
	var lastErr error
	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		id, listIDs, err := s.inner.GetRepositoryMemberships(ctx, nameWithOwner)
		if err == nil {
			return id, listIDs, nil
		}
		lastErr = err
		if !isTransientGraphQLError(err) {
			return "", nil, err
		}
		if attempt == s.maxAttempts-1 {
			break
		}
		delay := retryAfterDelay(err)
		if delay == 0 {
			delay = backoffWithJitter(s.baseDelay, attempt)
		}
		if err := sleepWithContext(ctx, delay); err != nil {
			return "", nil, err
		}
	}
	return "", nil, lastErr
}

func (s *RetryService) CreateStarList(
	ctx context.Context,
	input domain.StarListInput,
) (domain.StarList, error) {
	return s.inner.CreateStarList(ctx, input)
}

func (s *RetryService) UpdateStarList(
	ctx context.Context,
	input domain.UpdateStarListInput,
) (domain.StarList, error) {
	return s.inner.UpdateStarList(ctx, input)
}

func (s *RetryService) DeleteStarList(ctx context.Context, listID string) error {
	return s.inner.DeleteStarList(ctx, listID)
}

func (s *RetryService) UpdateRepositoryLists(
	ctx context.Context,
	repoID string,
	listIDs []string,
) error {
	return s.inner.UpdateRepositoryLists(ctx, repoID, listIDs)
}

func (s *RetryService) AddStar(ctx context.Context, repoID string) error {
	return s.inner.AddStar(ctx, repoID)
}

func (s *RetryService) RemoveStar(ctx context.Context, repoID string) error {
	return s.inner.RemoveStar(ctx, repoID)
}
