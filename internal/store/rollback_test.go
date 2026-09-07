package store

import (
	"strings"
	"testing"
	"time"
)

// Clock-rollback hardening: a system clock set back between start and stop
// must never shrink the persisted chain base nor surface negative elapsed
// time; sub-second manual ranges must be rejected instead of persisting a
// zero duration; pragmas must ride the DSN so pool-rebuilt connections keep
// them.

func TestStopTimerRollbackKeepsChainBase(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	// First session accumulates 600s into the chain.
	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StopTimer(base.Add(10 * time.Minute)); err != nil {
		t.Fatal(err)
	}

	// Resume one second later, then stop with the clock rolled back to just
	// before the resume: the session is discarded (too short) and the fold
	// must add zero, leaving the chain at 600 instead of shrinking it.
	if _, err := s.StartTimer(1, base.Add(10*time.Minute+time.Second)); err != nil {
		t.Fatal(err)
	}
	entry, err := s.StopTimer(base.Add(10 * time.Minute))
	if err != nil || entry != nil {
		t.Fatalf("rollback stop: entry=%+v err=%v", entry, err)
	}
	st, _ := s.GetTimerState(base.Add(11 * time.Minute))
	if st.Running || st.LastActivityID != 1 || st.LastElapsedSeconds != 600 {
		t.Fatalf("chain base must survive the rollback unchanged: %+v", st)
	}
}

func TestGetTimerStateRollbackClampsElapsed(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	// Build a 600s chain, resume, then read the state with the clock set
	// back an hour: session elapsed clamps to zero and the total stays at the
	// accumulated base instead of going negative.
	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StopTimer(base.Add(10 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTimer(1, base.Add(10*time.Minute+time.Second)); err != nil {
		t.Fatal(err)
	}
	st, err := s.GetTimerState(base.Add(10 * time.Minute))
	if err != nil || !st.Running {
		t.Fatalf("state: %+v err=%v", st, err)
	}
	if st.SessionElapsedSeconds != 0 || st.ElapsedSeconds != 600 {
		t.Fatalf("rollback elapsed must clamp: session=%d elapsed=%d", st.SessionElapsedSeconds, st.ElapsedSeconds)
	}
}

func TestValidateRangeMinimumOneSecond(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2025, 9, 2, 12, 0, 0, 0, time.Local)

	// 300ms would truncate to duration_seconds = 0 and poison the day/streak
	// aggregates, so it is rejected outright.
	if _, err := s.CreateManualEntry(1, "2025-09-02T10:00:00.300+08:00", "2025-09-02T10:00:00.600+08:00", "", now); err == nil {
		t.Fatal("sub-second range must be rejected")
	} else if !strings.Contains(err.Error(), "时长至少 1 秒") {
		t.Fatalf("unexpected validation error: %v", err)
	}

	// Exactly one second is the shortest acceptable range and stores as 1s.
	e, err := s.CreateManualEntry(1, "2025-09-02T10:00:00+08:00", "2025-09-02T10:00:01+08:00", "", now)
	if err != nil {
		t.Fatalf("one-second entry: %v", err)
	}
	if e.DurationSeconds != 1 {
		t.Fatalf("duration = %d, want 1", e.DurationSeconds)
	}
}

func TestSetSettingsWritesGroup(t *testing.T) {
	s := newTestStore(t)

	// Seed conflicting values, then overwrite the whole group atomically.
	if err := s.SetSetting("win_x", "1"); err != nil {
		t.Fatal(err)
	}
	err := s.SetSettings([][2]string{
		{"win_x", "10"},
		{"win_y", "20"},
		{"win_w", "800"},
		{"win_h", "600"},
	})
	if err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	for key, want := range map[string]string{"win_x": "10", "win_y": "20", "win_w": "800", "win_h": "600"} {
		got, found, err := s.GetSetting(key)
		if err != nil || !found || got != want {
			t.Fatalf("setting %q = %q found=%v err=%v, want %q", key, got, found, err, want)
		}
	}

	// An empty group is a no-op, not an error.
	if err := s.SetSettings(nil); err != nil {
		t.Fatalf("empty SetSettings: %v", err)
	}
}

// TestOpenAppliesPragmasOnConnect verifies the pragmas ride the DSN: they
// must be queryable on the live connection, and a rebuilt pool connection
// would re-apply them (a post-open Exec only configures the one connection
// that happened to exist at Open time).
func TestOpenAppliesPragmasOnConnect(t *testing.T) {
	s := newTestStore(t)

	var fk int
	if err := s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1 (cascade deletes would silently break)", fk)
	}
	var busy int
	if err := s.db.QueryRow(`PRAGMA busy_timeout`).Scan(&busy); err != nil {
		t.Fatal(err)
	}
	if busy != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busy)
	}
	var mode string
	if err := s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}
