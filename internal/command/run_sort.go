package command

import "github.com/maoyeedy/gh-star-lists/internal/app"

func toAppSortTerms(terms []SortTerm) []app.SortTerm {
	out := make([]app.SortTerm, len(terms))
	for i, t := range terms {
		out[i] = app.SortTerm{Key: t.Key, Desc: t.Desc}
	}
	return out
}
