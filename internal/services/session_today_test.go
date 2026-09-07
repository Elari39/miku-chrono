package services

import (
	"path/filepath"
	"testing"
	"time"

	"mikuchrono/internal/store"
)

// sessionSecondsToday clips the running session to the local day of now so a
// cross-midnight session cannot inflate today's goal bucket with seconds the
// recorded data attributes to yesterday.

func TestSessionSecondsToday(t *testing.T) {
	// Session started yesterday 23:00, checked today 00:30: only the
	// post-midnight half counts toward today.
	now := time.Date(2025, 9, 2, 0, 30, 0, 0, time.Local)
	started := store.FormatTime(time.Date(2025, 9, 1, 23, 0, 0, 0, time.Local))
	if got := sessionSecondsToday(started, now); got != 1800 {
		t.Fatalf("cross-midnight session = %d, want 1800", got)
	}

	// Session fully inside today counts whole.
	started = store.FormatTime(time.Date(2025, 9, 2, 10, 0, 0, 0, time.Local))
	now = time.Date(2025, 9, 2, 10, 30, 0, 0, time.Local)
	if got := sessionSecondsToday(started, now); got != 1800 {
		t.Fatalf("same-day session = %d, want 1800", got)
	}

	// A clock rolled back before the session start contributes nothing.
	started = store.FormatTime(time.Date(2025, 9, 2, 10, 0, 0, 0, time.Local))
	now = time.Date(2025, 9, 2, 9, 0, 0, 0, time.Local)
	if got := sessionSecondsToday(started, now); got != 0 {
		t.Fatalf("rollback before start = %d, want 0", got)
	}

	// Unparseable timestamps contribute nothing (retried next tick).
	if got := sessionSecondsToday("not-a-time", now); got != 0 {
		t.Fatalf("unparseable start = %d, want 0", got)
	}
}

// CachedState projects elapsed seconds from its snapshot; a system clock set
// back between snapshot and projection must not produce negative values.
func TestCachedStateRollbackClampsProjection(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := &TimerService{Store: st}

	// Start ~10 minutes ago so the fresh snapshot carries a sane elapsed.
	t0 := time.Now().Add(-10 * time.Minute)
	if _, err := st.StartTimer(1, t0); err != nil {
		t.Fatalf("start: %v", err)
	}
	snapshot, err := s.GetState()
	if err != nil || !snapshot.Running {
		t.Fatalf("get state: %+v err=%v", snapshot, err)
	}

	projected, err := s.CachedState(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("cached state: %v", err)
	}
	if !projected.Running || projected.ElapsedSeconds != snapshot.ElapsedSeconds ||
		projected.SessionElapsedSeconds != snapshot.SessionElapsedSeconds {
		t.Fatalf("rollback projection must hold the snapshot (no negative add-on): got %+v want %+v",
			projected, snapshot)
	}
}
