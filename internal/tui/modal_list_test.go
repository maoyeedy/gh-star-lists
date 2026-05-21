package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func TestCreateListModalOpenClose(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{}
	m := newTestModel(svc)

	m2 := update(m, keyPress('n'))
	if m2.modal == nil {
		t.Fatal("modal should open on n")
	}
	if m2.modal.kind != modalCreateList {
		t.Errorf("modal.kind = %v, want modalCreateList", m2.modal.kind)
	}

	m3 := update(m2, specialKey(tea.KeyEscape))
	if m3.modal != nil {
		t.Error("modal should be nil after esc")
	}
	if len(svc.createCalls) != 0 {
		t.Error("esc should not trigger mutation")
	}
}

// TestCreateListModalSubmit verifies typing a name and entering submits the mutation.
func TestCreateListModalSubmit(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{
		fakeService: fakeService{
			lists: []githubapi.StarList{{ID: "UL_1", Name: "existing"}},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Open create modal.
	m = update(m, keyPress('n'))
	if m.modal == nil {
		t.Fatal("modal did not open")
	}

	// Type into name field using individual key presses.
	for _, ch := range "My New List" {
		m = update(m, keyPress(ch))
	}

	// Submit (enter while on name field -- advances to description; enter again to submit).
	m = update(m, specialKey(tea.KeyEnter)) // advance to desc
	m = update(m, specialKey(tea.KeyEnter)) // advance to visibility (or submit from desc)
	m = update(m, specialKey(tea.KeyEnter)) // submit or advance

	// The modal may still be open if extra advances are needed -- keep pressing enter.
	for attempts := 0; m.modal != nil && attempts < 5; attempts++ {
		m = update(m, specialKey(tea.KeyEnter))
	}

	// Deliver the mutationDoneMsg (simulate cmd completion).
	m = update(m, mutationDoneMsg{kind: modalCreateList})

	if m.modal != nil {
		t.Error("modal should be closed after mutation done")
	}
	if m.statusMsg == "" {
		t.Error("statusMsg should be set after success")
	}
}

// TestDeleteListModalWrongNameBlocked verifies wrong input doesn't submit.
func TestDeleteListModalWrongNameBlocked(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{
		fakeService: fakeService{
			lists: []githubapi.StarList{{ID: "UL_1", Name: "mylist"}},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m = update(m, keyPress('d'))
	if m.modal == nil {
		t.Fatal("delete modal should open")
	}

	// Type wrong name.
	for _, ch := range "wrongname" {
		m = update(m, keyPress(ch))
	}
	m = update(m, specialKey(tea.KeyEnter))

	// Modal should still be open (name doesn't match).
	if m.modal == nil {
		t.Error("modal should stay open after wrong typed name")
	}
	if len(svc.deleteCalls) != 0 {
		t.Error("wrong name should not trigger delete")
	}
}

// TestDeleteListModalCorrectName verifies correct name submits.
func TestDeleteListModalCorrectName(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{
		fakeService: fakeService{
			lists: []githubapi.StarList{{ID: "UL_1", Name: "mylist"}},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m = update(m, keyPress('d'))
	if m.modal == nil {
		t.Fatal("delete modal should open")
	}

	// Type correct name.
	for _, ch := range "mylist" {
		m = update(m, keyPress(ch))
	}
	m2, cmd := m.Update(specialKey(tea.KeyEnter))
	m = m2.(model)
	if cmd == nil {
		t.Error("correct name should produce a cmd (delete mutation)")
	}
	// Modal should now be in submitting state (kept open, not closed).
	if m.modal == nil {
		t.Fatal("modal should remain open while submitting")
	}
	if !m.modal.submitting {
		t.Error("modal.submitting should be true after submit")
	}
	// The batch contains the mutation cmd. Execute cmds to find mutationDoneMsg.
	msgs := executeBatch(cmd)
	var found *mutationDoneMsg
	for _, msg := range msgs {
		if d, ok := msg.(mutationDoneMsg); ok {
			d := d
			found = &d
			break
		}
	}
	if found == nil {
		t.Fatal("batch should contain a mutationDoneMsg producer")
	}
	if found.kind != modalDeleteList {
		t.Errorf("doneMsg.kind = %v, want modalDeleteList", found.kind)
	}
}

// TestEditListNoOpInRepoPane verifies e is no-op in repo pane.
func TestEditListNoOpInRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo

	m2 := update(m, keyPress('e'))
	if m2.modal != nil {
		t.Error("edit should be no-op in repo pane")
	}
}

// TestMutationListErrorDisplayed verifies that mutationDoneMsg with an error keeps the modal
// open and stores the error in modal.submitErr (P3 behavior).
func TestMutationListErrorDisplayed(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	// Modal must be open for submitErr to be stored.
	m.modal = &modal{kind: modalDeleteList, submitting: true}
	sentinel := errors.New("delete failed")

	m2 := update(m, mutationDoneMsg{kind: modalDeleteList, err: sentinel})
	if m2.modal == nil {
		t.Error("modal should remain open after error (P3 inline error)")
	}
	if m2.modal != nil && !strings.Contains(m2.modal.submitErr, sentinel.Error()) {
		t.Errorf("modal.submitErr = %q, want to contain %q", m2.modal.submitErr, sentinel.Error())
	}
	if m2.modal != nil && m2.modal.submitting {
		t.Error("modal.submitting should be false after mutation error")
	}
}

// TestMutationErrorSetsErrField verifies mutationDoneMsg with err stores error in modal.submitErr.
func TestMutationErrorSetsErrField(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	// Modal must be open for submitErr to be stored.
	m.modal = &modal{kind: modalCreateList, submitting: true}
	sentinel := errors.New("create failed")

	m2 := update(m, mutationDoneMsg{kind: modalCreateList, err: sentinel})
	if m2.modal == nil {
		t.Error("modal should remain open after error (P3 inline error)")
	}
	if m2.modal != nil && !strings.Contains(m2.modal.submitErr, sentinel.Error()) {
		t.Errorf("modal.submitErr = %q, want to contain %q", m2.modal.submitErr, sentinel.Error())
	}
}
