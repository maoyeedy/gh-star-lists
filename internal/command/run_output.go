package command

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func writeFailure(stderr io.Writer, err error, options ...format.Options) int {
	_ = writeErrorDiagnostic(stderr, firstOptions(options), "%v\n", err)
	return ExitFailure
}

func writeRuntimeFailure(
	stderr io.Writer,
	action Action,
	listID string,
	err error,
	options ...format.Options,
) int {
	diagnosticOptions := firstOptions(options)
	_ = writeErrorDiagnostic(
		stderr,
		diagnosticOptions,
		"%s: %v\n",
		commandContext(action, listID),
		err,
	)
	var rateLimitErr *domain.RateLimitError
	switch {
	case errors.Is(err, githubapi.ErrInaccessibleList):
		_ = writeHintDiagnostic(
			stderr,
			diagnosticOptions,
			"The Star List ID may be deleted, private, inaccessible to this account, or from another GitHub account. Re-run `gh star-lists` with the intended account.\n",
		)
	case errors.Is(err, domain.ErrAuth):
		_ = writeHintDiagnostic(
			stderr,
			diagnosticOptions,
			"Run `gh auth status` to check GitHub CLI authentication, then `gh auth login` if needed.\n",
		)
	case errors.As(err, &rateLimitErr):
		hint := "GitHub rate limit exceeded. Wait and try again."
		if rateLimitErr.RetryAfter > 0 {
			hint = fmt.Sprintf(
				"GitHub rate limit exceeded. Retry after %s.",
				rateLimitErr.RetryAfter.Round(time.Second),
			)
		}
		_ = writeHintDiagnostic(stderr, diagnosticOptions, "%s\n", hint)
	}
	return mapErrorToExitCode(err)
}

func commandContext(action Action, listID string) string {
	switch action {
	case ActionList:
		return "failed to list Star Lists"
	case ActionRepos:
		return fmt.Sprintf("failed to list repositories for Star List %q", listID)
	case ActionCreate:
		return fmt.Sprintf("failed to create Star List %q", listID)
	case ActionEdit:
		return fmt.Sprintf("failed to edit Star List %q", listID)
	case ActionDelete:
		return fmt.Sprintf("failed to delete Star List %q", listID)
	case ActionAdd:
		return fmt.Sprintf("failed to add repository %q", listID)
	case ActionRemove:
		return fmt.Sprintf("failed to remove repository %q", listID)
	case ActionMove:
		return fmt.Sprintf("failed to move repository %q", listID)
	case ActionCopy:
		return fmt.Sprintf("failed to copy Star List %q", listID)
	case ActionUnstar:
		return fmt.Sprintf("failed to unstar repository %q", listID)
	default:
		return "failed to execute command"
	}
}

func mapErrorToExitCode(err error) int {
	if errors.Is(err, domain.ErrAuth) {
		return ExitAuth
	}
	if errors.Is(err, domain.ErrNotFound) {
		return ExitNotFound
	}
	if errors.Is(err, domain.ErrRateLimited) {
		return ExitRateLimited
	}
	return ExitFailure
}

func writeDiagnostic(stderr io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(stderr, format, args...)
	return err
}

func writeErrorDiagnostic(stderr io.Writer, options format.Options, msg string, args ...any) error {
	return writeStyledDiagnostic(stderr, options, format.Red, "error: "+msg, args...)
}

func writeHintDiagnostic(stderr io.Writer, options format.Options, msg string, args ...any) error {
	return writeStyledDiagnostic(stderr, options, format.Cyan, msg, args...)
}

func writeStyledDiagnostic(
	stderr io.Writer,
	options format.Options,
	styler func(bool) func(string) string,
	msg string,
	args ...any,
) error {
	text := fmt.Sprintf(msg, args...)
	text = styleText(styler, options.Color, text)
	_, err := io.WriteString(stderr, text)
	return err
}

func writeUsageFailure(stderr io.Writer, err error, options ...format.Options) int {
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		_ = writeStyledDiagnostic(
			stderr,
			firstOptions(options),
			format.Yellow,
			"error: %s\n\n%s",
			usageErr.Message,
			UsageText(),
		)
		return ExitUsage
	}
	return writeFailure(stderr, err, options...)
}

func firstOptions(options []format.Options) format.Options {
	if len(options) == 0 {
		return format.Options{}
	}
	return options[0]
}
