package githubapi

import (
	"context"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func TestMergeStarredAtMatchesByIDThenName(t *testing.T) {
	t.Parallel()

	repos := []domain.Repository{
		{ID: "R_1", NameWithOwner: "Owner/Repo"},
		{ID: "R_2", NameWithOwner: "owner/second"},
		{ID: "R_3", NameWithOwner: "owner/preserved", StarredAt: "2026-01-01T00:00:00Z"},
	}
	starred := []domain.Repository{
		{ID: "R_1", NameWithOwner: "other/name", StarredAt: "2026-03-01T00:00:00Z"},
		{NameWithOwner: "OWNER/SECOND", StarredAt: "2026-02-01T00:00:00Z"},
		{ID: "R_3", NameWithOwner: "owner/preserved", StarredAt: "2026-04-01T00:00:00Z"},
	}

	got := MergeStarredAt(repos, starred)

	if got[0].StarredAt != "2026-03-01T00:00:00Z" {
		t.Fatalf("got[0].StarredAt = %q", got[0].StarredAt)
	}
	if got[1].StarredAt != "2026-02-01T00:00:00Z" {
		t.Fatalf("got[1].StarredAt = %q", got[1].StarredAt)
	}
	if got[2].StarredAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("got[2].StarredAt = %q, want preserved value", got[2].StarredAt)
	}
	if repos[0].StarredAt != "" {
		t.Fatalf("MergeStarredAt mutated input repos: %#v", repos)
	}
}

func TestWithStarredAtSkipsFetchWhenAlreadyPresent(t *testing.T) {
	t.Parallel()

	svc := &fakeCacheInner{}
	repos := []domain.Repository{{NameWithOwner: "owner/repo", StarredAt: "2026-03-01T00:00:00Z"}}

	got, err := WithStarredAt(context.Background(), svc, repos)
	if err != nil {
		t.Fatalf("WithStarredAt returned error: %v", err)
	}
	if got[0].StarredAt != repos[0].StarredAt {
		t.Fatalf("got StarredAt = %q, want %q", got[0].StarredAt, repos[0].StarredAt)
	}
	if svc.starredCalls != 0 {
		t.Fatalf("starredCalls = %d, want 0", svc.starredCalls)
	}
}
