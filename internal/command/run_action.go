package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/app"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/format"
)

func topicsNeeded(parsed Parsed) bool {
	for _, f := range parsed.Filters {
		if f.Key == FilterKeyTopic {
			return true
		}
	}
	return parsed.Template != "" && strings.Contains(parsed.Template, "Topics")
}

func runListAction(inv runInvocation) int {
	appSvc := app.NewStarListService(inv.service)
	lists, err := appSvc.ListLists(inv.ctx, app.ListListsOptions{
		Filters:   toAppFilters(inv.parsed.Filters),
		SortKeys:  inv.parsed.SortKeys,
		SortTerms: toAppSortTerms(inv.parsed.SortTerms),
		SortDesc:  inv.parsed.SortDesc,
		Limit:     inv.parsed.Limit,
	})
	if err != nil {
		return writeRuntimeFailure(inv.stderr, ActionList, "", err)
	}
	if err := format.WriteStarListsWithOptions(inv.stdout, inv.outputOptions, lists); err != nil {
		return writeFailure(inv.stderr, fmt.Errorf("failed to write output: %w", err))
	}
	return ExitSuccess
}

func runReposAction(inv runInvocation) int {
	if inv.parsed.Web {
		rl, err := resolveList(inv.ctx, inv.service, inv.parsed.ListID)
		if err != nil {
			return writeRuntimeFailure(
				inv.stderr,
				ActionRepos,
				inv.parsed.ListID,
				err,
				inv.diagnosticOptions,
			)
		}
		listURL := rl.URL
		if listURL == "" {
			listURL = rl.ID
		}
		if err := openBrowser(listURL); err != nil {
			return writeFailure(
				inv.stderr,
				fmt.Errorf("failed to open browser: %w", err),
				inv.diagnosticOptions,
			)
		}
		return ExitSuccess
	}
	appSvc := app.NewStarListService(inv.service)
	repos, err := appSvc.ListRepos(inv.ctx, inv.parsed.ListID, app.ListReposOptions{
		All:       inv.parsed.All,
		Unlisted:  inv.parsed.Unlisted,
		Filters:   toAppFilters(inv.parsed.Filters),
		SortKeys:  inv.parsed.SortKeys,
		SortTerms: toAppSortTerms(inv.parsed.SortTerms),
		SortDesc:  inv.parsed.SortDesc,
		Limit:     inv.parsed.Limit,
		Search:    inv.parsed.Search,
		Topics:    topicsNeeded(inv.parsed),
	})
	if err != nil {
		return writeRuntimeFailure(
			inv.stderr,
			ActionRepos,
			inv.parsed.ListID,
			err,
			inv.diagnosticOptions,
		)
	}
	if err := format.WriteRepositoriesWithOptions(
		inv.stdout,
		inv.outputOptions,
		repos,
	); err != nil {
		return writeFailure(
			inv.stderr,
			fmt.Errorf("failed to write output: %w", err),
			inv.diagnosticOptions,
		)
	}
	return ExitSuccess
}

func runCreateAction(inv runInvocation) int {
	parsed := inv.parsed
	if err := ensureCreateInputs(&parsed); err != nil {
		if errors.Is(err, ErrPromptCancelled) {
			_ = writeHintDiagnostic(inv.stderr, inv.diagnosticOptions, "No changes made.\n")
			return ExitSuccess
		}
		return writeUsageFailure(inv.stderr, err, inv.diagnosticOptions)
	}
	if parsed.DryRun {
		_, _ = fmt.Fprintf(inv.stdout, "Would create Star List %q.\n", parsed.Name)
		return ExitSuccess
	}
	appSvc := app.NewStarListService(inv.service)
	list, err := appSvc.CreateList(inv.ctx, domain.StarListInput{
		Name:        parsed.Name,
		Description: parsed.Description,
		Private:     parsed.Private,
	})
	if err != nil {
		return writeRuntimeFailure(
			inv.stderr,
			parsed.Action,
			parsed.Name,
			err,
			inv.diagnosticOptions,
		)
	}
	_, _ = fmt.Fprintln(
		inv.stdout,
		styleText(
			format.Green,
			inv.outputOptions.Color,
			fmt.Sprintf("Created Star List %q (%s).", list.Name, list.ID),
		),
	)
	return ExitSuccess
}

