package tui

import (
	"context"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

// Options configures the TUI browser.
type Options struct {
	NoColor     bool
	Mouse       bool
	Stderr      io.Writer
	OpenBrowser func(string) error
}

// Run launches the interactive two-pane TUI browser.
func Run(ctx context.Context, svc githubapi.Service, opts Options) error {
	m := newModel(ctx, svc, opts)
	var progOpts []tea.ProgramOption
	if opts.NoColor {
		progOpts = append(progOpts, tea.WithColorProfile(colorprofile.NoTTY))
	}
	p := tea.NewProgram(m, progOpts...)
	_, err := p.Run()
	return err
}
