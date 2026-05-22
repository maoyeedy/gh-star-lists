package command

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"golang.org/x/sync/errgroup"
)

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
		index, err := githubapi.LoadMembershipIndex(ctx, service, lists)
		if err != nil {
			return nil, err
		}
		starred, err := service.ListStarredRepositories(ctx)
		if err != nil {
			return nil, err
		}
		unlisted := make([]githubapi.Repository, 0, len(starred))
		for _, r := range starred {
			if !index.ContainsRepository(r.NameWithOwner) {
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
	index, err := githubapi.LoadMembershipIndex(ctx, service, lists)
	if err != nil {
		return writeRuntimeFailure(stderr, parsed.Action, parsed.FromListID, err)
	}
	repos := index.RepositoriesForList(from)
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
			repoID, memberships, err := index.RepositoryMemberships(
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
