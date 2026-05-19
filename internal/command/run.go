package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cli/go-gh/v2/pkg/browser"
	"github.com/cli/go-gh/v2/pkg/prompter"
	ghterm "github.com/cli/go-gh/v2/pkg/term"
	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"github.com/maoyeedy/gh-star-lists/internal/tui"
	"golang.org/x/sync/errgroup"
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

func normalizePromptError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "interrupt") {
		return ErrPromptCancelled
	}
	return err
}

func styleText(fn func(bool) func(string) string, enabled bool, text string) string {
	style := fn(enabled)
	if style == nil {
		return text
	}
	return style(text)
}

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

var runTUI = tui.Run

func RunTUIForTest(
	fn func(context.Context, githubapi.Service, tui.Options) error,
) func(context.Context, githubapi.Service, tui.Options) error {
	prev := runTUI
	runTUI = fn
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
	case ActionBrowse:
		if !canPrompt() {
			_ = writeDiagnostic(
				stderr,
				"error: browse requires a terminal; use 'gh star-lists --help' for non-interactive commands\n",
			)
			return ExitUsage
		}
		if err := runTUI(ctx, service, tui.Options{
			NoColor:     parsed.NoColor,
			Mouse:       parsed.Mouse,
			Stderr:      stderr,
			OpenBrowser: openBrowser,
		}); err != nil {
			_ = writeErrorDiagnostic(stderr, diagnosticOptions, "%v\n", err)
			return ExitFailure
		}
		return ExitSuccess
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
	default:
		panic(fmt.Sprintf("unhandled action %q - this is a bug in Parse", parsed.Action))
	}

	return ExitSuccess
}

func fetchReposForAction(
	ctx context.Context,
	service githubapi.Service,
	parsed Parsed,
) ([]githubapi.Repository, error) {
	withTopics := topicsNeeded(parsed)
	switch {
	case parsed.Unlisted:
		lists, err := service.ListStarLists(ctx)
		if err != nil {
			return nil, err
		}
		index, err := loadMembershipIndex(ctx, service, lists)
		if err != nil {
			return nil, err
		}
		starred, err := service.ListStarredRepositories(ctx)
		if err != nil {
			return nil, err
		}
		unlisted := make([]githubapi.Repository, 0, len(starred))
		for _, r := range starred {
			if _, inList := index.byRepo[strings.ToLower(r.NameWithOwner)]; !inList {
				unlisted = append(unlisted, r)
			}
		}
		return unlisted, nil
	case parsed.All:
		return service.ListStarredRepositories(ctx, directListOptions(parsed, withTopics))
	default:
		resolvedID, err := resolveListID(ctx, service, parsed.ListID)
		if err != nil {
			return nil, err
		}
		return service.ListRepositories(ctx, resolvedID, directListOptions(parsed, withTopics))
	}
}

func topicsNeeded(parsed Parsed) bool {
	for _, f := range parsed.Filters {
		if f.Key == FilterKeyTopic {
			return true
		}
	}
	return parsed.Template != "" && strings.Contains(parsed.Template, "Topics")
}

func finalizeRepositories(repos []githubapi.Repository, parsed Parsed) []githubapi.Repository {
	repos = filterRepositories(repos, parsed.Filters)
	repos = searchRepositories(repos, parsed.Search)
	sortRepositories(repos, parsed.SortKeys, parsed.SortTerms, parsed.SortDesc)
	if parsed.Limit > 0 && len(repos) > parsed.Limit {
		repos = repos[:parsed.Limit]
	}
	return repos
}

