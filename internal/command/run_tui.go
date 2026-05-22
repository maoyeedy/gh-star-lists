package command

import (
	"context"
	"io"

	"github.com/cli/go-gh/v2/pkg/browser"

	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"github.com/maoyeedy/gh-star-lists/internal/tui"
)

var runTUI = tui.Run

func RunTUIForTest(
	fn func(context.Context, githubapi.Service, tui.Options) error,
) func(context.Context, githubapi.Service, tui.Options) error {
	prev := runTUI
	runTUI = fn
	return prev
}

func launchTUI(
	ctx context.Context,
	stderr io.Writer,
	parsed Parsed,
	svc githubapi.Service,
	diagnosticOpts format.Options,
) int {
	if err := runTUI(ctx, svc, tui.Options{
		NoColor: parsed.NoColor,
		Mouse:   parsed.Mouse,
		Stderr:  stderr,
		OpenBrowser: func(url string) error {
			_ = browser.New("", io.Discard, io.Discard).Browse(url)
			return nil
		},
	}); err != nil {
		_ = writeErrorDiagnostic(stderr, diagnosticOpts, "%v\n", err)
		return ExitFailure
	}
	return ExitSuccess
}