func runEditAction(inv runInvocation) int {
	parsed := inv.parsed
	appSvc := app.NewStarListService(inv.service)
	lists, err := appSvc.Service().ListStarLists(inv.ctx)
	if err != nil {
		return writeRuntimeFailure(
			inv.stderr,
			parsed.Action,
			parsed.ListID,
			err,
			inv.diagnosticOptions,
		)
	}
	currentList, found := listByRaw(lists, parsed.ListID)
	if err := ensureEditInputs(&parsed, currentList); err != nil {
		if errors.Is(err, ErrPromptCancelled) {
			_ = writeHintDiagnostic(inv.stderr, inv.diagnosticOptions, "No changes made.\n")
			return ExitSuccess
		}
		return writeUsageFailure(inv.stderr, err, inv.diagnosticOptions)
	}
	var resolvedID string
	if found {
		resolvedID = currentList.ID
	} else {
		resolvedID = parsed.ListID
	}
	if parsed.DryRun {
		_, _ = fmt.Fprintf(
			inv.stdout,
			"Would update Star List %q.\n",
			displayListName(currentList, parsed.ListID),
		)
		return ExitSuccess
	}
	private := (*bool)(nil)
	if parsed.PrivateSet {
		private = &parsed.Private
	}
	list, err := appSvc.UpdateList(inv.ctx, domain.UpdateStarListInput{
		ID:          resolvedID,
		Name:        parsed.Name,
		Description: parsed.Description,
		Private:     private,
	})
	if err != nil {
		return writeRuntimeFailure(
			inv.stderr,
			parsed.Action,
			parsed.ListID,
			err,
			inv.diagnosticOptions,
		)
	}
	_, _ = fmt.Fprintln(
		inv.stdout,
		styleText(
			format.Green,
			inv.outputOptions.Color,
			fmt.Sprintf("Updated Star List %q (%s).", list.Name, list.ID),
		),
	)
	return ExitSuccess
}

func runDeleteAction(inv runInvocation) int {
	parsed := inv.parsed
	rl, err := resolveList(inv.ctx, inv.service, parsed.ListID)
	if err != nil {
		return writeRuntimeFailure(
			inv.stderr,
			parsed.Action,
			parsed.ListID,
			err,
			inv.diagnosticOptions,
		)
	}
	listName := rl.Name
	if listName == "" {
		listName = parsed.ListID
	}
	if err := requireYes(parsed, fmt.Sprintf("delete Star List %q", listName)); err != nil {
		return writeUsageFailure(inv.stderr, err, inv.diagnosticOptions)
	}
	if parsed.DryRun {
		_, _ = fmt.Fprintf(inv.stdout, "Would delete Star List %q.\n", listName)
		return ExitSuccess
	}
	appSvc := app.NewStarListService(inv.service)
	if err := appSvc.DeleteList(inv.ctx, rl.ID); err != nil {
		return writeRuntimeFailure(
			inv.stderr,
			parsed.Action,
			parsed.ListID,
			err,
			inv.diagnosticOptions,
		)
	}
	_, _ = fmt.Fprintln(
		inv.stdout,
		styleText(
			format.Yellow,
			inv.outputOptions.Color,
			fmt.Sprintf("Deleted Star List %q.", listName),
		),
	)
	return ExitSuccess
}

func runRepoMembershipAction(inv runInvocation) int {
	parsed := inv.parsed
	fetchedLists, err := ensureListSelectors(inv.ctx, inv.service, &parsed)
	if err != nil {
		return handleSelectorError(
			inv.stderr,
			parsed.Action,
			parsed.RepoName,
			err,
			inv.diagnosticOptions,
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
			return writeUsageFailure(inv.stderr, err, inv.diagnosticOptions)
		}
	}
	appSvc := app.NewStarListService(inv.service)
	return runRepoListMutation(
		inv.ctx,
		inv.stdout,
		inv.stderr,
		appSvc,
		parsed,
		fetchedLists,
		inv.outputOptions,
	)
}

func runListCopyAction(inv runInvocation) int {
	parsed := inv.parsed
	fetchedLists, err := ensureListSelectors(inv.ctx, inv.service, &parsed)
	if err != nil {
		return handleSelectorError(
			inv.stderr,
			parsed.Action,
			parsed.FromListID,
			err,
			inv.diagnosticOptions,
		)
	}
	if parsed.DeleteSource {
		fromList, _ := listByRaw(fetchedLists, parsed.FromListID)
		actionPhrase := fmt.Sprintf(
			"copy and delete %q",
			displayListName(fromList, parsed.FromListID),
		)
		if err := requireYes(parsed, actionPhrase); err != nil {
			return writeUsageFailure(inv.stderr, err, inv.diagnosticOptions)
		}
	}
	appSvc := app.NewStarListService(inv.service)
	return runListCopy(
		inv.ctx,
		inv.stdout,
		inv.stderr,
		appSvc,
		parsed,
		fetchedLists,
		inv.outputOptions,
	)
}

