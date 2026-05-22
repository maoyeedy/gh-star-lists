package tui

import (
	"strings"
	"testing"
)

func TestBulkDoneMsgClearsSelectionAndSetsToast(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.selected = map[string]struct{}{
		"owner/a-repo": {},
		"owner/b-repo": {},
	}

	m2 := update(m, bulkDoneMsg{verb: "added", succeeded: 2, failed: 0})

	if len(m2.selected) != 0 {
		t.Errorf("selected should be cleared after bulkDoneMsg, got %d", len(m2.selected))
	}
	if m2.statusMsg == "" {
		t.Error("statusMsg should be set after bulkDoneMsg")
	}
	if !strings.Contains(m2.statusMsg, "added") {
		t.Errorf("statusMsg = %q, want to contain 'added'", m2.statusMsg)
	}
}

// TestBulkDoneMsgPartialFailureToast verifies toast mentions failed count.
func TestBulkDoneMsgPartialFailureToast(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m2 := update(m, bulkDoneMsg{verb: "removed", succeeded: 1, failed: 1})

	if !strings.Contains(m2.statusMsg, "failed") {
		t.Errorf("statusMsg = %q, want to contain 'failed'", m2.statusMsg)
	}
}

// TestBulkDoneMsgPartialFailureKeepsModalOpen verifies that partial bulk failures
// stay in the modal and list failed repositories.
func TestBulkDoneMsgPartialFailureKeepsModalOpen(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m.modal = newBulkRemoveModal(
		m.ctx,
		m.svc,
		[]string{"owner/a-repo", "owner/b-repo"},
		m.lists,
		m.focusedList.ID,
	)
	m.modal.submitting = true
	m.selected = map[string]struct{}{
		"owner/a-repo": {},
		"owner/b-repo": {},
	}

	m2 := update(m, bulkDoneMsg{
		verb:       "removed",
		succeeded:  1,
		failed:     1,
		failedNWOs: []string{"owner/b-repo"},
	})

	if m2.modal == nil {
		t.Fatal("modal should remain open after partial bulk failure")
	}
	if m2.modal.submitting {
		t.Error("modal.submitting should be false after partial bulk failure")
	}
	if m2.modal.bulkFailure == nil {
		t.Fatal("modal.bulkFailure should be set after partial bulk failure")
	}
	if got := m2.modal.bulkFailure.failedNWOs; len(got) != 1 || got[0] != "owner/b-repo" {
		t.Fatalf("failedNWOs = %v, want [owner/b-repo]", got)
	}
	if len(m2.selected) != 0 {
		t.Errorf("selected should be cleared after bulkDoneMsg, got %d", len(m2.selected))
	}
	rendered := m2.modal.view()
	if !strings.Contains(rendered, "owner/b-repo") {
		t.Errorf("modal view = %q, want failed repo name", rendered)
	}
	if !strings.Contains(rendered, "retry") {
		t.Errorf("modal view = %q, want retry hint", rendered)
	}
}
