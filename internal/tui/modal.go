package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type modalKind int

const (
	modalNone modalKind = iota
	modalCreateList
	modalEditList
	modalDeleteList
	modalPickList
	modalConfirmText
	modalConfirmYesNo
)

type modal struct {
	kind  modalKind
	title string
	body  string // placeholder body for stubs

	// form modals (create/edit): textinput fields
	inputs     []textinput.Model
	focusedIdx int
	// visibility toggle: 0=unset, 1=public, 2=private
	privateState int
	formErr      string // inline error shown in modal

	// confirm-text modals (delete, unstar): typed name
	confirmInput    textinput.Model
	confirmExpected string

	// list-picker modals (add, move)
	choices      []githubapi.StarList
	choiceCursor int

	// mutation to run on confirm (set by constructor)
	// Returns (nil, cmd) when triggered.
	onConfirm func(mo *modal) tea.Cmd

	// context for cancel-without-side-effect
	ctx context.Context
	svc githubapi.Service
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

func newAddRepoModal(ctx context.Context, svc githubapi.Service,
	repo githubapi.Repository, allLists []githubapi.StarList,
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
	repo githubapi.Repository, allLists []githubapi.StarList, fromListID string,
) *modal {
	// Filter out the current list from the picker.
	filtered := make([]githubapi.StarList, 0, len(allLists))
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
	repo githubapi.Repository, fromListID string,
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

func newBulkAddModal(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	allLists []githubapi.StarList,
) *modal {
	mo := &modal{
		kind:    modalPickList,
		title:   fmt.Sprintf("Add %d repos: pick list", len(nwos)),
		choices: allLists,
		ctx:     ctx,
		svc:     svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		if len(m.choices) == 0 {
			return nil
		}
		return bulkAddReposCmd(ctx, svc, nwos, m.choices[m.choiceCursor].ID)
	}
	return mo
}

func newBulkRemoveModal(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	fromListID string,
) *modal {
	mo := &modal{
		kind:  modalConfirmYesNo,
		title: fmt.Sprintf("Remove %d repos from list?", len(nwos)),
		ctx:   ctx,
		svc:   svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		return bulkRemoveReposCmd(ctx, svc, nwos, fromListID)
	}
	return mo
}

func newBulkMoveModal(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	allLists []githubapi.StarList,
	fromListID string,
) *modal {
	filtered := make([]githubapi.StarList, 0, len(allLists))
	for _, l := range allLists {
		if l.ID != fromListID {
			filtered = append(filtered, l)
		}
	}
	mo := &modal{
		kind:    modalPickList,
		title:   fmt.Sprintf("Move %d repos: pick list", len(nwos)),
		choices: filtered,
		ctx:     ctx,
		svc:     svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		if len(m.choices) == 0 {
			return nil
		}
		return bulkMoveReposCmd(ctx, svc, nwos, fromListID, m.choices[m.choiceCursor].ID)
	}
	return mo
}

func newUnstarRepoModal(ctx context.Context, svc githubapi.Service,
	repo githubapi.Repository,
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
		return unstarRepoCmd(ctx, svc, repo.NameWithOwner)
	}
	return mo, focusCmd
}

// update handles key events while a modal is active.
// Returns (nil, nil) to close, or (updated modal, cmd).
func (mo *modal) update(msg tea.KeyPressMsg) (*modal, tea.Cmd) {
	switch mo.kind {
	case modalCreateList, modalEditList:
		return mo.updateForm(msg)
	case modalDeleteList, modalConfirmText:
		return mo.updateConfirmText(msg)
	case modalPickList:
		return mo.updatePickList(msg)
	case modalConfirmYesNo:
		return mo.updateConfirmYesNo(msg)
	default:
		if msg.String() == "esc" {
			return nil, nil
		}
		return mo, nil
	}
}

func (mo *modal) updateForm(msg tea.KeyPressMsg) (*modal, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return nil, nil
	case "tab", "down":
		// advance focus through inputs + visibility toggle
		mo.focusedIdx = (mo.focusedIdx + 1) % (len(mo.inputs) + 1)
		return mo.syncFocus()
	case "shift+tab", "up":
		mo.focusedIdx = (mo.focusedIdx - 1 + len(mo.inputs) + 1) % (len(mo.inputs) + 1)
		return mo.syncFocus()
	case "enter":
		if mo.focusedIdx == len(mo.inputs) {
			// cursor is on visibility toggle -- toggle it
			mo.privateState = (mo.privateState + 1) % 3
			return mo, nil
		}
		// Enter on a text field: advance to next, or submit if on last field
		if mo.focusedIdx < len(mo.inputs)-1 {
			mo.focusedIdx++
			return mo.syncFocus()
		}
		// on last text field: validate and submit
		return mo.trySubmitForm()
	case " ":
		if mo.focusedIdx == len(mo.inputs) {
			mo.privateState = (mo.privateState + 1) % 3
			return mo, nil
		}
	}
	// delegate keypress to focused input
	if mo.focusedIdx < len(mo.inputs) {
		var cmd tea.Cmd
		mo.inputs[mo.focusedIdx], cmd = mo.inputs[mo.focusedIdx].Update(msg)
		return mo, cmd
	}
	return mo, nil
}

