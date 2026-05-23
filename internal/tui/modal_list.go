package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func (mo *modal) ensurePickFilter() {
	if mo.confirmInput.Placeholder != "" {
		return
	}
	filter := textinput.New()
	filter.Placeholder = "Filter lists"
	filter.CharLimit = 100
	filter.SetWidth(32)
	mo.confirmInput = filter
}

func (mo *modal) filteredChoices() []domain.StarList {
	filter := strings.TrimSpace(strings.ToLower(mo.confirmInput.Value()))
	if filter == "" {
		return mo.choices
	}
	choices := make([]domain.StarList, 0, len(mo.choices))
	for _, choice := range mo.choices {
		if strings.Contains(strings.ToLower(choice.Name), filter) {
			choices = append(choices, choice)
		}
	}
	return choices
}

func (mo *modal) clampChoiceCursor(choiceCount int) {
	if choiceCount <= 0 {
		mo.choiceCursor = 0
		return
	}
	if mo.choiceCursor >= choiceCount {
		mo.choiceCursor = choiceCount - 1
	}
	if mo.choiceCursor < 0 {
		mo.choiceCursor = 0
	}
}

func repoCountLabel(count int) string {
	if count == 1 {
		return "1 repo"
	}
	return fmt.Sprintf("%d repos", count)
}

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
		input := domain.StarListInput{
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
	list domain.StarList,
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
		input := domain.UpdateStarListInput{
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
	list domain.StarList,
) (*modal, tea.Cmd) {
	ci := textinput.New()
	ci.Placeholder = list.Name
	ci.CharLimit = 100
	ci.SetWidth(44)
	focusCmd := ci.Focus()

	mo := &modal{
		kind:  modalDeleteList,
		title: "Delete List",
		body: fmt.Sprintf(
			"Delete %q - %s will be removed from this list.",
			list.Name,
			repoCountLabel(list.RepoCount),
		),
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
	fromList domain.StarList,
	allLists []domain.StarList,
) *modal {
	filtered := make([]domain.StarList, 0, len(allLists))
	for _, l := range allLists {
		if l.ID != fromList.ID {
			filtered = append(filtered, l)
		}
	}
	mo := &modal{
		kind:            modalPickList,
		title:           "Copy List: pick destination",
		body:            "copy",
		choices:         filtered,
		confirmExpected: fromList.Name,
		privateState:    fromList.RepoCount,
		ctx:             ctx,
		svc:             svc,
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
	fromList domain.StarList,
	allLists []domain.StarList,
) *modal {
	filtered := make([]domain.StarList, 0, len(allLists))
	for _, l := range allLists {
		if l.ID != fromList.ID {
			filtered = append(filtered, l)
		}
	}
	mo := &modal{
		kind:            modalPickList,
		title:           "Merge Into (source deleted)",
		body:            "merge",
		choices:         filtered,
		confirmExpected: fromList.Name,
		privateState:    fromList.RepoCount,
		ctx:             ctx,
		svc:             svc,
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
