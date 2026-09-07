package store

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mikuchrono/internal/models"
)

// Regression coverage for the report fixes: ClearEntries atomicity, the
// atomic stop-and-delete flow, backend activity validation, half-open date
// boundaries and the seed-only-once rule.

// breakDeletesOn installs a trigger aborting every DELETE on the given table
// so the corresponding step of a multi-step operation can be failure-injected.
func breakDeletesOn(t *testing.T, s *Store, table string) {
	t.Helper()
	if _, err := s.db.Exec(`CREATE TRIGGER break_` + table + `_delete
		BEFORE DELETE ON ` + table + ` BEGIN
			SELECT RAISE(ABORT, 'injected delete failure');
		END`); err != nil {
		t.Fatalf("create delete trigger on %s: %v", table, err)
	}
}

// TestClearEntriesIsAtomic covers both the success path (every target cleared
// together, unrelated settings kept) and rollback (an injected failure on one
// step leaves entries and meta exactly as before).
func TestClearEntriesIsAtomic(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)
	now := base.Add(time.Hour)

	if _, err := s.CreateManualEntry(1, FormatTime(base), FormatTime(base.Add(30*time.Minute)), "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTimer(1, base.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// A goal dedupe key plus an unrelated setting that must survive.
	if err := s.SetSetting(GoalNotifiedKeyPrefix+"2025-09-01_1", "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting("close_action", "quit"); err != nil {
		t.Fatal(err)
	}

	if err := s.ClearEntries(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if n := countEntries(t, s); n != 0 {
		t.Fatalf("entries must be cleared, got %d", n)
	}
	st, err := s.GetTimerState(base.Add(2 * time.Hour))
	if err != nil || st.Running || st.LastActivityID != 0 || st.LastElapsedSeconds != 0 {
		t.Fatalf("running state and chain must be cleared: %+v err=%v", st, err)
	}
	var metaCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM meta WHERE key LIKE ?`, GoalNotifiedKeyPrefix+"%").Scan(&metaCount); err != nil {
		t.Fatal(err)
	}
	if metaCount != 0 {
		t.Fatalf("goal notify keys must be cleared, got %d", metaCount)
	}
	if v, ok, err := s.GetSetting("close_action"); err != nil || !ok || v != "quit" {
		t.Fatalf("unrelated setting must survive: v=%q ok=%v err=%v", v, ok, err)
	}

	// Failure injection: the running_state delete aborts; the transaction must
	// roll back so entries, the running session and all meta keys stay as
	// they were. A live running row is required so the BEFORE DELETE trigger
	// actually fires.
	if _, err := s.CreateManualEntry(1, FormatTime(base), FormatTime(base.Add(30*time.Minute)), "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTimer(1, base.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(GoalNotifiedKeyPrefix+"2025-09-01_1", "1"); err != nil {
		t.Fatal(err)
	}
	breakDeletesOn(t, s, "running_state")
	if err := s.ClearEntries(); err == nil || !strings.Contains(err.Error(), "injected delete failure") {
		t.Fatalf("expected the injected failure to surface, got %v", err)
	}
	if n := countEntries(t, s); n != 1 {
		t.Fatalf("entries must survive the rolled-back clear, got %d", n)
	}
	st, err = s.GetTimerState(base.Add(2 * time.Hour))
	if err != nil || !st.Running || st.ActivityID != 1 {
		t.Fatalf("running session must survive the rolled-back clear: %+v err=%v", st, err)
	}
	var chainCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM meta WHERE key IN (?, ?)`, keyTimerActivity, keyTimerSeconds).Scan(&chainCount); err != nil {
		t.Fatal(err)
	}
	// StartTimer wrote both chain keys; the rolled-back clear must keep them.
	if chainCount != 2 {
		t.Fatalf("timer chain keys must survive the rolled-back clear, got %d", chainCount)
	}
	var goalCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM meta WHERE key LIKE ?`, GoalNotifiedKeyPrefix+"%").Scan(&goalCount); err != nil {
		t.Fatal(err)
	}
	if goalCount != 1 {
		t.Fatalf("goal notify key must survive the rolled-back clear, got %d", goalCount)
	}
}

// TestStopAndDeleteRunningActivity verifies the atomic flow records the final
// segment, clears the running state and removes the activity with its entries.
func TestStopAndDeleteRunningActivity(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	// An unrelated entry of the same activity must cascade away too.
	if _, err := s.CreateManualEntry(1, FormatTime(base.Add(-time.Hour)), FormatTime(base.Add(-30*time.Minute)), "", base); err != nil {
		t.Fatal(err)
	}

	entry, stopped, err := s.StopAndDeleteActivity(1, base.Add(5*time.Minute))
	if err != nil || !stopped {
		t.Fatalf("stop and delete: entry=%+v stopped=%v err=%v", entry, stopped, err)
	}
	if entry == nil || entry.ActivityName != "学习" || entry.ActivityColor != "#cc785c" || entry.DurationSeconds != 300 {
		t.Fatalf("recorded final segment: %+v", entry)
	}
	if _, err := s.GetActivity(1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("activity must be gone, got %v", err)
	}
	// The recorded segment cascades away with the activity: the in-memory
	// return value exists for the delete confirmation only.
	if n := countEntries(t, s); n != 0 {
		t.Fatalf("all entries must cascade with the activity, got %d", n)
	}
	// Idle, chain parked exactly like a plain StopTimer before the delete.
	st, gerr := s.GetTimerState(base.Add(6 * time.Minute))
	if gerr != nil || st.Running {
		t.Fatalf("timer must be idle after stop and delete: %+v err=%v", st, gerr)
	}
	if st.LastActivityID != 1 || st.LastElapsedSeconds != 300 || st.LastActivityName != "" {
		t.Fatalf("chain mirrors StopTimer semantics: %+v", st)
	}

	// Repeating the operation reports the missing activity.
	if _, _, err := s.StopAndDeleteActivity(1, base.Add(7*time.Minute)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("repeated call must return ErrNotFound, got %v", err)
	}
}

// TestStopAndDeleteKeepsOtherRunningTimer covers deleting a different activity
// while a timer runs: the running session is untouched and nothing is recorded.
func TestStopAndDeleteKeepsOtherRunningTimer(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateManualEntry(2, FormatTime(base.Add(-time.Minute)), FormatTime(base.Add(-30*time.Second)), "", base); err != nil {
		t.Fatal(err)
	}

	entry, stopped, err := s.StopAndDeleteActivity(2, base.Add(30*time.Second))
	if err != nil || stopped || entry != nil {
		t.Fatalf("delete without stop: entry=%+v stopped=%v err=%v", entry, stopped, err)
	}
	if _, err := s.GetActivity(2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("activity 2 must be gone, got %v", err)
	}
	st, gerr := s.GetTimerState(base.Add(40 * time.Second))
	if gerr != nil || !st.Running || st.ActivityID != 1 || st.SessionElapsedSeconds != 40 {
		t.Fatalf("running session must be untouched: %+v err=%v", st, gerr)
	}
	if n := countEntries(t, s); n != 0 {
		t.Fatalf("no entry must be recorded, got %d", n)
	}
}

// TestStopAndDeleteRollback injects a stop failure and a delete failure and
// asserts the whole operation rolls back with no orphans.
func TestStopAndDeleteRollback(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}

	// Stop failure: the entry insert aborts before the delete happens.
	breakEntriesInsert(t, s)
	if _, stopped, err := s.StopAndDeleteActivity(1, base.Add(5*time.Minute)); err == nil || stopped {
		t.Fatalf("injected stop failure must surface: stopped=%v err=%v", stopped, err)
	}
	if _, err := s.GetActivity(1); err != nil {
		t.Fatalf("activity must survive the failed stop: %v", err)
	}
	st, gerr := s.GetTimerState(base.Add(5 * time.Minute))
	if gerr != nil || !st.Running || st.ActivityID != 1 || st.SessionElapsedSeconds != 300 {
		t.Fatalf("running state must survive the failed stop: %+v err=%v", st, gerr)
	}
	if n := countEntries(t, s); n != 0 {
		t.Fatalf("no entry must be recorded on failure, got %d", n)
	}
	fixEntriesInsert(t, s)

	// Delete failure: the session close succeeded inside the transaction but
	// the delete aborts — everything must roll back together.
	breakDeletesOn(t, s, "activities")
	if _, _, err := s.StopAndDeleteActivity(1, base.Add(10*time.Minute)); err == nil {
		t.Fatal("injected delete failure must surface")
	}
	if _, err := s.GetActivity(1); err != nil {
		t.Fatalf("activity must survive the failed delete: %v", err)
	}
	st, gerr = s.GetTimerState(base.Add(10 * time.Minute))
	if gerr != nil || !st.Running || st.ActivityID != 1 || st.SessionElapsedSeconds != 600 {
		t.Fatalf("running state must survive the failed delete: %+v err=%v", st, gerr)
	}
	if n := countEntries(t, s); n != 0 {
		t.Fatalf("the rolled-back session close must leave no entry, got %d", n)
	}
}

// TestActivityFieldValidation covers the shared create/update boundary rules.
func TestActivityFieldValidation(t *testing.T) {
	s := newTestStore(t)

	created, err := s.CreateActivity(models.Activity{Name: " 阅读 ", Color: "#5DB8A6", DailyGoalMinutes: 1440, SortOrder: 3})
	if err != nil {
		t.Fatalf("valid create: %v", err)
	}
	if created.Name != "阅读" || created.Color != "#5DB8A6" {
		t.Fatalf("create must trim the name and keep the color: %+v", created)
	}

	cases := []struct {
		name string
		a    models.Activity
	}{
		{"empty name", models.Activity{Name: "  "}},
		{"bad color", models.Activity{Name: "x", Color: "red"}},
		{"short color", models.Activity{Name: "x", Color: "#12345"}},
		{"negative goal", models.Activity{Name: "x", DailyGoalMinutes: -1}},
		{"goal over 1440", models.Activity{Name: "x", DailyGoalMinutes: 1441}},
		{"negative sort", models.Activity{Name: "x", SortOrder: -1}},
		{"unknown category", models.Activity{Name: "x", CategoryID: int64Ptr(999)}},
	}
	for _, tc := range cases {
		if _, err := s.CreateActivity(tc.a); !errors.Is(err, ErrValidation) {
			t.Fatalf("%s must fail with a validation error, got %v", tc.name, err)
		}
	}

	// A real category passes the existence check.
	if _, err := s.CreateCategory(models.Category{Name: "学习类"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateActivity(models.Activity{Name: "分类活动", CategoryID: int64Ptr(1)}); err != nil {
		t.Fatalf("valid category must pass: %v", err)
	}
	// Unknown category must not surface raw SQLite foreign-key text.
	if _, err := s.CreateActivity(models.Activity{Name: "x", CategoryID: int64Ptr(42)}); err == nil || strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("unknown category must be a validation error, got %v", err)
	}

	// Update paths: same boundaries, plus ErrNotFound for a missing activity.
	if err := s.UpdateActivity(models.Activity{ID: created.ID, Name: "阅读", DailyGoalMinutes: 0, SortOrder: 0}); err != nil {
		t.Fatalf("valid update: %v", err)
	}
	if err := s.UpdateActivity(models.Activity{ID: created.ID, Name: "x", DailyGoalMinutes: -1}); !errors.Is(err, ErrValidation) {
		t.Fatalf("negative goal update must fail validation, got %v", err)
	}
	if err := s.UpdateActivity(models.Activity{ID: created.ID, Name: "x", CategoryID: int64Ptr(999)}); !errors.Is(err, ErrValidation) {
		t.Fatalf("unknown category update must fail validation, got %v", err)
	}
	if err := s.UpdateActivity(models.Activity{ID: 9999, Name: "幽灵"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing activity id must return ErrNotFound, got %v", err)
	}
}

func int64Ptr(v int64) *int64 { return &v }

// TestHalfOpenOverlapBoundaries pins the [started_at, ended_at) semantics:
// a record ending exactly at a day's midnight occupies none of that day,
// while one starting exactly at midnight belongs to it fully.
func TestHalfOpenOverlapBoundaries(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2025, 9, 5, 12, 0, 0, 0, time.Local)

	// Ends exactly at 09-02 midnight: 3600s all on 09-01.
	if _, err := s.CreateManualEntry(1, localRFC3339(2025, time.September, 1, 23, 0), localRFC3339(2025, time.September, 2, 0, 0), "", now); err != nil {
		t.Fatal(err)
	}
	// Starts exactly at 09-02 midnight: 1800s on 09-02.
	if _, err := s.CreateManualEntry(2, localRFC3339(2025, time.September, 2, 0, 0), localRFC3339(2025, time.September, 2, 0, 30), "", now); err != nil {
		t.Fatal(err)
	}
	// Crosses midnight: 600s on each day.
	if _, err := s.CreateManualEntry(3, localRFC3339(2025, time.September, 1, 23, 50), localRFC3339(2025, time.September, 2, 0, 10), "", now); err != nil {
		t.Fatal(err)
	}

	// Stats splitting (loadDayPieces-based).
	day1, err := s.TodaySecondsByActivity("2025-09-01")
	if err != nil {
		t.Fatal(err)
	}
	if day1[1] != 3600 || day1[3] != 600 {
		t.Fatalf("09-01 seconds: %v", day1)
	}
	day2, err := s.TodaySecondsByActivity("2025-09-02")
	if err != nil {
		t.Fatal(err)
	}
	if day2[1] != 0 || day2[2] != 1800 || day2[3] != 600 {
		t.Fatalf("09-02 seconds must exclude the record ending at midnight: %v", day2)
	}

	// List overlap must agree with the stats split.
	overlap := true
	list, err := s.ListEntries(models.EntryFilter{FromDate: "2025-09-02", ToDate: "2025-09-02", Overlap: &overlap})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 2 || len(list.Items) != 2 {
		t.Fatalf("09-02 overlap must keep exactly the two touching records, got %+v", list)
	}
	for _, e := range list.Items {
		if e.ActivityID == 1 {
			t.Fatalf("record ending exactly at the day start must not show up: %+v", list.Items)
		}
	}
}

// TestSeedHappensOnlyOnceOnFreshDatabase deletes every activity from an
// existing database and reopens it: the defaults must not resurrect.
func TestSeedHappensOnlyOnceOnFreshDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	acts, err := s1.ListActivities(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 3 {
		t.Fatalf("fresh database must be seeded, got %d activities", len(acts))
	}
	for _, a := range acts {
		if err := s1.DeleteActivity(a.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	acts, err = s2.ListActivities(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 0 {
		t.Fatalf("an emptied database must stay empty after reopening, got %+v", acts)
	}
}
