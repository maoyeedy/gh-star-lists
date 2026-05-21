package tui

import (
	"testing"
)

func TestPreloaderRespectsConcurrencyCap(t *testing.T) {
	t.Parallel()
	svc := fiveListsSvc()
	m := newTestModel(svc)

	next, _ := m.Update(listsLoadedMsg{lists: svc.lists})
	m = next.(model)

	// At most 3 loads should be in flight.
	if m.preloadInFlight > 3 {
		t.Errorf("preloadInFlight = %d after listsLoadedMsg, want <= 3", m.preloadInFlight)
	}
	if m.preloadInFlight != 3 {
		t.Errorf("preloadInFlight = %d, want exactly 3 (cap not filled)", m.preloadInFlight)
	}
	if len(m.preloadQueue) != 2 {
		t.Errorf("preloadQueue len = %d, want 2 (remaining lists)", len(m.preloadQueue))
	}

	// Deliver one success -- a 4th load should now be scheduled.
	inflight := m.preloadInFlight
	next2, _ := m.Update(
		reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID, gen: m.generation},
	)
	m2 := next2.(model)

	// preloadInFlight should have decreased by 1 then increased by 1 (net same or +0).
	// It decreased when the response arrived, and then schedulePreload added the next one.
	// Since there are 2 in the queue, one is scheduled: inflight stays the same.
	if m2.preloadInFlight != inflight {
		t.Errorf(
			"preloadInFlight = %d after one reposLoadedMsg, want %d (one freed, one scheduled)",
			m2.preloadInFlight, inflight,
		)
	}
	// Queue should have shrunk by 1 (one was promoted to in-flight).
	if len(m2.preloadQueue) != 1 {
		t.Errorf("preloadQueue len = %d after one response, want 1", len(m2.preloadQueue))
	}
}

// TestCursorMoveUsesCacheNoNewCmd verifies that moving the cursor down to a
