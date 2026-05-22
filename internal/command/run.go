package command

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func Run(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	service githubapi.Service,
) int {
	return RunWithOptions(ctx, args, stdout, stderr, service, format.DefaultOptions)
}

func RunWithOptions(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	service githubapi.Service,
	outputOptionsForMode func(format.OutputMode) format.Options,
) int {
	if outputOptionsForMode == nil {
		outputOptionsForMode = format.DefaultOptions
	}
	diagnosticOptions := outputOptionsForMode(format.OutputHuman)
	if argsContainNoColor(args) {
		diagnosticOptions.Color = false
	}

	parsed, err := Parse(args)
	if err != nil {
		return writeParseFailure(stderr, err, diagnosticOptions)
	}

	if parsed.Action == ActionHelp {
		return writeHelp(stdout, stderr, parsed, outputOptionsForMode)
	}

	inv, closeOutput, exitCode := prepareRunInvocation(
		ctx,
		parsed,
		stdout,
		stderr,
		service,
		outputOptionsForMode,
		diagnosticOptions,
	)
	if closeOutput != nil {
		defer closeOutput()
	}
	if exitCode != ExitSuccess {
		return exitCode
	}

	return runParsedAction(inv)
}

func writeParseFailure(stderr io.Writer, err error, diagnosticOptions format.Options) int {
	var unknownErr *UnknownCommandError
	if errors.As(err, &unknownErr) {
		_ = writeErrorDiagnostic(stderr, diagnosticOptions, "%s\n", unknownErr.Error())
		_ = writeHintDiagnostic(
			stderr,
			diagnosticOptions,
			"Run 'gh star-lists --help' for usage.\n",
		)
		return ExitUsage
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		_ = writeStyledDiagnostic(
			stderr,
			diagnosticOptions,
			format.Yellow,
			"error: %s\n\n%s",
			usageErr.Message,
			UsageText(),
		)
		return ExitUsage
	}
	return writeFailure(stderr, err, diagnosticOptions)
}

func writeHelp(
	stdout, stderr io.Writer,
	parsed Parsed,
	outputOptionsForMode func(format.OutputMode) format.Options,
) int {
	helpOptions := outputOptionsForMode(format.OutputHuman)
	if _, err := io.WriteString(
		stdout,
		HelpTextFor(Action(parsed.HelpTopic), parsed.FullHelp, helpOptions),
	); err != nil {
		_ = writeDiagnostic(stderr, "error: failed to write help: %v\n", err)
		return ExitFailure
	}
	return ExitSuccess
}

func runParsedAction(inv runInvocation) int {
	switch inv.parsed.Action {
	case ActionList:
		return runListAction(inv)
	case ActionRepos:
		return runReposAction(inv)
	case ActionCreate:
		return runCreateAction(inv)
	case ActionEdit:
		return runEditAction(inv)
	case ActionDelete:
		return runDeleteAction(inv)
	case ActionAdd, ActionRemove, ActionMove:
		return runRepoMembershipAction(inv)
	case ActionCopy, ActionMerge:
		return runListCopyAction(inv)
	case ActionUnstar:
		return runUnstarAction(inv)
	case ActionTUI:
		return runTUIAction(inv)
	default:
		panic(fmt.Sprintf("unhandled action %q - this is a bug in Parse", inv.parsed.Action))
	}
}
