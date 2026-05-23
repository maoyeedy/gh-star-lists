package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
	"github.com/maoyeedy/gh-star-lists/internal/humanize"
)

func (mo *modal) view() string {
	title := styleModalTitle.Render(mo.title)
	var body string
	var hint string

	if mo.bulkFailure != nil && !mo.submitting {
		body = mo.viewBulkFailure()
		hint = stylePaneSubtitle.Render("enter/r: retry failed  esc: close")
		if len(mo.bulkFailure.failedNWOs) > bulkFailureMaxVisible {
			hint = stylePaneSubtitle.Render("j/k: scroll  enter/r: retry failed  esc: close")
		}
	} else {
		switch mo.kind {
		case modalCreateList, modalEditList:
			body = mo.viewForm()
			hint = stylePaneSubtitle.Render("tab: next field  esc: cancel")
		case modalDeleteList:
			body = mo.viewConfirmText("Type the list name to confirm deletion:")
			hint = stylePaneSubtitle.Render("enter: confirm  esc: cancel")
		case modalConfirmText:
			body = mo.viewConfirmText("Type the repo name to confirm:")
			hint = stylePaneSubtitle.Render("enter: confirm  esc: cancel")
		case modalPickList:
			body = mo.viewPickList()
			hint = stylePaneSubtitle.Render("j/k: move  enter: select  esc: cancel")
		case modalConfirmYesNo:
			body = mo.viewConfirmYesNo()
			hint = stylePaneSubtitle.Render("y: confirm  n/esc: cancel")
		case modalHelp:
			body = mo.viewHelp()
			hint = stylePaneSubtitle.Render("j/k: scroll  esc: close")
		case modalRepoDetail:
			body = mo.viewRepoDetail()
			hint = stylePaneSubtitle.Render("enter/o: open in browser  esc/q/p: close")
		case modalListDetail:
			body = mo.viewListDetail()
			hint = stylePaneSubtitle.Render("esc/q/p: close")
		default:
			body = mo.body
			if body == "" {
				body = "not wired yet"
			}
			hint = stylePaneSubtitle.Render("esc: cancel")
		}
	}

	result := title + "\n\n" + body
	if mo.formErr != "" {
		result += "\n" + styleError.Render(mo.formErr)
	}
	// Show previous submission error (only when not currently submitting).
	if mo.submitErr != "" && !mo.submitting {
		result += "\n" + styleError.Render(mo.submitErr)
	}
	if mo.submitting {
		result += "\n\n" + stylePaneSubtitle.Render("  ... submitting...")
	} else {
		result += "\n\n" + hint
	}
	return result
}

func (mo *modal) viewForm() string {
	labels := []string{"Name:", "Description:"}
	var lines []string
	for i, inp := range mo.inputs {
		prefix := "  "
		if i == mo.focusedIdx {
			prefix = "> "
		}
		lines = append(lines, stylePaneSubtitle.Render(labels[i]))
		lines = append(lines, prefix+inp.View())
	}
	// visibility toggle
	visPrefix := "  "
	if mo.focusedIdx == len(mo.inputs) {
		visPrefix = "> "
	}
	visLabel := "[unset]"
	switch mo.privateState {
	case 1:
		visLabel = "Public"
	case 2:
		visLabel = "Private"
	}
	lines = append(lines, stylePaneSubtitle.Render("Visibility:"))
	lines = append(lines, visPrefix+visLabel+" (space/enter to cycle)")
	return strings.Join(lines, "\n")
}

func (mo *modal) viewConfirmText(prompt string) string {
	if mo.body == "" {
		return stylePaneSubtitle.Render(prompt) + "\n> " + mo.confirmInput.View()
	}
	return mo.body + "\n\n" + stylePaneSubtitle.Render(prompt) + "\n> " + mo.confirmInput.View()
}

