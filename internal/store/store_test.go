package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"mikuchrono/internal/models"
)

// newTestStore opens a throwaway database seeded with the three default
// activities (ids 1..3).
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSeedOnFirstRun(t *testing.T) {
	s := newTestStore(t)
	acts, err := s.ListActivities(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 3 {
		t.Fatalf("want 3 seeded activities, got %d", len(acts))
	}
	if acts[0].Name != "学习" || acts[0].Color != "#cc785c" {
		t.Fatalf("unexpected first seed: %+v", acts[0])
	}
}

func TestTimerMutualExclusionSwitch(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	// Start activity 1, then switch to activity 2 thirty seconds later.
	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatalf("start 1: %v", err)
	}
	st, err := s.GetTimerState(base.Add(10 * time.Second))
	if err != nil || !st.Running || st.ActivityID != 1 {
		t.Fatalf("state after start: %+v err=%v", st, err)
	}
	if st.ElapsedSeconds != 10 {
		t.Fatalf("elapsed = %d, want 10", st.ElapsedSeconds)
	}
	if _, err := s.StartTimer(2, base.Add(30*time.Second)); err != nil {
		t.Fatalf("start 2: %v", err)
	}

	// The switch must have recorded a 30s entry for activity 1.
	list, err := s.ListEntries(models.EntryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || list.Items[0].ActivityID != 1 || list.Items[0].DurationSeconds != 30 {
		t.Fatalf("entries after switch: %+v", list)
	}

	// Stop 60s later: one more entry for activity 2, state idle.
	entry, err := s.StopTimer(base.Add(90 * time.Second))
	if err != nil || entry == nil {
		t.Fatalf("stop: entry=%v err=%v", entry, err)
	}
	if entry.ActivityID != 2 || entry.DurationSeconds != 60 {
		t.Fatalf("stopped entry: %+v", entry)
	}
	st, _ = s.GetTimerState(base.Add(91 * time.Second))
	if st.Running {
		t.Fatal("state should be idle after stop")
	}
	list, _ = s.ListEntries(models.EntryFilter{})
	if list.Total != 2 {
		t.Fatalf("want 2 entries after stop, got %d", list.Total)
	}
}

func TestTimerSameActivityStartIsNoop(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTimer(1, base.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListEntries(models.EntryFilter{})
	if list.Total != 0 {
		t.Fatalf("re-start same activity must not record, got %d entries", list.Total)
	}
	st, _ := s.GetTimerState(base.Add(25 * time.Second))
	if !st.Running || st.ActivityID != 1 || st.ElapsedSeconds != 25 {
		t.Fatalf("running session must be preserved: %+v", st)
	}
}

func TestTimerShortSessionDiscarded(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if _, err := s.StartTimer(1, base); err != nil {
		t.Fatal(err)
	}
	entry, err := s.StopTimer(base.Add(500 * time.Millisecond))
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if entry != nil {
		t.Fatalf("sub-second session must be discarded, got %+v", entry)
	}
	list, _ := s.ListEntries(models.EntryFilter{})
	if list.Total != 0 {
		t.Fatalf("want 0 entries, got %d", list.Total)
	}
}

func TestStartArchivedActivityRejected(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 9, 1, 9, 0, 0, 0, time.Local)

	if err := s.UpdateActivity(models.Activity{ID: 1, Name: "学习", Color: "#cc785c", Archived: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTimer(1, base); err == nil {
		t.Fatal("starting archived activity must fail")
	}
}

func TestManualEntryValidation(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2025, 9, 2, 12, 0, 0, 0, time.Local)
	base := "2025-09-02T10:00:00+08:00"

	// end <= start rejected
	if _, err := s.CreateManualEntry(1, "2025-09-02T10:00:00+08:00", "2025-09-02T10:00:00+08:00", "", now); err == nil {
		t.Fatal("end == start must fail")
	}
	// end in the future rejected
	if _, err := s.CreateManualEntry(1, base, "2025-09-02T13:00:00+08:00", "", now); err == nil {
		t.Fatal("future end must fail")
	}
	// unknown activity rejected
	if _, err := s.CreateManualEntry(999, base, "2025-09-02T11:00:00+08:00", "", now); err == nil {
		t.Fatal("unknown activity must fail")
	}
	// valid entry
	e, err := s.CreateManualEntry(1, base, "2025-09-02T11:30:00+08:00", "  测试备注  ", now)
	if err != nil {
		t.Fatalf("valid manual entry: %v", err)
	}
	if e.DurationSeconds != 5400 || e.Source != "manual" || e.Note != "测试备注" {
		t.Fatalf("unexpected entry: %+v", e)
	}

	// update path: extend to one hour
	e.StartedAt = base
	e.EndedAt = "2025-09-02T11:00:00+08:00"
	if err := s.UpdateEntry(e, now); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.GetEntry(e.ID)
	if got.DurationSeconds != 3600 {
		t.Fatalf("after update duration = %d, want 3600", got.DurationSeconds)
	}
}

func TestCrossMidnightBelongsToStartDay(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2025, 9, 2, 12, 0, 0, 0, time.Local)

	// 23:50 → next day 00:10
	if _, err := s.CreateManualEntry(1, "2025-09-01T23:50:00+08:00", "2025-09-02T00:10:00+08:00", "", now); err != nil {
		t.Fatal(err)
	}
	sets, err := s.ActiveDaySets()
	if err != nil {
		t.Fatal(err)
	}
	if sets[1]["2025-09-01"] != true {
		t.Fatalf("entry must belong to start day 2025-09-01, got %v", sets[1])
	}
	if sets[1]["2025-09-02"] {
		t.Fatal("end day must not be counted")
	}
}

func TestDeleteActivityCascadesEntries(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2025, 9, 2, 12, 0, 0, 0, time.Local)

	created, err := s.CreateActivity(models.Activity{Name: "临时", Color: "#5db872"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateManualEntry(created.ID, "2025-09-01T10:00:00+08:00", "2025-09-01T11:00:00+08:00", "", now); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteActivity(created.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListEntries(models.EntryFilter{ActivityID: &created.ID})
	if list.Total != 0 {
		t.Fatalf("entries must cascade-delete, got %d", list.Total)
	}
	if _, err := s.GetActivity(created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("activity should be gone, got %v", err)
	}
}

func TestListEntriesFilterAndPaging(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2025, 9, 5, 12, 0, 0, 0, time.Local)

	fixtures := []struct {
		act        int64
		start, end string
	}{
		{1, "2025-09-01T09:00:00+08:00", "2025-09-01T10:00:00+08:00"},
		{2, "2025-09-01T11:00:00+08:00", "2025-09-01T12:00:00+08:00"},
		{1, "2025-09-03T09:00:00+08:00", "2025-09-03T10:30:00+08:00"},
		{3, "2025-09-04T09:00:00+08:00", "2025-09-04T10:00:00+08:00"},
	}
	for _, f := range fixtures {
		if _, err := s.CreateManualEntry(f.act, f.start, f.end, "", now); err != nil {
			t.Fatal(err)
		}
	}

	all, _ := s.ListEntries(models.EntryFilter{})
	if all.Total != 4 {
		t.Fatalf("want 4 total, got %d", all.Total)
	}
	one := int64(1)
	byAct, _ := s.ListEntries(models.EntryFilter{ActivityID: &one})
	if byAct.Total != 2 {
		t.Fatalf("activity filter: want 2, got %d", byAct.Total)
	}
	ranged, _ := s.ListEntries(models.EntryFilter{FromDate: "2025-09-03", ToDate: "2025-09-04"})
	if ranged.Total != 2 {
		t.Fatalf("date filter: want 2, got %d", ranged.Total)
	}
	paged, _ := s.ListEntries(models.EntryFilter{Page: 2, PageSize: 2})
	if len(paged.Items) != 2 || paged.Total != 4 {
		t.Fatalf("paging: len=%d total=%d", len(paged.Items), paged.Total)
	}
	// Newest first: the 11:00+08:00 record, stored as its UTC instant since
	// schema v3.
	got, err := time.Parse(time.RFC3339, paged.Items[0].StartedAt)
	if err != nil || !got.Equal(time.Date(2025, 9, 1, 3, 0, 0, 0, time.UTC)) {
		t.Fatalf("newest-first order broken: %+v err=%v", paged.Items[0], err)
	}
}

func TestDayBucketsAndTotals(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2025, 9, 2, 12, 0, 0, 0, time.Local)

	if _, err := s.CreateManualEntry(1, "2025-09-01T09:00:00+08:00", "2025-09-01T10:00:00+08:00", "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateManualEntry(2, "2025-09-01T11:00:00+08:00", "2025-09-01T11:30:00+08:00", "", now); err != nil {
		t.Fatal(err)
	}
	buckets, err := s.DayBuckets("2025-09-01", "2025-09-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || buckets[0].Total != 5400 {
		t.Fatalf("day bucket total: %+v", buckets)
	}
	if buckets[0].ByActivity["1"] != 3600 || buckets[0].ByActivity["2"] != 1800 {
		t.Fatalf("day bucket per-activity: %+v", buckets[0].ByActivity)
	}
	totals, err := s.ActivityTotalsBetween("2025-09-01", "2025-09-30")
	if err != nil {
		t.Fatal(err)
	}
	if totals[1] != 3600 || totals[2] != 1800 {
		t.Fatalf("totals: %v", totals)
	}
}
