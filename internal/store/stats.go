package store

import (
	"fmt"
	"time"

	"mikuchrono/internal/models"
)

// dayPiece is one contiguous piece of an entry's duration lying inside one
// local day.
type dayPiece struct {
	activityID int64
	date       string // "YYYY-MM-DD" in the entry's own wall-clock offset
	secs       int64
}

// splitByLocalDay slices [start, end) into per-local-day pieces. Day
// boundaries are midnights in the passed times' location — callers convert
// stored UTC timestamps to the viewer's local zone first (start.Local()),
// so the pieces land on the calendar days the user sees. Stored
// timestamps are whole seconds (RFC3339 rendering drops sub-second parts)
// and so are the boundaries, hence the integer-second split never loses or
// gains a second and the pieces sum exactly to end.Sub(start).
func splitByLocalDay(start, end time.Time) []dayPiece {
	if !end.After(start) {
		return nil
	}
	var out []dayPiece
	cur := start
	for cur.Before(end) {
		y, m, d := cur.Date()
		next := time.Date(y, m, d+1, 0, 0, 0, 0, cur.Location())
		if next.After(end) {
			next = end
		}
		secs := int64(next.Sub(cur).Seconds())
		if secs > 0 {
			out = append(out, dayPiece{date: cur.Format("2006-01-02"), secs: secs})
		}
		cur = next
	}
	return out
}

// startOfDay returns date's local midnight rendered as a UTC RFC3339 string
// — the same UTC rendering entries are stored in (schema v3), so the string
// comparisons used by the overlap filters below stay in sync with
// timestamp order regardless of the machine's timezone changes.
func startOfDay(date string) (string, error) {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return "", fmt.Errorf("parse date %q: %w", date, err)
	}
	return FormatTime(t), nil
}

