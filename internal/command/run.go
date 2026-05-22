package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/browser"
	"github.com/cli/go-gh/v2/pkg/prompter"
	ghterm "github.com/cli/go-gh/v2/pkg/term"
	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitUsage   = 2
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

	if parsed.Action == ActionHelp {
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

	if service == nil {
		_ = writeDiagnostic(stderr, "error: GitHub service is not configured\n")
		return ExitFailure
	}

	outputOptions := outputOptionsForMode(parsed.Mode)
	outputOptions.Template = parsed.Template
	outputOptions.JQ = parsed.JQ
	if parsed.NoColor {
		outputOptions.Color = false
	}
	diagnosticOptions.Color = outputOptions.Color
	cacheTTL := 5 * time.Minute
	if parsed.CacheTTL != nil {
		cacheTTL = *parsed.CacheTTL
	}
	originalService := service
	if cacheTTL > 0 {
		service = githubapi.NewCacheServiceWithOptions(
			service,
			githubapi.CacheOptions{TTL: cacheTTL},
		)
	}
	if parsed.Action == ActionRepos {
		if err := ensureReposListSelector(ctx, service, &parsed); err != nil {
			if errors.Is(err, ErrPromptCancelled) {
				_ = writeHintDiagnostic(stderr, diagnosticOptions, "No changes made.\n")
				return ExitSuccess
			}
			var ue *UsageError
			if errors.As(err, &ue) {
				return writeUsageFailure(stderr, err, diagnosticOptions)
			}
			return writeRuntimeFailure(stderr, ActionRepos, parsed.ListID, err, diagnosticOptions)
		}
	}
	if parsed.OutputPath != "" {
		if _, statErr := os.Stat(parsed.OutputPath); statErr == nil {
			if !parsed.Yes {
				if !canPrompt() {
					_ = writeDiagnostic(stderr,
						"error: --output target %s already exists; pass --yes to overwrite\n",
						parsed.OutputPath)
					return ExitFailure
				}
				confirmed, err := confirmAction(fmt.Sprintf("Overwrite %s?", parsed.OutputPath))
				if err != nil {
					_ = writeDiagnostic(stderr, "error: %v\n", err)
					return ExitFailure
				}
				if !confirmed {
					return ExitFailure
				}
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			_ = writeDiagnostic(stderr, "error: failed to stat output file: %v\n", statErr)
			return ExitFailure
		}
		f, err := os.OpenFile(parsed.OutputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			_ = writeDiagnostic(stderr, "error: failed to open output file: %v\n", err)
			return ExitFailure
		}
		defer func() {
			_ = f.Close()
		}()
		stdout = f
	}
	switch parsed.Action {
	case ActionList:
		lists, err := service.ListStarLists(ctx, directListOptions(parsed, false))
		if err != nil {
			return writeRuntimeFailure(stderr, ActionList, "", err)
		}
		lists = filterStarLists(lists, parsed.Filters)
		sortStarLists(lists, parsed.SortKeys, parsed.SortTerms, parsed.SortDesc)
		if parsed.Limit > 0 && len(lists) > parsed.Limit {
			lists = lists[:parsed.Limit]
		}
		if err := format.WriteStarListsWithOptions(stdout, outputOptions, lists); err != nil {
			return writeFailure(stderr, fmt.Errorf("failed to write output: %w", err))
		}
	case ActionRepos:
		if parsed.Web {
			rl, err := resolveList(ctx, service, parsed.ListID)
			if err != nil {
				return writeRuntimeFailure(
					stderr,
					ActionRepos,
					parsed.ListID,
					err,
					diagnosticOptions,
				)
			}
			listURL := rl.URL
			if listURL == "" {
				listURL = rl.ID
			}
			if err := openBrowser(listURL); err != nil {
				return writeFailure(
					stderr,
					fmt.Errorf("failed to open browser: %w", err),
					diagnosticOptions,
				)
			}
			return ExitSuccess
		}
		repos, err := fetchReposForAction(ctx, service, parsed)
		if err != nil {
			return writeRuntimeFailure(stderr, ActionRepos, parsed.ListID, err, diagnosticOptions)
		}
		repos = finalizeRepositories(repos, parsed)
		if err := format.WriteRepositoriesWithOptions(stdout, outputOptions, repos); err != nil {
			return writeFailure(
				stderr,
				fmt.Errorf("failed to write output: %w", err),
				diagnosticOptions,
			)
		}
	case ActionCreate:
		if err := ensureCreateInputs(&parsed); err != nil {
			if errors.Is(err, ErrPromptCancelled) {
				_ = writeHintDiagnostic(stderr, diagnosticOptions, "No changes made.\n")
				return ExitSuccess
			}
			return writeUsageFailure(stderr, err, diagnosticOptions)
		}
		if parsed.DryRun {
			_, _ = fmt.Fprintf(stdout, "Would create Star List %q.\n", parsed.Name)
			return ExitSuccess
		}
		list, err := service.CreateStarList(ctx, githubapi.StarListInput{
			Name:        parsed.Name,
			Description: parsed.Description,
			Private:     parsed.Private,
		})
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.Name, err, diagnosticOptions)
		}
		_, _ = fmt.Fprintln(
			stdout,
			styleText(
				format.Green,
				outputOptions.Color,
				fmt.Sprintf("Created Star List %q (%s).", list.Name, list.ID),
			),
		)
	case ActionEdit:
		// Fetch current list first so ensureEditInputs can seed prompt defaults.
		lists, err := service.ListStarLists(ctx)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.ListID, err, diagnosticOptions)
		}
		currentList, found := listByRaw(lists, parsed.ListID)
		if err := ensureEditInputs(&parsed, currentList); err != nil {
			if errors.Is(err, ErrPromptCancelled) {
				_ = writeHintDiagnostic(stderr, diagnosticOptions, "No changes made.\n")
				return ExitSuccess
			}
			return writeUsageFailure(stderr, err, diagnosticOptions)
		}
		var resolvedID string
		if found {
			resolvedID = currentList.ID
		} else {
			resolvedID = parsed.ListID
		}
		if parsed.DryRun {
			_, _ = fmt.Fprintf(
				stdout,
				"Would update Star List %q.\n",
				displayListName(currentList, parsed.ListID),
			)
			return ExitSuccess
		}
		private := (*bool)(nil)
		if parsed.PrivateSet {
			private = &parsed.Private
		}
		list, err := service.UpdateStarList(ctx, githubapi.UpdateStarListInput{
			ID:          resolvedID,
			Name:        parsed.Name,
			Description: parsed.Description,
			Private:     private,
		})
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.ListID, err, diagnosticOptions)
		}
		_, _ = fmt.Fprintln(
			stdout,
			styleText(
				format.Green,
				outputOptions.Color,
				fmt.Sprintf("Updated Star List %q (%s).", list.Name, list.ID),
			),
		)
	case ActionDelete:
		// Resolve first so the confirmation prompt can name the target.
		rl, err := resolveList(ctx, service, parsed.ListID)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.ListID, err, diagnosticOptions)
		}
		listName := rl.Name
		if listName == "" {
			listName = parsed.ListID
		}
		if err := requireYes(parsed, fmt.Sprintf("delete Star List %q", listName)); err != nil {
			return writeUsageFailure(stderr, err, diagnosticOptions)
		}
		if parsed.DryRun {
			_, _ = fmt.Fprintf(stdout, "Would delete Star List %q.\n", listName)
			return ExitSuccess
		}
		if err := service.DeleteStarList(ctx, rl.ID); err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.ListID, err, diagnosticOptions)
		}
		_, _ = fmt.Fprintln(
			stdout,
			styleText(
				format.Yellow,
				outputOptions.Color,
				fmt.Sprintf("Deleted Star List %q.", listName),
			),
		)
	case ActionAdd, ActionRemove, ActionMove:
		fetchedLists, err := ensureListSelectors(ctx, service, &parsed)
		if err != nil {
			if errors.Is(err, ErrPromptCancelled) {
				_ = writeHintDiagnostic(stderr, diagnosticOptions, "No changes made.\n")
				return ExitSuccess
			}
			var ue *UsageError
			if errors.As(err, &ue) {
				return writeUsageFailure(stderr, err, diagnosticOptions)
			}
			return writeRuntimeFailure(
				stderr,
				parsed.Action,
				parsed.RepoName,
				err,
				diagnosticOptions,
			)
		}
		if parsed.Action != ActionAdd {
			fromList, _ := listByRaw(fetchedLists, parsed.FromListID)
			toList, _ := listByRaw(fetchedLists, parsed.ToListID)
			var actionPhrase string
			switch parsed.Action {
			case ActionRemove:
				actionPhrase = fmt.Sprintf(
					"remove %s from %q",
					parsed.RepoName,
					displayListName(fromList, parsed.FromListID),
				)
			case ActionMove:
				actionPhrase = fmt.Sprintf(
					"move %s from %q to %q",
					parsed.RepoName,
					displayListName(fromList, parsed.FromListID),
					displayListName(toList, parsed.ToListID),
				)
			default:
				actionPhrase = string(parsed.Action) + " a repository"
			}
			if err := requireYes(parsed, actionPhrase); err != nil {
				return writeUsageFailure(stderr, err, diagnosticOptions)
			}
		}
		return runRepoListMutation(
			ctx,
			stdout,
			stderr,
			service,
			parsed,
			fetchedLists,
			outputOptions,
		)
	case ActionCopy, ActionMerge:
		fetchedLists, err := ensureListSelectors(ctx, service, &parsed)
		if err != nil {
			if errors.Is(err, ErrPromptCancelled) {
				_ = writeHintDiagnostic(stderr, diagnosticOptions, "No changes made.\n")
				return ExitSuccess
			}
			var ue *UsageError
			if errors.As(err, &ue) {
				return writeUsageFailure(stderr, err, diagnosticOptions)
			}
			return writeRuntimeFailure(
				stderr,
				parsed.Action,
				parsed.FromListID,
				err,
				diagnosticOptions,
			)
		}
		if parsed.Action == ActionMerge || parsed.DeleteSource {
			fromList, _ := listByRaw(fetchedLists, parsed.FromListID)
			toList, _ := listByRaw(fetchedLists, parsed.ToListID)
			var actionPhrase string
			if parsed.Action == ActionMerge {
				actionPhrase = fmt.Sprintf(
					"merge %q into %q",
					displayListName(fromList, parsed.FromListID),
					displayListName(toList, parsed.ToListID),
				)
			} else {
				actionPhrase = fmt.Sprintf(
					"copy and delete %q",
					displayListName(fromList, parsed.FromListID),
				)
			}
			if err := requireYes(parsed, actionPhrase); err != nil {
				return writeUsageFailure(stderr, err, diagnosticOptions)
			}
		}
		return runListCopy(ctx, stdout, stderr, service, parsed, fetchedLists, outputOptions)
	case ActionUnstar:
		// Resolve first so the confirmation prompt can name the target.
		repo, err := service.GetRepository(ctx, parsed.RepoName)
		if err != nil {
			return writeRuntimeFailure(
				stderr,
				parsed.Action,
				parsed.RepoName,
				err,
				diagnosticOptions,
			)
		}
		if err := requireYes(parsed, fmt.Sprintf("unstar %s", repo.NameWithOwner)); err != nil {
			return writeUsageFailure(stderr, err, diagnosticOptions)
		}
		if parsed.DryRun {
			_, _ = fmt.Fprintf(stdout, "Would unstar %s.\n", repo.NameWithOwner)
			return ExitSuccess
		}
		if err := service.RemoveStar(ctx, repo.ID); err != nil {
			return writeRuntimeFailure(
				stderr,
				parsed.Action,
				parsed.RepoName,
				err,
				diagnosticOptions,
			)
		}
		_, _ = fmt.Fprintln(
			stdout,
			styleText(
				format.Yellow,
				outputOptions.Color,
				fmt.Sprintf("Unstarred %s.", repo.NameWithOwner),
			),
		)
	case ActionTUI:
		if !canPrompt() {
			_ = writeDiagnostic(
				stderr,
				"error: tui requires a terminal; use 'gh star-lists --help' for non-interactive commands\n",
			)
			return ExitUsage
		}
		return launchTUI(ctx, stderr, parsed, service, originalService, cacheTTL, diagnosticOptions)
	default:
		panic(fmt.Sprintf("unhandled action %q - this is a bug in Parse", parsed.Action))
	}

	return ExitSuccess
}

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

