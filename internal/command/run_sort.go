package command

import (
	"sort"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func sortStarLists(lists []githubapi.StarList, sortKeys []string, sortTerms []SortTerm, desc bool) {
	if len(sortKeys) == 0 {
		return
	}

	sort.Slice(lists, func(i, j int) bool {
		cmp, termDesc := compareStarLists(lists[i], lists[j], sortKeys, sortTerms, desc)
		if termDesc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareStarLists(
	left, right githubapi.StarList,
	sortKeys []string,
	sortTerms []SortTerm,
	globalDesc bool,
) (int, bool) {
	for idx, key := range sortKeys {
		termDesc := globalDesc
		if len(sortTerms) > idx {
			termDesc = sortTerms[idx].Desc
		}
		var cmp int
		switch key {
		case SortKeyAdded:
			if left.LastAddedAt != right.LastAddedAt {
				cmp = strings.Compare(left.LastAddedAt, right.LastAddedAt)
			}
		case SortKeyName:
			leftName := strings.ToLower(left.Name)
			rightName := strings.ToLower(right.Name)
			if leftName != rightName {
				cmp = strings.Compare(leftName, rightName)
			}
		case SortKeyRepoCount:
			if left.RepoCount != right.RepoCount {
				cmp = left.RepoCount - right.RepoCount
			}
		}
		if cmp != 0 {
			return cmp, termDesc
		}
	}
	if left.Name != right.Name {
		return strings.Compare(left.Name, right.Name), false
	}
	return strings.Compare(left.ID, right.ID), false
}

func sortRepositories(
	repos []githubapi.Repository,
	sortKeys []string,
	sortTerms []SortTerm,
	desc bool,
) {
	if len(sortKeys) == 0 {
		return
	}

	sort.Slice(repos, func(i, j int) bool {
		cmp, termDesc := compareRepositories(repos[i], repos[j], sortKeys, sortTerms, desc)
		if termDesc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareRepositories(
	left, right githubapi.Repository,
	sortKeys []string,
	sortTerms []SortTerm,
	globalDesc bool,
) (int, bool) {
	for idx, key := range sortKeys {
		termDesc := globalDesc
		if len(sortTerms) > idx {
			termDesc = sortTerms[idx].Desc
		}
		var cmp int
		switch key {
		case SortKeyName:
			cmp = strings.Compare(
				strings.ToLower(left.NameWithOwner),
				strings.ToLower(right.NameWithOwner),
			)
		case SortKeyStars:
			if left.StargazerCount != right.StargazerCount {
				cmp = left.StargazerCount - right.StargazerCount
			}
		case SortKeyPushed:
			if left.PushedAt != right.PushedAt {
				cmp = strings.Compare(left.PushedAt, right.PushedAt)
			}
		case SortKeyLanguage:
			cmp = strings.Compare(strings.ToLower(left.Language), strings.ToLower(right.Language))
		case SortKeyStarred:
			if left.StarredAt != right.StarredAt {
				cmp = strings.Compare(left.StarredAt, right.StarredAt)
			}
		}
		if cmp != 0 {
			return cmp, termDesc
		}
	}
	if left.NameWithOwner != right.NameWithOwner {
		return strings.Compare(left.NameWithOwner, right.NameWithOwner), false
	}
	return strings.Compare(left.URL, right.URL), false
}