func runRepoListMutation(
	ctx context.Context,
	stdout, stderr io.Writer,
	service githubapi.Service,
	parsed Parsed,
	prefetchedLists []githubapi.StarList,
	outputOptions format.Options,
) int {
	lists := prefetchedLists
	if lists == nil {
		var err error
		lists, err = service.ListStarLists(ctx)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.RepoName, err)
		}
	}
	var from, to githubapi.StarList
	if parsed.FromListID != "" {
		var ok bool
		from, ok = listByRaw(lists, parsed.FromListID)
		if !ok {
			return writeFailure(stderr, fmt.Errorf("star list %q not found", parsed.FromListID))
		}
	}
	if parsed.ToListID != "" {
		var ok bool
		to, ok = listByRaw(lists, parsed.ToListID)
		if !ok {
			return writeFailure(stderr, fmt.Errorf("star list %q not found", parsed.ToListID))
		}
	}
	if parsed.Action == ActionMove && from.ID == to.ID {
		return writeFailure(stderr, fmt.Errorf("--from and --to resolve to the same list"))
	}
	repoID, currentLists, err := service.GetRepositoryMemberships(ctx, parsed.RepoName)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.RepoName, err)
	}
	next := make(map[string]struct{}, len(currentLists)+1)
	for _, id := range currentLists {
		next[id] = struct{}{}
	}
	switch parsed.Action {
	case ActionAdd:
		next[to.ID] = struct{}{}
	case ActionRemove:
		delete(next, from.ID)
	case ActionMove:
		delete(next, from.ID)
		next[to.ID] = struct{}{}
	}
	listIDs := slices.Sorted(maps.Keys(next))
	if parsed.DryRun {
		_, _ = fmt.Fprintf(stdout, "Would %s.\n", mutationPlanSummary(parsed, from, to))
		return ExitSuccess
	}
	if sameStringSet(currentLists, listIDs) {
		_, _ = fmt.Fprintln(stdout, noChangeMessage(parsed, from, to))
		return ExitSuccess
	}
	if err := service.UpdateRepositoryLists(ctx, repoID, listIDs); err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.RepoName, err)
	}
	_, _ = fmt.Fprintln(
		stdout,
		styleText(format.Green, outputOptions.Color, mutationSummary(parsed, from, to)+"."),
	)
	return ExitSuccess
}

