package tui

import (
	"testing"
	"time"
)

func TestStatusToastSetAndExpire(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	m := newTestModel(svc)

	// Simulate a mutation completing.
	m2 := update(m, mutationDoneMsg{kind: modalCreateList})
	if m2.statusMsg == "" {
		t.Error("statusMsg should be set after mutationDoneMsg success")
	}
	if m2.statusExpiry.IsZero() {
		t.Error("statusExpiry should be set after mutationDoneMsg success")
	}

	// Simulate expiry.
	m3 := update(m2, statusExpiredMsg{})
	if m3.statusMsg != "" {
		t.Error("statusMsg should be cleared after statusExpiredMsg")
	}
}

// TestBulkDoneToastDurationScalesWithFailures verifies that the toast expiry
// duration is 2s for clean bulk ops and 4s when failedNWOs is non-empty.
func TestBulkDoneToastDurationScalesWithFailures(t *testing.T) {
	t.Parallel()
	svc := threeListsSvc()
	m := newTestModel(svc)
	m = update(m, listsLoadedMsg{lists: svc.lists})

	// No failures -> 2s toast.
	m2 := update(m, bulkDoneMsg{verb: "added", succeeded: 2, failed: 0})
	d := time.Until(m2.statusExpiry)
	if d < 1500*time.Millisecond || d > 2500*time.Millisecond {
		t.Errorf("no-failure expiry ~%v from now, want ~2s", d)
	}

	// Partial failure with named NWOs -> 4s toast.
	m3 := update(
		m2,
		bulkDoneMsg{verb: "removed", succeeded: 1, failed: 1, failedNWOs: []string{"owner/repo"}},
	)
	d2 := time.Until(m3.statusExpiry)
	if d2 < 3500*time.Millisecond || d2 > 4500*time.Millisecond {
		t.Errorf("failure expiry ~%v from now, want ~4s", d2)
	}
}
