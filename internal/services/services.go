// Package services implements the API surface Wails binds to the frontend.
// Every service is a thin facade over the store, adding validation, streak
// math and export helpers.
package services

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"mikuchrono/internal/applog"
	"mikuchrono/internal/models"
	"mikuchrono/internal/store"
)

// ActivityService manages the activity catalog.
type ActivityService struct {
	Store *store.Store
	// Emit, when set (wired in main.go), broadcasts the timer:stopped event
	// after a delete closed the activity's running timer so every window
	// refreshes. Nil in tests.
	Emit func(event string)
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

// StopAndDelete stops the activity's running timer (recording its session)
// and deletes the activity with all of its entries in one atomic operation.
// entry is nil when no timer was running or the session was too short to
// record. Broadcasts timer:stopped only when a running timer was closed.
func (s *ActivityService) StopAndDelete(id int64) (*models.Entry, error) {
	entry, stopped, err := s.Store.StopAndDeleteActivity(id, time.Now())
	if err != nil {
		return entry, err
	}
	if stopped && s.Emit != nil {
		s.Emit(EventTimerStopped)
	}
	return entry, nil
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

// cacheTTL bounds how long CachedState projects elapsed seconds from a
// snapshot before re-reading the store.
const cacheTTL = 30 * time.Second

// TimerService owns the at-most-one running timer.
type TimerService struct {
	Store *store.Store
	// Emit, when set (wired in main.go), broadcasts the existing app-wide
	// timer events after each successful state change so every window
	// (floating ball, tray-linked views) refreshes immediately. Nil in tests.
	Emit func(event string)

	// mu guards cached, the last state read from the store. The tray state
	// pump projects elapsed seconds from this snapshot once per second
	// instead of re-reading SQLite every tick (the store keeps a single
	// serialized connection, so that polling competed with every real
	// query). Expired or dropped snapshots fall back to one read.
	mu     sync.Mutex
	cached *cachedTimerState
}

// cachedTimerState is a store snapshot plus the local time it was taken;
// ticks project from the instant.
type cachedTimerState struct {
	st models.TimerState
	at time.Time
}

// remember installs st as the projection baseline.
func (s *TimerService) remember(st models.TimerState, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cached = &cachedTimerState{st: st, at: at}
}

// Invalidate drops the cached state so the next CachedState/GetState
// re-reads the store. Wired (via the Emit wrapper in main.go) to
// timer:stopped broadcasts that originate outside this service — a
// settings-page clear or a stop-and-delete — to keep the projection honest.
func (s *TimerService) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cached = nil
}

// GetState returns the current timer state (running or idle).
func (s *TimerService) GetState() (models.TimerState, error) {
	now := time.Now()
	st, err := s.Store.GetTimerState(now)
	if err != nil {
		return st, err
	}
	s.remember(st, now)
	return st, nil
}

// CachedState returns the current timer state without touching SQLite on
// the hot path: while the snapshot is fresh the elapsed seconds are
// projected from it (the same local-projection trick the frontend uses);
// a missing or expired snapshot falls back to one store read.
func (s *TimerService) CachedState(now time.Time) (models.TimerState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached == nil || now.Sub(s.cached.at) >= cacheTTL {
		st, err := s.Store.GetTimerState(now)
		if err != nil {
			return st, err
		}
		s.cached = &cachedTimerState{st: st, at: now}
		return st, nil
	}
	st := s.cached.st
	if st.Running {
		st.ElapsedSeconds += elapsedSince(s.cached.at, now)
		st.SessionElapsedSeconds += elapsedSince(s.cached.at, now)
	}
	return st, nil
}

// elapsedSince renders the whole seconds from t to now, clamped to zero: a
// system clock set back between the snapshot and now must not project a
// negative elapsed time into the tray/UI state.
func elapsedSince(t, now time.Time) int64 {
	if d := int64(now.Sub(t).Seconds()); d > 0 {
		return d
	}
	return 0
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
	s.remember(current, now)
	if current.Running && current.ActivityID == activityID {
		return current, nil
	}
	st, err := s.Store.StartTimer(activityID, now)
	if err != nil {
		return st, err
	}
	// Fill in display fields for the newly started timer. A read failure must
	// not undo the (already committed) start: degrade to empty display fields
	// — the UI falls back to its generic "计时中" text — instead of returning
	// an error that would make the frontend show idle while the timer runs.
	if a, err := s.Store.GetActivity(activityID); err == nil {
		st.ActivityName, st.ActivityColor = a.Name, a.Color
	} else {
		applog.Printf("timer start: load activity %d: %v", activityID, err)
	}
	s.remember(st, now)
	if s.Emit != nil {
		s.Emit(EventTimerStarted)
	}
	return st, nil
}

// StartLast resumes the timer for the most recently used activity, carrying
// the accumulated seconds of its earlier sessions forward. It errors when no
// timer has ever been started (no chain) or the chained activity vanished.
func (s *TimerService) StartLast() (models.TimerState, error) {
	st, err := s.GetState()
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
	// The idle state (last-activity fields) is unknown without a read; drop
	// the snapshot so the next CachedState/GetState re-reads once.
	s.Invalidate()
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
	// One grouped query feeds both the all-time totals and the streak day
	// sets (the SUM decomposes across days; the day keys are the sets). The
	// overview reloads on each mount and on every timer start/stop, so this
	// used to be two separate full-table aggregates.
	dayTotals, err := s.Store.ActivityDayTotals()
	if err != nil {
		return nil, err
	}
	totalSecs := make(map[int64]int64, len(dayTotals))
	sets := make(map[int64]map[string]bool, len(dayTotals))
	for id, days := range dayTotals {
		set := make(map[string]bool, len(days))
		var total int64
		for d, secs := range days {
			set[d] = true
			total += secs
		}
		sets[id] = set
		totalSecs[id] = total
	}
	out := make([]models.ActivityStat, 0, len(acts))
	for _, a := range acts {
		out = append(out, models.ActivityStat{
			Activity:      a,
			TodaySeconds:  todaySecs[a.ID],
			TotalSeconds:  totalSecs[a.ID],
			CurrentStreak: StreaksOf(sets[a.ID], today).Current,
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
	sets, err := s.Store.ActiveDaySets()
	if err != nil {
		return models.StreakInfo{}, err
	}
	days := map[string]bool{}
	if req.ActivityID == nil {
		for _, set := range sets {
			for d := range set {
				days[d] = true
			}
		}
	} else {
		days = sets[*req.ActivityID]
	}
	return StreaksOf(days, store.Today(time.Now())), nil
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
