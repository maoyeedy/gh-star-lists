package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (mo *modal) update(msg tea.KeyPressMsg) (*modal, tea.Cmd) {
	// While a mutation is in flight, discard all key input.
	if mo.submitting {
		return mo, nil
	}
	if mo.bulkFailure != nil {
		return mo.updateBulkFailure(msg)
	}
	switch mo.kind {
	case modalCreateList, modalEditList:
		return mo.updateForm(msg)
	case modalDeleteList, modalConfirmText:
		return mo.updateConfirmText(msg)
	case modalPickList:
		return mo.updatePickList(msg)
	case modalConfirmYesNo:
		return mo.updateConfirmYesNo(msg)
	case modalHelp:
		return mo.updateHelp(msg)
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

func (mo *modal) updateHelp(msg tea.KeyPressMsg) (*modal, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return nil, nil
	case "up", "k":
		if mo.scrollOffset > 0 {
			mo.scrollOffset--
		}
		return mo, nil
	case "down", "j":
		mo.scrollOffset++
		return mo, nil
	case "pgup":
		mo.scrollOffset -= 20
		if mo.scrollOffset < 0 {
			mo.scrollOffset = 0
		}
		return mo, nil
	case "pgdown":
		mo.scrollOffset += 20
		return mo, nil
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
