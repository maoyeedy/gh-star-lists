package tui

import (
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/search"
)

func (m model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	activeList := m.active == paneList
	switch {
	case key.Matches(msg, keys.Back):
		if activeList {
			m.listSearchActive = false
			m.listSearchQuery = ""
			m.listCursor = 0
			m.listOffset = 0
		} else {
			m.repoSearchActive = false
			m.repoSearchQuery = ""
			m.repoCursor = 0
			m.repoOffset = 0
		}
		m = m.rebuildDisplayed()
		return m, nil
	case key.Matches(msg, keys.Enter):
		if activeList {
			m.listSearchActive = false
		} else {
			m.repoSearchActive = false
		}
		return m, nil
	case msg.Code == tea.KeyBackspace || msg.Code == tea.KeyDelete:
		if activeList {
			m.listSearchQuery = dropLastRune(m.listSearchQuery)
			m.listCursor = 0
			m.listOffset = 0
		} else {
			m.repoSearchQuery = dropLastRune(m.repoSearchQuery)
			m.repoCursor = 0
			m.repoOffset = 0
		}
		m = m.rebuildDisplayed()
		return m, nil
	}
	// Pass navigation keys to handleKey so arrows/PgDn still work.
	if key.Matches(msg, keys.Up) || key.Matches(msg, keys.Down) ||
		key.Matches(msg, keys.PgUp) || key.Matches(msg, keys.PgDn) ||
		key.Matches(msg, keys.Home) || key.Matches(msg, keys.End) {
		return m.handleKey(msg)
	}
	if msg.Text != "" {
		if activeList {
			m.listSearchQuery += msg.Text
			m.listCursor = 0
			m.listOffset = 0
		} else {
			m.repoSearchQuery += msg.Text
			m.repoCursor = 0
			m.repoOffset = 0
		}
		m = m.rebuildDisplayed()
	}
	return m, nil
}

func (m model) activateSearch() (tea.Model, tea.Cmd) {
	if m.active == paneList {
		m.listSearchActive = true
		m.listSearchQuery = ""
		m.listCursor = 0
		m.listOffset = 0
	} else {
		m.repoSearchActive = true
		m.repoSearchQuery = ""
		m.repoCursor = 0
		m.repoOffset = 0
	}
	m = m.rebuildDisplayed()
	return m, nil
}

func (m model) rebuildDisplayed() model {
	repos := m.currentRepos()
	if m.listSearchQuery == "" {
		m.displayedLists = withVirtualLists(m.lists)
	} else {
		m.displayedLists = search.FilterStarLists(withVirtualLists(m.lists), m.listSearchQuery)
	}
	if m.repoSearchQuery == "" {
		m.displayedRepos = repos
	} else {
		m.displayedRepos = search.FilterRepositories(repos, m.repoSearchQuery)
	}
	return m
}

func withVirtualLists(lists []domain.StarList) []domain.StarList {
	displayed := make([]domain.StarList, 0, len(lists)+1)
	displayed = append(displayed, unlistedVirtualList())
	displayed = append(displayed, lists...)
	return displayed
}

func dropLastRune(s string) string {
	_, size := utf8.DecodeLastRuneInString(s)
	if size == 0 {
		return s
	}
	return s[:len(s)-size]
}
