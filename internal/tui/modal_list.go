package tui

import (
	"context"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func newCreateListModal(ctx context.Context, svc githubapi.Service) (*modal, tea.Cmd) {
	name := textinput.New()
	name.Placeholder = "List name"
	name.CharLimit = 100
	name.SetWidth(44)

	desc := textinput.New()
	desc.Placeholder = "Description (optional)"
	desc.CharLimit = 200
	desc.SetWidth(44)

	focusCmd := name.Focus()

	mo := &modal{
		kind:   modalCreateList,
		title:  "New List",
		inputs: []textinput.Model{name, desc},
		ctx:    ctx,
		svc:    svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		input := githubapi.StarListInput{
			Name:        m.inputs[0].Value(),
			Description: m.inputs[1].Value(),
			Private:     m.privateState == 2,
		}
		return createListCmd(ctx, svc, input)
	}
	return mo, focusCmd
}

func newEditListModal(
	ctx context.Context,
	svc githubapi.Service,
	list githubapi.StarList,
) (*modal, tea.Cmd) {
	name := textinput.New()
	name.Placeholder = "List name"
	name.CharLimit = 100
	name.SetWidth(44)
	name.SetValue(list.Name)

	desc := textinput.New()
	desc.Placeholder = "Description (optional)"
	desc.CharLimit = 200
	desc.SetWidth(44)
	desc.SetValue(list.Description)

	focusCmd := name.Focus()

	mo := &modal{
		kind:         modalEditList,
		title:        "Edit List",
		inputs:       []textinput.Model{name, desc},
		privateState: 0, // unset: only send Private if user picks
		ctx:          ctx,
		svc:          svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		input := githubapi.UpdateStarListInput{
			ID:          list.ID,
			Name:        m.inputs[0].Value(),
			Description: m.inputs[1].Value(),
		}
		switch m.privateState {
		case 1:
			pub := false
			input.Private = &pub
		case 2:
			priv := true
			input.Private = &priv
		}
		return updateListCmd(ctx, svc, input)
	}
	return mo, focusCmd
}

func newDeleteListModal(
	ctx context.Context,
	svc githubapi.Service,
	list githubapi.StarList,
) (*modal, tea.Cmd) {
	ci := textinput.New()
	ci.Placeholder = list.Name
	ci.CharLimit = 100
	ci.SetWidth(44)
	focusCmd := ci.Focus()

	mo := &modal{
		kind:            modalDeleteList,
		title:           "Delete List",
		confirmInput:    ci,
		confirmExpected: list.Name,
		ctx:             ctx,
		svc:             svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		return deleteListCmd(ctx, svc, list.ID)
	}
	return mo, focusCmd
}

func newCopyListModal(
	ctx context.Context,
	svc githubapi.Service,
	fromList githubapi.StarList,
	allLists []githubapi.StarList,
) *modal {
	filtered := make([]githubapi.StarList, 0, len(allLists))
	for _, l := range allLists {
		if l.ID != fromList.ID {
			filtered = append(filtered, l)
		}
	}
	mo := &modal{
		kind:    modalPickList,
		title:   "Copy List: pick destination",
		choices: filtered,
		ctx:     ctx,
		svc:     svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		if len(m.choices) == 0 {
			return nil
		}
		target := m.choices[m.choiceCursor]
		return copyListCmd(ctx, svc, fromList.ID, target.ID, false)
	}
	return mo
}

func newMergeListModal(
	ctx context.Context,
	svc githubapi.Service,
	fromList githubapi.StarList,
	allLists []githubapi.StarList,
) *modal {
	filtered := make([]githubapi.StarList, 0, len(allLists))
	for _, l := range allLists {
		if l.ID != fromList.ID {
			filtered = append(filtered, l)
		}
	}
	mo := &modal{
		kind:    modalPickList,
		title:   "Merge Into (source deleted)",
		choices: filtered,
		ctx:     ctx,
		svc:     svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		if len(m.choices) == 0 {
			return nil
		}
		target := m.choices[m.choiceCursor]
		return copyListCmd(ctx, svc, fromList.ID, target.ID, true)
	}
	return mo
}