func (mo *modal) syncFocus() (*modal, tea.Cmd) {
	var cmds []tea.Cmd
	for i := range mo.inputs {
		if i == mo.focusedIdx {
			cmds = append(cmds, mo.inputs[i].Focus())
		} else {
			mo.inputs[i].Blur()
		}
	}
	return mo, tea.Batch(cmds...)
}

func (mo *modal) trySubmitForm() (*modal, tea.Cmd) {
	name := mo.inputs[0].Value()
	if name == "" {
		mo.formErr = "Name is required."
		return mo, nil
	}
	cmd := mo.onConfirm(mo)
	return nil, cmd // close modal, fire mutation
}

func (mo *modal) updateConfirmText(msg tea.KeyPressMsg) (*modal, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return nil, nil
	case "enter":
		if mo.confirmInput.Value() == mo.confirmExpected {
			cmd := mo.onConfirm(mo)
			return nil, cmd
		}
		mo.formErr = "Name doesn't match."
		return mo, nil
	default:
		var cmd tea.Cmd
		mo.confirmInput, cmd = mo.confirmInput.Update(msg)
		return mo, cmd
	}
}

func (mo *modal) updatePickList(msg tea.KeyPressMsg) (*modal, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return nil, nil
	case "up", "k":
		if mo.choiceCursor > 0 {
			mo.choiceCursor--
		}
		return mo, nil
	case "down", "j":
		if mo.choiceCursor < len(mo.choices)-1 {
			mo.choiceCursor++
		}
		return mo, nil
	case "enter":
		if len(mo.choices) == 0 {
			return nil, nil
		}
		cmd := mo.onConfirm(mo)
		return nil, cmd
	}
	return mo, nil
}

func (mo *modal) updateConfirmYesNo(msg tea.KeyPressMsg) (*modal, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		return nil, nil
	case "y", "Y", "enter":
		cmd := mo.onConfirm(mo)
		return nil, cmd
	}
	return mo, nil
}

// view renders the modal box contents (without the outer overlay).
func (mo *modal) view() string {
	title := styleModalTitle.Render(mo.title)
	var body string
	var hint string

	switch mo.kind {
	case modalCreateList, modalEditList:
		body = mo.viewForm()
		hint = styleFaint.Render("tab: next field  esc: cancel")
	case modalDeleteList:
		body = mo.viewConfirmText("Type the list name to confirm deletion:")
		hint = styleFaint.Render("enter: confirm  esc: cancel")
	case modalConfirmText:
		body = mo.viewConfirmText("Type the repo name to confirm:")
		hint = styleFaint.Render("enter: confirm  esc: cancel")
	case modalPickList:
		body = mo.viewPickList()
		hint = styleFaint.Render("j/k: move  enter: select  esc: cancel")
	case modalConfirmYesNo:
		body = mo.viewConfirmYesNo()
		hint = styleFaint.Render("y: confirm  n/esc: cancel")
	default:
		body = mo.body
		if body == "" {
			body = "not wired yet"
		}
		hint = styleFaint.Render("esc: cancel")
	}

	result := title + "\n\n" + body
	if mo.formErr != "" {
		result += "\n" + styleError.Render(mo.formErr)
	}
	return result + "\n\n" + hint
}

func (mo *modal) viewForm() string {
	labels := []string{"Name:", "Description:"}
	var lines []string
	for i, inp := range mo.inputs {
		prefix := "  "
		if i == mo.focusedIdx {
			prefix = "> "
		}
		lines = append(lines, styleFaint.Render(labels[i]))
		lines = append(lines, prefix+inp.View())
	}
	// visibility toggle
	visPrefix := "  "
	if mo.focusedIdx == len(mo.inputs) {
		visPrefix = "> "
	}
	visLabel := "[unset]"
	switch mo.privateState {
	case 1:
		visLabel = "Public"
	case 2:
		visLabel = "Private"
	}
	lines = append(lines, styleFaint.Render("Visibility:"))
	lines = append(lines, visPrefix+visLabel+" (space/enter to cycle)")
	return strings.Join(lines, "\n")
}

func (mo *modal) viewConfirmText(prompt string) string {
	return styleFaint.Render(prompt) + "\n> " + mo.confirmInput.View()
}

func (mo *modal) viewPickList() string {
	if len(mo.choices) == 0 {
		return "(no lists available)"
	}
	const maxVisible = 8
	start := 0
	if mo.choiceCursor >= maxVisible {
		start = mo.choiceCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(mo.choices) {
		end = len(mo.choices)
	}
	var lines []string
	for i := start; i < end; i++ {
		prefix := "  "
		if i == mo.choiceCursor {
			prefix = "> "
			lines = append(lines, prefix+styleSelected.Render(mo.choices[i].Name))
		} else {
			lines = append(lines, prefix+mo.choices[i].Name)
		}
	}
	return strings.Join(lines, "\n")
}

func (mo *modal) viewConfirmYesNo() string {
	return styleFaint.Render("Remove repo from current list?") + "\n\n" +
		"[y] Yes  [n/esc] No"
}
