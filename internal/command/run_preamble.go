package command

import (
	"fmt"
	"os"

	"github.com/cli/go-gh/v2/pkg/browser"
	"github.com/cli/go-gh/v2/pkg/prompter"
	ghterm "github.com/cli/go-gh/v2/pkg/term"
)

const (
	ExitSuccess     = 0
	ExitFailure     = 1
	ExitUsage       = 2
	ExitAuth        = 3
	ExitNotFound    = 4
	ExitRateLimited = 5
)

var openBrowser = func(url string) error {
	return browser.New("", os.Stdout, os.Stderr).Browse(url)
}

var canPrompt = func() bool {
	return ghterm.IsTerminal(os.Stdin) && ghterm.IsTerminal(os.Stderr)
}

var confirmAction = func(prompt string) (bool, error) {
	value, err := prompter.New(os.Stdin, os.Stdout, os.Stderr).Confirm(prompt, false)
	return value, normalizePromptError(err)
}

var promptForList = func(label, defaultValue string, choices []string) (int, error) {
	idx, err := prompter.New(os.Stdin, os.Stdout, os.Stderr).Select(label, defaultValue, choices)
	return idx, normalizePromptError(err)
}

var promptInput = func(label, defaultValue string) (string, error) {
	value, err := prompter.New(os.Stdin, os.Stdout, os.Stderr).Input(label, defaultValue)
	return value, normalizePromptError(err)
}

var promptMultiSelect = func(label string, defaults, choices []string) ([]int, error) {
	values, err := prompter.New(os.Stdin, os.Stdout, os.Stderr).
		MultiSelect(label, defaults, choices)
	return values, normalizePromptError(err)
}

// ErrPromptCancelled is returned when the user cancels an interactive prompt.
var ErrPromptCancelled = fmt.Errorf("cancelled")

func OpenBrowserForTest(fn func(string) error) func(string) error {
	prev := openBrowser
	openBrowser = fn
	return prev
}

func CanPromptForTest(fn func() bool) func() bool {
	prev := canPrompt
	canPrompt = fn
	return prev
}

func PromptForListForTest(
	fn func(string, string, []string) (int, error),
) func(string, string, []string) (int, error) {
	prev := promptForList
	promptForList = fn
	return prev
}

func PromptInputForTest(
	fn func(string, string) (string, error),
) func(string, string) (string, error) {
	prev := promptInput
	promptInput = fn
	return prev
}

func PromptMultiSelectForTest(
	fn func(string, []string, []string) ([]int, error),
) func(string, []string, []string) ([]int, error) {
	prev := promptMultiSelect
	promptMultiSelect = fn
	return prev
}

func ConfirmActionForTest(fn func(string) (bool, error)) func(string) (bool, error) {
	prev := confirmAction
	confirmAction = fn
	return prev
}
