package tui

import (
	"fmt"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/humanize"
)

func truncateToWidth(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	const ellipsis = "..."
	ellipsisW := lipgloss.Width(ellipsis)
	if maxW < ellipsisW {
		return strings.Repeat(".", maxW)
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+ellipsisW > maxW {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

func wrapToWidth(s string, maxW int) []string {
	if maxW <= 0 {
		return []string{""}
	}
	paragraphs := strings.Split(s, "\n")
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := ""
		for _, word := range words {
			if line == "" {
				if lipgloss.Width(word) <= maxW {
					line = word
					continue
				}
				chunks := splitLongWord(word, maxW)
				lines = append(lines, chunks[:len(chunks)-1]...)
				line = chunks[len(chunks)-1]
				continue
			}
			candidate := line + " " + word
			if lipgloss.Width(candidate) <= maxW {
				line = candidate
				continue
			}
			lines = append(lines, line)
			if lipgloss.Width(word) <= maxW {
				line = word
				continue
			}
			chunks := splitLongWord(word, maxW)
			lines = append(lines, chunks[:len(chunks)-1]...)
			line = chunks[len(chunks)-1]
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitLongWord(s string, maxW int) []string {
	if maxW <= 0 {
		return []string{""}
	}
	var chunks []string
	var chunk []rune
	chunkW := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if len(chunk) > 0 && chunkW+rw > maxW {
			chunks = append(chunks, string(chunk))
			chunk = nil
			chunkW = 0
		}
		chunk = append(chunk, r)
		chunkW += rw
	}
	if len(chunk) > 0 {
		chunks = append(chunks, string(chunk))
	}
	if len(chunks) == 0 {
		return []string{""}
	}
	return chunks
}

func appendWrapped(lines []string, text string, maxW int, style lipgloss.Style) []string {
	for _, line := range wrapToWidth(text, maxW) {
		lines = append(lines, style.Render(line))
	}
	return lines
}

// formatPreviewContent builds styled preview lines for a single repository.
// The returned slice is not clipped to any height; the caller applies the
// scroll offset and pads to the desired height.
func formatPreviewContent(repo domain.Repository, w int) []string {
	maxW := w - 2
	if maxW < 1 {
		maxW = 1
	}

	now := time.Now().UTC()
	var lines []string

	lines = appendWrapped(lines, repo.NameWithOwner, maxW, stylePaneTitle)

	lines = appendWrapped(lines, repo.URL, maxW, styleRepoURL)

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
	}

	lines = append(lines, starsStr+langStr+badge)

	// Blank line
	lines = append(lines, "")

	// Description
	lines = append(lines, stylePaneSubtitle.Render("Description"))
	if repo.Description != "" {
		lines = appendWrapped(lines, repo.Description, maxW, styleRepoName)
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
	lines = append(
		lines,
		stylePaneSubtitle.Render("Pushed:")+" "+humanize.ShortAge(repo.PushedAt, now),
	)

	// Starred
	starredVal := repo.StarredAt
	if starredVal == "" {
		starredVal = styleEmptyState.Render("-")
	} else {
		starredVal = humanize.ShortAge(repo.StarredAt, now)
	}
	lines = append(lines, stylePaneSubtitle.Render("Starred:")+" "+starredVal)

	// Topics
	topicsVal := ""
	if len(repo.Topics) > 0 {
		topicsVal = strings.Join(repo.Topics, ", ")
	} else {
		topicsVal = styleEmptyState.Render("-")
	}
	lines = append(lines, stylePaneSubtitle.Render("Topics:"))
	if len(repo.Topics) > 0 {
		lines = appendWrapped(lines, topicsVal, maxW, styleRepoName)
	} else {
		lines = append(lines, topicsVal)
	}

	return lines
}

// previewContentLines builds the full list of styled content lines for the focused
// repo in the preview pane, preferring the withTopics cache entry when available.
func (m model) previewContentLines(w int) []string {
	repo := m.displayedRepos[m.repoCursor]
	if m.focusedList != nil {
		if e := m.preloader.cache[repoCacheKey{m.focusedList.ID, true}]; e != nil &&
			e.state == repoCacheLoaded {
			if detailed, ok := previewRepoByIdentity(repo, e.repos); ok {
				repo = detailed
			}
		}
	}
	return formatPreviewContent(repo, w)
}

func previewRepoByIdentity(
	selected domain.Repository,
	repos []domain.Repository,
) (domain.Repository, bool) {
	if selected.ID != "" {
		for _, repo := range repos {
			if repo.ID == selected.ID {
				return repo, true
			}
		}
	}
	for _, repo := range repos {
		if repo.NameWithOwner == selected.NameWithOwner {
			return repo, true
		}
	}
	return domain.Repository{}, false
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
	case n >= 1_000_000:
		m := n / 1_000_000
		if m >= 10 {
			return fmt.Sprintf("%dm", m)
		}
		d := (n % 1_000_000) / 100_000
		return fmt.Sprintf("%d.%dm", m, d)
	case n >= 1_000:
		k := n / 1_000
		if k >= 10 {
			return fmt.Sprintf("%dk", k)
		}
		d := (n % 1_000) / 100
		return fmt.Sprintf("%d.%dk", k, d)
	default:
		return fmt.Sprintf("%d", n)
	}
}
