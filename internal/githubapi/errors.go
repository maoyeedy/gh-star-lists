package githubapi

import (
	"errors"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

// normalizeError inspects api.HTTPError and api.GraphQLError values and wraps
// them into typed domain errors when the status code or message indicates an
// auth, not-found, or rate-limit condition. Other errors pass through unchanged.
func normalizeError(err error) error {
	if err == nil {
		return nil
	}

	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case 401, 403:
			if httpErr.StatusCode == 403 {
				if delay := retryAfterDelay(err); delay > 0 {
					return &domain.RateLimitError{
						Err:        err,
						RetryAfter: delay,
					}
				}
			}
			return &domain.AuthError{Err: err}
		case 404:
			return &domain.NotFoundError{Resource: "GitHub resource"}
		case 429:
			return &domain.RateLimitError{
				Err:        err,
				RetryAfter: retryAfterDelay(err),
			}
		}
	}

	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) {
		for _, item := range gqlErr.Errors {
			lower := strings.ToLower(item.Message)
			if strings.Contains(lower, "unauthorized") ||
				strings.Contains(lower, "authentication") ||
				strings.Contains(lower, "bad credentials") {
				return &domain.AuthError{Err: err}
			}
			if strings.Contains(lower, "not found") {
				return &domain.NotFoundError{Resource: "GitHub resource"}
			}
			if strings.Contains(lower, "rate limit") {
				return &domain.RateLimitError{Err: err}
			}
		}
	}

	return err
}