func displayListName(list githubapi.StarList, fallback string) string {
	if list.Name != "" {
		return list.Name
	}
	return fallback
}

func listByRaw(lists []githubapi.StarList, raw string) (githubapi.StarList, bool) {
	for _, l := range lists {
		if strings.EqualFold(l.Name, raw) || l.ID == raw {
			return l, true
		}
	}
	return githubapi.StarList{}, false
}

func actionNeedsFrom(a Action) bool {
	return a == ActionRemove || a == ActionMove || a == ActionCopy || a == ActionMerge
}

func actionNeedsTo(a Action) bool {
	return a == ActionAdd || a == ActionMove || a == ActionCopy || a == ActionMerge
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

func pickList(lists []githubapi.StarList, label, excludeID string) (string, error) {
	var filtered []githubapi.StarList
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

func resolveListID(ctx context.Context, service githubapi.Service, raw string) (string, error) {
	r, err := resolveList(ctx, service, raw)
	if err != nil {
		return "", err
	}
	return r.ID, nil
}

func directListOptions(parsed Parsed, withTopics bool) githubapi.ListOptions {
	opts := githubapi.ListOptions{WithTopics: withTopics}
	if parsed.Limit == 0 || len(parsed.Filters) > 0 || parsed.Search != "" ||
		len(parsed.SortKeys) > 0 {
		return opts
	}
	opts.Limit = parsed.Limit
	return opts
}

func argsContainNoColor(args []string) bool {
	return slices.Contains(args, "--no-color")
}
