package command

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	switch parsed.Action {
	case ActionList:
		lists, err := service.ListStarLists(ctx)
		if err != nil {
			return writeRuntimeFailure(stderr, ActionList, "", err)
		}
		if err := format.WriteStarListsWithOptions(stdout, outputOptions, lists); err != nil {
			return writeFailure(stderr, fmt.Errorf("failed to write output: %w", err))
		}
	case ActionRepos:
		repos, err := service.ListRepositories(ctx, parsed.ListID)
		if err != nil {
			return writeRuntimeFailure(stderr, ActionRepos, parsed.ListID, err)
		}
		if err := format.WriteRepositoriesWithOptions(stdout, outputOptions, repos); err != nil {
			return writeFailure(stderr, fmt.Errorf("failed to write output: %w", err))
		}
	default:
		panic(fmt.Sprintf("unhandled action %q — this is a bug in Parse", parsed.Action))
	}

	return ExitSuccess
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
