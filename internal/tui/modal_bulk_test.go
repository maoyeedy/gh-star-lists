package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func TestBulkFailureRetryUsesFailedNWOsOnly(t *testing.T) {
	t.Parallel()
	svc := &repoMutationFakeService{}
	svc.lists = []domain.StarList{{ID: "UL_1", Name: "one"}}
	svc.repos = []domain.Repository{
		{ID: "R_b", NameWithOwner: "owner/b-repo"},
		{ID: "R_c", NameWithOwner: "owner/c-repo"},
	}
	m := newTestModel(svc)
	m.modal = newBulkRemoveModal(
		m.ctx,
		m.svc,
		[]string{"owner/a-repo", "owner/b-repo", "owner/c-repo"},
		svc.lists,
		"UL_1",
	)
	m.modal.submitting = true

	m = update(m, bulkDoneMsg{
		verb:       "removed",
		succeeded:  1,
		failed:     2,
		failedNWOs: []string{"owner/b-repo", "owner/c-repo"},
	})
	if m.modal == nil || m.modal.bulkFailure == nil {
		t.Fatal("modal should show bulk failure before retry")
	}

	next, cmd := m.Update(keyPress('r'))
	m2 := next.(model)
	if cmd == nil {
		t.Fatal("retry key should produce a command")
	}
	if m2.modal == nil || !m2.modal.submitting {
		t.Fatal("modal should remain open and submitting during retry")
	}
	executeBatch(cmd)

	want := map[string]bool{"R_b": true, "R_c": true}
	got := make(map[string]bool, len(svc.updateListsCalls))
	for _, call := range svc.updateListsCalls {
		got[call.repoID] = true
	}
	for repoID := range want {
		if !got[repoID] {
			t.Errorf("updateListsCalls missing repo %q", repoID)
		}
	}
	for repoID := range got {
		if !want[repoID] {
			t.Errorf("updateListsCalls unexpected repo %q", repoID)
		}
	}
}

// TestBulkFailureListScrolls verifies long failed-repo lists can scroll.
func TestBulkFailureListScrolls(t *testing.T) {
	t.Parallel()
	failed := []string{
		"owner/repo-01",
		"owner/repo-02",
		"owner/repo-03",
		"owner/repo-04",
		"owner/repo-05",
		"owner/repo-06",
		"owner/repo-07",
		"owner/repo-08",
		"owner/repo-09",
	}
	mo := &modal{
		kind: modalConfirmYesNo,
		bulkFailure: &bulkFailureState{
			verb:       "removed",
			succeeded:  1,
			failedNWOs: failed,
		},
		bulkRetry: func([]string) tea.Cmd { return nil },
	}

	before := mo.view()
	if !strings.Contains(before, "owner/repo-01") {
		t.Fatalf("initial view = %q, want first failed repo", before)
	}
	if strings.Contains(before, "owner/repo-09") {
		t.Fatalf("initial view = %q, should clip final failed repo", before)
	}

	updated, cmd := mo.update(keyPress('j'))
	if cmd != nil {
		t.Fatal("scrolling failure list should not produce a command")
	}
	mo = updated

	after := mo.view()
	if !strings.Contains(after, "owner/repo-09") {
		t.Errorf("scrolled view = %q, want final failed repo", after)
	}
	if !strings.Contains(after, "above") {
		t.Errorf("scrolled view = %q, want above indicator", after)
	}
}

// TestBulkAddModalOpenedWithSelection verifies 'a' opens bulk modal when repos selected.
func TestBulkAddModalOpenedWithSelection(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.selected = map[string]struct{}{"owner/a-repo": {}}

	m2 := update(m, keyPress('a'))

	if m2.modal == nil {
		t.Fatal("modal should open on 'a' with selection")
	}
	if m2.modal.kind != modalPickList {
		t.Errorf("modal.kind = %v, want modalPickList", m2.modal.kind)
	}
	if !strings.Contains(m2.modal.title, "1 repo") {
		t.Errorf("modal.title = %q, want to contain '1 repo'", m2.modal.title)
	}
}

func TestBulkMutationModalSubmittingState(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	// Select a repo.
	m.selected = map[string]struct{}{"owner/b-repo": {}}

	// Open bulk-add modal.
	m = update(m, keyPress('a'))
	if m.modal == nil {
		t.Fatal("bulk-add modal should open")
	}

	// Submit (enter on the picker).
	m = update(m, specialKey(tea.KeyEnter))

	if m.modal == nil {
		t.Fatal("modal should remain open while submitting (bulk)")
	}
	if !m.modal.submitting {
		t.Error("modal.submitting should be true after bulk-add submit")
	}
}
