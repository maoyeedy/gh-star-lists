package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type runInvocation struct {
	ctx               context.Context
	stdout            io.Writer
	stderr            io.Writer
	parsed            Parsed
	service           githubapi.Service
	outputOptions     format.Options
	diagnosticOptions format.Options
}

func prepareRunInvocation(
	ctx context.Context,
	parsed Parsed,
	stdout, stderr io.Writer,
	service githubapi.Service,
	outputOptionsForMode func(format.OutputMode) format.Options,
	diagnosticOptions format.Options,
) (runInvocation, func(), int) {
	if service == nil {
		_ = writeDiagnostic(stderr, "error: GitHub service is not configured\n")
		return runInvocation{}, nil, ExitFailure
	}

	outputOptions := outputOptionsForMode(parsed.Mode)
	outputOptions.Template = parsed.Template
	outputOptions.JQ = parsed.JQ
	if parsed.NoColor {
		outputOptions.Color = false
	}
	diagnosticOptions.Color = outputOptions.Color

	service = githubapi.NewCacheServiceWithOptions(
		service,
		githubapi.CacheOptions{TTL: 5 * time.Minute},
	)

	if parsed.Action == ActionRepos {
		if err := ensureReposListSelector(ctx, service, &parsed); err != nil {
			return runInvocation{}, nil, handleSelectorError(
				stderr,
				ActionRepos,
				parsed.ListID,
				err,
				diagnosticOptions,
			)
		}
	}

	closeOutput, code := prepareOutputFile(&stdout, stderr, parsed)
	if code != ExitSuccess {
		return runInvocation{}, nil, code
	}

	return runInvocation{
		ctx:               ctx,
		stdout:            stdout,
		stderr:            stderr,
		parsed:            parsed,
		service:           service,
		outputOptions:     outputOptions,
		diagnosticOptions: diagnosticOptions,
	}, closeOutput, ExitSuccess
}

func prepareOutputFile(stdout *io.Writer, stderr io.Writer, parsed Parsed) (func(), int) {
	if parsed.OutputPath == "" {
		return nil, ExitSuccess
	}
	if _, statErr := os.Stat(parsed.OutputPath); statErr == nil {
		if !parsed.Yes {
			if !canPrompt() {
				_ = writeDiagnostic(stderr,
					"error: --output target %s already exists; pass --yes to overwrite\n",
					parsed.OutputPath)
				return nil, ExitFailure
			}
			confirmed, err := confirmAction(fmt.Sprintf("Overwrite %s?", parsed.OutputPath))
			if err != nil {
				_ = writeDiagnostic(stderr, "error: %v\n", err)
				return nil, ExitFailure
			}
			if !confirmed {
				return nil, ExitFailure
			}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		_ = writeDiagnostic(stderr, "error: failed to stat output file: %v\n", statErr)
		return nil, ExitFailure
	}
	f, err := os.OpenFile(parsed.OutputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		_ = writeDiagnostic(stderr, "error: failed to open output file: %v\n", err)
		return nil, ExitFailure
	}
	*stdout = f
	return func() {
		_ = f.Close()
	}, ExitSuccess
}

func handleSelectorError(
	stderr io.Writer,
	action Action,
	contextValue string,
	err error,
	diagnosticOptions format.Options,
) int {
	if errors.Is(err, ErrPromptCancelled) {
		_ = writeHintDiagnostic(stderr, diagnosticOptions, "No changes made.\n")
		return ExitSuccess
	}
	var ue *UsageError
	if errors.As(err, &ue) {
		return writeUsageFailure(stderr, err, diagnosticOptions)
	}
	return writeRuntimeFailure(stderr, action, contextValue, err, diagnosticOptions)
}
