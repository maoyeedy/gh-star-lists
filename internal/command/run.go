package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitUsage   = 2
)

// Run does not construct GitHub clients, so help and usage paths remain
// auth-free and testable.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, service githubapi.Service) int {
	return RunWithOptions(ctx, args, stdout, stderr, service, format.DefaultOptions)
}

// RunWithOptions is Run with injectable output settings for deterministic tests.
func RunWithOptions(ctx context.Context, args []string, stdout, stderr io.Writer, service githubapi.Service, outputOptionsForMode func(format.OutputMode) format.Options) int {
	parsed, err := Parse(args)
	if err != nil {
		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			_ = writeDiagnostic(stderr, "error: %s\n\n%s", usageErr.Message, UsageText())
			return ExitUsage
		}
		return writeFailure(stderr, err)
	}

	if parsed.Action == ActionHelp {
		if _, err := io.WriteString(stdout, HelpText()); err != nil {
			_ = writeDiagnostic(stderr, "error: failed to write help: %v\n", err)
			return ExitFailure
		}
		return ExitSuccess
	}

	if service == nil {
		_ = writeDiagnostic(stderr, "error: GitHub service is not configured\n")
		return ExitFailure
	}

	if outputOptionsForMode == nil {
		outputOptionsForMode = format.DefaultOptions
	}
	outputOptions := outputOptionsForMode(parsed.Mode)
	outputOptions.Template = parsed.Template
	if parsed.NoColor {
		outputOptions.Color = false
	}
	if parsed.Cache {
		service = githubapi.NewCacheService(service)
	}
	if parsed.OutputPath != "" {
		f, err := os.OpenFile(parsed.OutputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			_ = writeDiagnostic(stderr, "error: failed to open output file: %v\n", err)
			return ExitFailure
		}
		defer f.Close()
		stdout = f
	}
	switch parsed.Action {
	case ActionList:
		lists, err := service.ListStarLists(ctx)
		if err != nil {
			return writeRuntimeFailure(stderr, ActionList, "", err)
		}
		lists = filterStarLists(lists, parsed.Filters)
		sortStarLists(lists, parsed.SortKeys, parsed.SortDesc)
		if parsed.Limit > 0 && len(lists) > parsed.Limit {
			lists = lists[:parsed.Limit]
		}
		if err := format.WriteStarListsWithOptions(stdout, outputOptions, lists); err != nil {
			return writeFailure(stderr, fmt.Errorf("failed to write output: %w", err))
		}
	case ActionRepos:
		resolvedID, err := resolveListID(ctx, service, parsed.ListID)
		if err != nil {
			return writeRuntimeFailure(stderr, ActionRepos, parsed.ListID, err)
		}
		repos, err := service.ListRepositories(ctx, resolvedID)
		if err != nil {
			return writeRuntimeFailure(stderr, ActionRepos, parsed.ListID, err)
		}
		repos = filterRepositories(repos, parsed.Filters)
		sortRepositories(repos, parsed.SortKeys, parsed.SortDesc)
		if parsed.Limit > 0 && len(repos) > parsed.Limit {
			repos = repos[:parsed.Limit]
		}
		if err := format.WriteRepositoriesWithOptions(stdout, outputOptions, repos); err != nil {
			return writeFailure(stderr, fmt.Errorf("failed to write output: %w", err))
		}
	default:
		panic(fmt.Sprintf("unhandled action %q - this is a bug in Parse", parsed.Action))
	}

	return ExitSuccess
}

func resolveListID(ctx context.Context, service githubapi.Service, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	lists, err := service.ListStarLists(ctx)
	if err != nil {
		return "", err
	}
	for _, l := range lists {
		if strings.EqualFold(l.Name, raw) {
			return l.ID, nil
		}
	}
	return raw, nil
}

func filterStarLists(lists []githubapi.StarList, filters []Filter) []githubapi.StarList {
	if len(filters) == 0 {
		return lists
	}
	out := make([]githubapi.StarList, 0, len(lists))
	for _, l := range lists {
		keep := true
		for _, f := range filters {
			switch f.Key {
			case FilterKeyName:
				if !strings.Contains(strings.ToLower(l.Name), f.Value) {
					keep = false
				}
			}
		}
		if keep {
			out = append(out, l)
		}
	}
	return out
}

func filterRepositories(repos []githubapi.Repository, filters []Filter) []githubapi.Repository {
	if len(filters) == 0 {
		return repos
	}
	out := make([]githubapi.Repository, 0, len(repos))
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
			}
		}
		if keep {
			out = append(out, r)
		}
	}
	return out
}

func sortStarLists(lists []githubapi.StarList, sortKeys []string, desc bool) {
	if len(sortKeys) == 0 {
		return
	}

	sort.Slice(lists, func(i, j int) bool {
		cmp := compareStarLists(lists[i], lists[j], sortKeys)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareStarLists(left, right githubapi.StarList, sortKeys []string) int {
	for _, key := range sortKeys {
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
		}
		if cmp != 0 {
			return cmp
		}
	}
	if left.Name != right.Name {
		return strings.Compare(left.Name, right.Name)
	}
	return strings.Compare(left.ID, right.ID)
}

func sortRepositories(repos []githubapi.Repository, sortKeys []string, desc bool) {
	if len(sortKeys) == 0 {
		return
	}

	sort.Slice(repos, func(i, j int) bool {
		cmp := compareRepositories(repos[i], repos[j], sortKeys)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareRepositories(left, right githubapi.Repository, sortKeys []string) int {
	for _, key := range sortKeys {
		var cmp int
		switch key {
		case SortKeyName:
			cmp = strings.Compare(strings.ToLower(left.NameWithOwner), strings.ToLower(right.NameWithOwner))
		case SortKeyStars:
			if left.StargazerCount != right.StargazerCount {
				cmp = left.StargazerCount - right.StargazerCount
			}
		case SortKeyPushed:
			if left.PushedAt != right.PushedAt {
				cmp = strings.Compare(left.PushedAt, right.PushedAt)
			}
		}
		if cmp != 0 {
			return cmp
		}
	}
	if left.NameWithOwner != right.NameWithOwner {
		return strings.Compare(left.NameWithOwner, right.NameWithOwner)
	}
	return strings.Compare(left.URL, right.URL)
}

func writeFailure(stderr io.Writer, err error) int {
	_ = writeDiagnostic(stderr, "error: %v\n", err)
	return ExitFailure
}

func writeRuntimeFailure(stderr io.Writer, action Action, listID string, err error) int {
	_ = writeDiagnostic(stderr, "error: %s: %v\n", commandContext(action, listID), err)
	if errors.Is(err, githubapi.ErrInaccessibleList) {
		_ = writeDiagnostic(stderr, "The Star List ID may be deleted, private, inaccessible to this account, or from another GitHub account. Re-run `gh star-lists` with the intended account.\n")
	} else if looksLikeAuthError(err) {
		_ = writeDiagnostic(stderr, "Run `gh auth status` to check GitHub CLI authentication, then `gh auth login` if needed.\n")
	}
	return ExitFailure
}

func commandContext(action Action, listID string) string {
	switch action {
	case ActionRepos:
		return fmt.Sprintf("failed to list repositories for Star List %q", listID)
	default:
		return "failed to list Star Lists"
	}
}

func looksLikeAuthError(err error) bool {
	message := strings.ToLower(err.Error())
	authMarkers := []string{
		"authentication",
		"bad credentials",
		"gh auth",
		"oauth",
		"token",
		"unauthorized",
		"401",
	}
	for _, marker := range authMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func writeDiagnostic(stderr io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(stderr, format, args...)
	return err
}
