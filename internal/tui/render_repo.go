package tui

import (
	"strings"
	"unicode/utf8"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

const (
	starGlyph = "\u2605"
	langWidth = 20
)

func (m model) renderRepoPane(w, h int) string {
	if h <= 0 {
		return ""
	}
	totalH := h
	out := make([]string, 0, totalH)

	out = append(out, paneTitle("Repos", len(m.displayedRepos), w))
	h--
	if h <= 0 {
		return strings.Join(out, "\n")
	}

	// Search bar (active search in repo pane).
	if m.repoSearchActive && m.active == paneRepo {
		qDisplay := m.repoSearchQuery
		prefix := styleSuccess.Render("/") + " "
		prefixW := 2 // "/" + space
		if lipgloss.Width(prefix)+lipgloss.Width(qDisplay) > w {
			tail := ""
			for _, r := range qDisplay {
				candidate := "..." + string([]rune(tail)) + string(r)
				if prefixW+lipgloss.Width(candidate) <= w {
					tail += string(r)
				}
			}
			qDisplay = "..." + tail
		}
		bar := prefix + qDisplay
		out = append(out, padRight(bar, w))
		h--
	}

	// No list focused yet.
	if m.focusedList == nil {
		out = append(out, stylePaneSubtitle.Render("(no list selected)"))
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	// Loading/error state: check focused list's cache entry.
	if m.focusedList != nil {
		entry := m.repoPaneCacheEntry()
		if entry != nil && entry.state == repoCacheError {
			errStr := entry.err.Error()
			const maxErrW = 60
			if len(errStr) > maxErrW {
				errStr = errStr[:maxErrW] + "..."
			}
			out = append(out, styleError.Render("error: "+errStr+"  (ctrl+r to retry)"))
			for len(out) < totalH {
				out = append(out, "")
			}
			return strings.Join(out, "\n")
		}
		if entry == nil || entry.state == repoCacheLoading {
			out = append(out, "  Loading "+m.spinner.View())
			for len(out) < totalH {
				out = append(out, "")
			}
			return strings.Join(out, "\n")
		}
	}

	// Empty state.
	if len(m.displayedRepos) == 0 {
		out = append(out, m.renderRepoColumnHeader(w, false))
		h--
		if h <= 0 {
			return strings.Join(out, "\n")
		}
		label := "(no repos)"
		if m.repoSearchQuery != "" {
			q := m.repoSearchQuery
			if utf8.RuneCountInString(q) > 20 {
				q = string([]rune(q)[:20]) + "..."
			}
			label = "(no matches for \"" + q + "\")"
		}
		out = append(out, stylePaneSubtitle.Render(label))
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	hasSel := len(m.selected) > 0

	const starWidth = 6 // " " + up to 4 star digits + " " + glyph

	start := m.repoOffset

	const (
		cursorW  = 2 // "> " or "  "
		markerW  = 4 // "[x] " or "[ ] " - only when hasSel
		minNameW = 4
	)

	baseW := cursorW
	if hasSel {
		baseW += markerW
	}
	showStars := w >= 30 && baseW+starWidth+2+minNameW <= w
	if showStars {
		baseW += starWidth + 2
	}
	showLang := w >= 34 && baseW+langWidth+2+minNameW <= w
	showBadges := w >= 55

	out = append(out, m.renderRepoColumnHeader(w, hasSel))
	h--
	if h <= 0 {
		return strings.Join(out, "\n")
	}
	if m.repoCursor < start {
		start = m.repoCursor
	} else if m.repoCursor >= start+h {
		start = m.repoCursor - h + 1
	}
	start = clampInt(start, 0, max(0, len(m.displayedRepos)-h))
	end := min(start+h, len(m.displayedRepos))

	// Convert visible repos to RepoRow for rendering with pre-computed fields.
	repoRows := make([]domain.RepoRow, end-start)
	for j := 0; j < end-start; j++ {
		repoRows[j] = repoToRow(m.displayedRepos[start+j])
	}

	for i, row := range repoRows {
		isCursor := start+i == m.repoCursor
		_, checked := m.selected[row.NameWithOwner]

		// -- Cursor prefix (2 chars) --
		var cursorStr string
		if isCursor {
			if m.active == paneRepo {
				cursorStr = styleCursorActive.Render("> ")
			} else {
				cursorStr = styleCursorInactive.Render("> ")
			}
		} else {
			cursorStr = "  "
		}

		// -- Selection marker (4 chars, only when hasSel) --
		var markerStr string
		if hasSel {
			if checked {
				markerStr = styleChecked.Render("[x]") + " "
			} else {
				markerStr = stylePaneSubtitle.Render("[ ]") + " "
			}
		}

		// -- Stars field --
		var starsStr string
		if showStars {
			countRaw := formatStars(row.StargazerCount)
			// Right-align count within (starWidth - 2) then append " " + glyph.
			countFieldW := starWidth - 2 // space + glyph
			if countFieldW < 1 {
				countFieldW = 1
			}
			paddedCount := padLeft(countRaw, countFieldW)
			starsStr = styleRepoStars.Render(paddedCount+" "+starGlyph) + "  "
		}

		// -- Language field (right-aligned at end) --
		var langStr string
		if showLang {
			lang := row.Language
			if lang == "" {
				langStr = "  " + stylePaneSubtitle.Render(padLeft("-", langWidth))
			} else {
				lw := lipgloss.Width(lang)
				if lw > langWidth {
					// Truncate with ellipsis.
					runes := []rune(lang)
					for len(runes) > 0 && lipgloss.Width(string(runes))+3 > langWidth {
						runes = runes[:len(runes)-1]
					}
					lang = string(runes) + "..."
				}
				langStr = "  " + styleRepoLanguage.Render(padLeft(lang, langWidth))
			}
		}

		// -- Compute remaining width for name (and optional badges + age) --
		fixedW := cursorW
		if hasSel {
			fixedW += markerW
		}
		if showStars {
			fixedW += starWidth + 2 // field + two spaces separator
		}
		nameAvail := w - fixedW
		if showLang {
			nameAvail -= langWidth + 2 // "  " + right-aligned field
		}
		if nameAvail < 1 {
			nameAvail = 1
		}

		// -- Badges --
		var badgesRaw string
		if showBadges {
			if row.IsFork {
				badgesRaw += " fork"
			}
			if row.IsArchived {
				badgesRaw += " archived"
			}
		}

		// -- Name: truncate raw, then style --
		nameRaw := row.NameWithOwner
		// Reserve space for badges if they exist.
		badgesW := lipgloss.Width(badgesRaw)
		nameMaxW := nameAvail
		if badgesW > 0 && nameAvail > badgesW+6 {
			nameMaxW = nameAvail - badgesW
		} else {
			// Not enough room for badges.
			badgesRaw = ""
		}

		nameRaw = truncateToWidth(nameRaw, nameMaxW)

		owner, repo, hasSep := strings.Cut(nameRaw, "/")
		var nameStr string
		if hasSep {
			var repoStyle lipgloss.Style
			if isCursor {
				if m.active == paneRepo {
					repoStyle = styleRepoNameFocused
				} else {
					repoStyle = styleRepoName
				}
			} else {
				repoStyle = styleRepoName
			}
			nameStr = styleRepoOwner.Render(
				owner,
			) + styleRepoOwner.Render(
				"/",
			) + repoStyle.Render(
				repo,
			)
		} else {
			if isCursor {
				if m.active == paneRepo {
					nameStr = styleRepoNameFocused.Render(nameRaw)
				} else {
					nameStr = styleRepoName.Render(nameRaw)
				}
			} else {
				nameStr = styleRepoName.Render(nameRaw)
			}
		}

		var badgesStr string
		if badgesRaw != "" {
			badgesStr = styleRepoBadge.Render(badgesRaw)
		}

		// -- Pad name to fill available width so lang sits at consistent column --
		nameFilled := padRight(nameStr+badgesStr, nameAvail)

		// -- Assemble row (lang at end, right-aligned) --
		row := cursorStr + markerStr + starsStr + nameFilled + langStr

		out = append(out, row)
	}

	for len(out) < totalH {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func (m model) renderRepoColumnHeader(w int, hasSel bool) string {
	const starWidth = 6
	const (
		cursorW  = 2
		markerW  = 4
		minNameW = 4
	)

	baseW := cursorW
	if hasSel {
		baseW += markerW
	}
	showStars := w >= 30 && baseW+starWidth+2+minNameW <= w
	if showStars {
		baseW += starWidth + 2
	}
	showLang := w >= 34 && baseW+langWidth+2+minNameW <= w

	fixedW := cursorW
	if hasSel {
		fixedW += markerW
	}
	if showStars {
		fixedW += starWidth + 2
	}
	nameAvail := w - fixedW
	if showLang {
		nameAvail -= langWidth + 2
	}
	if nameAvail < 1 {
		nameAvail = 1
	}

	prefix := strings.Repeat(" ", cursorW)
	if hasSel {
		prefix += strings.Repeat(" ", markerW)
	}

	starsStr := ""
	if showStars {
		starsStr = stylePaneSubtitle.Render(padLeft("stars", starWidth)) + "  "
	}

	nameStr := stylePaneSubtitle.Render(truncateToWidth("name", nameAvail))
	nameStr = padRight(nameStr, nameAvail)

	langStr := ""
	if showLang {
		langStr = "  " + stylePaneSubtitle.Render(padLeft("language", langWidth))
	}

	return padRight(prefix+starsStr+nameStr+langStr, w)
}

// repoToRow converts a domain.Repository to a domain.RepoRow for rendering.
func repoToRow(repo domain.Repository) domain.RepoRow {
	owner, name, _ := strings.Cut(repo.NameWithOwner, "/")
	return domain.RepoRow{
		Owner:          owner,
		Name:           name,
		NameWithOwner:  repo.NameWithOwner,
		StargazerCount: repo.StargazerCount,
		Language:       repo.Language,
		IsFork:         repo.IsFork,
		IsArchived:     repo.IsArchived,
		URL:            repo.URL,
		Description:    repo.Description,
		PushedAt:       repo.PushedAt,
	}
}

// truncateToWidth truncates a raw (unstyled) string so its visual width is at
