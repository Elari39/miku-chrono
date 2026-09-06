package store

import (
	"strings"
	"testing"
	"time"

	"mikuchrono/internal/models"
)

// Regression coverage for the reliability fixes: closeRunningTx must
// propagate write errors so StartTimer/StopTimer roll back and keep the
// running state intact, exports must bypass the 200-per-page cap, and note
// validation must count code points, not bytes.

// breakEntriesInsert installs a trigger that aborts every entries INSERT so
// timer finalization can be failure-injected on a throwaway database.
func breakEntriesInsert(t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.db.Exec(`CREATE TRIGGER break_entries_insert
		BEFORE INSERT ON entries BEGIN
			SELECT RAISE(ABORT, 'injected insert failure');
		END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
}

// fixEntriesInsert removes the failure trigger so a retry can succeed.
func fixEntriesInsert(t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.db.Exec(`DROP TRIGGER break_entries_insert`); err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}
}

// injectFailed asserts that the error is the injected SQLite abort and not a
// validation result (which would mean the error never reached the caller).
func injectFailed(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected the injected insert failure to surface as an error")
	}
	if !strings.Contains(err.Error(), "injected insert failure") {
		t.Fatalf("unexpected error kind: %v", err)
	}
}

// countEntries returns the number of stored entries.
func countEntries(t *testing.T, s *Store) int64 {
	t.Helper()
	var n int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&n); err != nil {
		t.Fatalf("count entries: %v", err)
	}
	return n
}

func TestStopTimerInsertFailureKeepsRunningState(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatalf("start: %v", err)
	}

	breakEntriesInsert(t, s)
	_, err := s.StopTimer(base.Add(5 * time.Minute))
	injectFailed(t, err)

	// The transaction rolled back: still running on the same activity with
	// the original started_at and nothing recorded.
	st, gerr := s.GetTimerState(base.Add(5 * time.Minute))
	if gerr != nil {
		t.Fatalf("state after failed stop: %v", gerr)
	}
	if !st.Running || st.ActivityID != 1 {
		t.Fatalf("timer must keep running after a failed stop: %+v", st)
	}
	if st.ElapsedSeconds != 300 || st.SessionElapsedSeconds != 300 {
		t.Fatalf("elapsed must be preserved: %+v", st)
	}
	if n := countEntries(t, s); n != 0 {
		t.Fatalf("no entry must be recorded on failure, got %d", n)
	}

	// Removing the fault makes a plain retry succeed exactly once and fold
	// the full 360s segment into the paused chain.
	fixEntriesInsert(t, s)
	entry, err := s.StopTimer(base.Add(6 * time.Minute))
	if err != nil || entry == nil || entry.DurationSeconds != 360 {
		t.Fatalf("retry after failure: entry=%+v err=%v", entry, err)
	}
	if n := countEntries(t, s); n != 1 {
		t.Fatalf("retry must record exactly one segment, got %d", n)
	}
	st, gerr = s.GetTimerState(base.Add(6 * time.Minute))
	if gerr != nil || st.Running || st.LastActivityID != 1 || st.LastElapsedSeconds != 360 {
		t.Fatalf("chain after retry: %+v err=%v", st, gerr)
	}
}

func TestSwitchTimerInsertFailureKeepsPreviousSession(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatalf("start: %v", err)
	}

	breakEntriesInsert(t, s)
	_, err := s.StartTimer(2, base.Add(30*time.Second))
	injectFailed(t, err)

	// The old session stays intact; the switch did not half-apply.
	st, gerr := s.GetTimerState(base.Add(30 * time.Second))
	if gerr != nil {
		t.Fatalf("state after failed switch: %v", gerr)
	}
	if !st.Running || st.ActivityID != 1 {
		t.Fatalf("previous session must survive a failed switch: %+v", st)
	}
	if st.SessionElapsedSeconds != 30 {
		t.Fatalf("previous session elapsed must survive: %+v", st)
	}
	if n := countEntries(t, s); n != 0 {
		t.Fatalf("no entry must be recorded on failure, got %d", n)
	}

	// Retry the switch without the fault: closes the 学习 segment once.
	fixEntriesInsert(t, s)
	if _, err := s.StartTimer(2, base.Add(time.Minute)); err != nil {
		t.Fatalf("retry switch: %v", err)
	}
	if n := countEntries(t, s); n != 1 {
		t.Fatalf("retry must record exactly one segment, got %d", n)
	}
}

func TestShortSessionStopStillIdleAfterFix(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	// Sub-second session: discarded without an entry, but the stop succeeds
	// and the timer still becomes idle.
	entry, err := s.StopTimer(base.Add(300 * time.Millisecond))
	if err != nil {
		t.Fatalf("short stop must succeed: %v", err)
	}
	if entry != nil {
		t.Fatalf("short session must not produce an entry: %+v", entry)
	}
	st, gerr := s.GetTimerState(base.Add(time.Second))
	if gerr != nil || st.Running {
		t.Fatalf("short stop must leave the timer idle: %+v err=%v", st, gerr)
	}
	if n := countEntries(t, s); n != 0 {
		t.Fatalf("short session must not be recorded, got %d entries", n)
	}
}

func TestListAllEntriesBypassesPaginationCap(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)
	// "now" sits after every seeded range so validateRange accepts them all.
	now := base.Add(300 * time.Minute)

	// Seed 251 manual entries (well above the 200-per-page cap).
	for i := 0; i < 251; i++ {
		start := base.Add(time.Duration(i) * time.Minute)
		if _, err := s.CreateManualEntry(1, FormatTime(start), FormatTime(start.Add(time.Second)), "", now); err != nil {
			t.Fatalf("seed entry %d: %v", i, err)
		}
	}

	all, err := s.ListAllEntries()
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 251 {
		t.Fatalf("ListAllEntries must return every entry, got %d", len(all))
	}
	// Newest first, and every id appears exactly once.
	seen := map[int64]bool{}
	for i, e := range all {
		if seen[e.ID] {
			t.Fatalf("duplicate entry id %d", e.ID)
		}
		seen[e.ID] = true
		if i > 0 && all[i-1].StartedAt < e.StartedAt {
			t.Fatalf("ordering must be newest first")
		}
	}

	page, err := s.ListEntries(models.EntryFilter{Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.PageSize != 200 || page.Total != 251 || len(page.Items) != 200 {
		t.Fatalf("ListEntries must keep its 200 cap: page=%d total=%d items=%d", page.PageSize, page.Total, len(page.Items))
	}
}

func TestNoteValidationCountsCodePoints(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)
	now := base.Add(time.Hour)

	// 167 汉字 = 167 code points but 501 UTF-8 bytes: must be accepted now.
	note167 := strings.Repeat("中", 167)
	if _, err := s.CreateManualEntry(1, FormatTime(base), FormatTime(base.Add(time.Minute)), note167, now); err != nil {
		t.Fatalf("167 Chinese chars must pass code-point validation: %v", err)
	}

	// 500 code points pass on both create and update, 501 are rejected.
	note500 := strings.Repeat("中", 500)
	e, err := s.CreateManualEntry(1, FormatTime(base.Add(time.Minute)), FormatTime(base.Add(2*time.Minute)), note500, now)
	if err != nil {
		t.Fatalf("500 code points must pass: %v", err)
	}
	if err := s.UpdateEntry(e, now); err != nil {
		t.Fatalf("update with 500 code points must pass: %v", err)
	}

	note501 := strings.Repeat("中", 501)
	if _, err := s.CreateManualEntry(1, FormatTime(base.Add(3*time.Minute)), FormatTime(base.Add(4*time.Minute)), note501, now); err == nil {
		t.Fatal("501 code points must be rejected on create")
	}
	e.Note = note501
	if err := s.UpdateEntry(e, now); err == nil {
		t.Fatal("501 code points must be rejected on update")
	}
}
