package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func TestAddRepoModalOpensInRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	m2 := update(m, keyPress('a'))
	if m2.modal == nil {
		t.Fatal("modal should open on 'a' in repo pane")
	}
	if m2.modal.kind != modalPickList {
		t.Errorf("modal.kind = %v, want modalPickList", m2.modal.kind)
	}
	if len(m2.modal.choices) != len(svc.lists) {
		t.Errorf("picker choices = %d, want %d (all lists)", len(m2.modal.choices), len(svc.lists))
	}
}

// TestAddRepoNoOpInListPane verifies 'a' is no-op in list pane.
func TestAddRepoNoOpInListPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList

	m2 := update(m, keyPress('a'))
	if m2.modal != nil {
		t.Error("'a' should be no-op in list pane")
	}
}

// TestMoveRepoExcludesCurrentList verifies move picker excludes the current list.
func TestMoveRepoExcludesCurrentList(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0] // UL_1

	m2 := update(m, keyPress('m'))
	if m2.modal == nil {
		t.Fatal("modal should open on 'm'")
	}
	for _, choice := range m2.modal.choices {
		if choice.ID == "UL_1" {
			t.Error("move picker should not include current list UL_1")
		}
	}
}

// TestPickListNavigation verifies j/k cursor movement and Enter calls onConfirm.
func TestPickListNavigation(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	m = update(m, keyPress('a'))
	if m.modal == nil {
		t.Fatal("modal should be open")
	}
	// Move cursor down.
	m = update(m, keyPress('j'))
	if m.modal.choiceCursor != 1 {
		t.Errorf("choiceCursor after j = %d, want 1", m.modal.choiceCursor)
	}
	// Esc cancels.
	m = update(m, specialKey(tea.KeyEscape))
	if m.modal != nil {
		t.Error("modal should close on esc")
	}
}

// TestRemoveRepoConfirmYesNo verifies y confirms, n cancels.
func TestRemoveRepoConfirmYesNo(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	// Open remove modal.
	m2 := update(m, keyPress('x'))
	if m2.modal == nil {
		t.Fatal("remove modal should open")
	}
	if m2.modal.kind != modalConfirmYesNo {
		t.Errorf("modal.kind = %v, want modalConfirmYesNo", m2.modal.kind)
	}

	// 'n' cancels.
	m3 := update(m2, keyPress('n'))
	if m3.modal != nil {
		t.Error("modal should close on 'n'")
	}

	// Reopen and 'y' fires the command.
	m4 := update(m, keyPress('x'))
	_, cmd := m4.Update(keyPress('y'))
	if cmd == nil {
		t.Error("'y' should produce a mutation cmd")
	}
}

// TestAddRepoCmd verifies the set-union logic in addRepoToListCmd.
func TestAddRepoCmd(t *testing.T) {
	t.Parallel()
	svc := &repoMutationFakeService{}
	svc.membershipsResult.repoID = "R_1"
	svc.membershipsResult.listIDs = []string{"UL_2"}

	cmd := addRepoToListCmd(context.Background(), svc, "owner/repo", "UL_3")
	msg := cmd()
	done, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want mutationDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if len(svc.updateListsCalls) != 1 {
		t.Fatalf("UpdateRepositoryLists calls = %d, want 1", len(svc.updateListsCalls))
	}
	got := svc.updateListsCalls[0].listIDs
	want := []string{"UL_2", "UL_3"} // sorted
	if len(got) != len(want) {
		t.Fatalf("listIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("listIDs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestRemoveRepoCmd verifies the set-remove logic in removeRepoFromListCmd.
func TestRemoveRepoCmd(t *testing.T) {
	t.Parallel()
	svc := &repoMutationFakeService{}
	svc.membershipsResult.repoID = "R_1"
	svc.membershipsResult.listIDs = []string{"UL_1", "UL_2"}

	cmd := removeRepoFromListCmd(context.Background(), svc, "owner/repo", "UL_1")
	msg := cmd()
	done, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want mutationDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	got := svc.updateListsCalls[0].listIDs
	want := []string{"UL_2"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("listIDs = %v, want %v", got, want)
	}
}

func TestUnstarRepoCmd(t *testing.T) {
	t.Parallel()
	svc := &repoMutationFakeService{}

	cmd := unstarRepoCmd(context.Background(), svc, githubapi.Repository{
		ID:            "R_star_1",
		NameWithOwner: "owner/repo",
	})
	msg := cmd()
	done, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want mutationDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if len(svc.removeStarCalls) != 1 || svc.removeStarCalls[0] != "R_star_1" {
		t.Errorf("RemoveStar calls = %v, want [R_star_1]", svc.removeStarCalls)
	}
	if len(svc.membershipsCalls) != 0 {
		t.Errorf("GetRepositoryMemberships calls = %v, want none", svc.membershipsCalls)
	}
}
