package store

import (
	"fmt"
	"strings"
	"time"

	"mikuchrono/internal/models"
)

// CreateManualEntry validates and inserts a backfilled entry (source
// "manual"). Overlap with other entries is allowed.
func (s *Store) CreateManualEntry(activityID int64, startedAt, endedAt, note string, now time.Time) (models.Entry, error) {
	var e models.Entry
	if _, err := s.GetActivity(activityID); err != nil {
		return e, validationf("活动不存在")
	}
	start, end, dur, err := validateRange(startedAt, endedAt, now)
	if err != nil {
		return e, err
	}
	note = strings.TrimSpace(note)
	if len(note) > 500 {
		return e, validationf("备注最多 500 字")
	}
	ts := FormatTime(now)
	res, err := s.db.Exec(
		`INSERT INTO entries(activity_id, started_at, ended_at, duration_seconds, note, source, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		activityID, FormatTime(start), FormatTime(end), dur, note, "manual", ts, ts,
	)
	if err != nil {
		return e, fmt.Errorf("create manual entry: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetEntry(id)
}

// UpdateEntry validates and updates an existing entry's times, note and
// activity.
func (s *Store) UpdateEntry(e models.Entry, now time.Time) error {
	if _, err := s.GetActivity(e.ActivityID); err != nil {
		return validationf("活动不存在")
	}
	start, end, dur, err := validateRange(e.StartedAt, e.EndedAt, now)
	if err != nil {
		return err
	}
	e.Note = strings.TrimSpace(e.Note)
	if len(e.Note) > 500 {
		return validationf("备注最多 500 字")
	}
	res, err := s.db.Exec(
		`UPDATE entries SET activity_id=?, started_at=?, ended_at=?, duration_seconds=?, note=?, updated_at=? WHERE id=?`,
		e.ActivityID, FormatTime(start), FormatTime(end), dur, e.Note, FormatTime(now), e.ID,
	)
	if err != nil {
		return fmt.Errorf("update entry %d: %w", e.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteEntry removes one entry by id.
func (s *Store) DeleteEntry(id int64) error {
	res, err := s.db.Exec(`DELETE FROM entries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete entry %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearEntries deletes every entry, clears any running timer state and
// resets the paused timer chain. Activities are kept.
func (s *Store) ClearEntries() error {
	if _, err := s.db.Exec(`DELETE FROM entries`); err != nil {
		return fmt.Errorf("clear entries: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM running_state`); err != nil {
		return fmt.Errorf("clear running state: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM meta WHERE key IN (?, ?)`, keyTimerActivity, keyTimerSeconds); err != nil {
		return fmt.Errorf("clear timer chain: %w", err)
	}
	return nil
}

// validateRange parses two RFC3339 timestamps and enforces the manual-entry
// rules: end strictly after start, and end not in the future (a one-minute
// clock-skew tolerance is granted).
func validateRange(startedAt, endedAt string, now time.Time) (time.Time, time.Time, int64, error) {
	start, err := ParseTime(startedAt)
	if err != nil {
		return time.Time{}, time.Time{}, 0, validationf("开始时间格式不正确")
	}
	end, err := ParseTime(endedAt)
	if err != nil {
		return time.Time{}, time.Time{}, 0, validationf("结束时间格式不正确")
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, 0, validationf("结束时间必须晚于开始时间")
	}
	if end.After(now.Add(time.Minute)) {
		return time.Time{}, time.Time{}, 0, validationf("结束时间不能晚于现在")
	}
	return start, end, int64(end.Sub(start).Seconds()), nil
}
