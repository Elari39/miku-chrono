package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
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

// validHexColor matches the #RRGGBB palette the frontend's color picker offers.
var validHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validateActivity enforces the shared create/update rules on a in place: a
// non-empty trimmed name, an optional color that must be a #RRGGBB hex value
// (empty falls back to the default), a daily goal within 0..1440 minutes and
// a non-negative sort order.
func validateActivity(a *models.Activity) error {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return validationf("活动名称不能为空")
	}
	if a.Color == "" {
		a.Color = DefaultColor
	} else if !validHexColor.MatchString(a.Color) {
		return validationf("颜色格式不正确，需为 #RRGGBB 十六进制色值")
	}
	if a.DailyGoalMinutes < 0 || a.DailyGoalMinutes > 1440 {
		return validationf("每日目标分钟数需在 0 到 1440 之间")
	}
	if a.SortOrder < 0 {
		return validationf("排序值不能为负数")
	}
	return nil
}

// categoryExists reports whether a.CategoryID (nil means 未分类) references a
// stored category, so callers get a uniform validation error instead of the
// raw SQLite foreign-key text.
func (s *Store) categoryExists(id *int64) (bool, error) {
	if id == nil {
		return true, nil
	}
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM categories WHERE id = ?`, *id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check category %d: %w", *id, err)
	}
	return true, nil
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

// CreateActivity validates and inserts a new activity.
func (s *Store) CreateActivity(a models.Activity) (models.Activity, error) {
	if err := validateActivity(&a); err != nil {
		return a, err
	}
	if ok, err := s.categoryExists(a.CategoryID); err != nil {
		return a, err
	} else if !ok {
		return a, validationf("分类不存在")
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

// UpdateActivity validates and updates name/color/icon/goal/sort/archived for
// an activity.
func (s *Store) UpdateActivity(a models.Activity) error {
	if err := validateActivity(&a); err != nil {
		return err
	}
	if ok, err := s.categoryExists(a.CategoryID); err != nil {
		return err
	} else if !ok {
		return validationf("分类不存在")
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

// entryCols are the joined columns of one entry row. The '#cc785c' fallback
// mirrors store.DefaultColor; the literal stays because migrations are the
// only place SQL may embed it as data.
const entryCols = `e.id, e.activity_id, COALESCE(a.name,''), COALESCE(a.color,'#cc785c'), e.started_at, e.ended_at, e.duration_seconds, e.note, e.source`

// maxPageSize caps one ListEntries page. The UI asks for exactly this; the
// full-database exports use ListAllEntries instead.
const maxPageSize = 200

// ListEntries returns a page of entries matching the filter, newest first.
// The default FromDate/ToDate semantics keep entries whose start day lies in
// the range; with f.Overlap set (and both dates present) the filter switches
// to time-overlap: any entry whose [started_at, ended_at) intersects the
// range, so cross-midnight entries appear on every day they touch.
func (s *Store) ListEntries(f models.EntryFilter) (models.EntryList, error) {
	where := []string{"1=1"}
	// entriesFrom carries the table hint: the overlap branch forces
	// idx_entries_ended (see stats.go loadDayPieces) so a narrow range seeks
	// instead of walking history to satisfy the ORDER BY.
	entriesFrom := `FROM entries e`
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
		// Half-open overlap [started_at, ended_at): a record ending exactly
		// at the range start occupies none of it and must not show up.
		where = append(where, "e.ended_at > ? AND e.started_at < ?")
		args = append(args, fromStart, toExcl)
		entriesFrom = `FROM entries e INDEXED BY idx_entries_ended`
	} else {
		// Start-day membership via the indexed local_day column (schema v3):
		// slicing started_at would both miss the index and — before the UTC
		// migration — compare date parts across mixed offsets.
		if f.FromDate != "" {
			where = append(where, "e.local_day >= ?")
			args = append(args, f.FromDate)
		}
		if f.ToDate != "" {
			where = append(where, "e.local_day <= ?")
			args = append(args, f.ToDate)
		}
	}
	w := strings.Join(where, " AND ")

	page, size := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = maxPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}

	var total int64
	if err := s.db.QueryRow(`SELECT COUNT(*) `+entriesFrom+` WHERE `+w, args...).Scan(&total); err != nil {
		return models.EntryList{}, fmt.Errorf("count entries: %w", err)
	}

	q := `SELECT ` + entryCols + ` ` + entriesFrom + ` LEFT JOIN activities a ON a.id = e.activity_id
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

// IterateAllEntries walks every entry (joined with activity info), newest
// first, invoking fn per row. It backs the full-database JSON/CSV exports:
// rows stream straight from the cursor to the file instead of being
// materialized, so memory stays flat on large histories. The store keeps a
// single connection (SetMaxOpenConns(1)), so the sequential reads see a
// consistent snapshot. fn returning an error aborts the iteration.
func (s *Store) IterateAllEntries(fn func(models.Entry) error) error {
	rows, err := s.db.Query(`SELECT ` + entryCols + ` FROM entries e
	      LEFT JOIN activities a ON a.id = e.activity_id
	      ORDER BY e.started_at DESC, e.id DESC`)
	if err != nil {
		return fmt.Errorf("iterate all entries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return err
		}
		if err := fn(e); err != nil {
			return err
		}
	}
	return rows.Err()
}

// ListAllEntries returns every entry without pagination, newest first. A
// thin materialization of IterateAllEntries — prefer the iterator in
// streaming contexts; this remains for callers that genuinely want the
// whole slice.
func (s *Store) ListAllEntries() ([]models.Entry, error) {
	items := []models.Entry{}
	err := s.IterateAllEntries(func(e models.Entry) error {
		items = append(items, e)
		return nil
	})
	return items, err
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
