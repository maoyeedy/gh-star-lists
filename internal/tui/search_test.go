package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSearchActivatesOnSlash(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	m2 := update(m, keyPress('/'))

	if !m2.listSearchActive {
		t.Error("listSearchActive should be true after /")
	}
}

// TestSearchFiltersListsByQuery verifies that typing narrows displayedLists.
func TestSearchFiltersListsByQuery(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, keyPress('/'))

	// Type "alp" - should match "Alpha" from threeListsSvc.
	for _, ch := range "alp" {
		m = update(m, keyPress(ch))
	}

	if len(m.displayedLists) == 0 {
		t.Fatal("displayedLists should have at least one match for 'alp'")
	}
	if m.displayedLists[0].Name != "Alpha" {
		t.Errorf("first result = %q, want Alpha", m.displayedLists[0].Name)
	}
	// non-matching lists should not be displayed
	for _, l := range m.displayedLists {
		if l.Name != "Alpha" {
			t.Errorf("unexpected match %q for query 'alp'", l.Name)
		}
	}
}

// TestSearchEscClearsFilter verifies Esc restores full list.
func TestSearchEscClearsFilter(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, keyPress('/'))
	m = update(m, keyPress('z')) // "z" matches "zeta"
	if len(m.displayedLists) == 0 {
		t.Fatal("need at least one match to test clear")
	}

	m2 := update(m, specialKey(tea.KeyEscape))

	if m2.listSearchActive {
		t.Error("listSearchActive should be false after Esc")
	}
	if m2.listSearchQuery != "" {
		t.Errorf("listSearchQuery = %q, want empty after Esc", m2.listSearchQuery)
	}
	if len(m2.displayedLists) != len(svc.lists)+1 {
		t.Errorf(
			"displayedLists len = %d after Esc, want %d (all)",
			len(m2.displayedLists),
			len(svc.lists)+1,
		)
	}
}

// TestSearchEnterDeactivatesKeepsFilter verifies Enter deactivates search
// but keeps the current filter.
func TestSearchEnterDeactivatesKeepsFilter(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, keyPress('/'))
	// "alpha" uniquely matches only the "Alpha" list (not "zeta" or "beta").
	for _, ch := range "alpha" {
		m = update(m, keyPress(ch))
	}

	m2 := update(m, specialKey(tea.KeyEnter))

	if m2.listSearchActive {
		t.Error("listSearchActive should be false after Enter")
	}
	if m2.listSearchQuery == "" {
		t.Error("listSearchQuery should still be non-empty after Enter")
	}
	// displayedLists should still be filtered - only "Alpha" matches.
	if len(m2.displayedLists) >= len(svc.lists) {
		t.Error("displayedLists should still be filtered after Enter")
	}
}

// TestSearchResetsCursorToZero verifies cursor resets on each keystroke.
func TestSearchResetsCursorToZero(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.listCursor = 2
	m = update(m, keyPress('/'))
	m = update(m, keyPress('z'))

	if m.listCursor != 0 {
		t.Errorf("listCursor = %d after search input, want 0", m.listCursor)
	}
}

// TestSearchBackspaceRemovesLastChar verifies Backspace trims the query.
func TestSearchBackspaceRemovesLastChar(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, keyPress('/'))
	m = update(m, keyPress('a'))
	m = update(m, keyPress('l'))

	if m.listSearchQuery != "al" {
		t.Fatalf("listSearchQuery = %q before backspace, want 'al'", m.listSearchQuery)
	}
	m2 := update(m, specialKey(tea.KeyBackspace))

	if m2.listSearchQuery != "a" {
		t.Errorf("listSearchQuery = %q after backspace, want 'a'", m2.listSearchQuery)
	}
}

// TestSearchFilterRepoPane verifies filtering works in the repo pane.
func TestSearchFilterRepoPane(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneRepo
	m.focusedList = &m.lists[0]
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: "UL_1"})

	// Query "a-repo": the exact-name match "owner/a-repo" should appear and
	// rank first. The fuzzy algorithm may also surface "owner/b-repo" via
	// single-char edit distance on the "a" token - that is expected behaviour;
	// what matters is the correct repo is present and ranked at the top.
	m = update(m, keyPress('/'))
	for _, ch := range "a-repo" {
		m = update(m, keyPress(ch))
	}

	if len(m.displayedRepos) == 0 {
		t.Fatal("displayedRepos should have at least one match for 'a-repo'")
	}
	if m.displayedRepos[0].NameWithOwner != "owner/a-repo" {
		t.Errorf(
			"top result = %q, want 'owner/a-repo' (exact match should rank first)",
			m.displayedRepos[0].NameWithOwner,
		)
	}
}

func TestDropLastRuneMultiByte(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  string
	}{
		{"abc\u4e2d", "abc"},       // CJK ideograph (3 bytes)
		{"\u4e2d\u6587", "\u4e2d"}, // two CJK chars, drop last
		{"a\U0001F600", "a"},       // emoji (4 bytes)
		{"", ""},                   // empty -- no panic
	}
	for _, tc := range cases {
		got := dropLastRune(tc.input)
		if got != tc.want {
			t.Errorf("dropLastRune(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestSearchWhileFilterActiveActionKeys verifies action keys operate on
// displayedRepos when a search filter query is set but search input is committed
// (searchActive == false, searchQuery != "").
func TestSearchWhileFilterActiveActionKeys(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	// Commit a filter query via Enter (searchActive=false, query kept).
	m.repoSearchQuery = string([]rune(svc.repos[0].NameWithOwner)[:1]) // first char of first repo
	m.repoSearchActive = false
	m = m.rebuildDisplayed()
	if len(m.displayedRepos) == 0 {
		t.Skip("filter removed all repos -- fixture mismatch")
	}

	// Press 'a' (AddRepo): should open a modal targeting displayedRepos[cursor].
	m2 := update(m, keyPress('a'))
	if m2.modal == nil {
		t.Error("expected modal to open when pressing 'a' with filter active")
	}
}

// TestNarrowRepoPaneHidesMetadata verifies renderRepoPane omits

// TestHeaderPriorityTruncation verifies that with a very long list name and
// sort label on a narrow terminal, the app name is preserved and the sort label
