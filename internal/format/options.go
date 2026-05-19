package format

import (
	"time"

	"github.com/cli/go-gh/v2/pkg/term"
)

const defaultHumanWidth = 120

// Options controls terminal-aware human output without changing JSON or TSV.
type Options struct {
	Mode     OutputMode
	Width    int
	Color    bool
	Now      time.Time
	Template string
	JQ       string
}

// DefaultOptions returns terminal-aware output settings for normal CLI use.
func DefaultOptions(mode OutputMode) Options {
	options := Options{
		Mode: mode,
		Now:  time.Now().UTC(),
	}
	if mode != OutputHuman {
		return options
	}

	terminal := term.FromEnv()
	width, _, err := terminal.Size()
	if err != nil || width <= 0 {
		width = defaultHumanWidth
	}
	options.Width = width
	options.Color = terminal.IsColorEnabled()
	return options
}

func normalizeOptions(options Options) Options {
	if options.Mode == "" {
		options.Mode = OutputHuman
	}
	if options.Width <= 0 {
		options.Width = defaultHumanWidth
	}
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	return options
}
