package githubapi

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

type fakeRetryExecutor struct {
	responses []error // return these in sequence; last one repeated if exhausted
	calls     int
}

func (f *fakeRetryExecutor) DoWithContext(
	_ context.Context,
	_ string,
	_ map[string]any,
	_ any,
) error {
	idx := f.calls
	if idx >= len(f.responses) {
		idx = len(f.responses) - 1
	}
	f.calls++
	return f.responses[idx]
}

func noopSleep(_ context.Context, _ time.Duration) error { return nil }

func newTestRetryDoer(inner graphQLDoer, maxAttempts int) *retryDoer {
	return &retryDoer{
		inner:       inner,
		maxAttempts: maxAttempts,
		baseDelay:   0,
		sleep:       noopSleep,
	}
}

func TestRetryDoer_SuccessOnFirstCall(t *testing.T) {
	exec := &fakeRetryExecutor{responses: []error{nil}}
	r := newTestRetryDoer(exec, 3)
	if err := r.DoWithContext(context.Background(), "", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.calls != 1 {
		t.Fatalf("expected 1 call, got %d", exec.calls)
	}
}

func TestRetryDoer_TransientThenSuccess(t *testing.T) {
	transient := errors.New("secondary rate limit exceeded")
	exec := &fakeRetryExecutor{responses: []error{transient, transient, nil}}
	r := newTestRetryDoer(exec, 3)
	if err := r.DoWithContext(context.Background(), "", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.calls != 3 {
		t.Fatalf("expected 3 calls, got %d", exec.calls)
	}
}

func TestRetryDoer_TransientEveryAttempt(t *testing.T) {
	transient := errors.New("rate limit exceeded")
	exec := &fakeRetryExecutor{responses: []error{transient}}
	r := newTestRetryDoer(exec, 3)
	err := r.DoWithContext(context.Background(), "", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if exec.calls != 3 {
		t.Fatalf("expected 3 calls, got %d", exec.calls)
	}
	if !errors.Is(err, transient) {
		t.Fatalf("expected transient error, got %v", err)
	}
}

func TestRetryDoer_NonTransientNoRetry(t *testing.T) {
	permanent := errors.New("not found")
	exec := &fakeRetryExecutor{responses: []error{permanent}}
	r := newTestRetryDoer(exec, 3)
	err := r.DoWithContext(context.Background(), "", nil, nil)
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent error, got %v", err)
	}
	if exec.calls != 1 {
		t.Fatalf("expected 1 call (no retry), got %d", exec.calls)
	}
}

func TestRetryDoer_ContextCanceledDuringSleep(t *testing.T) {
	transient := errors.New("secondary rate limit exceeded")
	exec := &fakeRetryExecutor{responses: []error{transient}}
	ctx, cancel := context.WithCancel(context.Background())
	r := &retryDoer{
		inner:       exec,
		maxAttempts: 3,
		baseDelay:   0,
		sleep: func(ctx context.Context, _ time.Duration) error {
			cancel()
			return ctx.Err()
		},
	}
	err := r.DoWithContext(ctx, "", nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if exec.calls != 1 {
		t.Fatalf("expected 1 call before cancel, got %d", exec.calls)
	}
}

func TestRetryDoer_ContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	exec := &fakeRetryExecutor{responses: []error{nil}}
	r := newTestRetryDoer(exec, 3)
	err := r.DoWithContext(ctx, "", nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if exec.calls != 0 {
		t.Fatalf("expected 0 calls, got %d", exec.calls)
	}
}

func TestIsTransientGraphQLError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{"nil", nil, false},
		{"generic error", errors.New("not found"), false},
		{"secondary rate limit string", errors.New("secondary rate limit exceeded"), true},
		{"rate limit exceeded string", errors.New("rate limit exceeded"), true},
		{"case insensitive", errors.New("Secondary Rate Limit"), true},
		{
			"HTTPError 429",
			&api.HTTPError{StatusCode: 429},
			true,
		},
		{
			"HTTPError 500",
			&api.HTTPError{StatusCode: 500},
			true,
		},
		{
			"HTTPError 503",
			&api.HTTPError{StatusCode: 503},
			true,
		},
		{
			"HTTPError 404",
			&api.HTTPError{StatusCode: 404},
			false,
		},
		{
			"GraphQLError rate limit",
			&api.GraphQLError{
				Errors: []api.GraphQLErrorItem{{Message: "secondary rate limit exceeded"}},
			},
			true,
		},
		{
			"GraphQLError non-rate-limit",
			&api.GraphQLError{Errors: []api.GraphQLErrorItem{{Message: "field does not exist"}}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientGraphQLError(tt.err)
			if got != tt.transient {
				t.Fatalf("isTransientGraphQLError(%v) = %v, want %v", tt.err, got, tt.transient)
			}
		})
	}
}

func TestRetryAfterDelay(t *testing.T) {
	t.Run("no header returns zero", func(t *testing.T) {
		err := &api.HTTPError{StatusCode: 429, Headers: http.Header{}}
		if d := retryAfterDelay(err); d != 0 {
			t.Fatalf("expected 0, got %v", d)
		}
	})
	t.Run("valid header returns duration", func(t *testing.T) {
		h := http.Header{}
		h.Set("Retry-After", "5")
		err := &api.HTTPError{StatusCode: 429, Headers: h}
		if d := retryAfterDelay(err); d != 5*time.Second {
			t.Fatalf("expected 5s, got %v", d)
		}
	})
	t.Run("non-http error returns zero", func(t *testing.T) {
		if d := retryAfterDelay(errors.New("some error")); d != 0 {
			t.Fatalf("expected 0, got %v", d)
		}
	})
}

func TestBackoffWithJitter(t *testing.T) {
	base := time.Second
	for attempt := range 5 {
		d := backoffWithJitter(base, attempt)
		if d <= 0 {
			t.Fatalf("attempt %d: delay must be positive, got %v", attempt, d)
		}
		if d > 30*time.Second+time.Second { // cap + a little slack for jitter
			t.Fatalf("attempt %d: delay %v exceeds 30s cap", attempt, d)
		}
	}
}
