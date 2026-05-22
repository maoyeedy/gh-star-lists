package command

import (
	"reflect"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func makeRepo(nameWithOwner, description, language string, stars int) domain.Repository {
	return domain.Repository{
		ID:             nameWithOwner,
		NameWithOwner:  nameWithOwner,
		Description:    description,
		Language:       language,
		StargazerCount: stars,
		URL:            "https://github.com/" + nameWithOwner,
	}
}

func repoNames(repos []domain.Repository) []string {
	out := make([]string, len(repos))
	for i, r := range repos {
		out[i] = r.NameWithOwner
	}
	return out
}

func TestSearchRepositories(t *testing.T) {
	t.Parallel()

	gin := makeRepo("gin-gonic/gin", "HTTP web framework written in Go", "Go", 70000)
	echo := makeRepo("labstack/echo", "High performance HTTP framework", "Go", 28000)
	react := makeRepo(
		"facebook/react",
		"A declarative library for building user interfaces",
		"JavaScript",
		220000,
	)
	rails := makeRepo("rails/rails", "Ruby on Rails web framework", "Ruby", 55000)
	all := []domain.Repository{gin, echo, react, rails}

	tests := []struct {
		name  string
		query string
		repos []domain.Repository
		want  []string
	}{
		{
			name:  "empty query returns all repos unchanged",
			query: "",
			repos: all,
			want:  []string{"gin-gonic/gin", "labstack/echo", "facebook/react", "rails/rails"},
		},
		{
			name:  "whitespace only returns all",
			query: "   \t  ",
			repos: all,
			want:  []string{"gin-gonic/gin", "labstack/echo", "facebook/react", "rails/rails"},
		},
		{
			name:  "no matches returns empty",
			query: "kubernetes",
			repos: all,
			want:  []string{},
		},
		{
			name:  "phrase bonus and name match rank gin first",
			query: "gin",
			repos: all,
			want:  []string{"gin-gonic/gin"},
		},
		{
			name:  "multi-term AND semantics filter out partial matches",
			query: "web framework",
			repos: all,
			want:  []string{"gin-gonic/gin", "rails/rails"},
		},
		{
			name:  "plural folding matches library/libraries",
			query: "library",
			repos: []domain.Repository{
				makeRepo("a/lib", "Reusable libraries for parsing", "Go", 100),
			},
			want: []string{"a/lib"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := searchRepositories(tt.repos, tt.query)
			names := repoNames(got)
			if !reflect.DeepEqual(names, tt.want) {
				t.Fatalf("searchRepositories(%q) = %v; want %v", tt.query, names, tt.want)
			}
		})
	}
}

func TestSearchRepositoriesOrdering(t *testing.T) {
	t.Parallel()

	exact := makeRepo("foo/go", "static site generator", "Rust", 100)
	prefix := makeRepo("foo/gopher", "tools", "Go", 200)
	descOnly := makeRepo("foo/zzz", "written in go", "Rust", 50)

	got := searchRepositories([]domain.Repository{descOnly, prefix, exact}, "go")
	if len(got) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(got))
	}
	if got[0].NameWithOwner != "foo/go" {
		t.Errorf("first should be exact name match foo/go, got %s", got[0].NameWithOwner)
	}
	if got[2].NameWithOwner != "foo/zzz" {
		t.Errorf("last should be description-only foo/zzz, got %s", got[2].NameWithOwner)
	}
}

func TestSearchRepositoriesStarsTiebreaker(t *testing.T) {
	t.Parallel()

	low := makeRepo("foo/echo", "x", "Go", 100)
	high := makeRepo("bar/echo", "x", "Go", 5000)

	got := searchRepositories([]domain.Repository{low, high}, "echo")
	if got[0].NameWithOwner != "bar/echo" {
		t.Errorf("higher stars should rank first, got %s", got[0].NameWithOwner)
	}
}
