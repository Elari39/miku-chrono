package services

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mikuchrono/internal/models"
	"mikuchrono/internal/store"
)

// newGoalFixture opens a store with one activity whose daily goal is one
// minute and records two minutes for it today (via the regular write path,
// so local_day attribution matches production).
func newGoalFixture(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	act, err := st.CreateActivity(models.Activity{Name: "学习", DailyGoalMinutes: 1})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	now := time.Now()
	if _, err := st.CreateManualEntry(act.ID, store.FormatTime(now.Add(-2*time.Minute)), store.FormatTime(now), "", now); err != nil {
		t.Fatalf("record today's minutes: %v", err)
	}
	return st
}

func TestGoalNotifierNotifiesOncePerDay(t *testing.T) {
	st := newGoalFixture(t)
	var got []string
	g := NewGoalNotifier(st)
	g.Notify = func(title, body string) { got = append(got, title+"|"+body) }

	g.check()
	if len(got) != 1 || !strings.Contains(got[0], "目标达成") {
		t.Fatalf("first check must notify once, got %v", got)
	}
	// The second check on the same day dedupes via the meta key.
	g.check()
	if len(got) != 1 {
		t.Fatalf("same-day recheck must dedupe, got %v", got)
	}
}

func TestGoalNotifierRespectsSwitch(t *testing.T) {
	st := newGoalFixture(t)
	if err := st.SetSetting(keyGoalNotify, "0"); err != nil {
		t.Fatal(err)
	}
	var got []string
	g := NewGoalNotifier(st)
	g.Notify = func(title, body string) { got = append(got, title) }
	g.check()
	if len(got) != 0 {
		t.Fatalf("disabled notifier must stay silent, got %v", got)
	}
}

func TestGoalNotifierCleansStaleKeys(t *testing.T) {
	st := newGoalFixture(t)
	stale := store.GoalNotifiedKeyPrefix + "2020-01-01_999"
	if err := st.SetSetting(stale, "1"); err != nil {
		t.Fatal(err)
	}
	g := NewGoalNotifier(st)
	g.Notify = func(string, string) {}
	g.check()
	if _, found, err := st.GetSetting(stale); err != nil || found {
		t.Fatalf("stale key must be swept, found=%v err=%v", found, err)
	}
}

func TestOverviewStreaksPerActivity(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := &StatsService{Store: st}

	now := time.Now()
	today := store.Today(now)
	// Yesterday: a fixed morning hour, always in the past (machine-local, so
	// the day attribution holds in any timezone). Today: anchored to now so
	// the no-future rule holds at any hour of the day.
	y9 := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local).AddDate(0, 0, -1)
	if _, err := st.CreateManualEntry(1, y9.Format(time.RFC3339), y9.Add(time.Hour).Format(time.RFC3339), "", now); err != nil {
		t.Fatalf("record yesterday: %v", err)
	}
	todayEntry, err := st.CreateManualEntry(1, store.FormatTime(now.Add(-time.Minute)), store.FormatTime(now), "", now)
	if err != nil {
		t.Fatalf("record today: %v", err)
	}
	// Where the "today" entry actually landed (a run in the first minutes
	// after midnight writes it to yesterday instead) decides the expected
	// streak: two consecutive days, or just yesterday's single day.
	landed, err := store.ParseTime(todayEntry.StartedAt)
	if err != nil {
		t.Fatal(err)
	}
	want := 1
	if store.Today(landed.Local()) == today {
		want = 2
	}

	overview, err := s.Overview()
	if err != nil {
		t.Fatal(err)
	}
	wantStreak := map[int64]int{1: want, 2: 0, 3: 0}
	for _, stat := range overview {
		if stat.CurrentStreak != wantStreak[stat.Activity.ID] {
			t.Fatalf("activity %d streak = %d, want %d", stat.Activity.ID, stat.CurrentStreak, wantStreak[stat.Activity.ID])
		}
	}

	// The stats-page streaks endpoint shares the same single grouped query.
	overall, err := s.Streaks(models.StreakRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if overall.Current != want {
		t.Fatalf("overall streak = %d, want %d", overall.Current, want)
	}
}
