package search

import (
	"slices"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

// FilterRepositories returns repos matching query ranked by relevance
// (score desc, stars desc, name asc). Returns repos unchanged if query is empty.
func FilterRepositories(repos []githubapi.Repository, query string) []githubapi.Repository {
	query = strings.TrimSpace(query)
	if query == "" {
		return repos
	}
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil
	}
	phrase := normalize(query)

	var editPrev, editCurr []int

	type repoMatch struct {
		repo    githubapi.Repository
		score   int
		sortKey string
	}
	matches := make([]repoMatch, 0, len(repos))
	for _, repo := range repos {
		score := scoreRepository(repo, terms, phrase, &editPrev, &editCurr)
		if score > 0 {
			matches = append(matches, repoMatch{
				repo:    repo,
				score:   score,
				sortKey: strings.ToLower(repo.NameWithOwner),
			})
		}
	}
	slices.SortFunc(matches, func(a, b repoMatch) int {
		if a.score != b.score {
			return b.score - a.score
		}
		if a.repo.StargazerCount != b.repo.StargazerCount {
			return b.repo.StargazerCount - a.repo.StargazerCount
		}
		return strings.Compare(a.sortKey, b.sortKey)
	})
	out := make([]githubapi.Repository, len(matches))
	for i, m := range matches {
		out[i] = m.repo
	}
	return out
}

// FilterStarLists returns star lists matching query ranked by relevance
// (score desc, name asc). Returns lists unchanged if query is empty.
func FilterStarLists(lists []githubapi.StarList, query string) []githubapi.StarList {
	query = strings.TrimSpace(query)
	if query == "" {
		return lists
	}
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil
	}
	phrase := normalize(query)

	var editPrev, editCurr []int

	type listMatch struct {
		list    githubapi.StarList
		score   int
		sortKey string
	}
	matches := make([]listMatch, 0, len(lists))
	for _, list := range lists {
		score := scoreStarList(list, terms, phrase, &editPrev, &editCurr)
		if score > 0 {
			matches = append(matches, listMatch{
				list:    list,
				score:   score,
				sortKey: strings.ToLower(list.Name),
			})
		}
	}
	slices.SortFunc(matches, func(a, b listMatch) int {
		if a.score != b.score {
			return b.score - a.score
		}
		return strings.Compare(a.sortKey, b.sortKey)
	})
	out := make([]githubapi.StarList, len(matches))
	for i, m := range matches {
		out[i] = m.list
	}
	return out
}
