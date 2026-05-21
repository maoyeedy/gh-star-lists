package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type bulkFailureState struct {
	verb       string
	succeeded  int
	failedNWOs []string
	offset     int
}

func newBulkAddModal(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	allLists []githubapi.StarList,
) *modal {
	mo := &modal{
		kind:    modalPickList,
		title:   fmt.Sprintf("Add %d repos: pick list", len(nwos)),
		choices: allLists,
		ctx:     ctx,
		svc:     svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		if len(m.choices) == 0 {
			return nil
		}
		targetID := m.choices[m.choiceCursor].ID
		m.bulkRetry = func(failedNWOs []string) tea.Cmd {
			return bulkAddReposCmd(ctx, svc, failedNWOs, targetID)
		}
		return bulkAddReposCmd(ctx, svc, nwos, targetID)
	}
	return mo
}

func newBulkRemoveModal(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	fromListID string,
) *modal {
	mo := &modal{
		kind:  modalConfirmYesNo,
		title: fmt.Sprintf("Remove %d repos from list?", len(nwos)),
		ctx:   ctx,
		svc:   svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		return bulkRemoveReposCmd(ctx, svc, nwos, fromListID)
	}
	mo.bulkRetry = func(failedNWOs []string) tea.Cmd {
		return bulkRemoveReposCmd(ctx, svc, failedNWOs, fromListID)
	}
	return mo
}

func newBulkMoveModal(
	ctx context.Context,
	svc githubapi.Service,
	nwos []string,
	allLists []githubapi.StarList,
	fromListID string,
) *modal {
	filtered := make([]githubapi.StarList, 0, len(allLists))
	for _, l := range allLists {
		if l.ID != fromListID {
			filtered = append(filtered, l)
		}
	}
	mo := &modal{
		kind:    modalPickList,
		title:   fmt.Sprintf("Move %d repos: pick list", len(nwos)),
		choices: filtered,
		ctx:     ctx,
		svc:     svc,
	}
	mo.onConfirm = func(m *modal) tea.Cmd {
		if len(m.choices) == 0 {
			return nil
		}
		targetID := m.choices[m.choiceCursor].ID
		m.bulkRetry = func(failedNWOs []string) tea.Cmd {
			return bulkMoveReposCmd(ctx, svc, failedNWOs, fromListID, targetID)
		}
		return bulkMoveReposCmd(ctx, svc, nwos, fromListID, targetID)
	}
	return mo
}

func (mo *modal) updateBulkFailure(msg tea.KeyPressMsg) (*modal, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return nil, nil
	case "enter", "r", "R":
		if mo.bulkRetry == nil || len(mo.bulkFailure.failedNWOs) == 0 {
			return mo, nil
		}
		failedNWOs := append([]string(nil), mo.bulkFailure.failedNWOs...)
		cmd := mo.bulkRetry(failedNWOs)
		if cmd == nil {
			return mo, nil
		}
		mo.bulkFailure = nil
		return nil, cmd
	case "up", "k":
		if mo.bulkFailure.offset > 0 {
			mo.bulkFailure.offset--
		}
		return mo, nil
	case "down", "j":
		maxOffset := len(mo.bulkFailure.failedNWOs) - bulkFailureMaxVisible
		if maxOffset < 0 {
			maxOffset = 0
		}
		if mo.bulkFailure.offset < maxOffset {
			mo.bulkFailure.offset++
		}
		return mo, nil
	}
	return mo, nil
}

const bulkFailureMaxVisible = 8

func (mo *modal) viewBulkFailure() string {
	if mo.bulkFailure == nil {
		return ""
	}
	failure := mo.bulkFailure
	if failure.offset < 0 {
		failure.offset = 0
	}
	maxOffset := len(failure.failedNWOs) - bulkFailureMaxVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if failure.offset > maxOffset {
		failure.offset = maxOffset
	}

	header := fmt.Sprintf(
		"%d %s, %d failed.",
		failure.succeeded,
		failure.verb,
		len(failure.failedNWOs),
	)
	if failure.succeeded == 0 {
		header = fmt.Sprintf("%d failed.", len(failure.failedNWOs))
	}
	lines := []string{styleError.Render(header), stylePaneSubtitle.Render("Failed repositories:")}
	start := failure.offset
	end := start + bulkFailureMaxVisible
	if end > len(failure.failedNWOs) {
		end = len(failure.failedNWOs)
	}
	for _, nwo := range failure.failedNWOs[start:end] {
		lines = append(lines, "  "+nwo)
	}
	if start > 0 {
		lines = append(lines, stylePaneSubtitle.Render(fmt.Sprintf("  ... %d above", start)))
	}
	if remaining := len(failure.failedNWOs) - end; remaining > 0 {
		lines = append(lines, stylePaneSubtitle.Render(fmt.Sprintf("  ... %d more", remaining)))
	}
	return strings.Join(lines, "\n")
}
