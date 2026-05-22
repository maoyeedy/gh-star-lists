package search

import (
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type preparedField struct {
	text   string
	weight int
}

func (f preparedField) scoreTerm(term string, editPrev, editCurr *[]int) int {
	if f.text == term {
		return f.weight + 100
	}
	best := 0
	walkTokens(f.text, func(start, end int) bool {
		token := f.text[start:end]
		switch {
		case equivalentToken(token, term):
			best = max(best, f.weight+80)
		case strings.HasPrefix(token, term):
			best = max(best, f.weight+65)
		case strings.Contains(token, term):
			best = max(best, f.weight+45)
		case isSubsequence(term, token):
			best = max(best, f.weight+20)
		}
		if distance, ok := boundedEditDistance(
			term, token, maxDistance(term), editPrev, editCurr,
		); ok {
			best = max(best, f.weight+35-(distance*10))
		}
		return true
	})
	if strings.Contains(f.text, term) {
		best = max(best, f.weight+50)
	}
	return best
}

func scorePrepared(
	prepared []preparedField,
	terms []string,
	phrase string,
	editPrev, editCurr *[]int,
) int {
	if len(terms) == 0 {
		return 0
	}

	score := 0
	if phrase != "" {
		for _, field := range prepared {
			if strings.Contains(field.text, phrase) {
				score += 120
				break
			}
		}
	}
	for _, term := range terms {
		best := 0
		for _, field := range prepared {
			if s := field.scoreTerm(term, editPrev, editCurr); s > best {
				best = s
			}
		}
		if best == 0 {
			return 0
		}
		score += best
	}
	return score
}

func scoreRepository(
	repo githubapi.Repository,
	terms []string,
	phrase string,
	editPrev, editCurr *[]int,
) int {
	normNameWithOwner := repo.NormNameWithOwner
	if normNameWithOwner == "" {
		normNameWithOwner = normalize(repo.NameWithOwner)
	}
	normDesc := repo.NormDescription
	if normDesc == "" && repo.Description != "" {
		normDesc = normalize(repo.Description)
	}
	normLang := repo.NormLanguage
	if normLang == "" && repo.Language != "" {
		normLang = normalize(repo.Language)
	}
	normOwner, normName, _ := strings.Cut(normNameWithOwner, "/")

	prepared := make([]preparedField, 0, 5)

	if normName != "" {
		prepared = append(prepared, preparedField{normName, 120})
	}
	if normNameWithOwner != "" {
		prepared = append(prepared, preparedField{normNameWithOwner, 100})
	}
	if normOwner != "" {
		prepared = append(prepared, preparedField{normOwner, 80})
	}
	if normDesc != "" {
		prepared = append(prepared, preparedField{normDesc, 55})
	}
	if normLang != "" {
		prepared = append(prepared, preparedField{normLang, 45})
	}

	return scorePrepared(prepared, terms, phrase, editPrev, editCurr)
}

func scoreStarList(
	list githubapi.StarList,
	terms []string,
	phrase string,
	editPrev, editCurr *[]int,
) int {
	normName := list.NormName
	if normName == "" && list.Name != "" {
		normName = normalize(list.Name)
	}
	normDesc := list.NormDescription
	if normDesc == "" && list.Description != "" {
		normDesc = normalize(list.Description)
	}

	prepared := make([]preparedField, 0, 2)

	if normName != "" {
		prepared = append(prepared, preparedField{normName, 120})
	}
	if normDesc != "" {
		prepared = append(prepared, preparedField{normDesc, 70})
	}

	return scorePrepared(prepared, terms, phrase, editPrev, editCurr)
}
