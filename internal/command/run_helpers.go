package command

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func sameStringSet(before []string, after []string) bool {
	if len(before) != len(after) {
		return false
	}
	counts := make(map[string]int, len(before))
	for _, value := range before {
		counts[value]++
	}
	for _, value := range after {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
}

func displayListName(list domain.StarList, fallback string) string {
	if list.Name != "" {
		return list.Name
	}
	return fallback
}

func listByRaw(lists []domain.StarList, raw string) (domain.StarList, bool) {
	for _, l := range lists {
		if strings.EqualFold(l.Name, raw) || l.ID == raw {
			return l, true
		}
	}
	return domain.StarList{}, false
}

func actionNeedsFrom(a Action) bool {
	return a == ActionRemove || a == ActionMove || a == ActionCopy
}

func actionNeedsTo(a Action) bool {
	return a == ActionAdd || a == ActionMove || a == ActionCopy
}

func missingSelectorError(a Action, needFrom, needTo bool) error {
	switch {
	case needFrom && needTo:
		return usage("%s requires --from and --to (or run in a TTY to choose interactively)", a)
	case needFrom:
		return usage(
			"%s requires --from <LIST_ID_OR_NAME> (or run in a TTY to choose interactively)",
			a,
		)
	default:
		return usage(
			"%s requires --to <LIST_ID_OR_NAME> (or run in a TTY to choose interactively)",
			a,
		)
	}
}

func pickList(lists []domain.StarList, label, excludeID string) (string, error) {
	var filtered []domain.StarList
	var choices []string
	nameCount := make(map[string]int)
	for _, l := range lists {
		if l.ID == excludeID {
			continue
		}
		nameCount[l.Name]++
	}
	for _, l := range lists {
		if l.ID == excludeID {
			continue
		}
		filtered = append(filtered, l)
		if nameCount[l.Name] > 1 {
			choices = append(choices, fmt.Sprintf("%s (%s, %d repos)", l.Name, l.ID, l.RepoCount))
		} else {
			choices = append(choices, fmt.Sprintf("%s (%d repos)", l.Name, l.RepoCount))
		}
	}
	if len(choices) == 0 {
		return "", fmt.Errorf("no eligible Star Lists to select")
	}
	idx, err := promptForList(label, "", choices)
	if err != nil {
		return "", err
	}
	if idx < 0 || idx >= len(filtered) {
		return "", fmt.Errorf("invalid Star List selection")
	}
	return filtered[idx].ID, nil
}

type resolvedList struct {
	ID   string
	URL  string
	Name string
}

func resolveList(ctx context.Context, service githubapi.Service, raw string) (resolvedList, error) {
	if raw == "" {
		return resolvedList{}, nil
	}
	lists, err := service.ListStarLists(ctx)
	if err != nil {
		return resolvedList{}, err
	}
	if l, ok := listByRaw(lists, raw); ok {
		return resolvedList{ID: l.ID, URL: l.URL, Name: l.Name}, nil
	}
	return resolvedList{ID: raw}, nil
}

func argsContainNoColor(args []string) bool {
	return slices.Contains(args, "--no-color")
}