// loadDayPieces queries every entry half-open-overlapping the inclusive
// local date range [fromDate, toDate] (started_at < the day after toDate AND
// ended_at > fromDate's midnight) and splits each into per-day pieces.
// Pieces outside the range are dropped. Time-ordered output keeps a
// first-seen aggregation stable.
func (s *Store) loadDayPieces(fromDate, toDate string) ([]dayPiece, error) {
	fromStart, err := startOfDay(fromDate)
	if err != nil {
		return nil, fmt.Errorf("day pieces: %w", err)
	}
	next, err := AddDays(toDate, 1)
	if err != nil {
		return nil, fmt.Errorf("day pieces: %w", err)
	}
	toExcl, err := startOfDay(next)
	if err != nil {
		return nil, fmt.Errorf("day pieces: %w", err)
	}
	rows, err := s.db.Query(
		`SELECT activity_id, started_at, ended_at FROM entries
		 WHERE ended_at > ? AND started_at < ?
		 ORDER BY started_at, id`, fromStart, toExcl)
	if err != nil {
		return nil, fmt.Errorf("day pieces: %w", err)
	}
	defer rows.Close()
	var out []dayPiece
	for rows.Next() {
		var act int64
		var startS, endS string
		if err := rows.Scan(&act, &startS, &endS); err != nil {
			return nil, err
		}
		start, err := ParseTime(startS)
		if err != nil {
			return nil, fmt.Errorf("day pieces: parse started_at: %w", err)
		}
		end, err := ParseTime(endS)
		if err != nil {
			return nil, fmt.Errorf("day pieces: parse ended_at: %w", err)
		}
		// Stored timestamps are UTC (schema v3); the split buckets by the
		// viewer's current local calendar, matching how from/to dates are
		// produced on this machine.
		for _, p := range splitByLocalDay(start.Local(), end.Local()) {
			if p.date >= fromDate && p.date <= toDate {
				out = append(out, dayPiece{activityID: act, date: p.date, secs: p.secs})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// TodaySecondsByActivity sums today's recorded seconds per activity. Cross-
// midnight entries contribute only the part that actually falls on today.
func (s *Store) TodaySecondsByActivity(today string) (map[int64]int64, error) {
	pieces, err := s.loadDayPieces(today, today)
	if err != nil {
		return nil, fmt.Errorf("today seconds: %w", err)
	}
	out := map[int64]int64{}
	for _, p := range pieces {
		out[p.activityID] += p.secs
	}
	return out, nil
}

// TotalSecondsByActivity sums all recorded seconds per activity.
func (s *Store) TotalSecondsByActivity() (map[int64]int64, error) {
	rows, err := s.db.Query(`SELECT activity_id, SUM(duration_seconds) FROM entries GROUP BY activity_id`)
	if err != nil {
		return nil, fmt.Errorf("total seconds: %w", err)
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var id, secs int64
		if err := rows.Scan(&id, &secs); err != nil {
			return nil, err
		}
		out[id] = secs
	}
	return out, rows.Err()
}

// ActiveDaySets returns, per activity, the set of local start days on which
// at least one record started. A cross-midnight entry belongs to the day it
// started on (the project's accounting convention), so local_day — the
// writer's start-day wall clock, maintained since schema v3 — is the
// attribution column. One grouped query over the covering index replaces
// the previous per-activity full-table scans (the overview page needs every
// activity's set on each reload).
func (s *Store) ActiveDaySets() (map[int64]map[string]bool, error) {
	rows, err := s.db.Query(
		`SELECT activity_id, local_day FROM entries
		 WHERE duration_seconds > 0 GROUP BY activity_id, local_day`)
	if err != nil {
		return nil, fmt.Errorf("active day sets: %w", err)
	}
	defer rows.Close()
	out := map[int64]map[string]bool{}
	for rows.Next() {
		var id int64
		var day string
		if err := rows.Scan(&id, &day); err != nil {
			return nil, err
		}
		set := out[id]
		if set == nil {
			set = map[string]bool{}
			out[id] = set
		}
		set[day] = true
	}
	return out, rows.Err()
}

// DayBuckets returns per-day per-activity seconds between the two local
// dates (inclusive), for the stacked bar chart. Cross-midnight entries are
// split across the days they touch.
func (s *Store) DayBuckets(fromDate, toDate string) ([]models.DayBucket, error) {
	pieces, err := s.loadDayPieces(fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("day buckets: %w", err)
	}
	buckets := map[string]*models.DayBucket{}
	var order []string
	for _, p := range pieces {
		b, ok := buckets[p.date]
		if !ok {
			b = &models.DayBucket{Date: p.date, ByActivity: map[string]int64{}}
			buckets[p.date] = b
			order = append(order, p.date)
		}
		b.ByActivity[fmt.Sprint(p.activityID)] += p.secs
		b.Total += p.secs
	}
	out := make([]models.DayBucket, 0, len(order))
	for _, d := range order {
		out = append(out, *buckets[d])
	}
	return out, nil
}

// MonthBuckets returns per-month per-activity seconds for the given local
// year ("2006" layout), only for months that have data. Used by the yearly
// view of the statistics page. Cross-month entries are split across the
// months they touch.
func (s *Store) MonthBuckets(year string) ([]models.MonthBucket, error) {
	pieces, err := s.loadDayPieces(year+"-01-01", year+"-12-31")
	if err != nil {
		return nil, fmt.Errorf("month buckets: %w", err)
	}
	buckets := map[string]*models.MonthBucket{}
	var order []string
	for _, p := range pieces {
		month := p.date[:7]
		b, ok := buckets[month]
		if !ok {
			b = &models.MonthBucket{Month: month, ByActivity: map[string]int64{}}
			buckets[month] = b
			order = append(order, month)
		}
		b.ByActivity[fmt.Sprint(p.activityID)] += p.secs
		b.Total += p.secs
	}
	out := make([]models.MonthBucket, 0, len(order))
	for _, m := range order {
		out = append(out, *buckets[m])
	}
	return out, nil
}

// ActivityTotalsBetween sums recorded seconds per activity between two
// local dates (inclusive). Only the parts of entries that fall inside the
// range count.
func (s *Store) ActivityTotalsBetween(fromDate, toDate string) (map[int64]int64, error) {
	pieces, err := s.loadDayPieces(fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("activity totals: %w", err)
	}
	out := map[int64]int64{}
	for _, p := range pieces {
		out[p.activityID] += p.secs
	}
	return out, nil
}

// Heatmap returns per-day total seconds for the last n local days ending
// today, keyed by date string.
func (s *Store) Heatmap(days int, now time.Time) (map[string]int64, error) {
	today := Today(now)
	from, err := AddDays(today, -(days - 1))
	if err != nil {
		return nil, err
	}
	pieces, err := s.loadDayPieces(from, today)
	if err != nil {
		return nil, fmt.Errorf("heatmap: %w", err)
	}
	out := map[string]int64{}
	for _, p := range pieces {
		out[p.date] += p.secs
	}
	return out, nil
}
