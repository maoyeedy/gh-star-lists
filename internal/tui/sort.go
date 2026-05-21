package tui

import (
	"sort"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type sortListsKey int

const (
	sortListsGitHub sortListsKey = iota
	sortListsName
	sortListsRepos
	sortListsAdded
)

type sortReposKey int

const (
	sortReposGitHub sortReposKey = iota
	sortReposName
	sortReposStars
	sortReposPushed
	sortReposLanguage
	sortReposStarredAt
)

func sortStarLists(lists []githubapi.StarList, key sortListsKey) {
	sort.Slice(lists, func(i, j int) bool {
		switch key {
		case sortListsName:
			a, b := strings.ToLower(lists[i].Name), strings.ToLower(lists[j].Name)
			if a != b {
				return a < b
			}
		case sortListsRepos:
			if lists[i].RepoCount != lists[j].RepoCount {
				return lists[i].RepoCount > lists[j].RepoCount
			}
		case sortListsAdded:
			if lists[i].LastAddedAt != lists[j].LastAddedAt {
				return lists[i].LastAddedAt > lists[j].LastAddedAt
			}
		case sortListsGitHub:
			// server order - no sort
			return false
		}
		return lists[i].ID < lists[j].ID
	})
}

func sortRepos(repos []githubapi.Repository, key sortReposKey) {
	sort.Slice(repos, func(i, j int) bool {
		switch key {
		case sortReposName:
			a, b := strings.ToLower(repos[i].NameWithOwner), strings.ToLower(repos[j].NameWithOwner)
			if a != b {
				return a < b
			}
		case sortReposStars:
			if repos[i].StargazerCount != repos[j].StargazerCount {
				return repos[i].StargazerCount > repos[j].StargazerCount
			}
		case sortReposPushed:
			if repos[i].PushedAt != repos[j].PushedAt {
				return repos[i].PushedAt > repos[j].PushedAt
			}
		case sortReposLanguage:
			a, b := strings.ToLower(repos[i].Language), strings.ToLower(repos[j].Language)
			// empty language sorts last
			if a == "" && b != "" {
				return false
			}
			if a != "" && b == "" {
				return true
			}
			if a != b {
				return a < b
			}
		case sortReposStarredAt:
			// descending: newer first; empty StarredAt sorts last
			ai, aj := repos[i].StarredAt, repos[j].StarredAt
			if ai == "" && aj != "" {
				return false
			}
			if ai != "" && aj == "" {
				return true
			}
			if ai != aj {
				return ai > aj
			}
		case sortReposGitHub:
			// server order - no sort
			return false
		}
		return repos[i].NameWithOwner < repos[j].NameWithOwner
	})
}

func (m model) cycleSort() model {
	if m.active == paneList {
		m.sortLists = (m.sortLists + 1) % 4
		sortStarLists(m.lists, m.sortLists)
		m.listCursor = 0
		m.listOffset = 0
	} else {
		m.sortRepos = (m.sortRepos + 1) % 6
		sortRepos(m.displayedRepos, m.sortRepos)
		m.repoCursor = 0
		m.previewOffset = 0
		m.repoOffset = 0
	}
	return m
}
