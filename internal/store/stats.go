package store

import (
	"fmt"
	"time"

	"mikuchrono/internal/models"
)

// TodaySecondsByActivity sums today's recorded seconds per activity.
func (s *Store) TodaySecondsByActivity(today string) (map[int64]int64, error) {
	rows, err := s.db.Query(
		`SELECT activity_id, SUM(duration_seconds) FROM entries
		 WHERE substr(started_at,1,10) = ? GROUP BY activity_id`, today)
	if err != nil {
		return nil, fmt.Errorf("today seconds: %w", err)
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

// ActiveDays returns the set of local dates with at least one recorded entry.
// A nil activityID means "any activity".
func (s *Store) ActiveDays(activityID *int64) (map[string]bool, error) {
	q := `SELECT DISTINCT substr(started_at,1,10) FROM entries WHERE duration_seconds > 0`
	var args []any
	if activityID != nil {
		q += ` AND activity_id = ?`
		args = append(args, *activityID)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("active days: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d] = true
	}
	return out, rows.Err()
}

// DayBuckets returns per-day per-activity seconds between the two local
// dates (inclusive), for the stacked bar chart.
func (s *Store) DayBuckets(fromDate, toDate string) ([]models.DayBucket, error) {
	rows, err := s.db.Query(
		`SELECT substr(started_at,1,10), activity_id, SUM(duration_seconds) FROM entries
		 WHERE substr(started_at,1,10) BETWEEN ? AND ?
		 GROUP BY substr(started_at,1,10), activity_id ORDER BY substr(started_at,1,10)`,
		fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("day buckets: %w", err)
	}
	defer rows.Close()
	buckets := map[string]*models.DayBucket{}
	var order []string
	for rows.Next() {
		var date string
		var activityID, secs int64
		if err := rows.Scan(&date, &activityID, &secs); err != nil {
			return nil, err
		}
		b, ok := buckets[date]
		if !ok {
			b = &models.DayBucket{Date: date, ByActivity: map[string]int64{}}
			buckets[date] = b
			order = append(order, date)
		}
		b.ByActivity[fmt.Sprint(activityID)] = secs
		b.Total += secs
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]models.DayBucket, 0, len(order))
	for _, d := range order {
		out = append(out, *buckets[d])
	}
	return out, nil
}

// MonthBuckets returns per-month per-activity seconds for the given local
// year ("2006" layout), only for months that have data. Used by the yearly
// view of the statistics page.
func (s *Store) MonthBuckets(year string) ([]models.MonthBucket, error) {
	rows, err := s.db.Query(
		`SELECT substr(started_at,1,7), activity_id, SUM(duration_seconds) FROM entries
		 WHERE substr(started_at,1,7) LIKE ? || '-%'
		 GROUP BY substr(started_at,1,7), activity_id ORDER BY substr(started_at,1,7)`,
		year)
	if err != nil {
		return nil, fmt.Errorf("month buckets: %w", err)
	}
	defer rows.Close()
	buckets := map[string]*models.MonthBucket{}
	var order []string
	for rows.Next() {
		var month string
		var activityID, secs int64
		if err := rows.Scan(&month, &activityID, &secs); err != nil {
			return nil, err
		}
		b, ok := buckets[month]
		if !ok {
			b = &models.MonthBucket{Month: month, ByActivity: map[string]int64{}}
			buckets[month] = b
			order = append(order, month)
		}
		b.ByActivity[fmt.Sprint(activityID)] = secs
		b.Total += secs
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]models.MonthBucket, 0, len(order))
	for _, m := range order {
		out = append(out, *buckets[m])
	}
	return out, nil
}

// ActivityTotalsBetween sums recorded seconds per activity between two local
// dates (inclusive).
func (s *Store) ActivityTotalsBetween(fromDate, toDate string) (map[int64]int64, error) {
	rows, err := s.db.Query(
		`SELECT activity_id, SUM(duration_seconds) FROM entries
		 WHERE substr(started_at,1,10) BETWEEN ? AND ?
		 GROUP BY activity_id`, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("activity totals: %w", err)
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

// Heatmap returns per-day total seconds for the last n local days ending
// today, keyed by date string.
func (s *Store) Heatmap(days int, now time.Time) (map[string]int64, error) {
	today := Today(now)
	from, err := AddDays(today, -(days - 1))
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT substr(started_at,1,10), SUM(duration_seconds) FROM entries
		 WHERE substr(started_at,1,10) >= ? AND substr(started_at,1,10) <= ?
		 GROUP BY substr(started_at,1,10)`, from, today)
	if err != nil {
		return nil, fmt.Errorf("heatmap: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var d string
		var secs int64
		if err := rows.Scan(&d, &secs); err != nil {
			return nil, err
		}
		out[d] = secs
	}
	return out, rows.Err()
}
