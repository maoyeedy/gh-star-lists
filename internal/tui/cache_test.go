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
	if m.preloader.inFlight > 3 {
		t.Errorf("preloadInFlight = %d after listsLoadedMsg, want <= 3", m.preloader.inFlight)
	}
	if m.preloader.inFlight != 3 {
		t.Errorf("preloadInFlight = %d, want exactly 3 (cap not filled)", m.preloader.inFlight)
	}
	if len(m.preloader.queue) != 2 {
		t.Errorf("preloadQueue len = %d, want 2 (remaining lists)", len(m.preloader.queue))
	}

	// Deliver one success -- a 4th load should now be scheduled.
	inflight := m.preloader.inFlight
	next2, _ := m.Update(
		reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID, gen: m.preloader.generation},
	)
	m2 := next2.(model)

	// preloadInFlight should have decreased by 1 then increased by 1 (net same or +0).
	// It decreased when the response arrived, and then schedulePreload added the next one.
	// Since there are 2 in the queue, one is scheduled: inflight stays the same.
	if m2.preloader.inFlight != inflight {
		t.Errorf(
			"preloadInFlight = %d after one reposLoadedMsg, want %d (one freed, one scheduled)",
			m2.preloader.inFlight, inflight,
		)
	}
	// Queue should have shrunk by 1 (one was promoted to in-flight).
	if len(m2.preloader.queue) != 1 {
		t.Errorf("preloadQueue len = %d after one response, want 1", len(m2.preloader.queue))
	}
}

func TestFocusedLoadPriority(t *testing.T) {
	t.Parallel()

	// Test enqueueFront directly (core focus promotion logic).
	p := newPreloader()
	p.queue = []string{"UL_4", "UL_5"}
	p.enqueueFront("UL_3")
	want := []string{"UL_3", "UL_4", "UL_5"}
	for i, id := range want {
		if p.queue[i] != id {
			t.Errorf("after enqueueFront(UL_3): queue[%d] = %s, want %s", i, p.queue[i], id)
		}
	}

	// Dedup: enqueueFront removes existing occurrence.
	p.enqueueFront("UL_4")
	want2 := []string{"UL_4", "UL_3", "UL_5"}
	for i, id := range want2 {
		if p.queue[i] != id {
			t.Errorf("after enqueueFront(UL_4) dedup: queue[%d] = %s, want %s", i, p.queue[i], id)
		}
	}

	// Through focusList: verify an idle list gets promoted to front of queue.
	svc := fiveListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// Reset: 3 preload slots full, no cancels, empty cache.
	// This isolates enqueueFront from schedulePreload consumption.
	m.preloader.cache = make(map[string]*repoCacheEntry)
	m.preloader.queue = []string{"UL_5", "UL_3"}
	m.preloader.inFlight = 3
	m.preloader.preloadCancels = nil

	// Focus UL_4. Since cache is empty focusList enters the idle default case,
	// enqueues UL_4 at front, and schedulePreload is a no-op (inFlight capped).
	_ = m.focusList(3)

	if len(m.preloader.queue) == 0 {
		t.Fatal("queue should not be empty after focusList with inFlight capped")
	}
	if m.preloader.queue[0] != "UL_4" {
		t.Errorf("queue[0] = %s, want UL_4; full queue = %v",
			m.preloader.queue[0], m.preloader.queue)
	}
}

func TestPreviewKeyOpensRepoDetailModal(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m = update(m, reposLoadedMsg{repos: svc.repos, listID: svc.lists[0].ID})
	m.active = paneRepo
	m.focusedList = &m.lists[0]

	m2 := update(m, keyPress('p'))

	if m2.modal == nil {
		t.Fatal("modal should be non-nil after 'p' in repo pane")
	}
	if m2.modal.kind != modalRepoDetail {
		t.Errorf("modal kind = %d, want modalRepoDetail", m2.modal.kind)
	}
}

func TestPreviewKeyNoopInListPane(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)
	m.active = paneList

	m2 := update(m, keyPress('p'))
	if m2.modal != nil {
		t.Error("modal should be nil after 'p' in list pane")
	}
}
