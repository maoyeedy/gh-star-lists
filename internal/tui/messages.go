package tui

import (
	"context"
	"maps"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"golang.org/x/sync/errgroup"
)

type (
	listsLoadedMsg struct{ lists []githubapi.StarList }
	reposLoadedMsg struct {
		repos      []githubapi.Repository
		err        error
		listID     string
		withTopics bool
		gen        uint64
	}
)
type errMsg struct{ err error }

func loadListsCmd(ctx context.Context, svc githubapi.Service) tea.Cmd {
	return func() tea.Msg {
		lists, err := svc.ListStarLists(ctx)
		if err != nil {
			return errMsg{err}
		}
		return listsLoadedMsg{lists}
	}
}

func loadReposCmd(
	ctx context.Context,
	svc githubapi.Service,
	listID string,
	withTopics bool,
	gen uint64,
) tea.Cmd {
	return func() tea.Msg {
		if ctx.Err() != nil {
			return nil
		}
		opts := []githubapi.ListOptions{}
		if withTopics {
			opts = append(opts, githubapi.ListOptions{WithTopics: true})
		}
		repos, err := svc.ListRepositories(ctx, listID, opts...)
		if ctx.Err() != nil {
			return nil
		}
		return reposLoadedMsg{
			repos:      repos,
			err:        err,
			listID:     listID,
			withTopics: withTopics,
			gen:        gen,
		}
	}
}

type mutationDoneMsg struct {
	kind modalKind
	err  error
}

type bulkDoneMsg struct {
	verb       string // "added", "removed", "moved"
	succeeded  int
	failed     int
	failedNWOs []string
}

type statusExpiredMsg struct{}

func statusClearCmd(expiry time.Time) tea.Cmd {
	return tea.Tick(time.Until(expiry)+10*time.Millisecond, func(time.Time) tea.Msg {
		return statusExpiredMsg{}
	})
}

func createListCmd(
	ctx context.Context,
	svc githubapi.Service,
	input githubapi.StarListInput,
) tea.Cmd {
	return func() tea.Msg {
		_, err := svc.CreateStarList(ctx, input)
		return mutationDoneMsg{kind: modalCreateList, err: err}
	}
}

func updateListCmd(
	ctx context.Context,
	svc githubapi.Service,
	input githubapi.UpdateStarListInput,
) tea.Cmd {
	return func() tea.Msg {
		_, err := svc.UpdateStarList(ctx, input)
		return mutationDoneMsg{kind: modalEditList, err: err}
	}
}

func deleteListCmd(ctx context.Context, svc githubapi.Service, listID string) tea.Cmd {
	return func() tea.Msg {
		err := svc.DeleteStarList(ctx, listID)
		return mutationDoneMsg{kind: modalDeleteList, err: err}
	}
}

func addRepoToListCmd(
	ctx context.Context,
	svc githubapi.Service,
	nameWithOwner, targetListID string,
) tea.Cmd {
	return func() tea.Msg {
		repoID, currentIDs, err := svc.GetRepositoryMemberships(ctx, nameWithOwner)
		if err != nil {
			return mutationDoneMsg{kind: modalPickList, err: err}
		}
		next := make(map[string]struct{}, len(currentIDs)+1)
		for _, id := range currentIDs {
			next[id] = struct{}{}
		}
		next[targetListID] = struct{}{}
		newIDs := slices.Sorted(maps.Keys(next))
		err = svc.UpdateRepositoryLists(ctx, repoID, newIDs)
		return mutationDoneMsg{kind: modalPickList, err: err}
	}
}

func moveRepoCmd(
	ctx context.Context,
	svc githubapi.Service,
	nameWithOwner, fromListID, toListID string,
) tea.Cmd {
	return func() tea.Msg {
		repoID, currentIDs, err := svc.GetRepositoryMemberships(ctx, nameWithOwner)
		if err != nil {
			return mutationDoneMsg{kind: modalPickList, err: err}
		}
		next := make(map[string]struct{}, len(currentIDs))
		for _, id := range currentIDs {
			next[id] = struct{}{}
		}
		delete(next, fromListID)
		next[toListID] = struct{}{}
		newIDs := slices.Sorted(maps.Keys(next))
		err = svc.UpdateRepositoryLists(ctx, repoID, newIDs)
		return mutationDoneMsg{kind: modalPickList, err: err}
	}
}

func removeRepoFromListCmd(
	ctx context.Context,
	svc githubapi.Service,
	nameWithOwner, fromListID string,
) tea.Cmd {
	return func() tea.Msg {
		repoID, currentIDs, err := svc.GetRepositoryMemberships(ctx, nameWithOwner)
		if err != nil {
			return mutationDoneMsg{kind: modalConfirmYesNo, err: err}
		}
		next := make(map[string]struct{}, len(currentIDs))
		for _, id := range currentIDs {
			next[id] = struct{}{}
		}
		delete(next, fromListID)
		newIDs := slices.Sorted(maps.Keys(next))
		err = svc.UpdateRepositoryLists(ctx, repoID, newIDs)
		return mutationDoneMsg{kind: modalConfirmYesNo, err: err}
	}
}

