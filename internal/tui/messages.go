package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type (
	listsLoadedMsg struct{ lists []githubapi.StarList }
	reposLoadedMsg struct {
		repos  []githubapi.Repository
		listID string
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

func loadReposCmd(ctx context.Context, svc githubapi.Service, listID string) tea.Cmd {
	return func() tea.Msg {
		repos, err := svc.ListRepositories(ctx, listID)
		if err != nil {
			return errMsg{err}
		}
		return reposLoadedMsg{repos, listID}
	}
}