func (mo *modal) viewPickList() string {
	mo.ensurePickFilter()
	choices := mo.filteredChoices()
	mo.clampChoiceCursor(len(choices))

	lines := []string{stylePaneSubtitle.Render("Filter:") + " " + mo.confirmInput.View()}
	if consequence := mo.pickListConsequence(choices); consequence != "" {
		lines = append(lines, consequence)
	}
	lines = append(lines, "")

	if len(mo.choices) == 0 {
		return strings.Join(append(lines, "(no lists available)"), "\n")
	}
	if len(choices) == 0 {
		return strings.Join(append(lines, "(no matching lists)"), "\n")
	}
	const maxVisible = 8
	const prefixW = 2           // "> " or "  "
	nameW := mo.width - prefixW // 0 when unconstrained; truncation skipped below
	start := 0
	if mo.choiceCursor >= maxVisible {
		start = mo.choiceCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(choices) {
		end = len(choices)
	}
	for i := start; i < end; i++ {
		prefix := "  "
		name := choices[i].Name
		if nameW > 0 {
			name = truncateToWidth(name, nameW)
		}
		if i == mo.choiceCursor {
			prefix = "> "
			lines = append(lines, prefix+styleCursorActive.Render(name))
		} else {
			lines = append(lines, prefix+name)
		}
	}
	return strings.Join(lines, "\n")
}

func (mo *modal) pickListConsequence(choices []domain.StarList) string {
	if len(choices) == 0 || mo.confirmExpected == "" {
		return ""
	}
	target := choices[mo.choiceCursor]
	switch mo.body {
	case "copy":
		return fmt.Sprintf(
			"Copy into %q - %q stays, %s copied.",
			target.Name,
			mo.confirmExpected,
			repoCountLabel(mo.privateState),
		)
	case "merge":
		return fmt.Sprintf(
			"Merge into %q - %q will be deleted, %s moved.",
			target.Name,
			mo.confirmExpected,
			repoCountLabel(mo.privateState),
		)
	default:
		return ""
	}
}

func (mo *modal) viewHelp() string {
	helpW := mo.width
	if helpW <= 0 {
		helpW = 60
	}
	lines := renderHelpLines(helpW)
	if mo.scrollOffset >= len(lines) {
		mo.scrollOffset = max(0, len(lines)-1)
	}
	const maxVisible = 20
	start := mo.scrollOffset
	end := start + maxVisible
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func (mo *modal) viewConfirmYesNo() string {
	return stylePaneSubtitle.Render("Remove repo from current list?") + "\n\n" +
		"[y] Yes  [n/esc] No"
}

func (mo *modal) viewRepoDetail() string {
	r := mo.repo
	now := time.Now().UTC()
	var lines []string

	nwo := r.NameWithOwner
	url := r.URL
	desc := r.Description
	if mo.width > 0 {
		nwo = truncateToWidth(nwo, mo.width)
		url = truncateToWidth(url, mo.width)
		if desc != "" {
			desc = truncateToWidth(desc, mo.width)
		}
	}

	lines = append(lines, stylePaneTitle.Render(nwo))
	lines = append(lines, styleRepoURL.Render(url))
	lines = append(lines, "")

	starsStr := styleRepoStars.Render(
		fmt.Sprintf("%s %s", formatStars(r.StargazerCount), starGlyph),
	)
	langStr := ""
	if r.Language != "" {
		langStr = "  " + styleRepoLanguage.Render(r.Language)
	} else {
		langStr = "  " + styleEmptyState.Render("-")
	}
	var badge string
	switch {
	case r.IsArchived:
		badge = "  " + styleRepoBadge.Render("archived")
	case r.IsFork:
		badge = "  " + styleRepoBadge.Render("fork")
	}
	lines = append(lines, starsStr+langStr+badge)
	lines = append(lines, "")

	lines = append(lines, stylePaneSubtitle.Render("Description"))
	if desc != "" {
		lines = append(lines, styleRepoName.Render(desc))
	} else {
		lines = append(lines, styleEmptyState.Render("(no description)"))
	}
	lines = append(lines, "")

	licenseVal := r.License
	if licenseVal == "" {
		licenseVal = styleEmptyState.Render("-")
	}
	lines = append(lines, stylePaneSubtitle.Render("License:")+" "+licenseVal)

	lines = append(
		lines,
		stylePaneSubtitle.Render("Pushed:")+" "+humanize.ShortAge(r.PushedAt, now),
	)

	starredVal := r.StarredAt
	if starredVal == "" {
		starredVal = styleEmptyState.Render("-")
	} else {
		starredVal = humanize.ShortAge(r.StarredAt, now)
	}
	lines = append(lines, stylePaneSubtitle.Render("Starred:")+" "+starredVal)

	return strings.Join(lines, "\n")
}

func (mo *modal) viewListDetail() string {
	l := mo.list
	now := time.Now().UTC()
	var lines []string

	name := l.Name
	url := l.URL
	desc := l.Description
	if mo.width > 0 {
		name = truncateToWidth(name, mo.width)
		url = truncateToWidth(url, mo.width)
		if desc != "" {
			desc = truncateToWidth(desc, mo.width)
		}
	}

	badge := "public"
	if l.IsPrivate {
		badge = "private"
	}
	lines = append(lines, stylePaneTitle.Render(name)+"  "+styleRepoBadge.Render(badge))
	lines = append(lines, styleRepoURL.Render(url))
	lines = append(lines, "")

	lines = append(lines, stylePaneSubtitle.Render("Repos:")+" "+fmt.Sprintf("%d", l.RepoCount))
	lines = append(lines, "")

	lines = append(lines, stylePaneSubtitle.Render("Description"))
	if desc != "" {
		lines = append(lines, styleRepoName.Render(desc))
	} else {
		lines = append(lines, styleEmptyState.Render("(no description)"))
	}
	lines = append(lines, "")

	lastAddedVal := l.LastAddedAt
	if lastAddedVal == "" {
		lastAddedVal = styleEmptyState.Render("-")
	} else {
		lastAddedVal = humanize.ShortAge(l.LastAddedAt, now)
	}
	lines = append(lines, stylePaneSubtitle.Render("Last added:")+" "+lastAddedVal)

	return strings.Join(lines, "\n")
}
