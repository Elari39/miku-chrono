package services

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"mikuchrono/internal/store"
)

// StopAndDelete must broadcast timer:stopped only when a running timer was
// actually closed by the delete.

func TestActivityServiceStopAndDeleteBroadcastsOnStop(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	events := []string{}
	svc := &ActivityService{Store: st, Emit: func(e string) { events = append(events, e) }}
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := st.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	entry, err := svc.StopAndDelete(1)
	if err != nil || entry == nil {
		t.Fatalf("stop and delete: entry=%+v err=%v", entry, err)
	}
	if len(events) != 1 || events[0] != EventTimerStopped {
		t.Fatalf("stopping a running timer must broadcast once, got %v", events)
	}

	// Deleting an idle activity must not broadcast.
	if _, err := svc.StopAndDelete(2); err != nil {
		t.Fatalf("delete idle activity: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("no broadcast expected without a running timer, got %v", events)
	}

	// Unknown ids surface the store error.
	if _, err := svc.StopAndDelete(999); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown id must return ErrNotFound, got %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("a failed delete must not broadcast, got %v", events)
	}
}