func copyListCmd(
	ctx context.Context,
	svc githubapi.Service,
	fromListID, toListID string,
	deleteSource bool,
) tea.Cmd {
	return func() tea.Msg {
		repos, err := svc.ListRepositories(ctx, fromListID)
		if err != nil {
			return mutationDoneMsg{kind: modalPickList, err: err}
		}
		group, groupCtx := errgroup.WithContext(ctx)
		group.SetLimit(5)
		for _, repo := range repos {
			repo := repo
			group.Go(func() error {
				repoID, currentIDs, e := svc.GetRepositoryMemberships(groupCtx, repo.NameWithOwner)
				if e != nil {
					return e
				}
				next := make(map[string]struct{}, len(currentIDs)+1)
				for _, id := range currentIDs {
					next[id] = struct{}{}
				}
				if _, already := next[toListID]; already {
					return nil // already a member, skip
				}
				next[toListID] = struct{}{}
				return svc.UpdateRepositoryLists(groupCtx, repoID, slices.Sorted(maps.Keys(next)))
			})
		}
		if e := group.Wait(); e != nil {
			return mutationDoneMsg{kind: modalPickList, err: e}
		}
		if deleteSource {
			if e := svc.DeleteStarList(ctx, fromListID); e != nil {
				return mutationDoneMsg{kind: modalPickList, err: e}
			}
		}
		return mutationDoneMsg{kind: modalPickList}
	}
}

func unstarRepoCmd(ctx context.Context, svc githubapi.Service, nameWithOwner string) tea.Cmd {
	return func() tea.Msg {
		repoID, _, err := svc.GetRepositoryMemberships(ctx, nameWithOwner)
		if err != nil {
			return mutationDoneMsg{kind: modalConfirmText, err: err}
		}
		err = svc.RemoveStar(ctx, repoID)
		return mutationDoneMsg{kind: modalConfirmText, err: err}
	}
}

func bulkAddReposCmd(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	targetListID string,
) tea.Cmd {
	return func() tea.Msg {
		succeeded, failed := 0, 0
		var failedNWOs []string
		for _, nwo := range nwos {
			if ctx.Err() != nil {
				break
			}
			repoID, currentIDs, err := svc.GetRepositoryMemberships(ctx, nwo)
			if err != nil {
				failed++
				failedNWOs = append(failedNWOs, nwo)
				continue
			}
			next := make(map[string]struct{}, len(currentIDs)+1)
			for _, id := range currentIDs {
				next[id] = struct{}{}
			}
			next[targetListID] = struct{}{}
			newIDs := slices.Sorted(maps.Keys(next))
			if err = svc.UpdateRepositoryLists(ctx, repoID, newIDs); err != nil {
				failed++
				failedNWOs = append(failedNWOs, nwo)
			} else {
				succeeded++
			}
		}
		return bulkDoneMsg{
			verb:       "added",
			succeeded:  succeeded,
			failed:     failed,
			failedNWOs: failedNWOs,
		}
	}
}

func bulkRemoveReposCmd(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	fromListID string,
) tea.Cmd {
	return func() tea.Msg {
		succeeded, failed := 0, 0
		var failedNWOs []string
		for _, nwo := range nwos {
			if ctx.Err() != nil {
				break
			}
			repoID, currentIDs, err := svc.GetRepositoryMemberships(ctx, nwo)
			if err != nil {
				failed++
				failedNWOs = append(failedNWOs, nwo)
				continue
			}
			next := make(map[string]struct{}, len(currentIDs))
			for _, id := range currentIDs {
				next[id] = struct{}{}
			}
			delete(next, fromListID)
			newIDs := slices.Sorted(maps.Keys(next))
			if err = svc.UpdateRepositoryLists(ctx, repoID, newIDs); err != nil {
				failed++
				failedNWOs = append(failedNWOs, nwo)
			} else {
				succeeded++
			}
		}
		return bulkDoneMsg{
			verb:       "removed",
			succeeded:  succeeded,
			failed:     failed,
			failedNWOs: failedNWOs,
		}
	}
}

func bulkMoveReposCmd(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	fromListID, toListID string,
) tea.Cmd {
	return func() tea.Msg {
		succeeded, failed := 0, 0
		var failedNWOs []string
		for _, nwo := range nwos {
			if ctx.Err() != nil {
				break
			}
			repoID, currentIDs, err := svc.GetRepositoryMemberships(ctx, nwo)
			if err != nil {
				failed++
				failedNWOs = append(failedNWOs, nwo)
				continue
			}
			next := make(map[string]struct{}, len(currentIDs))
			for _, id := range currentIDs {
				next[id] = struct{}{}
			}
			delete(next, fromListID)
			next[toListID] = struct{}{}
			newIDs := slices.Sorted(maps.Keys(next))
			if err = svc.UpdateRepositoryLists(ctx, repoID, newIDs); err != nil {
				failed++
				failedNWOs = append(failedNWOs, nwo)
			} else {
				succeeded++
			}
		}
		return bulkDoneMsg{
			verb:       "moved",
			succeeded:  succeeded,
			failed:     failed,
			failedNWOs: failedNWOs,
		}
	}
}
