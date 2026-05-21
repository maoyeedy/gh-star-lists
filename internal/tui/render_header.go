package tui

import (
	lipgloss "charm.land/lipgloss/v2"
)

func (m model) renderHeader() string {
	const appName = "gh star-lists"
	appW := lipgloss.Width(appName)

	// Build separator and list name segments.
	sep := styleSeparator.Render(" > ")
	sepW := lipgloss.Width(sep)

	// Sort label (only when non-default).
	sortLabel := m.currentSortLabel()
	sortSuffix := ""
	sortSuffixW := 0
	if sortLabel != "" {
		sortSuffix = "  [sort: " + sortLabel + "]"
		sortSuffixW = lipgloss.Width(sortSuffix)
	}

	if m.focusedList == nil {
		// No list focused: just app name, no sort label.
		return styleAppTitle.Render(appName)
	}

	// Available budget after app name + separator.
	budget := m.width - appW - sepW
	if budget < 0 {
		budget = 0
	}

	// Try to fit: list name + sort suffix.
	listName := m.focusedList.Name
	listNameW := lipgloss.Width(listName)

	if listNameW+sortSuffixW <= budget {
		// Everything fits.
		return styleAppTitle.Render(appName) +
			sep +
			stylePaneTitle.Render(listName) +
			stylePaneSubtitle.Render(sortSuffix)
	}

	// Sort label doesn't fit: drop it, try list name alone.
	if listNameW <= budget {
		return styleAppTitle.Render(appName) +
			sep +
			stylePaneTitle.Render(listName)
	}

	// Truncate list name to budget.
	const ellipsis = "..."
	ellipsisW := len(ellipsis)
	// Trim runes until visual width fits.
	runes := []rune(listName)
	for len(runes) > 0 && lipgloss.Width(string(runes))+ellipsisW > budget {
		runes = runes[:len(runes)-1]
	}
	truncated := string(runes) + ellipsis
	return styleAppTitle.Render(appName) +
		sep +
		stylePaneTitle.Render(truncated)
}

func (m model) currentSortLabel() string {
	if m.active == paneList {
		switch m.sortLists {
		case sortListsName:
			return "name"
		case sortListsRepos:
			return "repos"
		case sortListsAdded:
			return "added"
		default:
			return ""
		}
	}
	switch m.sortRepos {
	case sortReposName:
		return "name"
	case sortReposStars:
		return "stars"
	case sortReposPushed:
		return "pushed"
	case sortReposLanguage:
		return "language"
	case sortReposStarredAt:
		return "starred"
	default:
		return ""
	}
}
