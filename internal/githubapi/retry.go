package githubapi

import (
	"context"
	"errors"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

type retryDoer struct {
	inner       graphQLDoer
	maxAttempts int
	baseDelay   time.Duration
	sleep       func(ctx context.Context, d time.Duration) error
}

func newRetryDoer(inner graphQLDoer, maxAttempts int, baseDelay time.Duration) *retryDoer {
	return &retryDoer{
		inner:       inner,
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
		sleep:       sleepWithContext,
	}
}

func (r *retryDoer) DoWithContext(ctx context.Context, query string, variables map[string]any, response any) error {
	var lastErr error
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := r.inner.DoWithContext(ctx, query, variables, response)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isTransientGraphQLError(err) {
			return err
		}
		if attempt == r.maxAttempts-1 {
			break
		}
		delay := retryAfterDelay(err)
		if delay == 0 {
			delay = backoffWithJitter(r.baseDelay, attempt)
		}
		if err := r.sleep(ctx, delay); err != nil {
			return err
		}
	}
	return lastErr
}

func isTransientGraphQLError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == 429 || httpErr.StatusCode >= 500
	}
	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) {
		for _, item := range gqlErr.Errors {
			lower := strings.ToLower(item.Message)
			if strings.Contains(lower, "rate limit") {
				return true
			}
		}
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "secondary rate limit") ||
		strings.Contains(lower, "rate limit exceeded")
}

// retryAfterDelay extracts the Retry-After header value from an HTTPError, if present.
// Returns 0 when not available.
func retryAfterDelay(err error) time.Duration {
	var httpErr *api.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Headers == nil {
		return 0
	}
	val := httpErr.Headers.Get("Retry-After")
	if val == "" {
		return 0
	}
	secs, err := strconv.Atoi(val)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func backoffWithJitter(base time.Duration, attempt int) time.Duration {
	delay := base
	for range attempt {
		delay *= 2
	}
	const cap = 30 * time.Second
	if delay > cap {
		delay = cap
	}
	// ±20% jitter
	jitter := float64(delay) * (0.8 + 0.4*rand.Float64())
	return time.Duration(jitter)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
