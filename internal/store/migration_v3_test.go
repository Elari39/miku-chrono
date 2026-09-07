package store

import (
	"path/filepath"
	"testing"
	"time"

	"mikuchrono/internal/models"
)

// TestMigrationV3RewritesTimestampsToUTC builds a real database, then
// downgrades it in place to simulate a legacy v2 install: the local_day
// column (added by v3) is dropped again, timestamps are rewritten back to
// local-offset strings — including a mixed-offset row, as after a trip —
// and schema_version is pinned to 2. Reopening through Open then re-runs
// v3 against the exact production schema. It asserts the UTC rewrite, the
// local_day backfill preserving each row's wall-clock start day, and that
// day filtering and streaks keep working on the migrated data.
func TestMigrationV3RewritesTimestampsToUTC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open fresh store: %v", err)
	}
	// Two sessions through the regular write path (stored as UTC):
	// an evening session crossing midnight, and one with a +09:00 wall
	// clock — the travel case the old string comparisons broke on:
	// 01:00+09:00 is 2025-09-02T16:00:00Z with wall-clock day 09-03.
	if _, err := s.CreateManualEntry(1, "2025-09-01T23:50:00+08:00", "2025-09-02T00:10:00+08:00", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateManualEntry(1, "2025-09-03T01:00:00+09:00", "2025-09-03T02:00:00+09:00", "", time.Now()); err != nil {
		t.Fatal(err)
	}

	// Downgrade to the v2 data shape: UTC strings become local-offset
	// strings again and the v3 column plus its indexes disappear.
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS idx_entries_activity_local_day`,
		`DROP INDEX IF EXISTS idx_entries_local_day`,
		`DROP INDEX IF EXISTS idx_entries_ended`,
		`ALTER TABLE entries DROP COLUMN local_day`,
		`UPDATE entries SET started_at='2025-09-01T23:50:00+08:00', ended_at='2025-09-02T00:10:00+08:00' WHERE id=1`,
		`UPDATE entries SET started_at='2025-09-03T01:00:00+09:00', ended_at='2025-09-03T02:00:00+09:00' WHERE id=2`,
		`UPDATE entries SET created_at='2025-09-02T00:10:00+08:00', updated_at='2025-09-02T00:10:00+08:00' WHERE id=1`,
		`UPDATE entries SET created_at='2025-09-03T02:00:00+09:00', updated_at='2025-09-03T02:00:00+09:00' WHERE id=2`,
		`UPDATE meta SET value='2' WHERE key='schema_version'`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatalf("downgrade %q: %v", stmt, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen with migration: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Timestamps are stored as their UTC instants again.
	assertInstant := func(label, got, want string) {
		t.Helper()
		g, err := time.Parse(time.RFC3339, got)
		if err != nil {
			t.Fatalf("%s: parse %q: %v", label, got, err)
		}
		w, err := time.Parse(time.RFC3339, want)
		if err != nil {
			t.Fatalf("%s: parse want %q: %v", label, want, err)
		}
		if !g.Equal(w) {
			t.Fatalf("%s = %s, want %s", label, got, want)
		}
	}
	e1, err := s.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	assertInstant("entry1 started_at", e1.StartedAt, "2025-09-01T15:50:00Z")
	assertInstant("entry1 ended_at", e1.EndedAt, "2025-09-01T16:10:00Z")
	e2, err := s.GetEntry(2)
	if err != nil {
		t.Fatal(err)
	}
	assertInstant("entry2 started_at", e2.StartedAt, "2025-09-02T16:00:00Z")

	// local_day preserves the wall-clock start day of each record, mixed
	// offsets included, and the day filter keys on it.
	for _, c := range []struct {
		from, to string
		want     int64
	}{
		{"2025-09-01", "2025-09-01", 1},
		{"2025-09-02", "2025-09-02", 0}, // the +09:00 record must NOT land here
		{"2025-09-03", "2025-09-03", 1},
	} {
		list, err := s.ListEntries(models.EntryFilter{FromDate: c.from, ToDate: c.to})
		if err != nil {
			t.Fatal(err)
		}
		if list.Total != c.want {
			t.Fatalf("filter %s..%s: total = %d, want %d", c.from, c.to, list.Total, c.want)
		}
	}

	// Streak day sets match the same attribution.
	sets, err := s.ActiveDaySets()
	if err != nil {
		t.Fatal(err)
	}
	if !sets[1]["2025-09-01"] || !sets[1]["2025-09-03"] || sets[1]["2025-09-02"] {
		t.Fatalf("active day sets: %v", sets[1])
	}

	// Aggregates stay consistent: both entries fall inside a wide range.
	buckets, err := s.DayBuckets("2025-09-01", "2025-09-30")
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, b := range buckets {
		total += b.Total
	}
	if total != 4800 {
		t.Fatalf("day bucket total = %d, want 4800", total)
	}
}

func TestActiveDaySetsPerActivity(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2025, 9, 5, 12, 0, 0, 0, time.Local)
	for _, f := range []struct {
		act        int64
		start, end string
	}{
		{1, "2025-09-04T09:00:00+08:00", "2025-09-04T10:00:00+08:00"},
		{1, "2025-09-05T09:00:00+08:00", "2025-09-05T10:00:00+08:00"},
		{2, "2025-09-05T11:00:00+08:00", "2025-09-05T12:00:00+08:00"},
	} {
		if _, err := s.CreateManualEntry(f.act, f.start, f.end, "", now); err != nil {
			t.Fatal(err)
		}
	}

	sets, err := s.ActiveDaySets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets[1]) != 2 || !sets[1]["2025-09-04"] || !sets[1]["2025-09-05"] {
		t.Fatalf("activity 1 sets: %v", sets[1])
	}
	if len(sets[2]) != 1 || !sets[2]["2025-09-05"] {
		t.Fatalf("activity 2 sets: %v", sets[2])
	}
}

func TestHeatmapSumsPerDay(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	today := Today(now)
	day := func(offset int, hour, min int) time.Time {
		t.Helper()
		d, err := AddDays(today, offset)
		if err != nil {
			t.Fatal(err)
		}
		base, err := time.ParseInLocation("2006-01-02", d, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return base.Add(time.Duration(hour)*time.Hour + time.Duration(min)*time.Minute)
	}
	// All fixtures lie fully in the past so CreateManualEntry's no-future
	// rule holds regardless of when the test runs: an hour yesterday, 90
	// minutes three days ago, and a cross-midnight session ending yesterday.
	yStart := day(-1, 10, 0)
	if _, err := s.CreateManualEntry(1, FormatTime(yStart), FormatTime(yStart.Add(time.Hour)), "", now); err != nil {
		t.Fatal(err)
	}
	oldStart := day(-3, 8, 0)
	if _, err := s.CreateManualEntry(1, FormatTime(oldStart), FormatTime(oldStart.Add(90*time.Minute)), "", now); err != nil {
		t.Fatal(err)
	}
	cmStart := day(-2, 23, 50)
	cmEnd := day(-1, 0, 10) // yesterday 00:10
	if _, err := s.CreateManualEntry(1, FormatTime(cmStart), FormatTime(cmEnd), "", now); err != nil {
		t.Fatal(err)
	}

	heat, err := s.Heatmap(7, now)
	if err != nil {
		t.Fatal(err)
	}
	yDay, _ := AddDays(today, -1)
	if heat[yDay] != 4200 { // the hour plus the cross-midnight tail
		t.Fatalf("yesterday = %d, want 4200", heat[yDay])
	}
	oldDay, _ := AddDays(today, -3)
	if heat[oldDay] != 5400 {
		t.Fatalf("three days ago = %d, want 5400", heat[oldDay])
	}
	// The cross-midnight session must have contributed its pre-midnight
	// piece to two days ago as well.
	cmDay, _ := AddDays(today, -2)
	if heat[cmDay] == 0 {
		t.Fatalf("two days ago should hold the pre-midnight piece: %v", heat)
	}
	var total int64
	for _, v := range heat {
		total += v
	}
	if total != 3600+5400+1200 {
		t.Fatalf("heatmap total = %d, want 10200", total)
	}
}
