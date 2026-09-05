// Package models defines the data structures shared between the Go backend
// and the TypeScript frontend. The Wails binding generator turns these into
// TypeScript interfaces under frontend/bindings.
package models

// Category is a user-defined group that activities belong to. Activities
// without a category are shown as "未分类".
type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Activity is a trackable activity the user can check in to. CategoryID is
// nil when the activity belongs to no category (未分类).
type Activity struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Color            string `json:"color"`
	Icon             string `json:"icon"`
	CategoryID       *int64 `json:"categoryId,omitempty"`
	DailyGoalMinutes int    `json:"dailyGoalMinutes"`
	SortOrder        int    `json:"sortOrder"`
	Archived         bool   `json:"archived"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// Entry is one completed check-in (a start/end pair). It may come from the
// live timer (source "timer") or from manual backfill (source "manual").
type Entry struct {
	ID              int64  `json:"id"`
	ActivityID      int64  `json:"activityId"`
	ActivityName    string `json:"activityName"`
	ActivityColor   string `json:"activityColor"`
	StartedAt       string `json:"startedAt"`
	EndedAt         string `json:"endedAt"`
	DurationSeconds int64  `json:"durationSeconds"`
	Note            string `json:"note"`
	Source          string `json:"source"`
}

// TimerState describes the at-most-one running timer. The Last* fields are
// populated only while idle: they carry the paused timer chain (the most
// recently used activity plus its accumulated seconds) so the UI can show
// the previous session and offer to resume it.
type TimerState struct {
	Running        bool   `json:"running"`
	ActivityID     int64  `json:"activityId"`
	ActivityName   string `json:"activityName"`
	ActivityColor  string `json:"activityColor"`
	StartedAt      string `json:"startedAt"`
	ElapsedSeconds int64  `json:"elapsedSeconds"`

	LastActivityID     int64  `json:"lastActivityId"`
	LastActivityName   string `json:"lastActivityName"`
	LastActivityColor  string `json:"lastActivityColor"`
	LastElapsedSeconds int64  `json:"lastElapsedSeconds"`
}

// EntryFilter narrows the entry list. Dates are local "YYYY-MM-DD" strings;
// empty strings mean "no bound".
type EntryFilter struct {
	ActivityID *int64 `json:"activityId"`
	FromDate   string `json:"fromDate"`
	ToDate     string `json:"toDate"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
}

// EntryList is a page of entries plus the total count for the filter.
type EntryList struct {
	Items    []Entry `json:"items"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
}

// ActivityStat is a per-activity snapshot for the check-in page.
type ActivityStat struct {
	Activity      Activity `json:"activity"`
	TodaySeconds  int64    `json:"todaySeconds"`
	TotalSeconds  int64    `json:"totalSeconds"`
	CurrentStreak int      `json:"currentStreak"`
}

// DayBucket aggregates one local day for the stacked bar chart.
type DayBucket struct {
	Date       string           `json:"date"`
	Total      int64            `json:"total"`
	ByActivity map[string]int64 `json:"byActivity"`
}

// MonthBucket aggregates one local month ("YYYY-MM") for the yearly view
// of the statistics page. Same shape as DayBucket, month granularity.
type MonthBucket struct {
	Month      string           `json:"month"`
	Total      int64            `json:"total"`
	ByActivity map[string]int64 `json:"byActivity"`
}

// ActivityTotal is the total recorded time of one activity in a range.
type ActivityTotal struct {
	Activity Activity `json:"activity"`
	Seconds  int64    `json:"seconds"`
}

// StreakInfo carries current and longest consecutive check-in days.
type StreakInfo struct {
	Current int `json:"current"`
	Longest int `json:"longest"`
}

// StreakRequest selects which streak to compute; nil activity means overall.
type StreakRequest struct {
	ActivityID *int64 `json:"activityId"`
}

// ExportData is the full-database JSON export payload.
type ExportData struct {
	Version    int        `json:"version"`
	ExportedAt string     `json:"exportedAt"`
	Categories []Category `json:"categories"`
	Activities []Activity `json:"activities"`
	Entries    []Entry    `json:"entries"`
}

// BallPosition is the persisted floating-ball window position. Set is false
// when no position has ever been saved (the frontend then snaps to a default).
type BallPosition struct {
	X   int  `json:"x"`
	Y   int  `json:"y"`
	Set bool `json:"set"`
}
