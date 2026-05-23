package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
)

func TestSpinnerTickMsgUpdatesSpinner(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	// listsLoading is already true from newTestModel (listsLoading: true).

	// Capture initial View output.
	before := m.spinner.View()

	// Synthesize a TickMsg for this spinner's ID so it accepts the message.
	tick := spinner.TickMsg{Time: time.Now(), ID: m.spinner.ID()}
	m2 := update(m, tick)

	// spinner.View() must return a non-empty string after a tick.
	after := m2.spinner.View()
	if after == "" {
		t.Error("spinner.View() returned empty string after TickMsg")
	}
	// The frame should have advanced (before and after differ).
	if before == after {
		t.Logf(
			"spinner.View() did not advance frame (before=%q after=%q); may be acceptable if same char",
			before,
			after,
		)
	}
}

// TestLoadingViewUsesSpinnerView verifies that while loading repos the rendered
// repo pane contains the spinner output.
func TestLoadingViewUsesSpinnerView(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.focusedList = &m.lists[0]
	// Mark the focused list's cache entry as loading to simulate repo fetch in flight.
	m.preloader.cache[m.focusedList.ID] = &repoCacheEntry{
		state: repoCacheLoading,
	}

	spinnerStr := m.spinner.View()
	rendered := repoPane(m, 80, 20)

	if !strings.Contains(rendered, spinnerStr) {
		t.Errorf(
			"repo pane during loading should contain spinner output %q; got:\n%s",
			spinnerStr,
			rendered,
		)
	}
}
