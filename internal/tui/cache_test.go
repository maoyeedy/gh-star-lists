package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
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
	_ = m.focusList(4)

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

func TestPreviewKeyOpensListDetailModal(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})
	m.active = paneList

	m2 := update(m, keyPress('p'))
	if m2.modal == nil {
		t.Fatal("modal should be non-nil after 'p' in list pane")
	}
	if m2.modal.kind != modalListDetail {
		t.Errorf("modal kind = %d, want modalListDetail", m2.modal.kind)
	}
	if m2.modal.list.ID == "" {
		t.Error("modal list should be populated from displayed lists")
	}
}

func TestListDetailPrivacyBadge(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	publicList := svc.lists[0]
	publicList.IsPrivate = false
	privateList := svc.lists[1]
	privateList.IsPrivate = true

	publicView := newListDetailModal(publicList).viewListDetail()
	privateView := newListDetailModal(privateList).viewListDetail()

	if !strings.Contains(publicView, "public") {
		t.Fatalf("public list detail should include public badge; view = %q", publicView)
	}
	if !strings.Contains(privateView, "private") {
		t.Fatalf("private list detail should include private badge; view = %q", privateView)
	}
	if strings.Contains(publicView, "private") {
		t.Fatalf("public list detail should not include private badge; view = %q", publicView)
	}
	if strings.Contains(privateView, "public") {
		t.Fatalf("private list detail should not include public badge; view = %q", privateView)
	}
}

func TestUnlistedVirtualEntry(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	if len(m.displayedLists) == 0 || m.displayedLists[0].Name != "Unlisted" {
		t.Fatalf("first displayed list = %+v, want Unlisted virtual entry", m.displayedLists)
	}
	counts := map[string]int{}
	for _, list := range m.displayedLists {
		counts[list.Name]++
	}
	if counts["Unlisted"] != 1 {
		t.Fatalf("Unlisted count = %d, want 1", counts["Unlisted"])
	}
	if counts["All Starred"] != 0 {
		t.Fatalf("All Starred should not be present; displayedLists = %+v", m.displayedLists)
	}

	m.active = paneList
	m.listCursor = 0
	for _, key := range []rune{'e', 'd', 'c', 'C'} {
		next := update(m, keyPress(key))
		if next.modal != nil {
			t.Fatalf("key %q opened modal for virtual list", key)
		}
		if !strings.Contains(next.statusMsg, "virtual list") {
			t.Fatalf("key %q statusMsg = %q, want virtual-list explanation", key, next.statusMsg)
		}
	}
}

func TestUnlistedVirtualEntryLoadsThroughRepoCache(t *testing.T) {
	t.Parallel()
	svc := &fakeService{
		lists: []domain.StarList{{ID: "UL_1", Name: "Go"}},
		repos: []domain.Repository{
			{ID: "R_listed", NameWithOwner: "owner/listed"},
		},
		starred: []domain.Repository{
			{ID: "R_listed", NameWithOwner: "owner/listed"},
			{ID: "R_unlisted", NameWithOwner: "owner/unlisted"},
		},
	}
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	cmd := m.focusList(0)
	if cmd == nil {
		t.Fatal("focusList(Unlisted) should schedule a repo-cache load")
	}
	entry := m.preloader.cache[virtualUnlistedListID]
	if entry == nil || entry.state != repoCacheLoading {
		t.Fatalf("virtual cache entry = %+v, want loading", entry)
	}

	msg, ok := loadReposCmd(
		context.Background(),
		svc,
		virtualUnlistedListID,
		m.preloader.generation,
	)().(reposLoadedMsg)
	if !ok {
		t.Fatal("loadReposCmd should return reposLoadedMsg")
	}
	m = update(m, msg)
	entry = m.preloader.cache[virtualUnlistedListID]
	if entry == nil || entry.state != repoCacheLoaded {
		t.Fatalf("virtual cache entry after load = %+v, want loaded", entry)
	}
	if len(entry.repos) != 1 || entry.repos[0].NameWithOwner != "owner/unlisted" {
		t.Fatalf("virtual repos = %+v, want only owner/unlisted", entry.repos)
	}
}
