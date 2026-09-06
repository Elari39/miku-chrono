package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"mikuchrono/internal/models"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// ErrValidation is returned for user-input problems.
var ErrValidation = errors.New("validation")

func validationf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

func scanActivity(row interface{ Scan(...any) error }) (models.Activity, error) {
	var a models.Activity
	var archived int
	var categoryID sql.NullInt64
	err := row.Scan(&a.ID, &a.Name, &a.Color, &a.Icon, &categoryID, &a.DailyGoalMinutes, &a.SortOrder, &archived, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	a.Archived = archived != 0
	if categoryID.Valid {
		a.CategoryID = &categoryID.Int64
	}
	return a, nil
}

const activityCols = `id, name, color, icon, category_id, daily_goal_minutes, sort_order, archived, created_at, updated_at`

// ListActivities returns activities ordered by sort_order, then id.
// includeArchived controls whether archived ones are included.
func (s *Store) ListActivities(includeArchived bool) ([]models.Activity, error) {
	q := `SELECT ` + activityCols + ` FROM activities`
	if !includeArchived {
		q += ` WHERE archived = 0`
	}
	q += ` ORDER BY sort_order, id`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	defer rows.Close()
	var out []models.Activity
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetActivity fetches one activity by id.
func (s *Store) GetActivity(id int64) (models.Activity, error) {
	a, err := scanActivity(s.db.QueryRow(`SELECT `+activityCols+` FROM activities WHERE id = ?`, id))
	if err != nil {
		return a, fmt.Errorf("get activity %d: %w", id, err)
	}
	return a, nil
}

// CreateActivity inserts a new activity.
func (s *Store) CreateActivity(a models.Activity) (models.Activity, error) {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return a, validationf("活动名称不能为空")
	}
	if a.Color == "" {
		a.Color = "#cc785c"
	}
	now := NowString()
	var categoryID any
	if a.CategoryID != nil {
		categoryID = *a.CategoryID
	}
	res, err := s.db.Exec(
		`INSERT INTO activities(name,color,icon,category_id,daily_goal_minutes,sort_order,archived,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		a.Name, a.Color, a.Icon, categoryID, a.DailyGoalMinutes, a.SortOrder, boolInt(a.Archived), now, now,
	)
	if err != nil {
		return a, fmt.Errorf("create activity: %w", err)
	}
	a.ID, _ = res.LastInsertId()
	a.CreatedAt, a.UpdatedAt = now, now
	return a, nil
}

// UpdateActivity updates name/color/icon/goal/sort/archived for an activity.
func (s *Store) UpdateActivity(a models.Activity) error {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return validationf("活动名称不能为空")
	}
	var categoryID any
	if a.CategoryID != nil {
		categoryID = *a.CategoryID
	}
	res, err := s.db.Exec(
		`UPDATE activities SET name=?, color=?, icon=?, category_id=?, daily_goal_minutes=?, sort_order=?, archived=?, updated_at=? WHERE id=?`,
		a.Name, a.Color, a.Icon, categoryID, a.DailyGoalMinutes, a.SortOrder, boolInt(a.Archived), NowString(), a.ID,
	)
	if err != nil {
		return fmt.Errorf("update activity %d: %w", a.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteActivity removes the activity and (via cascade) all its entries.
func (s *Store) DeleteActivity(id int64) error {
	res, err := s.db.Exec(`DELETE FROM activities WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete activity %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanEntry(row interface{ Scan(...any) error }) (models.Entry, error) {
	var e models.Entry
	err := row.Scan(&e.ID, &e.ActivityID, &e.ActivityName, &e.ActivityColor, &e.StartedAt, &e.EndedAt, &e.DurationSeconds, &e.Note, &e.Source)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

const entryCols = `e.id, e.activity_id, COALESCE(a.name,''), COALESCE(a.color,'#cc785c'), e.started_at, e.ended_at, e.duration_seconds, e.note, e.source`

// ListEntries returns a page of entries matching the filter, newest first.
// The default FromDate/ToDate semantics keep entries whose start day lies in
// the range; with f.Overlap set (and both dates present) the filter switches
// to time-overlap: any entry whose [started_at, ended_at) intersects the
// range, so cross-midnight entries appear on every day they touch.
func (s *Store) ListEntries(f models.EntryFilter) (models.EntryList, error) {
	where := []string{"1=1"}
	var args []any
	if f.ActivityID != nil {
		where = append(where, "e.activity_id = ?")
		args = append(args, *f.ActivityID)
	}
	if f.Overlap != nil && *f.Overlap && f.FromDate != "" && f.ToDate != "" {
		fromStart, err := startOfDay(f.FromDate)
		if err != nil {
			return models.EntryList{}, err
		}
		next, err := AddDays(f.ToDate, 1)
		if err != nil {
			return models.EntryList{}, err
		}
		toExcl, err := startOfDay(next)
		if err != nil {
			return models.EntryList{}, err
		}
		where = append(where, "e.ended_at >= ? AND e.started_at < ?")
		args = append(args, fromStart, toExcl)
	} else {
		if f.FromDate != "" {
			where = append(where, "substr(e.started_at,1,10) >= ?")
			args = append(args, f.FromDate)
		}
		if f.ToDate != "" {
			where = append(where, "substr(e.started_at,1,10) <= ?")
			args = append(args, f.ToDate)
		}
	}
	w := strings.Join(where, " AND ")

	page, size := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 200 {
		size = 200
	}

	var total int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM entries e WHERE `+w, args...).Scan(&total); err != nil {
		return models.EntryList{}, fmt.Errorf("count entries: %w", err)
	}

	q := `SELECT ` + entryCols + ` FROM entries e LEFT JOIN activities a ON a.id = e.activity_id
	      WHERE ` + w + ` ORDER BY e.started_at DESC, e.id DESC LIMIT ? OFFSET ?`
	args = append(args, size, (page-1)*size)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return models.EntryList{}, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()
	items := []models.Entry{}
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return models.EntryList{}, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return models.EntryList{}, err
	}
	return models.EntryList{Items: items, Total: total, Page: page, PageSize: size}, nil
}

// ListAllEntries returns every entry without pagination, newest first. It
// backs the full-database JSON/CSV exports; the paginated ListEntries keeps
// its 200-per-page cap for the UI. The store keeps a single connection
// (SetMaxOpenConns(1)), so the sequential reads see a consistent snapshot.
func (s *Store) ListAllEntries() ([]models.Entry, error) {
	rows, err := s.db.Query(`SELECT ` + entryCols + ` FROM entries e
	      LEFT JOIN activities a ON a.id = e.activity_id
	      ORDER BY e.started_at DESC, e.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all entries: %w", err)
	}
	defer rows.Close()
	items := []models.Entry{}
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// GetEntry fetches one entry with activity info joined.
func (s *Store) GetEntry(id int64) (models.Entry, error) {
	e, err := scanEntry(s.db.QueryRow(
		`SELECT `+entryCols+` FROM entries e LEFT JOIN activities a ON a.id = e.activity_id WHERE e.id = ?`, id,
	))
	if err != nil {
		return e, fmt.Errorf("get entry %d: %w", id, err)
	}
	return e, nil
}
