package tui

import (
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/search"
)

func (m model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.searchActive = false
		m.searchQuery = ""
		m = m.rebuildDisplayed()
		m.listCursor = 0
		m.repoCursor = 0
		m.previewOffset = 0
		m.listOffset = 0
		m.repoOffset = 0
		return m, nil
	case key.Matches(msg, keys.Enter):
		m.searchActive = false
		return m, nil
	case msg.Code == tea.KeyBackspace || msg.Code == tea.KeyDelete:
		m.searchQuery = dropLastRune(m.searchQuery)
		m = m.rebuildDisplayed()
		m.listCursor = 0
		m.repoCursor = 0
		m.previewOffset = 0
		m.listOffset = 0
		m.repoOffset = 0
		return m, nil
	}
	// Pass navigation keys to handleKey so arrows/PgDn still work.
	if key.Matches(msg, keys.Up) || key.Matches(msg, keys.Down) ||
		key.Matches(msg, keys.PgUp) || key.Matches(msg, keys.PgDn) ||
		key.Matches(msg, keys.Home) || key.Matches(msg, keys.End) {
		return m.handleKey(msg)
	}
	if msg.Text != "" {
		m.searchQuery += msg.Text
		m = m.rebuildDisplayed()
		m.listCursor = 0
		m.repoCursor = 0
		m.previewOffset = 0
		m.listOffset = 0
		m.repoOffset = 0
	}
	return m, nil
}

func (m model) rebuildDisplayed() model {
	repos := m.currentRepos()
	if m.searchQuery == "" {
		m.displayedLists = m.lists
		m.displayedRepos = repos
	} else {
		m.displayedLists = search.FilterStarLists(m.lists, m.searchQuery)
		m.displayedRepos = search.FilterRepositories(repos, m.searchQuery)
	}
	return m
}

func dropLastRune(s string) string {
	_, size := utf8.DecodeLastRuneInString(s)
	if size == 0 {
		return s
	}
	return s[:len(s)-size]
}
