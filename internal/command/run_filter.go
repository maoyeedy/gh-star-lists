package command

import "github.com/maoyeedy/gh-star-lists/internal/app"

func toAppFilters(filters []Filter) []app.Filter {
	out := make([]app.Filter, len(filters))
	for i, f := range filters {
		out[i] = app.Filter{Key: f.Key, Value: f.Value}
	}
	return out
}
