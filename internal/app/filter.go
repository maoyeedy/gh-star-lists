package app

import (
	"slices"
	"strconv"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func filterStarLists(lists []domain.StarList, filters []Filter) []domain.StarList {
	if len(filters) == 0 {
		return lists
	}
	out := make([]domain.StarList, 0, len(lists))
	for _, l := range lists {
		keep := true
		for _, f := range filters {
			if f.Key == FilterKeyName && !strings.Contains(strings.ToLower(l.Name), f.Value) {
				keep = false
			}
		}
		if keep {
			out = append(out, l)
		}
	}
	return out
}

func filterRepositories(repos []domain.Repository, filters []Filter) []domain.Repository {
	if len(filters) == 0 {
		return repos
	}
	out := make([]domain.Repository, 0, len(repos))
	for _, r := range repos {
		keep := true
		for _, f := range filters {
			switch f.Key {
			case FilterKeyName:
				if !strings.Contains(strings.ToLower(r.NameWithOwner), f.Value) {
					keep = false
				}
			case FilterKeyFork:
				wantFork := f.Value == "true"
				if r.IsFork != wantFork {
					keep = false
				}
			case FilterKeyLanguage:
				if strings.ToLower(r.Language) != f.Value {
					keep = false
				}
			case FilterKeyArchived:
				wantArchived := f.Value == "true"
				if r.IsArchived != wantArchived {
					keep = false
				}
			case FilterKeyLicense:
				if strings.ToLower(r.License) != f.Value {
					keep = false
				}
			case FilterKeyMinStars:
				minStars, _ := strconv.Atoi(f.Value)
				if r.StargazerCount < minStars {
					keep = false
				}
			case FilterKeyMaxStars:
				maxStars, _ := strconv.Atoi(f.Value)
				if r.StargazerCount > maxStars {
					keep = false
				}
			case FilterKeyTopic:
				if !hasTopic(r.Topics, f.Value) {
					keep = false
				}
			}
		}
		if keep {
			out = append(out, r)
		}
	}
	return out
}

func hasTopic(topics []string, want string) bool {
	return slices.ContainsFunc(topics, func(t string) bool {
		return strings.ToLower(t) == want
	})
}
