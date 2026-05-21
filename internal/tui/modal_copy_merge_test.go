package tui

import (
	"context"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func TestCopyListModalOpens(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList

	m2 := update(m, keyPress('c'))
	if m2.modal == nil {
		t.Fatal("copy modal should open on 'c'")
	}
	if m2.modal.kind != modalPickList {
		t.Errorf("modal.kind = %v, want modalPickList", m2.modal.kind)
	}
	// Source list excluded from choices.
	if len(m2.modal.choices) != len(svc.lists)-1 {
		t.Errorf(
			"choices = %d, want %d (all except source)",
			len(m2.modal.choices),
			len(svc.lists)-1,
		)
	}
}

// TestMergeListModalTitle verifies 'C' modal has destructive indicator.
func TestMergeListModalTitle(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList

	m2 := update(m, keyPress('C'))
	if m2.modal == nil {
		t.Fatal("merge modal should open on 'C'")
	}
	if !containsStr(m2.modal.title, "source deleted") && !containsStr(m2.modal.title, "Merge") {
		t.Errorf("merge modal title = %q, want to contain 'Merge'", m2.modal.title)
	}
}

// TestCopyMergeNoOpInRepoPane verifies c/C are no-ops in repo pane.
func TestCopyMergeNoOpInRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo

	for _, k := range []rune{'c', 'C'} {
		m2 := update(m, keyPress(k))
		if m2.modal != nil {
			t.Errorf("key %c should be no-op in repo pane", k)
		}
	}
}

// TestCopyListCmd verifies repos are added to target list via UpdateRepositoryLists.
func TestCopyListCmd(t *testing.T) {
	t.Parallel()
	svc := &copyMergeFakeService{
		reposResult: []githubapi.Repository{
			{NameWithOwner: "owner/repo1"},
		},
		membershipsRepoID:  "R_1",
		membershipsListIDs: []string{"UL_src"},
	}
	cmd := copyListCmd(context.Background(), svc, "UL_src", "UL_dst", false)
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
	got := svc.updateListsCalls[0]
	// Should contain both src and dst.
	found := false
	for _, id := range got {
		if id == "UL_dst" {
			found = true
		}
	}
	if !found {
		t.Errorf("listIDs %v should contain UL_dst", got)
	}
	// No delete because deleteSource=false.
	if len(svc.deleteListCalls) != 0 {
		t.Error("DeleteStarList should not be called for copy (not merge)")
	}
}

// TestMergeListCmdDeletesSource verifies DeleteStarList is called when deleteSource=true.
func TestMergeListCmdDeletesSource(t *testing.T) {
	t.Parallel()
	svc := &copyMergeFakeService{
		reposResult: []githubapi.Repository{
			{NameWithOwner: "owner/repo1"},
		},
		membershipsRepoID:  "R_1",
		membershipsListIDs: []string{"UL_src"},
	}
	cmd := copyListCmd(context.Background(), svc, "UL_src", "UL_dst", true)
	msg := cmd()
	done, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want mutationDoneMsg", msg)
	}
	if done.err != nil {
		t.Fatalf("unexpected error: %v", done.err)
	}
	if len(svc.deleteListCalls) != 1 || svc.deleteListCalls[0] != "UL_src" {
		t.Errorf("DeleteStarList calls = %v, want [UL_src]", svc.deleteListCalls)
	}
}
