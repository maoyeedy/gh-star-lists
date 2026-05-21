// Benchmark results (2026-05-21, linux/amd64, i5-1135G7 @ 2.40GHz):
//
// BenchmarkFilterRepositories_500-8    10000    433204 ns/op    155760 B/op    7 allocs/op
// BenchmarkFilterRepositories_5000-8    1033   3392394 ns/op   1474672 B/op    7 allocs/op
// BenchmarkFilterStarLists_500-8        8469    415048 ns/op    147664 B/op    7 allocs/op
//
// 7 allocs/op regardless of input size: matches slice (1) + out slice (1) +
// editPrev/editCurr first-grow (2) + slices.SortFunc pdqsort temp (3).
// Bench data sets NormX fields to simulate production repos from the GraphQL layer.
// Without NormX (fallback path): ~1007 allocs/500 repos from normalize() on
// mixed-case Description and Language.
//
// FilterRepositories_500 = ~0.43 ms/op, well below the 5 ms debounce threshold.
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
	normLangs := []string{"go", "python", "typescript", "rust", "ruby"}
	repos := make([]githubapi.Repository, n)
	for i := range repos {
		nwo := fmt.Sprintf("owner/repo-%d", i)
		desc := fmt.Sprintf("Description for repo %d with some extra text", i)
		lang := langs[i%len(langs)]
		repos[i] = githubapi.Repository{
			NameWithOwner:     nwo,
			Description:       desc,
			Language:          lang,
			StargazerCount:    i * 10,
			NormNameWithOwner: nwo, // already lowercase
			NormDescription:   fmt.Sprintf("description for repo %d with some extra text", i),
			NormLanguage:      normLangs[i%len(normLangs)],
		}
	}
	return repos
}

func makeBenchLists(n int) []githubapi.StarList {
	lists := make([]githubapi.StarList, n)
	for i := range lists {
		name := fmt.Sprintf("list-%d", i)
		desc := fmt.Sprintf("A list about topic %d with data and things", i)
		lists[i] = githubapi.StarList{
			Name:            name,
			Description:     desc,
			NormName:        name, // already lowercase
			NormDescription: fmt.Sprintf("a list about topic %d with data and things", i),
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
