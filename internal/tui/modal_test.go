package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func TestModalOpenAndClose(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	m2 := update(m, keyPress('n'))
	if m2.modal == nil {
		t.Fatal("modal should be open after 'n'")
	}

	m3 := update(m2, specialKey(tea.KeyEscape))
	if m3.modal != nil {
		t.Error("modal should be closed after esc")
	}
}

func TestSearchNoModalOnSlash(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.modal = &modal{kind: modalConfirmYesNo}

	m2 := update(m, keyPress('/'))

	if m2.listSearchActive || m2.repoSearchActive {
		t.Error("search should stay inactive when modal is open")
	}
}

func TestMutationModalStaysOpenWhileSubmitting(t *testing.T) {
	t.Parallel()
	svc := &recordingFakeService{
		fakeService: fakeService{
			lists: []domain.StarList{{ID: "UL_1", Name: "existing"}},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Open create-list modal.
	m = update(m, keyPress('n'))
	if m.modal == nil {
		t.Fatal("create modal should be open")
	}

	// Type a name.
	for _, ch := range "NewList" {
		m = update(m, keyPress(ch))
	}

	// Submit: advance to last field, then submit.
	m = update(m, specialKey(tea.KeyEnter)) // advance to desc
	m = update(m, specialKey(tea.KeyEnter)) // advance to visibility or submit

	// Keep pressing enter until modal is submitting or we give up.
	for i := 0; i < 5; i++ {
		if m.modal != nil && m.modal.submitting {
			break
		}
		m = update(m, specialKey(tea.KeyEnter))
	}

	if m.modal == nil {
		t.Fatal("modal should remain open while submitting")
	}
	if !m.modal.submitting {
		t.Error("modal.submitting should be true after submit")
	}
	if !m.mutationPending {
		t.Error("mutationPending should be true while modal is submitting")
	}
}

// TestMutationDoneClosesModalAndInvalidatesEntry verifies that a successful
// mutationDoneMsg closes the modal, sets a toast, and removes the focused list's
// cache entries.
func TestMutationDoneClosesModalAndInvalidatesEntry(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.focusedList = &m.lists[0]

	// Pre-populate a cache entry to confirm it gets invalidated.
	m.preloader.cache["UL_1"] = &repoCacheEntry{state: repoCacheLoaded}
	m.preloader.cache["UL_1"] = &repoCacheEntry{state: repoCacheLoaded}

	// Open a modal in submitting state.
	m.modal = &modal{kind: modalCreateList, submitting: true}
	m.mutationPending = true

	m2 := update(m, mutationDoneMsg{kind: modalCreateList})

	if m2.modal != nil {
		t.Error("modal should be nil after successful mutationDoneMsg")
	}
	if m2.statusMsg == "" {
		t.Error("statusMsg should be set after successful mutation")
	}
	if m2.mutationPending {
		t.Error("mutationPending should be false after successful mutation")
	}
	// Both cache entries for UL_1 should be deleted (invalidated).
	if e := m2.preloader.cache["UL_1"]; e != nil &&
		e.state == repoCacheLoaded {
		t.Error("repoCache[UL_1, false] should be invalidated after mutation")
	}
	if e := m2.preloader.cache["UL_1"]; e != nil && e.state == repoCacheLoaded {
		t.Error("repoCache[UL_1, true] should be invalidated after mutation")
	}
}

// TestMutationErrorKeepsModalOpenWithMessage verifies that a failed mutationDoneMsg
// keeps the modal open with submitting=false and submitErr set.
func TestMutationErrorKeepsModalOpenWithMessage(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.modal = &modal{kind: modalCreateList, submitting: true}
	m.mutationPending = true
	someErr := errors.New("network timeout")

	m2 := update(m, mutationDoneMsg{kind: modalCreateList, err: someErr})

	if m2.modal == nil {
		t.Fatal("modal should remain open after mutation error")
	}
	if m2.modal.submitting {
		t.Error("modal.submitting should be false after error")
	}
	if !strings.Contains(m2.modal.submitErr, someErr.Error()) {
		t.Errorf("modal.submitErr = %q, want to contain %q", m2.modal.submitErr, someErr.Error())
	}
	if m2.mutationPending {
		t.Error("mutationPending should be false after error")
	}
}
