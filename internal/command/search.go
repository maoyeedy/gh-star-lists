package command

import (
	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/search"
)

func searchRepositories(repos []domain.Repository, query string) []domain.Repository {
	return search.FilterRepositories(repos, query)
}
