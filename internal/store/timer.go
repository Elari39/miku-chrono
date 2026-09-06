package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"mikuchrono/internal/models"
)

// minTimerSeconds is the shortest recorded duration. Shorter running periods
// (accidental taps) are silently discarded instead of being stored.
const minTimerSeconds = 1

// Timer-chain settings in the meta table. The chain remembers the most
// recently used activity plus the seconds accumulated before the current
// session, so a stopped timer can be resumed ("开始计时") and the ball can
// show the previous session while idle. No schema migration is needed.
const (
	keyTimerActivity = "timer_activity"
	keyTimerSeconds  = "timer_base_seconds"
)

// rowQuerier abstracts *sql.DB and *sql.Tx so chain reads work both inside
// transactions and on the bare store (the store keeps a single connection,
// so nested db reads inside a tx would deadlock).
type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

// readChain loads the paused timer chain. ok is false when no timer has ever
// been started (chain cleared or corrupt).
func readChain(q rowQuerier) (activityID, baseSeconds int64, ok bool, err error) {
	var act string
	err = q.QueryRow(`SELECT value FROM meta WHERE key = ?`, keyTimerActivity).Scan(&act)
	if isNoRows(err) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, fmt.Errorf("read chain activity: %w", err)
	}
	id, perr := strconv.ParseInt(act, 10, 64)
	if perr != nil || id <= 0 {
		return 0, 0, false, nil
	}
	var secs string
	err = q.QueryRow(`SELECT value FROM meta WHERE key = ?`, keyTimerSeconds).Scan(&secs)
	if isNoRows(err) {
		return id, 0, true, nil
	}
	if err != nil {
		return 0, 0, false, fmt.Errorf("read chain base: %w", err)
	}
	b, _ := strconv.ParseInt(secs, 10, 64)
	if b < 0 {
		b = 0
	}
	return id, b, true, nil
}

