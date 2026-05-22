package command

import (
	"fmt"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/app"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func makeBenchStarLists(n int) []domain.StarList {
	lists := make([]domain.StarList, n)
	for i := 0; i < n; i++ {
		lists[i] = domain.StarList{
			Name:        "list-" + string(rune('A'+i%26)),
			Description: "bench",
			LastAddedAt: "2025-01-01T00:00:00Z",
			ID:          "UL_" + string(rune('0'+i%10)),
			URL:         "https://github.com/stars/maoyeedy/lists/list-" + string(rune('A'+i%26)),
		}
	}
	return lists
}

func makeBenchRepos(n int) []domain.Repository {
	repos := make([]domain.Repository, n)
	for i := 0; i < n; i++ {
		repos[i] = domain.Repository{
			NameWithOwner:  "owner/repo-" + string(rune('A'+i%26)),
			Description:    "bench",
			IsFork:         i%2 == 0,
			StargazerCount: i * 100,
			PushedAt:       "2025-01-01T00:00:00Z",
			URL:            "https://github.com/owner/repo-" + string(rune('A'+i%26)),
		}
	}
	return repos
}

func BenchmarkSortStarLists(b *testing.B) {
	lists := makeBenchStarLists(1000)
	sortKeys := []string{"name"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cp := make([]domain.StarList, len(lists))
		copy(cp, lists)
		app.SortStarLists(cp, sortKeys, nil, false)
	}
}

func BenchmarkSortRepositories(b *testing.B) {
	repos := makeBenchRepos(1000)
	sortKeys := []string{"stars"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cp := make([]domain.Repository, len(repos))
		copy(cp, repos)
		app.SortRepositories(cp, sortKeys, nil, true)
	}
}

func makeBenchReposForSearch(n int) []domain.Repository {
	langs := []string{"Go", "Rust", "Python", "TypeScript", "C++"}
	descs := []string{
		"web framework with routing",
		"CLI tool for git workflows",
		"distributed key-value store",
		"static site generator",
		"machine learning toolkit",
	}
	repos := make([]domain.Repository, n)
	for i := range repos {
		repos[i] = domain.Repository{
			ID:            fmt.Sprintf("R_%d", i),
			NameWithOwner: fmt.Sprintf("owner%d/repo-%d", i%500, i),
			Description:   descs[i%len(descs)],
			Language:      langs[i%len(langs)],
			URL:           fmt.Sprintf("https://example.test/repo-%d", i),
		}
	}
	return repos
}

func BenchmarkSearchRepositories(b *testing.B) {
	repos := makeBenchReposForSearch(5000)
	query := "web framework go"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = searchRepositories(repos, query)
	}
}

func BenchmarkSortStarListsMultiKey(b *testing.B) {
	lists := makeBenchStarLists(1000)
	sortKeys := []string{"added", "name"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cp := make([]domain.StarList, len(lists))
		copy(cp, lists)
		app.SortStarLists(cp, sortKeys, nil, false)
	}
}
