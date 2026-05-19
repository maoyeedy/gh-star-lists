package command

import (
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"github.com/maoyeedy/gh-star-lists/internal/search"
)

func searchRepositories(repos []githubapi.Repository, query string) []githubapi.Repository {
	return search.FilterRepositories(repos, query)
}
