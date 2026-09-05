package services

import (
	"path/filepath"
	"testing"

	"mikuchrono/internal/store"
)

func TestTimerServiceStartLast(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := &TimerService{Store: st}

	// No chain yet: StartLast must explain itself.
	if _, err := s.StartLast(); err == nil {
		t.Fatal("StartLast without a chain must fail")
	}

	// Build a chain: start 学习 then stop (accumulation itself is covered by
	// the store tests with injected timestamps).
	if _, err := s.Start(1); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := s.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Resuming picks the chained activity and carries its elapsed forward.
	got, err := s.StartLast()
	if err != nil {
		t.Fatalf("StartLast: %v", err)
	}
	if !got.Running || got.ActivityID != 1 {
		t.Fatalf("StartLast must resume the chained activity: %+v", got)
	}

	// StartLast while running is a no-op.
	same, err := s.StartLast()
	if err != nil || !same.Running || same.ActivityID != 1 {
		t.Fatalf("StartLast while running: %+v err=%v", same, err)
	}
}
