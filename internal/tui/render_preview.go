package tui

import (
	"fmt"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
)

func truncateToWidth(s string, maxW int) string {
	if lipgloss.Width(s) <= maxW {
		return s
	}
	const ellipsis = "..."
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+lipgloss.Width(ellipsis) > maxW {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

// previewContentLines builds the full list of styled content lines for the focused
// repo in the preview pane. The returned slice is not clipped to any height; the
// caller applies the scroll offset and pads to the desired height.
func (m model) previewContentLines(w int) []string {
	repo := m.displayedRepos[m.repoCursor]
	// Use the withTopics=true cache entry for topic data when available.
	if m.focusedList != nil {
		if e := m.repoCache[repoCacheKey{m.focusedList.ID, true}]; e != nil &&
			e.state == repoCacheLoaded && m.repoCursor < len(e.repos) {
			repo = e.repos[m.repoCursor]
		}
	}

	maxW := w - 2
	if maxW < 1 {
		maxW = 1
	}

	now := time.Now().UTC()
	var lines []string

	// Line 1: NameWithOwner
	lines = append(lines, stylePaneTitle.Render(truncateToWidth(repo.NameWithOwner, maxW)))

	// Line 2: URL
	lines = append(lines, styleRepoURL.Render(truncateToWidth(repo.URL, maxW)))

	// Blank line
	lines = append(lines, "")

	// Line 4: stars  language  badge
	starsStr := styleRepoStars.Render(
		fmt.Sprintf("%s %s", formatStars(repo.StargazerCount), starGlyph),
	)

	langStr := ""
	if repo.Language != "" {
		langStr = "  " + styleRepoLanguage.Render(repo.Language)
	} else {
		langStr = "  " + styleEmptyState.Render("-")
	}

	var badge string
	switch {
	case repo.IsArchived:
		badge = "  " + styleRepoBadge.Render("archived")
	case repo.IsFork:
		badge = "  " + styleRepoBadge.Render("fork")
	default:
		badge = "  " + styleRepoBadge.Render("source")
	}

	lines = append(lines, starsStr+langStr+badge)

	// Blank line
	lines = append(lines, "")

	// Description
	lines = append(lines, stylePaneSubtitle.Render("Description"))
	if repo.Description != "" {
		lines = append(lines, styleRepoName.Render(truncateToWidth(repo.Description, maxW)))
	} else {
		lines = append(lines, styleEmptyState.Render("(no description)"))
	}

	// Blank line
	lines = append(lines, "")

	// License
	licenseVal := repo.License
	if licenseVal == "" {
		licenseVal = styleEmptyState.Render("-")
	}
	lines = append(lines, stylePaneSubtitle.Render("License:")+" "+licenseVal)

	// Pushed
	lines = append(lines, stylePaneSubtitle.Render("Pushed:")+" "+shortAge(repo.PushedAt, now))

	// Starred
	starredVal := repo.StarredAt
	if starredVal == "" {
		starredVal = styleEmptyState.Render("-")
	} else {
		starredVal = shortAge(repo.StarredAt, now)
	}
	lines = append(lines, stylePaneSubtitle.Render("Starred:")+" "+starredVal)

	// Topics
	topicsVal := ""
	if len(repo.Topics) > 0 {
		topicsVal = truncateToWidth(strings.Join(repo.Topics, ", "), maxW)
	} else {
		topicsVal = styleEmptyState.Render("-")
	}
	lines = append(lines, stylePaneSubtitle.Render("Topics:")+" "+topicsVal)

	return lines
}

func (m model) renderPreviewPane(w, h int) string {
	if m.active != paneRepo || len(m.displayedRepos) == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).Render("(select a repo)")
	}

	lines := m.previewContentLines(w)

	// Apply scroll offset: slice [previewOffset, previewOffset+h].
	contentLen := len(lines)
	start := m.previewOffset
	if start > contentLen {
		start = contentLen
	}
	end := start + h
	if end > contentLen {
		end = contentLen
	}
	visible := lines[start:end]
	// Pad to height.
	for len(visible) < h {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}

func formatStars(n int) string {
	switch {
	case n >= 1000000:
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	case n >= 1000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func shortAge(value string, now time.Time) string {
	if value == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if parsed.After(now) {
		return parsed.Format("2006-01-02")
	}
	duration := now.Sub(parsed)
	switch {
	case duration < time.Minute:
		return "now"
	case duration < time.Hour:
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	case duration < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	case duration < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(duration.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(duration.Hours()/(24*365)))
	}
}