func runUnstarAction(inv runInvocation) int {
	parsed := inv.parsed
	appSvc := app.NewStarListService(inv.service)
	repo, err := appSvc.GetRepository(inv.ctx, parsed.RepoName)
	if err != nil {
		return writeRuntimeFailure(
			inv.stderr,
			parsed.Action,
			parsed.RepoName,
			err,
			inv.diagnosticOptions,
		)
	}
	if err := requireYes(parsed, fmt.Sprintf("unstar %s", repo.NameWithOwner)); err != nil {
		return writeUsageFailure(inv.stderr, err, inv.diagnosticOptions)
	}
	if parsed.DryRun {
		_, _ = fmt.Fprintf(inv.stdout, "Would unstar %s.\n", repo.NameWithOwner)
		return ExitSuccess
	}
	if err := appSvc.RemoveStar(inv.ctx, repo.ID); err != nil {
		return writeRuntimeFailure(
			inv.stderr,
			parsed.Action,
			parsed.RepoName,
			err,
			inv.diagnosticOptions,
		)
	}
	_, _ = fmt.Fprintln(
		inv.stdout,
		styleText(
			format.Yellow,
			inv.outputOptions.Color,
			fmt.Sprintf("Unstarred %s.", repo.NameWithOwner),
		),
	)
	return ExitSuccess
}

func runTUIAction(inv runInvocation) int {
	if !canPrompt() {
		_ = writeDiagnostic(
			inv.stderr,
			"error: tui requires a terminal; use 'gh star-lists --help' for non-interactive commands\n",
		)
		return ExitUsage
	}
	return launchTUI(
		inv.ctx,
		inv.stderr,
		inv.parsed,
		inv.service,
		inv.diagnosticOptions,
	)
}

func runRepoListMutation(
	ctx context.Context,
	stdout, stderr io.Writer,
	appSvc *app.StarListService,
	parsed Parsed,
	prefetchedLists []domain.StarList,
	outputOptions format.Options,
) int {
	lists := prefetchedLists
	if lists == nil {
		var err error
		lists, err = appSvc.Service().ListStarLists(ctx)
		if err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.RepoName, err)
		}
	}
	var from, to domain.StarList
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
	repoID, currentLists, err := appSvc.GetRepositoryMemberships(ctx, parsed.RepoName)
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
	if err := appSvc.UpdateRepositoryLists(ctx, repoID, listIDs); err != nil {
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
	appSvc *app.StarListService,
	parsed Parsed,
	prefetchedLists []domain.StarList,
	outputOptions format.Options,
) int {
	lists := prefetchedLists
	if lists == nil {
		var err error
		lists, err = appSvc.Service().ListStarLists(ctx)
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
	srcRepos, err := appSvc.Service().ListRepositories(ctx, from)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
	}
	total := len(srcRepos)
	if parsed.DryRun {
		_, _ = fmt.Fprintf(
			stdout,
			"Would %s %d repositories from %q to %q.\n",
			"copy",
			total,
			displayListName(fromList, parsed.FromListID),
			displayListName(toList, parsed.ToListID),
		)
		if parsed.DeleteSource {
			_, _ = fmt.Fprintf(
				stdout,
				"Would delete source Star List %q.\n",
				displayListName(fromList, parsed.FromListID),
			)
		}
		return ExitSuccess
	}
	changed, total, err := appSvc.CopyList(ctx, from, to)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
	}
	if parsed.DeleteSource {
		if err := appSvc.DeleteList(ctx, from); err != nil {
			return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
		}
	}
	summary := copySummary(parsed, int64(changed), total, fromList, toList)
	_, _ = fmt.Fprintln(stdout, styleText(format.Green, outputOptions.Color, summary))
	return ExitSuccess
}

func mutationSummary(parsed Parsed, from, to domain.StarList) string {
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

func mutationPlanSummary(parsed Parsed, from, to domain.StarList) string {
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

func noChangeMessage(parsed Parsed, from, to domain.StarList) string {
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

func copySummary(
	parsed Parsed,
	changed int64,
	total int,
	fromList, toList domain.StarList,
) string {
	from := displayListName(fromList, parsed.FromListID)
	to := displayListName(toList, parsed.ToListID)
	verb := "Copied"
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
	if parsed.DeleteSource {
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
