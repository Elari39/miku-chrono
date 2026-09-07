package store

import (
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// indexColumns returns the column list of an existing index in order.
func indexColumns(t *testing.T, s *Store, index string) []string {
	t.Helper()
	rows, err := s.db.Query(`PRAGMA index_info(` + index + `)`)
	if err != nil {
		t.Fatalf("index_info(%s): %v", index, err)
	}
	defer rows.Close()
	type col struct {
		seq  int
		name string
	}
	var cols []col
	for rows.Next() {
		var seq int
		var cid int64
		var name string
		if err := rows.Scan(&seq, &cid, &name); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, col{seq, name})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	// A missing index yields zero rows; report it as a failure here so the
	// callers can compare column lists directly.
	if len(cols) == 0 {
		t.Fatalf("index %s does not exist", index)
	}
	slices.SortFunc(cols, func(a, b col) int { return a.seq - b.seq })
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	return names
}

// TestMigrationV4CreatesQueryIndexes runs the v4 upgrade path against a
// database that predates it (indexes dropped, schema_version pinned to 3)
// and asserts both query-shape indexes exist with the exact columns the day
// loaders and the overview aggregate rely on.
func TestMigrationV4CreatesQueryIndexes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open fresh store: %v", err)
	}
	// Downgrade to the v3 shape so Open re-runs v4 on reopen. Only
	// idx_entries_ended is dropped here: v4's own "DROP INDEX IF EXISTS"
	// handles the activity local-day index it replaces.
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS idx_entries_ended`,
		`UPDATE meta SET value='3' WHERE key='schema_version'`,
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

	if got := indexColumns(t, s, "idx_entries_ended"); !slices.Equal(got, []string{"ended_at"}) {
		t.Fatalf("idx_entries_ended columns = %v", got)
	}
	if got := indexColumns(t, s, "idx_entries_activity_local_day"); !slices.Equal(got, []string{"activity_id", "local_day", "duration_seconds"}) {
		t.Fatalf("idx_entries_activity_local_day columns = %v", got)
	}
}

// TestActivityDayTotalsAggregates seeds same-day and cross-midnight entries
// and asserts the per-activity-per-day sums, the totals derived from them
// (what the removed TotalSecondsByActivity returned), and that
// ActiveDaySets is exactly the key projection. The cross-midnight fixture
// also pins the attribution convention: the whole duration lands on the
// start day, even though the day-piece loaders split it.
func TestActivityDayTotalsAggregates(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2025, 9, 5, 12, 0, 0, 0, time.Local)
	fixtures := []struct {
		act        int64
		start, end string
	}{
		// Activity 1: 3600s on 09-04, 3600s on 09-05, and a cross-midnight
		// 1200s session starting 09-03.
		{1, "2025-09-04T09:00:00+08:00", "2025-09-04T10:00:00+08:00"},
		{1, "2025-09-05T09:00:00+08:00", "2025-09-05T10:00:00+08:00"},
		{1, "2025-09-03T23:50:00+08:00", "2025-09-04T00:10:00+08:00"},
		// Activity 2: one session on 09-05.
		{2, "2025-09-05T11:00:00+08:00", "2025-09-05T12:00:00+08:00"},
	}
	for _, f := range fixtures {
		if _, err := s.CreateManualEntry(f.act, f.start, f.end, "", now); err != nil {
			t.Fatal(err)
		}
	}

	totals, err := s.ActivityDayTotals()
	if err != nil {
		t.Fatal(err)
	}
	// local_day is the start-day wall clock: the cross-midnight session's
	// whole 1200s lands on 09-03 where it started, even though the day-piece
	// loaders split those seconds across 09-03/09-04.
	if got := totals[1]["2025-09-03"]; got != 1200 {
		t.Fatalf("activity 1 on 09-03 = %d, want 1200 (start-day attribution)", got)
	}
	if got := totals[1]["2025-09-04"]; got != 3600 {
		t.Fatalf("activity 1 on 09-04 = %d, want 3600", got)
	}
	if got := totals[1]["2025-09-05"]; got != 3600 {
		t.Fatalf("activity 1 on 09-05 = %d, want 3600", got)
	}
	if got := totals[2]["2025-09-05"]; got != 3600 {
		t.Fatalf("activity 2 on 09-05 = %d, want 3600", got)
	}

	// All-time totals decompose across days.
	wantTotal := map[int64]int64{1: 3600 + 3600 + 1200, 2: 3600}
	for act, want := range wantTotal {
		var sum int64
		for _, secs := range totals[act] {
			sum += secs
		}
		if sum != want {
			t.Fatalf("activity %d total = %d, want %d", act, sum, want)
		}
	}

	sets, err := s.ActiveDaySets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets[1]) != 3 || !sets[1]["2025-09-03"] || !sets[1]["2025-09-04"] || !sets[1]["2025-09-05"] {
		t.Fatalf("activity 1 sets = %v", sets[1])
	}
	if len(sets[2]) != 1 || !sets[2]["2025-09-05"] {
		t.Fatalf("activity 2 sets = %v", sets[2])
	}
}
