package format

import (
	"strconv"
	"strings"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/humanize"
)

// RepoRowFromDomain constructs a RepoRow from a domain Repository.
func RepoRowFromDomain(repo domain.Repository, now time.Time) domain.RepoRow {
	owner, name, _ := strings.Cut(repo.NameWithOwner, "/")
	return domain.RepoRow{
		Owner:          owner,
		Name:           name,
		Stars:          strconv.Itoa(repo.StargazerCount),
		PushedAge:      humanize.ShortAge(repo.PushedAt, now),
		Fork:           yesNo(repo.IsFork),
		NameWithOwner:  repo.NameWithOwner,
		Description:    repo.Description,
		IsFork:         repo.IsFork,
		IsArchived:     repo.IsArchived,
		StargazerCount: repo.StargazerCount,
		PushedAt:       repo.PushedAt,
		URL:            repo.URL,
		Language:       repo.Language,
		StarredAt:      repo.StarredAt,
	}
}

// ListRowFromDomain constructs a ListRow from a domain StarList.
func ListRowFromDomain(list domain.StarList, now time.Time) domain.ListRow {
	return domain.ListRow{
		RepoCountStr: strconv.Itoa(list.RepoCount),
		LastAddedAge: humanize.ShortAge(list.LastAddedAt, now),
		Name:         list.Name,
		Description:  list.Description,
		LastAddedAt:  list.LastAddedAt,
		IsPrivate:    list.IsPrivate,
		ID:           list.ID,
		RepoCount:    list.RepoCount,
		URL:          list.URL,
	}
}
