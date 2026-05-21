package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	lipgloss "charm.land/lipgloss/v2"
)

func (m *model) ensureRepoWidths() {
	sig := ""
	if m.focusedList != nil {
		sig = m.focusedList.ID
	}
	sig += fmt.Sprintf("|%d|%s|%d", m.sortRepos, m.searchQuery, len(m.displayedRepos))
	if sig == m.cachedRepoSig {
		return // cache hit
	}
	m.cachedRepoSig = sig
	maxLang := 0
	for _, r := range m.displayedRepos {
		if n := len(r.Language); n > maxLang {
			maxLang = n
		}
	}
	if maxLang > 12 {
		maxLang = 12
	}
	m.cachedStarWidth = 4
	m.cachedLangWidth = maxLang
}

const starGlyph = "\u2605"

func (m model) renderRepoPane(w, h int) string {
	totalH := h
	out := make([]string, 0, totalH)

	// Search bar (active search in repo pane).
	if m.searchActive && m.active == paneRepo {
		qDisplay := m.searchQuery
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
		out = append(out, styleFaint.Render("(no list selected)"))
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	// Loading/error state: check focused list's cache entry.
	if m.focusedList != nil {
		entry := m.repoCache[repoCacheKey{m.focusedList.ID, m.showPreview}]
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
		label := "(no repos)"
		if m.searchQuery != "" {
			q := m.searchQuery
			if utf8.RuneCountInString(q) > 20 {
				q = string([]rune(q)[:20]) + "..."
			}
			label = "(no matches for \"" + q + "\")"
		}
		out = append(out, styleFaint.Render(label))
		for len(out) < totalH {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	// ---- Width-based feature flags ----
	showBadges := w >= 55
	showLang := w >= 42
	showStars := w >= 30

	hasSel := len(m.selected) > 0

	// ---- Column widths (cached across frames) ----
	(&m).ensureRepoWidths()

	// starWidth: digits + " " + glyph = cachedStarWidth+2; minimum 4.
	starWidth := 4
	if sw := m.cachedStarWidth + 2; sw > starWidth {
		starWidth = sw
	}
	// langWidth: minimum 4; capped at 12 by ensureRepoWidths.
	langWidth := 4
	if lw := m.cachedLangWidth; lw > langWidth {
		langWidth = lw
	}

	start := m.repoOffset
	end := min(start+h, len(m.displayedRepos))

	const (
		cursorW = 2 // "> " or "  "
		markerW = 4 // "[x] " or "[ ] " - only when hasSel
	)

	for i := start; i < end; i++ {
		r := m.displayedRepos[i]
		isCursor := i == m.repoCursor
		_, checked := m.selected[r.NameWithOwner]

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
				markerStr = styleFaint.Render("[ ]") + " "
			}
		}

		// -- Stars field --
		var starsStr string
		if showStars {
			countRaw := formatStars(r.StargazerCount)
			// Right-align count within (starWidth - 2) then append " " + glyph.
			countFieldW := starWidth - 2 // space + glyph
			if countFieldW < 1 {
				countFieldW = 1
			}
			paddedCount := padLeft(countRaw, countFieldW)
			starsStr = styleRepoStars.Render(paddedCount+" "+starGlyph) + "  "
		}

		// -- Language field --
		var langStr string
		if showLang {
			lang := r.Language
			if lang == "" {
				langStr = styleFaint.Render(padRight("-", langWidth)) + "  "
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
				langStr = styleRepoLanguage.Render(padRight(lang, langWidth)) + "  "
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
		if showLang {
			fixedW += langWidth + 2 // field + two spaces separator
		}
		nameAvail := w - fixedW
		if nameAvail < 12 {
			nameAvail = 12
		}

		// -- Badges --
		var badgesRaw string
		if showBadges {
			if r.IsFork {
				badgesRaw += " fork"
			}
			if r.IsArchived {
				badgesRaw += " archived"
			}
		}

		// -- Name: truncate raw, then style --
		nameRaw := r.NameWithOwner
		// Reserve space for badges if they exist.
		badgesW := lipgloss.Width(badgesRaw)
		nameMaxW := nameAvail
		if badgesW > 0 && nameAvail > badgesW+6 {
			nameMaxW = nameAvail - badgesW
		} else {
			// Not enough room for badges.
			badgesRaw = ""
		}

		if lipgloss.Width(nameRaw) > nameMaxW {
			const ellipsis = "..."
			runes := []rune(nameRaw)
			for len(runes) > 0 && lipgloss.Width(string(runes))+lipgloss.Width(ellipsis) > nameMaxW {
				runes = runes[:len(runes)-1]
			}
			nameRaw = string(runes) + ellipsis
		}

		var nameStr string
		if isCursor {
			if m.active == paneRepo {
				nameStr = styleRepoNameFocused.Render(nameRaw)
			} else {
				nameStr = styleRepoNameInactive.Render(nameRaw)
			}
		} else {
			nameStr = styleRepoName.Render(nameRaw)
		}

		var badgesStr string
		if badgesRaw != "" {
			badgesStr = styleRepoBadge.Render(badgesRaw)
		}

		// -- Assemble row --
		row := cursorStr + markerStr + starsStr + langStr + nameStr + badgesStr

		out = append(out, row)
	}

	for len(out) < totalH {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// truncateToWidth truncates a raw (unstyled) string so its visual width is at
