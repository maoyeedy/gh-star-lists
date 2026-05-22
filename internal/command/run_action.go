package command

import (
	"context"
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
			copyVerb(parsed),
			total,
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
	changed, total, err := appSvc.CopyList(ctx, from, to)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
	}
	if parsed.DeleteSource || parsed.Action == ActionMerge {
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
	fromList, toList domain.StarList,
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
