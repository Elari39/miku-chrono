// Package services implements the API surface Wails binds to the frontend.
// Every service is a thin facade over the store, adding validation, streak
// math and export helpers.
package services

import (
	"fmt"
	"sort"
	"time"

	"mikuchrono/internal/models"
	"mikuchrono/internal/store"
)

// ActivityService manages the activity catalog.
type ActivityService struct {
	Store *store.Store
}

// List returns activities, optionally including archived ones.
func (s *ActivityService) List(includeArchived bool) ([]models.Activity, error) {
	return s.Store.ListActivities(includeArchived)
}

// Create adds a new activity.
func (s *ActivityService) Create(a models.Activity) (models.Activity, error) {
	return s.Store.CreateActivity(a)
}

// Update modifies an existing activity.
func (s *ActivityService) Update(a models.Activity) error {
	return s.Store.UpdateActivity(a)
}

// SetArchived archives or restores an activity.
func (s *ActivityService) SetArchived(id int64, archived bool) error {
	a, err := s.Store.GetActivity(id)
	if err != nil {
		return err
	}
	a.Archived = archived
	return s.Store.UpdateActivity(a)
}

// Delete removes an activity together with all of its entries.
func (s *ActivityService) Delete(id int64) error {
	return s.Store.DeleteActivity(id)
}

// CategoryService manages the user-defined categories activities belong to.
type CategoryService struct {
	Store *store.Store
}

// List returns every category in display order.
func (s *CategoryService) List() ([]models.Category, error) {
	return s.Store.ListCategories()
}

// Create adds a new category.
func (s *CategoryService) Create(c models.Category) (models.Category, error) {
	return s.Store.CreateCategory(c)
}

// Update modifies an existing category.
func (s *CategoryService) Update(c models.Category) error {
	return s.Store.UpdateCategory(c)
}

// Delete removes a category; its activities become uncategorized.
func (s *CategoryService) Delete(id int64) error {
	return s.Store.DeleteCategory(id)
}

// TimerService owns the at-most-one running timer.
type TimerService struct {
	Store *store.Store
	// Emit, when set (wired in main.go), broadcasts the existing app-wide
	// timer events after each successful state change so every window
	// (floating ball, tray-linked views) refreshes immediately. Nil in tests.
	Emit func(event string)
}

// GetState returns the current timer state (running or idle).
func (s *TimerService) GetState() (models.TimerState, error) {
	return s.Store.GetTimerState(time.Now())
}

// Start begins timing the given activity, closing any running timer first
// (mutual exclusion). Starting the already-running activity is a no-op;
// starting an idle activity that matches the paused chain resumes it.
// Broadcasts timer:started only when the state actually changed.
func (s *TimerService) Start(activityID int64) (models.TimerState, error) {
	now := time.Now()
	current, err := s.Store.GetTimerState(now)
	if err != nil {
		return current, err
	}
	if current.Running && current.ActivityID == activityID {
		return current, nil
	}
	st, err := s.Store.StartTimer(activityID, now)
	if err != nil {
		return st, err
	}
	// Fill in display fields for the newly started timer.
	a, err := s.Store.GetActivity(activityID)
	if err != nil {
		return st, err
	}
	st.ActivityName, st.ActivityColor = a.Name, a.Color
	if s.Emit != nil {
		s.Emit(EventTimerStarted)
	}
	return st, nil
}

// StartLast resumes the timer for the most recently used activity, carrying
// the accumulated seconds of its earlier sessions forward. It errors when no
// timer has ever been started (no chain) or the chained activity vanished.
func (s *TimerService) StartLast() (models.TimerState, error) {
	st, err := s.Store.GetTimerState(time.Now())
	if err != nil {
		return st, err
	}
	if st.Running {
		return st, nil // already running — nothing to resume
	}
	if st.LastActivityID == 0 {
		return st, fmt.Errorf("还没有计时记录，先去打卡页开始一次计时吧")
	}
	return s.Start(st.LastActivityID)
}

// Stop ends the running timer and records the entry. Returns nil when the
// session was too short to record. Broadcasts timer:stopped on success —
// including discarded short sessions, since the state still became idle.
func (s *TimerService) Stop() (*models.Entry, error) {
	entry, err := s.Store.StopTimer(time.Now())
	if err != nil {
		return entry, err
	}
	if s.Emit != nil {
		s.Emit(EventTimerStopped)
	}
	return entry, nil
}

// EntryService handles listing and manual backfill of entries.
type EntryService struct {
	Store *store.Store
}

// List returns a filtered, paginated page of entries.
func (s *EntryService) List(f models.EntryFilter) (models.EntryList, error) {
	return s.Store.ListEntries(f)
}

// CreateManual records a backfilled entry.
func (s *EntryService) CreateManual(activityID int64, startedAt, endedAt, note string) (models.Entry, error) {
	return s.Store.CreateManualEntry(activityID, startedAt, endedAt, note, time.Now())
}

