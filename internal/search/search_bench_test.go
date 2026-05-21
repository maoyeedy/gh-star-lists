// Benchmark results (2026-05-21, linux/amd64, i5-1135G7 @ 2.40GHz):
//
// BenchmarkFilterRepositories_500-8    560    2173078 ns/op    627131 B/op    4536 allocs/op
// BenchmarkFilterRepositories_5000-8    44   24594505 ns/op   7353334 B/op   45155 allocs/op
// BenchmarkFilterStarLists_500-8       567    2037170 ns/op    505104 B/op    3030 allocs/op
//
// FilterRepositories_500 = ~2.17 ms/op, well below the 5 ms debounce threshold.
// Search stays synchronous -- no tea.Tick debounce needed.
package search_test

import (
	"fmt"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"github.com/maoyeedy/gh-star-lists/internal/search"
)

func makeBenchRepos(n int) []githubapi.Repository {
	langs := []string{"Go", "Python", "TypeScript", "Rust", "Ruby"}
	repos := make([]githubapi.Repository, n)
	for i := range repos {
		repos[i] = githubapi.Repository{
			NameWithOwner:  fmt.Sprintf("owner/repo-%d", i),
			Description:    fmt.Sprintf("Description for repo %d with some extra text", i),
			Language:       langs[i%len(langs)],
			StargazerCount: i * 10,
		}
	}
	return repos
}

func makeBenchLists(n int) []githubapi.StarList {
	lists := make([]githubapi.StarList, n)
	for i := range lists {
		lists[i] = githubapi.StarList{
			Name:        fmt.Sprintf("list-%d", i),
			Description: fmt.Sprintf("A list about topic %d with data and things", i),
		}
	}
	return lists
}

func BenchmarkFilterRepositories_500(b *testing.B) {
	repos := makeBenchRepos(500)
	b.ResetTimer()
	for range b.N {
		_ = search.FilterRepositories(repos, "go")
	}
}

func BenchmarkFilterRepositories_5000(b *testing.B) {
	repos := makeBenchRepos(5000)
	b.ResetTimer()
	for range b.N {
		_ = search.FilterRepositories(repos, "go")
	}
}

func BenchmarkFilterStarLists_500(b *testing.B) {
	lists := makeBenchLists(500)
	b.ResetTimer()
	for range b.N {
		_ = search.FilterStarLists(lists, "data")
	}
}
