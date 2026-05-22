package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrAuth        = errors.New("GitHub authentication failed")
	ErrNotFound    = errors.New("resource not found")
	ErrRateLimited = errors.New("GitHub rate limit exceeded")
)

type AuthError struct {
	Err error
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("GitHub authentication failed: %v", e.Err)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

func (e *AuthError) Is(target error) bool {
	return target == ErrAuth
}

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
	}
	return fmt.Sprintf("%s not found", e.Resource)
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

type RateLimitError struct {
	RetryAfter time.Duration
	Err        error
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("GitHub rate limit exceeded: %v", e.Err)
}

func (e *RateLimitError) Unwrap() error {
	return e.Err
}

func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimited
}
