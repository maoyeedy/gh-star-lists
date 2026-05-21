package tui

import (
	"testing"
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