// writeChain persists the timer chain inside the given transaction.
func writeChain(tx *sql.Tx, activityID, baseSeconds int64) error {
	if _, err := tx.Exec(
		`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		keyTimerActivity, strconv.FormatInt(activityID, 10),
	); err != nil {
		return fmt.Errorf("write chain activity: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		keyTimerSeconds, strconv.FormatInt(baseSeconds, 10),
	); err != nil {
		return fmt.Errorf("write chain base: %w", err)
	}
	return nil
}

// GetTimerState returns the current timer state. When a timer is running the
// returned state carries the activity info and the elapsed time including
// everything accumulated by earlier sessions of the same chain. When idle it
// carries the last (paused) chain so the UI can show the previous session
// and offer to resume it.
func (s *Store) GetTimerState(now time.Time) (models.TimerState, error) {
	var st models.TimerState
	var startedAt string
	row := s.db.QueryRow(
		`SELECT r.activity_id, r.started_at, COALESCE(a.name,''), COALESCE(a.color,'#cc785c')
		 FROM running_state r LEFT JOIN activities a ON a.id = r.activity_id WHERE r.id = 1`)
	err := row.Scan(&st.ActivityID, &startedAt, &st.ActivityName, &st.ActivityColor)
	if err != nil {
		if isNoRows(err) {
			return s.idleTimerState()
		}
		return st, fmt.Errorf("get timer state: %w", err)
	}
	st.Running = true
	st.StartedAt = startedAt
	_, base, _, err := readChain(s.db)
	if err != nil {
		return st, fmt.Errorf("get timer state: %w", err)
	}
	if t, err := ParseTime(startedAt); err == nil {
		session := int64(now.Sub(t).Seconds())
		st.ElapsedSeconds = base + session
		st.SessionElapsedSeconds = session
	}
	return st, nil
}

// idleTimerState renders the idle timer state from the paused chain: the
// last activity and the seconds accumulated across its sessions. A missing
// chain yields an all-zero state ("never timed yet").
func (s *Store) idleTimerState() (models.TimerState, error) {
	var st models.TimerState
	id, base, ok, err := readChain(s.db)
	if err != nil {
		return st, fmt.Errorf("get timer state: %w", err)
	}
	if !ok {
		return st, nil
	}
	st.LastActivityID = id
	st.LastElapsedSeconds = base
	// Deleted activity: name/color stay empty and the UI falls back to its
	// generic idle text.
	_ = s.db.QueryRow(`SELECT COALESCE(name,''), COALESCE(color,'#cc785c') FROM activities WHERE id = ?`, id).
		Scan(&st.LastActivityName, &st.LastActivityColor)
	return st, nil
}

// StartTimer starts timing activityID. Timers are mutually exclusive:
// switching activities first closes and records the running session, and a
// running session for the same activity is a no-op (kept as-is). When idle,
// starting the same activity as the paused chain RESUMES it, carrying the
// accumulated seconds forward; any other start begins a fresh chain. Returns
// the fresh state; the current session's elapsed starts at zero plus any
// resumed base carried in ElapsedSeconds.
func (s *Store) StartTimer(activityID int64, now time.Time) (models.TimerState, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return models.TimerState{}, fmt.Errorf("start timer: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var runActivity int64
	var runStarted string
	err = tx.QueryRow(`SELECT activity_id, started_at FROM running_state WHERE id = 1`).Scan(&runActivity, &runStarted)
	base := int64(0)
	switch {
	case err == nil:
		// Starting the already-running activity is a no-op: keep the session.
		if runActivity == activityID {
			_ = tx.Rollback() // nothing written; read path below rebuilds state
			return s.GetTimerState(now)
		}
		if _, cerr := s.closeRunningTx(tx, runActivity, runStarted, now); cerr != nil {
			return models.TimerState{}, fmt.Errorf("start timer: close previous session: %w", cerr)
		}
	case isNoRows(err):
		// Idle: resume the chain when it belongs to the target activity,
		// otherwise start a fresh chain from zero.
		id, b, ok, cerr := readChain(tx)
		if cerr != nil {
			return models.TimerState{}, fmt.Errorf("start timer: read chain: %w", cerr)
		}
		if ok && id == activityID {
			base = b
		}
	default:
		return models.TimerState{}, fmt.Errorf("start timer: read state: %w", err)
	}

	// The target activity must exist and be active.
	var archived int
	err = tx.QueryRow(`SELECT archived FROM activities WHERE id = ?`, activityID).Scan(&archived)
	if isNoRows(err) {
		return models.TimerState{}, fmt.Errorf("%w: 活动不存在", ErrNotFound)
	}
	if err != nil {
		return models.TimerState{}, fmt.Errorf("start timer: load activity: %w", err)
	}
	if archived != 0 {
		return models.TimerState{}, validationf("活动已归档，无法开始计时")
	}

	started := FormatTime(now)
	if _, err := tx.Exec(
		`INSERT INTO running_state(id, activity_id, started_at, updated_at) VALUES(1, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET activity_id=excluded.activity_id, started_at=excluded.started_at, updated_at=excluded.updated_at`,
		activityID, started, started,
	); err != nil {
		return models.TimerState{}, fmt.Errorf("start timer: write state: %w", err)
	}
	// Persist the chain so the accumulated base survives crashes/restarts.
	if err := writeChain(tx, activityID, base); err != nil {
		return models.TimerState{}, fmt.Errorf("start timer: write chain: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return models.TimerState{}, fmt.Errorf("start timer: commit: %w", err)
	}
	return models.TimerState{
		Running:        true,
		ActivityID:     activityID,
		StartedAt:      started,
		ElapsedSeconds: base,
	}, nil
}

// StopTimer stops the running timer, records this session's segment (only
// now - started_at, never the accumulated total) and folds the segment into
// the paused chain so the timer can be resumed later. Returns nil entry when
// the session was too short to record.
func (s *Store) StopTimer(now time.Time) (*models.Entry, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("stop timer: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var runActivity int64
	var runStarted string
	err = tx.QueryRow(`SELECT activity_id, started_at FROM running_state WHERE id = 1`).Scan(&runActivity, &runStarted)
	if isNoRows(err) {
		return nil, validationf("当前没有进行中的计时")
	}
	if err != nil {
		return nil, fmt.Errorf("stop timer: read state: %w", err)
	}
	entry, cerr := s.closeRunningTx(tx, runActivity, runStarted, now)
	if cerr != nil {
		return nil, fmt.Errorf("stop timer: close session: %w", cerr)
	}

	// Fold the finished segment into the chain. A missing chain (a timer
	// started before the chain feature existed) starts one from zero.
	_, base, _, err := readChain(tx)
	if err != nil {
		return nil, fmt.Errorf("stop timer: read chain: %w", err)
	}
	if start, perr := ParseTime(runStarted); perr == nil {
		base += int64(now.Sub(start).Seconds())
	}
	if err := writeChain(tx, runActivity, base); err != nil {
		return nil, fmt.Errorf("stop timer: write chain: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM running_state WHERE id = 1`); err != nil {
		return nil, fmt.Errorf("stop timer: clear state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("stop timer: commit: %w", err)
	}
	return entry, nil
}

// closeRunningTx finalizes the running session inside tx, inserting an entry
// when the duration is at least minTimerSeconds. Returns the inserted entry
// (nil, nil when the session was discarded as too short). Any error must
// abort the surrounding transaction via the caller so the running state
// stays intact and the session can be retried.
func (s *Store) closeRunningTx(tx *sql.Tx, activityID int64, startedAt string, now time.Time) (*models.Entry, error) {
	start, err := ParseTime(startedAt)
	if err != nil {
		return nil, fmt.Errorf("close session: parse started_at: %w", err)
	}
	duration := int64(now.Sub(start).Seconds())
	if duration < minTimerSeconds {
		return nil, nil
	}
	ended := FormatTime(now)
	res, err := tx.Exec(
		`INSERT INTO entries(activity_id, started_at, ended_at, duration_seconds, note, source, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		activityID, startedAt, ended, duration, "", "timer", ended, ended,
	)
	if err != nil {
		return nil, fmt.Errorf("close session: insert entry: %w", err)
	}
	id, _ := res.LastInsertId()
	return &models.Entry{
		ID:              id,
		ActivityID:      activityID,
		StartedAt:       startedAt,
		EndedAt:         ended,
		DurationSeconds: duration,
		Source:          "timer",
	}, nil
}

// isNoRows reports whether err is the database/sql empty-result sentinel.
func isNoRows(err error) bool {
	return err != nil && err.Error() == "sql: no rows in result set"
}