// Update modifies an existing entry.
func (s *EntryService) Update(e models.Entry) error {
	return s.Store.UpdateEntry(e, time.Now())
}

// Delete removes an entry.
func (s *EntryService) Delete(id int64) error {
	return s.Store.DeleteEntry(id)
}

// StatsService computes the aggregates shown on the statistics page.
type StatsService struct {
	Store *store.Store
}

// Overview returns per-activity today totals, all-time totals and current
// streaks for the check-in page.
func (s *StatsService) Overview() ([]models.ActivityStat, error) {
	acts, err := s.Store.ListActivities(false)
	if err != nil {
		return nil, err
	}
	today := store.Today(time.Now())
	todaySecs, err := s.Store.TodaySecondsByActivity(today)
	if err != nil {
		return nil, err
	}
	totalSecs, err := s.Store.TotalSecondsByActivity()
	if err != nil {
		return nil, err
	}
	out := make([]models.ActivityStat, 0, len(acts))
	for _, a := range acts {
		id := a.ID
		streak, err := computeStreak(s.Store, &id)
		if err != nil {
			return nil, err
		}
		out = append(out, models.ActivityStat{
			Activity:      a,
			TodaySeconds:  todaySecs[a.ID],
			TotalSeconds:  totalSecs[a.ID],
			CurrentStreak: streak.Current,
		})
	}
	return out, nil
}

// DailyStacked returns per-day totals (with per-activity buckets) between
// two local dates inclusive.
func (s *StatsService) DailyStacked(fromDate, toDate string) ([]models.DayBucket, error) {
	return s.Store.DayBuckets(fromDate, toDate)
}

// ActivityTotals returns per-activity totals between two local dates.
func (s *StatsService) ActivityTotals(fromDate, toDate string) ([]models.ActivityTotal, error) {
	acts, err := s.Store.ListActivities(true)
	if err != nil {
		return nil, err
	}
	totals, err := s.Store.ActivityTotalsBetween(fromDate, toDate)
	if err != nil {
		return nil, err
	}
	out := make([]models.ActivityTotal, 0, len(acts))
	for _, a := range acts {
		if secs := totals[a.ID]; secs > 0 {
			out = append(out, models.ActivityTotal{Activity: a, Seconds: secs})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seconds > out[j].Seconds })
	return out, nil
}

// MonthlyStacked returns per-month totals (with per-activity buckets) for the
// given local year, used by the statistics page's yearly view. Months without
// data are simply absent; the frontend fills the empty ones.
func (s *StatsService) MonthlyStacked(year int) ([]models.MonthBucket, error) {
	if year < 1970 || year > 9999 {
		return nil, fmt.Errorf("年份超出支持范围: %d", year)
	}
	return s.Store.MonthBuckets(fmt.Sprintf("%04d", year))
}

// Streaks returns current and longest consecutive check-in days. A nil
// activity id means across all activities.
func (s *StatsService) Streaks(req models.StreakRequest) (models.StreakInfo, error) {
	return computeStreak(s.Store, req.ActivityID)
}

// Heatmap returns per-day total seconds for the last n days.
func (s *StatsService) Heatmap(days int) (map[string]int64, error) {
	if days < 7 {
		days = 7
	}
	if days > 366 {
		days = 366
	}
	return s.Store.Heatmap(days, time.Now())
}

// computeStreak calculates current/longest consecutive check-in days.
func computeStreak(s *store.Store, activityID *int64) (models.StreakInfo, error) {
	days, err := s.ActiveDays(activityID)
	if err != nil {
		return models.StreakInfo{}, err
	}
	return StreaksOf(days, store.Today(time.Now())), nil
}

// StreaksOf computes streaks from a set of active "YYYY-MM-DD" dates and a
// today string. The current streak tolerates "today not checked in yet" by
// anchoring on yesterday; longest counts every maximal run.
func StreaksOf(days map[string]bool, today string) models.StreakInfo {
	info := models.StreakInfo{}
	if len(days) == 0 {
		return info
	}

	// Longest: walk sorted dates counting maximal consecutive runs.
	dates := make([]string, 0, len(days))
	for d := range days {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	longest, run := 0, 0
	var prev time.Time
	for _, d := range dates {
		t, err := time.ParseInLocation("2006-01-02", d, time.Local)
		if err != nil {
			continue
		}
		if run > 0 && prev.AddDate(0, 0, 1).Equal(t) {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
		prev = t
	}
	info.Longest = longest

	// Current: start from today, or yesterday when today has no check-in.
	anchor, err := time.ParseInLocation("2006-01-02", today, time.Local)
	if err != nil {
		return info
	}
	if !days[today] {
		anchor = anchor.AddDate(0, 0, -1)
	}
	cur := 0
	for {
		key := anchor.Format("2006-01-02")
		if !days[key] {
			break
		}
		cur++
		anchor = anchor.AddDate(0, 0, -1)
	}
	info.Current = cur
	return info
}
