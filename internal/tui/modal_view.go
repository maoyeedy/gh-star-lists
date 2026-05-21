package tui

import (
	"strings"
)

func (mo *modal) view() string {
	title := styleModalTitle.Render(mo.title)
	var body string
	var hint string

	if mo.bulkFailure != nil && !mo.submitting {
		body = mo.viewBulkFailure()
		hint = stylePaneSubtitle.Render("enter/r: retry failed  esc: close")
		if len(mo.bulkFailure.failedNWOs) > bulkFailureMaxVisible {
			hint = stylePaneSubtitle.Render("j/k: scroll  enter/r: retry failed  esc: close")
		}
	} else {
		switch mo.kind {
		case modalCreateList, modalEditList:
			body = mo.viewForm()
			hint = stylePaneSubtitle.Render("tab: next field  esc: cancel")
		case modalDeleteList:
			body = mo.viewConfirmText("Type the list name to confirm deletion:")
			hint = stylePaneSubtitle.Render("enter: confirm  esc: cancel")
		case modalConfirmText:
			body = mo.viewConfirmText("Type the repo name to confirm:")
			hint = stylePaneSubtitle.Render("enter: confirm  esc: cancel")
		case modalPickList:
			body = mo.viewPickList()
			hint = stylePaneSubtitle.Render("j/k: move  enter: select  esc: cancel")
		case modalConfirmYesNo:
			body = mo.viewConfirmYesNo()
			hint = stylePaneSubtitle.Render("y: confirm  n/esc: cancel")
		default:
			body = mo.body
			if body == "" {
				body = "not wired yet"
			}
			hint = stylePaneSubtitle.Render("esc: cancel")
		}
	}

	result := title + "\n\n" + body
	if mo.formErr != "" {
		result += "\n" + styleError.Render(mo.formErr)
	}
	// Show previous submission error (only when not currently submitting).
	if mo.submitErr != "" && !mo.submitting {
		result += "\n" + styleError.Render(mo.submitErr)
	}
	if mo.submitting {
		result += "\n\n" + stylePaneSubtitle.Render("  ... submitting...")
	} else {
		result += "\n\n" + hint
	}
	return result
}

func (mo *modal) viewForm() string {
	labels := []string{"Name:", "Description:"}
	var lines []string
	for i, inp := range mo.inputs {
		prefix := "  "
		if i == mo.focusedIdx {
			prefix = "> "
		}
		lines = append(lines, stylePaneSubtitle.Render(labels[i]))
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
	lines = append(lines, stylePaneSubtitle.Render("Visibility:"))
	lines = append(lines, visPrefix+visLabel+" (space/enter to cycle)")
	return strings.Join(lines, "\n")
}

func (mo *modal) viewConfirmText(prompt string) string {
	return stylePaneSubtitle.Render(prompt) + "\n> " + mo.confirmInput.View()
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
			lines = append(lines, prefix+styleCursorActive.Render(mo.choices[i].Name))
		} else {
			lines = append(lines, prefix+mo.choices[i].Name)
		}
	}
	return strings.Join(lines, "\n")
}

func (mo *modal) viewConfirmYesNo() string {
	return stylePaneSubtitle.Render("Remove repo from current list?") + "\n\n" +
		"[y] Yes  [n/esc] No"
}
