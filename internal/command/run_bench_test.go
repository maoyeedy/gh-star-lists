package command

import (
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func makeBenchStarLists(n int) []githubapi.StarList {
	lists := make([]githubapi.StarList, n)
	for i := 0; i < n; i++ {
		lists[i] = githubapi.StarList{
			Name:        "list-" + string(rune('A'+i%26)),
			Description: "bench",
			LastAddedAt: "2025-01-01T00:00:00Z",
			ID:          "UL_" + string(rune('0'+i%10)),
			URL:         "https://github.com/stars/maoyeedy/lists/list-" + string(rune('A'+i%26)),
		}
	}
	return lists
}

func makeBenchRepos(n int) []githubapi.Repository {
	repos := make([]githubapi.Repository, n)
	for i := 0; i < n; i++ {
		repos[i] = githubapi.Repository{
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
		cp := make([]githubapi.StarList, len(lists))
		copy(cp, lists)
		sortStarLists(cp, sortKeys, nil, false)
	}
}

func BenchmarkSortRepositories(b *testing.B) {
	repos := makeBenchRepos(1000)
	sortKeys := []string{"stars"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cp := make([]githubapi.Repository, len(repos))
		copy(cp, repos)
		sortRepositories(cp, sortKeys, nil, true)
	}
}

func BenchmarkSortStarListsMultiKey(b *testing.B) {
	lists := makeBenchStarLists(1000)
	sortKeys := []string{"added", "name"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cp := make([]githubapi.StarList, len(lists))
		copy(cp, lists)
		sortStarLists(cp, sortKeys, nil, false)
	}
}
