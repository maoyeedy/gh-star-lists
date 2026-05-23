package tui

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func TestFooterCoreHintsOnly(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.width = 60

	footer := renderFooter(
		m.active,
		m.listSearchActive,
		m.repoSearchActive,
		m.selected,
		m.statusMsg,
		m.statusExpiry,
	)
	// Core hints must be present.
	for _, want := range []string{"search", "help", "quit"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer missing %q; got: %s", want, footer)
		}
	}
	// Should not contain the old dense hint markers.
	for _, banned := range []string{"ctrl+r:refresh", "pg/g/G:scroll"} {
		if strings.Contains(footer, banned) {
			t.Errorf("footer contains banned dense hint %q", banned)
		}
	}
}

func TestHeaderPriorityTruncation(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.height = 24
	// Give it a very long focused list name.
	longName := "this-is-a-very-long-list-name-that-exceeds-the-terminal-width"
	fl := domain.StarList{ID: "UL_1", Name: longName, RepoCount: 5}
	m.lists = []domain.StarList{fl}
	m.focusedList = &m.lists[0]
	// Set a non-default sort so sortLabel would appear if there is room.
	m.sortLists = sortListsName // produces "name" label

	m.width = 60
	header := m.renderHeader()

	// App name must always be present.
	if !strings.Contains(header, "gh star-lists") {
		t.Errorf("header must contain 'gh star-lists'; got: %s", header)
	}
	// Sort label should be absent (dropped due to narrow width).
	if strings.Contains(header, "[sort:") {
		t.Errorf("header should not contain sort label when terminal is narrow; got: %s", header)
	}
	// Visible width of the rendered header must not exceed m.width.
	visW := lipgloss.Width(header)
	if visW > m.width {
		t.Errorf("header visible width %d exceeds terminal width %d", visW, m.width)
	}

	for _, width := range []int{4, 12, 20} {
		m.width = width
		header = m.renderHeader()
		if visW := lipgloss.Width(header); visW > width {
			t.Errorf("header visible width %d exceeds terminal width %d: %q", visW, width, header)
		}
	}
}

// TestFooterKeyTokensStyled verifies that with a real color profile the footer
// wraps key tokens in ANSI escape sequences (i.e., they are styled, not plain).
func TestFooterKeyTokensStyled(t *testing.T) {
	t.Parallel()
	// styleFooterKey is Bold; in a real (non-NoTTY) profile the renderer
	// emits escape sequences. We can detect this by comparing the rendered
	// key with the raw key string.
	keyRendered := styleFooterKey.Render("q")
	if keyRendered == "q" {
		// lipgloss may strip styles when it detects no TTY; skip rather than fail.
		t.Skip("color profile strips ANSI (no TTY); skipping styled-token assertion")
	}

	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.width = 80

	footer := renderFooter(
		m.active,
		m.listSearchActive,
		m.repoSearchActive,
		m.selected,
		m.statusMsg,
		m.statusExpiry,
	)

	// The footer must contain the styled rendering of the "q" key, not just "q".
	if !strings.Contains(footer, keyRendered) {
		t.Errorf("footer does not contain styled 'q' token %q; got: %s", keyRendered, footer)
	}
}
