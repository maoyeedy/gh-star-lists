package tui

import (
	"context"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
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
	starredAtEnrichedMsg struct {
		repos  []githubapi.Repository
		err    error
		listID string
		gen    uint64
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

func enrichStarredAtCmd(
	ctx context.Context,
	svc githubapi.Service,
	p *preloader,
	listID string,
	repos []githubapi.Repository,
	gen uint64,
) tea.Cmd {
	return func() tea.Msg {
		if ctx.Err() != nil {
			return nil
		}
		enriched, err := p.getStarredAt(ctx, svc, repos)
		if ctx.Err() != nil {
			return nil
		}
		return starredAtEnrichedMsg{
			repos:  enriched,
			err:    err,
			listID: listID,
			gen:    gen,
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
		err := githubapi.ModifyRepositoryMemberships(
			ctx,
			svc,
			nameWithOwner,
			[]string{targetListID},
			nil,
		)
		return mutationDoneMsg{kind: modalPickList, err: err}
	}
}

func moveRepoCmd(
	ctx context.Context,
	svc githubapi.Service,
	nameWithOwner, fromListID, toListID string,
) tea.Cmd {
	return func() tea.Msg {
		err := githubapi.ModifyRepositoryMemberships(
			ctx,
			svc,
			nameWithOwner,
			[]string{toListID},
			[]string{fromListID},
		)
		return mutationDoneMsg{kind: modalPickList, err: err}
	}
}

func removeRepoFromListCmd(
	ctx context.Context,
	svc githubapi.Service,
	nameWithOwner, fromListID string,
) tea.Cmd {
	return func() tea.Msg {
		err := githubapi.ModifyRepositoryMemberships(
			ctx,
			svc,
			nameWithOwner,
			nil,
			[]string{fromListID},
		)
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
				return githubapi.ModifyRepositoryMemberships(
					groupCtx,
					svc,
					repo.NameWithOwner,
					[]string{toListID},
					nil,
				)
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

func unstarRepoCmd(ctx context.Context, svc githubapi.Service, repo githubapi.Repository) tea.Cmd {
	return func() tea.Msg {
		repoID := repo.ID
		if repoID == "" {
			resolved, err := svc.GetRepository(ctx, repo.NameWithOwner)
			if err != nil {
				return mutationDoneMsg{kind: modalConfirmText, err: err}
			}
			repoID = resolved.ID
		}
		err := svc.RemoveStar(ctx, repoID)
		return mutationDoneMsg{kind: modalConfirmText, err: err}
	}
}

func bulkMutateReposCmd(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	verb string,
	allLists []githubapi.StarList,
	addIDs, removeIDs []string,
) tea.Cmd {
	return func() tea.Msg {
		index, err := githubapi.LoadMembershipIndex(ctx, svc, allLists)
		if err != nil {
			return bulkDoneMsg{
				verb:       verb,
				failed:     len(nwos),
				failedNWOs: append([]string(nil), nwos...),
			}
		}
		var succeeded, failed atomic.Int64
		var mu sync.Mutex
		var failedNWOs []string
		group, groupCtx := errgroup.WithContext(ctx)
		group.SetLimit(5)
		for _, nwo := range nwos {
			nwo := nwo
			group.Go(func() error {
				repoID, memberships, err := index.RepositoryMemberships(groupCtx, svc, nwo)
				if err == nil {
					for _, id := range addIDs {
						memberships[id] = struct{}{}
					}
					for _, id := range removeIDs {
						delete(memberships, id)
					}
					err = svc.UpdateRepositoryLists(
						groupCtx,
						repoID,
						slices.Sorted(maps.Keys(memberships)),
					)
				}
				if err != nil {
					failed.Add(1)
					mu.Lock()
					failedNWOs = append(failedNWOs, nwo)
					mu.Unlock()
					return nil
				}
				succeeded.Add(1)
				return nil
			})
		}
		_ = group.Wait()
		return bulkDoneMsg{
			verb:       verb,
			succeeded:  int(succeeded.Load()),
			failed:     int(failed.Load()),
			failedNWOs: failedNWOs,
		}
	}
}
