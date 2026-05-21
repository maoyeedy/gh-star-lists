package tui

import (
	"context"

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
	modalHelp
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
	formErr      string // inline validation error shown in modal

	// confirm-text modals (delete, unstar): typed name
	confirmInput    textinput.Model
	confirmExpected string

	// list-picker modals (add, move)
	choices      []githubapi.StarList
	choiceCursor int

	// mutation to run on confirm (set by constructor)
	// Returns (nil, cmd) when triggered.
	onConfirm func(mo *modal) tea.Cmd
	bulkRetry func(failedNWOs []string) tea.Cmd

	// submitting is true while a mutation command is in flight.
	// Input events are discarded and the view shows a "submitting" indicator.
	submitting bool
	// submitErr holds the error message from the last failed mutation attempt.
	// Cleared when submitting starts; displayed in the view when not submitting.
	submitErr   string
	bulkFailure *bulkFailureState

	// scroll offset for scrollable modals (help)
	scrollOffset int

	// context for cancel-without-side-effect
	ctx context.Context
	svc githubapi.Service
}
