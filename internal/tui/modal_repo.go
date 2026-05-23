package tui

import (
	"context"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func newAddRepoModal(ctx context.Context, svc githubapi.Service,
	repo domain.Repository, allLists []domain.StarList,
) *modal {
	mo := &modal{
		kind:    modalPickList,
		title:   "Add to List",
		choices: allLists,
		ctx:     ctx,
		svc:     svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		if len(m.choices) == 0 {
			return nil
		}
		target := m.choices[m.choiceCursor]
		return addRepoToListCmd(ctx, svc, repo.NameWithOwner, target.ID)
	}
	return mo
}

func newMoveRepoModal(ctx context.Context, svc githubapi.Service,
	repo domain.Repository, allLists []domain.StarList, fromListID string,
) *modal {
	// Filter out the current list from the picker.
	filtered := make([]domain.StarList, 0, len(allLists))
	for _, l := range allLists {
		if l.ID != fromListID {
			filtered = append(filtered, l)
		}
	}
	mo := &modal{
		kind:    modalPickList,
		title:   "Move to List",
		choices: filtered,
		ctx:     ctx,
		svc:     svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		if len(m.choices) == 0 {
			return nil
		}
		target := m.choices[m.choiceCursor]
		return moveRepoCmd(ctx, svc, repo.NameWithOwner, fromListID, target.ID)
	}
	return mo
}

func newRemoveRepoModal(ctx context.Context, svc githubapi.Service,
	repo domain.Repository, fromListID string,
) *modal {
	mo := &modal{
		kind:  modalConfirmYesNo,
		title: "Remove from List",
		ctx:   ctx,
		svc:   svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		return removeRepoFromListCmd(ctx, svc, repo.NameWithOwner, fromListID)
	}
	return mo
}

func newUnstarRepoModal(ctx context.Context, svc githubapi.Service,
	repo domain.Repository,
) (*modal, tea.Cmd) {
	ci := textinput.New()
	ci.Placeholder = repo.NameWithOwner
	ci.CharLimit = 200
	ci.SetWidth(44)
	focusCmd := ci.Focus()
	mo := &modal{
		kind:            modalConfirmText,
		title:           "Unstar Repo",
		confirmInput:    ci,
		confirmExpected: repo.NameWithOwner,
		ctx:             ctx,
		svc:             svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		return unstarRepoCmd(ctx, svc, repo)
	}
	return mo, focusCmd
}

func newRepoDetailModal(repo domain.Repository, openBrowser func(string) error) *modal {
	return &modal{
		kind:  modalRepoDetail,
		title: "Repo Details",
		repo:  repo,
		onConfirm: func(m *modal) tea.Cmd {
			return openBrowserCmd(openBrowser, m.repo.URL)
		},
	}
}

// update handles key events while a modal is active.
// Returns (nil, nil) to close, or (updated modal, cmd).
// Returns (nil, cmd) with cmd != nil when a mutation should be submitted;
