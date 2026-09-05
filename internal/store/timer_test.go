package store

import (
	"testing"
	"time"

	"mikuchrono/internal/models"
)

// Chain semantics: a stopped timer parks activity + accumulated seconds in
// the meta table, resuming the same activity carries the base forward, and
// only the current session's segment (never the accumulated total) is
// recorded as an entry.

func TestTimerResumeAccumulatesAndRecordsSegments(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	// First session: 10 minutes of 学习.
	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatalf("start: %v", err)
	}
	st, err := s.GetTimerState(base.Add(10 * time.Minute))
	if err != nil || !st.Running || st.ElapsedSeconds != 600 {
		t.Fatalf("state after 10min: %+v err=%v", st, err)
	}

	// Stop: records the first segment and parks the chain.
	entry, err := s.StopTimer(base.Add(10 * time.Minute))
	if err != nil || entry == nil || entry.DurationSeconds != 600 {
		t.Fatalf("first stop: entry=%+v err=%v", entry, err)
	}

	// Idle state now carries the paused chain for display/resume.
	st, _ = s.GetTimerState(base.Add(10*time.Minute + time.Second))
	if st.Running {
		t.Fatal("state must be idle after stop")
	}
	if st.LastActivityID != 1 || st.LastElapsedSeconds != 600 || st.LastActivityName != "学习" {
		t.Fatalf("idle chain: %+v", st)
	}

	// Resume the same activity ("开始计时"): elapsed continues from 600.
	st, err = s.StartTimer(1, base.Add(10*time.Minute+3*time.Second))
	if err != nil || !st.Running || st.ActivityID != 1 {
		t.Fatalf("resume: %+v err=%v", st, err)
	}
	// Re-starting the running activity stays a no-op even on a chain.
	if _, err := s.StartTimer(1, base.Add(11*time.Minute)); err != nil {
		t.Fatalf("noop re-start: %v", err)
	}
	st, _ = s.GetTimerState(base.Add(12 * time.Minute))
	if st.ElapsedSeconds != 717 { // 600 base + 117 resumed segment (resume at +3s)
		t.Fatalf("elapsed after resume = %d, want 717", st.ElapsedSeconds)
	}

	// Second stop: records ONLY the resumed segment (117 seconds).
	entry, err = s.StopTimer(base.Add(12 * time.Minute))
	if err != nil || entry == nil || entry.DurationSeconds != 117 {
		t.Fatalf("second stop: entry=%+v err=%v", entry, err)
	}
	st, _ = s.GetTimerState(base.Add(12*time.Minute + time.Second))
	if st.LastActivityID != 1 || st.LastElapsedSeconds != 717 {
		t.Fatalf("chain after second stop: %+v", st)
	}

	// Entries are disjoint segments; the sum equals the accumulated total.
	list, err := s.ListEntries(models.EntryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 2 {
		t.Fatalf("want 2 entries, got %d", list.Total)
	}
	// Newest first (ListEntries ordering); segment 2 = 117s, segment 1 = 600s.
	if list.Items[0].DurationSeconds != 117 || list.Items[1].DurationSeconds != 600 {
		t.Fatalf("segment durations: %+v", list.Items)
	}
	if list.Items[0].DurationSeconds+list.Items[1].DurationSeconds != 717 {
		t.Fatalf("segments must sum to the accumulated total")
	}
}

func TestTimerSwitchActivityResetsChain(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	// Switch to 工作 after 30s: records the 学习 segment, chain resets.
	if _, err := s.StartTimer(2, base.Add(30*time.Second)); err != nil {
		t.Fatalf("switch: %v", err)
	}
	st, _ := s.GetTimerState(base.Add(60 * time.Second))
	if !st.Running || st.ActivityID != 2 || st.ElapsedSeconds != 30 {
		t.Fatalf("switched session must start from zero: %+v", st)
	}

	// Stop at +90s: segment is 60s only; chain parks activity 2 at 60s.
	entry, err := s.StopTimer(base.Add(90 * time.Second))
	if err != nil || entry == nil || entry.DurationSeconds != 60 {
		t.Fatalf("stop after switch: entry=%+v err=%v", entry, err)
	}
	st, _ = s.GetTimerState(base.Add(91 * time.Second))
	if st.LastActivityID != 2 || st.LastElapsedSeconds != 60 {
		t.Fatalf("chain after switch: %+v", st)
	}
	list, _ := s.ListEntries(models.EntryFilter{})
	if list.Total != 2 {
		t.Fatalf("want 2 entries (switch + stop), got %d", list.Total)
	}

	// Returning to the OLD activity starts fresh, not from its old chain.
	if _, err := s.StartTimer(1, base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	st, _ = s.GetTimerState(base.Add(2*time.Minute + 10*time.Second))
	if st.ElapsedSeconds != 10 {
		t.Fatalf("old activity must start from zero: %+v", st)
	}
}

func TestTimerIdleStateWithoutChain(t *testing.T) {
	s := newTestStore(t)
	st, err := s.GetTimerState(time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if st.Running || st.LastActivityID != 0 || st.LastElapsedSeconds != 0 || st.LastActivityName != "" {
		t.Fatalf("fresh store must yield an empty idle state: %+v", st)
	}
}

func TestTimerChainSurvivesDeletedActivity(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StopTimer(base.Add(5 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	// Deleting the activity cascades its entries but not the chain. The idle
	// state keeps the id while the name falls back to empty; resuming then
	// fails with 活动不存在 (surfaced as a toast by the menu layer).
	if err := s.DeleteActivity(1); err != nil {
		t.Fatal(err)
	}
	st, _ := s.GetTimerState(base.Add(6 * time.Minute))
	if st.LastActivityID != 1 || st.LastElapsedSeconds != 300 || st.LastActivityName != "" {
		t.Fatalf("chain after delete: %+v", st)
	}
	if _, err := s.StartTimer(1, base.Add(7*time.Minute)); err == nil {
		t.Fatal("resuming a deleted activity must fail")
	}
}

func TestTimerChainClearedByClearEntries(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StopTimer(base.Add(5 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearEntries(); err != nil {
		t.Fatal(err)
	}
	st, _ := s.GetTimerState(base.Add(6 * time.Minute))
	if st.Running || st.LastActivityID != 0 || st.LastElapsedSeconds != 0 {
		t.Fatalf("clear must reset the chain: %+v", st)
	}
}