func runListCopy(
	ctx context.Context,
	stdout, stderr io.Writer,
	service githubapi.Service,
	parsed Parsed,
	prefetchedLists []githubapi.StarList,
	outputOptions format.Options,
) int {
	lists := prefetchedLists
	if lists == nil {
		var err error
		lists, err = service.ListStarLists(ctx)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
		}
	}
	fromList, fromFound := listByRaw(lists, parsed.FromListID)
	from := parsed.FromListID
	if fromFound {
		from = fromList.ID
	}
	toList, toFound := listByRaw(lists, parsed.ToListID)
	to := parsed.ToListID
	if toFound {
		to = toList.ID
	}
	if from == to {
		return writeFailure(stderr, fmt.Errorf("--from and --to resolve to the same list"))
	}
	index, err := loadMembershipIndex(ctx, service, lists)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
	}
	repos := index.reposByList[from]
	if !fromFound {
		repos, err = service.ListRepositories(ctx, from)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
		}
	}
	if parsed.DryRun {
		_, _ = fmt.Fprintf(
			stdout,
			"Would %s %d repositories from %q to %q.\n",
			copyVerb(parsed),
			len(repos),
			displayListName(fromList, parsed.FromListID),
			displayListName(toList, parsed.ToListID),
		)
		if parsed.DeleteSource || parsed.Action == ActionMerge {
			_, _ = fmt.Fprintf(
				stdout,
				"Would delete source Star List %q.\n",
				displayListName(fromList, parsed.FromListID),
			)
		}
		return ExitSuccess
	}
	var changed atomic.Int64
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(5)
	for _, repo := range repos {
		repo := repo
		group.Go(func() error {
			repoID, memberships, err := index.repositoryMemberships(
				groupCtx,
				service,
				repo.NameWithOwner,
			)
			if err != nil {
				return fmt.Errorf("%s: %w", repo.NameWithOwner, err)
			}
			if _, ok := memberships[to]; ok {
				return nil
			}
			memberships[to] = struct{}{}
			if err := service.UpdateRepositoryLists(
				groupCtx,
				repoID,
				slices.Sorted(maps.Keys(memberships)),
			); err != nil {
				return fmt.Errorf("%s: %w", repo.NameWithOwner, err)
			}
			changed.Add(1)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
	}
	if parsed.DeleteSource || parsed.Action == ActionMerge {
		if err := service.DeleteStarList(ctx, from); err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
		}
	}
	summary := copySummary(parsed, changed.Load(), len(repos), fromList, toList)
	_, _ = fmt.Fprintln(stdout, styleText(format.Green, outputOptions.Color, summary))
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

func mutationSummary(parsed Parsed, from, to githubapi.StarList) string {
	switch parsed.Action {
	case ActionAdd:
		return fmt.Sprintf("Added %s to %q", parsed.RepoName, displayListName(to, parsed.ToListID))
	case ActionRemove:
		return fmt.Sprintf(
			"Removed %s from %q",
			parsed.RepoName,
			displayListName(from, parsed.FromListID),
		)
	case ActionMove:
		return fmt.Sprintf(
			"Moved %s from %q to %q",
			parsed.RepoName,
			displayListName(from, parsed.FromListID),
			displayListName(to, parsed.ToListID),
		)
	default:
		return fmt.Sprintf("%s %s", pastTense(parsed.Action), parsed.RepoName)
	}
}

func mutationPlanSummary(parsed Parsed, from, to githubapi.StarList) string {
	switch parsed.Action {
	case ActionAdd:
		return fmt.Sprintf("add %s to %q", parsed.RepoName, displayListName(to, parsed.ToListID))
	case ActionRemove:
		return fmt.Sprintf(
			"remove %s from %q",
			parsed.RepoName,
			displayListName(from, parsed.FromListID),
		)
	case ActionMove:
		return fmt.Sprintf(
			"move %s from %q to %q",
			parsed.RepoName,
			displayListName(from, parsed.FromListID),
			displayListName(to, parsed.ToListID),
		)
	default:
		return fmt.Sprintf("%s %s", parsed.Action, parsed.RepoName)
	}
}

func noChangeMessage(parsed Parsed, from, to githubapi.StarList) string {
	switch parsed.Action {
	case ActionAdd:
		return fmt.Sprintf(
			"No changes: %s is already in %q.",
			parsed.RepoName,
			displayListName(to, parsed.ToListID),
		)
	case ActionRemove:
		return fmt.Sprintf(
			"No changes: %s is not in %q.",
			parsed.RepoName,
			displayListName(from, parsed.FromListID),
		)
	default:
		return fmt.Sprintf(
			"No changes: %s already has the requested Star List membership.",
			parsed.RepoName,
		)
	}
}

func copyVerb(parsed Parsed) string {
	if parsed.Action == ActionMerge {
		return "merge"
	}
	return "copy"
}

func copySummary(
	parsed Parsed,
	changed int64,
	total int,
	fromList, toList githubapi.StarList,
) string {
	from := displayListName(fromList, parsed.FromListID)
	to := displayListName(toList, parsed.ToListID)
	verb := "Copied"
	if parsed.Action == ActionMerge {
		verb = "Merged"
	}
	if total == 0 {
		return fmt.Sprintf("Source list %q is empty, nothing to %s.", from, strings.ToLower(verb))
	}
	summary := fmt.Sprintf(
		"%s %d of %d repositories from %q to %q.",
		verb,
		changed,
		total,
		from,
		to,
	)
	if parsed.DeleteSource || parsed.Action == ActionMerge {
		summary += fmt.Sprintf(" Source %q deleted.", from)
	} else if changed == 0 {
		summary = fmt.Sprintf(
			"No changes: all %d repositories from %q were already in %q.",
			total,
			from,
			to,
		)
	}
	return summary
}

func displayListName(list githubapi.StarList, fallback string) string {
	if list.Name != "" {
		return list.Name
	}
	return fallback
}

type repoMembership struct {
	repoID string
	lists  map[string]struct{}
}

type membershipIndex struct {
	byRepo      map[string]repoMembership
	reposByList map[string][]githubapi.Repository
}

func loadMembershipIndex(
	ctx context.Context,
	service githubapi.Service,
	lists []githubapi.StarList,
) (membershipIndex, error) {
	index := membershipIndex{
		byRepo:      make(map[string]repoMembership),
		reposByList: make(map[string][]githubapi.Repository),
	}
	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(5)
	for _, list := range lists {
		list := list
		group.Go(func() error {
			repos, err := service.ListRepositories(groupCtx, list.ID)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			index.reposByList[list.ID] = repos
			for _, repo := range repos {
				key := strings.ToLower(repo.NameWithOwner)
				entry := index.byRepo[key]
				if entry.lists == nil {
					entry.lists = make(map[string]struct{})
				}
				entry.lists[list.ID] = struct{}{}
				if repo.ID != "" {
					entry.repoID = repo.ID
				}
				index.byRepo[key] = entry
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return membershipIndex{}, err
	}
	return index, nil
}

func (i membershipIndex) repositoryMemberships(
	ctx context.Context,
	service githubapi.Service,
	repoName string,
) (string, map[string]struct{}, error) {
	entry := i.byRepo[strings.ToLower(repoName)]
	memberships := maps.Clone(entry.lists)
	if memberships == nil {
		memberships = make(map[string]struct{})
	}
	repoID := entry.repoID
	if repoID == "" {
		repo, err := service.GetRepository(ctx, repoName)
		if err != nil {
			return "", nil, err
		}
		repoID = repo.ID
	}
	return repoID, memberships, nil
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

func ensureListSelectors(
	ctx context.Context,
	service githubapi.Service,
	parsed *Parsed,
) ([]githubapi.StarList, error) {
	needFrom := actionNeedsFrom(parsed.Action) && parsed.FromListID == ""
	needTo := actionNeedsTo(parsed.Action) && parsed.ToListID == ""
	if !needFrom && !needTo {
		return nil, nil
	}
	if !canPrompt() {
		return nil, missingSelectorError(parsed.Action, needFrom, needTo)
	}
	lists, err := service.ListStarLists(ctx)
	if err != nil {
		return nil, err
	}
	if len(lists) == 0 {
		return nil, fmt.Errorf("no Star Lists exist; create one with `gh star-lists create <NAME>`")
	}
	if needFrom {
		id, err := pickList(lists, "Select source Star List (--from):", "")
		if err != nil {
			return nil, err
		}
		parsed.FromListID = id
	}
	if needTo {
		excludeID := ""
		if actionNeedsFrom(parsed.Action) {
			excludeID = parsed.FromListID
		}
		id, err := pickList(lists, "Select target Star List (--to):", excludeID)
		if err != nil {
			return nil, err
		}
		parsed.ToListID = id
	}
	return lists, nil
}

func ensureReposListSelector(ctx context.Context, service githubapi.Service, parsed *Parsed) error {
	if parsed.Action != ActionRepos || parsed.ListID != "" || parsed.All || parsed.Unlisted {
		return nil
	}
	if !canPrompt() {
		return usage(
			"repos requires <LIST_ID_OR_NAME>, --all, or --unlisted (or run in a TTY to choose a list interactively)",
		)
	}
	lists, err := service.ListStarLists(ctx)
	if err != nil {
		return err
	}
	if len(lists) == 0 {
		return fmt.Errorf("no Star Lists exist; create one with `gh star-lists create <NAME>`")
	}
	id, err := pickList(lists, "Select Star List:", "")
	if err != nil {
		return err
	}
	parsed.ListID = id
	return nil
}

func ensureCreateInputs(parsed *Parsed) error {
	if parsed.Name != "" {
		return nil
	}
	if !canPrompt() {
		return usage("create requires a list name (or run in a TTY to be prompted)")
	}
	name, err := promptRequiredInput("List name:", "")
	if err != nil {
		return err
	}
	parsed.Name = name
	if !parsed.DescriptionSet {
		description, err := promptInput("Description:", "")
		if err != nil {
			return err
		}
		parsed.Description = description
		parsed.DescriptionSet = true
	}
	if !parsed.PrivateSet {
		idx, err := promptForList("Visibility:", "Public", []string{"Public", "Private"})
		if err != nil {
			return err
		}
		if idx < 0 || idx > 1 {
			return fmt.Errorf("invalid visibility selection")
		}
		parsed.Private = idx == 1
		parsed.PrivateSet = true
	}
	return nil
}

func ensureEditInputs(parsed *Parsed, current githubapi.StarList) error {
	if parsed.Name != "" || parsed.DescriptionSet || parsed.PrivateSet {
		return nil
	}
	if !canPrompt() {
		return usage(
			"edit requires --name, --description, --private, or --public (or run in a TTY to be prompted)",
		)
	}
	choices := []string{"Name", "Description", "Visibility"}
	selected, err := promptMultiSelect("Select fields to update:", nil, choices)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return ErrPromptCancelled
	}
	for _, idx := range selected {
		if idx < 0 || idx >= len(choices) {
			return fmt.Errorf("invalid edit field selection")
		}
		switch choices[idx] {
		case "Name":
			name, err := promptRequiredInput("New name:", current.Name)
			if err != nil {
				return err
			}
			parsed.Name = name
		case "Description":
			description, err := promptInput("New description:", current.Description)
			if err != nil {
				return err
			}
			parsed.Description = description
			parsed.DescriptionSet = true
		case "Visibility":
			visibility, err := promptForList("Visibility:", "", []string{"Public", "Private"})
			if err != nil {
				return err
			}
			if visibility < 0 || visibility > 1 {
				return fmt.Errorf("invalid visibility selection")
			}
			parsed.Private = visibility == 1
			parsed.PrivateSet = true
		}
	}
	return nil
}

func promptRequiredInput(label, defaultValue string) (string, error) {
	value, err := promptInput(label, defaultValue)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", usage("%s cannot be empty", strings.TrimSuffix(label, ":"))
	}
	return value, nil
}

func requireYes(parsed Parsed, action string) error {
	if parsed.Yes || parsed.DryRun {
		return nil
	}
	if canPrompt() {
		confirmed, err := confirmAction("Confirm " + action + "?")
		if err != nil {
			return err
		}
		if confirmed {
			return nil
		}
		return usage("%s was not confirmed", action)
	}
	return usage("%s requires --yes or --dry-run (or run interactively in a TTY)", action)
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

func pastTense(action Action) string {
	switch action {
	case ActionAdd:
		return "Added"
	case ActionRemove:
		return "Removed"
	case ActionMove:
		return "Moved"
	default:
		return "Updated"
	}
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

func filterStarLists(lists []githubapi.StarList, filters []Filter) []githubapi.StarList {
	if len(filters) == 0 {
		return lists
	}
	out := make([]githubapi.StarList, 0, len(lists))
	for _, l := range lists {
		keep := true
		for _, f := range filters {
			if f.Key == FilterKeyName && !strings.Contains(strings.ToLower(l.Name), f.Value) {
				keep = false
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
			case FilterKeyLanguage:
				if strings.ToLower(r.Language) != f.Value {
					keep = false
				}
			case FilterKeyArchived:
				wantArchived := f.Value == "true"
				if r.IsArchived != wantArchived {
					keep = false
				}
			case FilterKeyLicense:
				if strings.ToLower(r.License) != f.Value {
					keep = false
				}
			case FilterKeyMinStars:
				minStars, _ := strconv.Atoi(f.Value)
				if r.StargazerCount < minStars {
					keep = false
				}
			case FilterKeyMaxStars:
				maxStars, _ := strconv.Atoi(f.Value)
				if r.StargazerCount > maxStars {
					keep = false
				}
			case FilterKeyTopic:
				if !hasTopic(r.Topics, f.Value) {
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

func hasTopic(topics []string, want string) bool {
	return slices.ContainsFunc(topics, func(t string) bool {
		return strings.ToLower(t) == want
	})
}

func sortStarLists(lists []githubapi.StarList, sortKeys []string, sortTerms []SortTerm, desc bool) {
	if len(sortKeys) == 0 {
		return
	}

	sort.Slice(lists, func(i, j int) bool {
		cmp, termDesc := compareStarLists(lists[i], lists[j], sortKeys, sortTerms, desc)
		if termDesc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareStarLists(
	left, right githubapi.StarList,
	sortKeys []string,
	sortTerms []SortTerm,
	globalDesc bool,
) (int, bool) {
	for idx, key := range sortKeys {
		termDesc := globalDesc
		if len(sortTerms) > idx {
			termDesc = sortTerms[idx].Desc
		}
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
		case SortKeyRepoCount:
			if left.RepoCount != right.RepoCount {
				cmp = left.RepoCount - right.RepoCount
			}
		}
		if cmp != 0 {
			return cmp, termDesc
		}
	}
	if left.Name != right.Name {
		return strings.Compare(left.Name, right.Name), false
	}
	return strings.Compare(left.ID, right.ID), false
}

func sortRepositories(
	repos []githubapi.Repository,
	sortKeys []string,
	sortTerms []SortTerm,
	desc bool,
) {
	if len(sortKeys) == 0 {
		return
	}

	sort.Slice(repos, func(i, j int) bool {
		cmp, termDesc := compareRepositories(repos[i], repos[j], sortKeys, sortTerms, desc)
		if termDesc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareRepositories(
	left, right githubapi.Repository,
	sortKeys []string,
	sortTerms []SortTerm,
	globalDesc bool,
) (int, bool) {
	for idx, key := range sortKeys {
		termDesc := globalDesc
		if len(sortTerms) > idx {
			termDesc = sortTerms[idx].Desc
		}
		var cmp int
		switch key {
		case SortKeyName:
			cmp = strings.Compare(
				strings.ToLower(left.NameWithOwner),
				strings.ToLower(right.NameWithOwner),
			)
		case SortKeyStars:
			if left.StargazerCount != right.StargazerCount {
				cmp = left.StargazerCount - right.StargazerCount
			}
		case SortKeyPushed:
			if left.PushedAt != right.PushedAt {
				cmp = strings.Compare(left.PushedAt, right.PushedAt)
			}
		case SortKeyLanguage:
			cmp = strings.Compare(strings.ToLower(left.Language), strings.ToLower(right.Language))
		case SortKeyStarred:
			if left.StarredAt != right.StarredAt {
				cmp = strings.Compare(left.StarredAt, right.StarredAt)
			}
		}
		if cmp != 0 {
			return cmp, termDesc
		}
	}
	if left.NameWithOwner != right.NameWithOwner {
		return strings.Compare(left.NameWithOwner, right.NameWithOwner), false
	}
	return strings.Compare(left.URL, right.URL), false
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
	if errors.Is(err, githubapi.ErrInaccessibleList) {
		_ = writeHintDiagnostic(
			stderr,
			diagnosticOptions,
			"The Star List ID may be deleted, private, inaccessible to this account, or from another GitHub account. Re-run `gh star-lists` with the intended account.\n",
		)
	} else if looksLikeAuthError(err) {
		_ = writeHintDiagnostic(
			stderr,
			diagnosticOptions,
			"Run `gh auth status` to check GitHub CLI authentication, then `gh auth login` if needed.\n",
		)
	}
	return ExitFailure
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
	case ActionMerge:
		return fmt.Sprintf("failed to merge Star List %q", listID)
	case ActionUnstar:
		return fmt.Sprintf("failed to unstar repository %q", listID)
	default:
		return "failed to execute command"
	}
}

var authMarkers = []string{
	"authentication",
	"bad credentials",
	"gh auth",
	"oauth",
	"token",
	"unauthorized",
	"401",
}

func looksLikeAuthError(err error) bool {
	message := strings.ToLower(err.Error())
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

func firstOptions(options []format.Options) format.Options {
	if len(options) == 0 {
		return format.Options{}
	}
	return options[0]
}

func argsContainNoColor(args []string) bool {
	return slices.Contains(args, "--no-color")
}
