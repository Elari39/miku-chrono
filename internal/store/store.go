// Package store owns the SQLite database: opening, migrations, seeding and
// every SQL query. The services layer wraps this store behind the API that
// Wails binds to the frontend.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite handle. A single connection is used so writes are
// naturally serialized (SQLite is happiest with one writer).
type Store struct {
	db *sql.DB
}

// DefaultColor is the fallback color for activities and categories without
// a user-chosen one (the DESIGN.md palette's first swatch). Migration
// scripts embed the literal instead on purpose: they are immutable
// snapshots and must not change if the default ever does.
const DefaultColor = "#cc785c"

// migrations holds every schema version in order. Index i+1 is applied when
// the stored schema_version equals i. Each script is an immutable snapshot:
// it must stay self-contained and never reference Go constants.
var migrations = []string{
	// v1: initial schema.
	`
	CREATE TABLE activities (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		name               TEXT    NOT NULL,
		color              TEXT    NOT NULL DEFAULT '#cc785c',
		icon               TEXT    NOT NULL DEFAULT '',
		daily_goal_minutes INTEGER NOT NULL DEFAULT 0,
		sort_order         INTEGER NOT NULL DEFAULT 0,
		archived           INTEGER NOT NULL DEFAULT 0,
		created_at         TEXT    NOT NULL,
		updated_at         TEXT    NOT NULL
	);
	CREATE TABLE entries (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		activity_id      INTEGER NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
		started_at       TEXT    NOT NULL,
		ended_at         TEXT    NOT NULL,
		duration_seconds INTEGER NOT NULL,
		note             TEXT    NOT NULL DEFAULT '',
		source           TEXT    NOT NULL DEFAULT 'timer',
		created_at       TEXT    NOT NULL,
		updated_at       TEXT    NOT NULL
	);
	CREATE INDEX idx_entries_activity ON entries(activity_id, started_at);
	CREATE INDEX idx_entries_started  ON entries(started_at);
	CREATE TABLE running_state (
		id          INTEGER PRIMARY KEY CHECK (id = 1),
		activity_id INTEGER NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
		started_at  TEXT    NOT NULL,
		updated_at  TEXT    NOT NULL
	);
	`,
	// v2: user-defined categories grouping activities. Deleting a category
	// leaves its activities uncategorized (ON DELETE SET NULL) instead of
	// cascading into activity/entry data.
	`
	CREATE TABLE categories (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT    NOT NULL,
		icon       TEXT    NOT NULL DEFAULT '',
		color      TEXT    NOT NULL DEFAULT '#cc785c',
		sort_order INTEGER NOT NULL DEFAULT 0,
		created_at TEXT    NOT NULL,
		updated_at TEXT    NOT NULL
	);
	ALTER TABLE activities ADD COLUMN category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL;
	`,
	// v3: timestamps become timezone-proof. Entries used to store local
	// RFC3339 strings whose UTC offset was the writer's at write time, so
	// SQL string range comparisons silently broke after a timezone/DST
	// change (mixed offsets no longer sort chronologically). From now on
	// every time column stores UTC ("Z" suffix) and the writer's local start
	// day lives in the indexed entries.local_day, which the day filters and
	// streak queries use instead of slicing started_at.
	//
	// The rewrite is pure SQLite: strftime parses an offset-suffixed
	// RFC3339 value and renders it in UTC, and COALESCE leaves any
	// unparseable legacy value untouched instead of NULLing a NOT NULL
	// column. local_day is backfilled from the ORIGINAL started_at's date
	// part (one UPDATE's SET clauses all read the old row), preserving the
	// day each record was attributed to when it was written.
	`
	ALTER TABLE entries ADD COLUMN local_day TEXT NOT NULL DEFAULT '';
	UPDATE entries SET
		local_day  = substr(started_at, 1, 10),
		started_at = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', started_at), started_at),
		ended_at   = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', ended_at),   ended_at);
	UPDATE activities SET
		created_at = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', created_at), created_at),
		updated_at = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', updated_at), updated_at);
	UPDATE categories SET
		created_at = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', created_at), created_at),
		updated_at = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', updated_at), updated_at);
	UPDATE running_state SET
		started_at = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', started_at), started_at),
		updated_at = COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ', updated_at), updated_at);
	CREATE INDEX idx_entries_local_day ON entries(local_day);
	CREATE INDEX idx_entries_activity_local_day ON entries(activity_id, local_day);
	`,
	// v4: query-shape indexes. The day-piece loader (today totals, heatmap,
	// bar charts) and the overlap entry filter select on
	// "ended_at > rangeStart AND started_at < rangeEnd"; started_at could
	// never bound the scan (almost all history starts before tomorrow), so
	// idx_entries_ended gives the range a seekable lower bound instead of a
	// full table walk that grows with total history. The activity local-day
	// index gains duration_seconds so the per-activity-per-day aggregate
	// behind the overview page reads purely from the index.
	`
	CREATE INDEX idx_entries_ended ON entries(ended_at);
	DROP INDEX IF EXISTS idx_entries_activity_local_day;
	CREATE INDEX idx_entries_activity_local_day ON entries(activity_id, local_day, duration_seconds);
	`,
}

// seedActivities are created on first launch so the check-in page is never
// empty. Colors come from the DESIGN.md palette.
var seedActivities = []struct {
	name  string
	color string
}{
	{"学习", DefaultColor},
	{"工作", "#5db8a6"},
	{"运动", "#e8a55a"},
}

// DefaultPath returns the database path under the per-user config directory.
func DefaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	dir := filepath.Join(base, "Miku_Chrono")
	return filepath.Join(dir, "mikuchrono.db"), nil
}

// dsnPragmas are applied through DSN query parameters instead of a post-open
// Exec. The driver parses them per connection, so a connection the pool has
// to rebuild (driver ErrBadConn) keeps them too — most importantly
// foreign_keys: with MaxOpenConns(1) a replaced connection silently dropping
// it would disable ON DELETE CASCADE and orphan every entry of a deleted
// activity. The modernc driver splits the query off any plain path at the
// first '?' (no file: prefix needed).
const dsnPragmas = "?_busy_timeout=5000&_foreign_keys=1&_journal_mode=WAL"

// Open opens (creating if needed) the database at path, runs pending
// migrations and seeds default activities on first run.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+dsnPragmas)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// One connection serializes writes; no SQLITE_BUSY juggling needed.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	fresh, err := s.migrate()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store migrate: %w", err)
	}
	if err := s.seed(fresh); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store seed: %w", err)
	}
	return s, nil
}

// migrate applies pending schema migrations. fresh reports whether the
// database was just created (no schema_version row before migrating) — the
// transactional version bump makes this exact, and seed() uses it to plant
// the default activities only once.
func (s *Store) migrate() (fresh bool, err error) {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return false, fmt.Errorf("create meta: %w", err)
	}
	var version int
	row := s.db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`)
	switch err := row.Scan(&version); {
	case errors.Is(err, sql.ErrNoRows):
		fresh, version = true, 0
	case err != nil:
		return false, fmt.Errorf("read schema_version: %w", err)
	}
	for v := version; v < len(migrations); v++ {
		tx, err := s.db.Begin()
		if err != nil {
			return fresh, fmt.Errorf("begin migration %d: %w", v+1, err)
		}
		if _, err := tx.Exec(migrations[v]); err != nil {
			_ = tx.Rollback()
			return fresh, fmt.Errorf("apply migration %d: %w", v+1, err)
		}
		if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES('schema_version',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, fmt.Sprint(v+1)); err != nil {
			_ = tx.Rollback()
			return fresh, fmt.Errorf("bump schema_version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fresh, fmt.Errorf("commit migration %d: %w", v+1, err)
		}
	}
	return fresh, nil
}

// seed inserts the default activities on a freshly created database only.
// Existing databases never re-seed: a user who deleted every activity must
// not see the defaults resurrect on the next launch.
func (s *Store) seed(fresh bool) error {
	if !fresh {
		return nil
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM activities`).Scan(&count); err != nil {
		return fmt.Errorf("seed count activities: %w", err)
	}
	if count > 0 {
		return nil
	}
	now := NowString()
	for i, seed := range seedActivities {
		if _, err := s.db.Exec(
			`INSERT INTO activities(name,color,sort_order,created_at,updated_at) VALUES(?,?,?,?,?)`,
			seed.name, seed.color, i+1, now, now,
		); err != nil {
			return fmt.Errorf("seed activity %s: %w", seed.name, err)
		}
	}
	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// GoalNotifiedKeyPrefix prefixes the daily-goal notification dedupe keys
// ("<prefix><date>_<activityID>") that the services.GoalNotifier stores in
// the meta table, one per activity per day.
const GoalNotifiedKeyPrefix = "goal_notified_"

// DeleteStaleGoalNotifyKeys removes daily-goal notification dedupe keys
// written before today, keeping today's keys, so the meta table does not
// grow without bound. today is a local "YYYY-MM-DD" date.
func (s *Store) DeleteStaleGoalNotifyKeys(today string) error {
	prefix := GoalNotifiedKeyPrefix + today
	if _, err := s.db.Exec(
		`DELETE FROM meta WHERE key LIKE ? AND substr(key, 1, ?) <> ?`,
		GoalNotifiedKeyPrefix+"%", len(prefix), prefix,
	); err != nil {
		return fmt.Errorf("delete stale goal notify keys: %w", err)
	}
	return nil
}

// GetSetting reads a key/value setting from the meta table. found is false
// when the key has never been written.
func (s *Store) GetSetting(key string) (value string, found bool, err error) {
	err = s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, true, nil
}

// SetSetting writes a key/value setting into the meta table.
func (s *Store) SetSetting(key, value string) error {
	if _, err := s.db.Exec(
		`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	); err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// SetSettings writes several key/value settings into the meta table in one
// transaction, so a crash mid-write can never leave a half-updated group
// (e.g. a window's new X next to its old height). The caller should group
// keys that are only meaningful together.
func (s *Store) SetSettings(pairs [][2]string) error {
	if len(pairs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("set settings: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, kv := range pairs {
		if _, err := tx.Exec(
			`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
			kv[0], kv[1],
		); err != nil {
			return fmt.Errorf("set setting %q: %w", kv[0], err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("set settings: commit: %w", err)
	}
	return nil
}

// NowString renders time.Now as a UTC RFC3339 timestamp, the canonical
// storage format for every time column since schema v3. UTC strings carry a
// fixed offset, so lexicographic order equals chronological order and the
// SQL string range filters stay correct across timezone/DST changes.
func NowString() string { return FormatTime(time.Now()) }

// FormatTime renders t as a UTC RFC3339 timestamp.
func FormatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// ParseTime parses a stored RFC3339 timestamp (UTC since v3, but any offset
// variant parses too — the instant is what matters).
func ParseTime(v string) (time.Time, error) { return time.Parse(time.RFC3339, v) }

// Today returns the local date string ("YYYY-MM-DD") for t, formatted in
// t's own location: pass a local time for the machine's current local day,
// or a parsed offset-aware timestamp for that timestamp's wall-clock date.
func Today(t time.Time) string { return t.Format("2006-01-02") }

// AddDays returns the date string n days from date (date is "YYYY-MM-DD").
func AddDays(date string, n int) (string, error) {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return "", fmt.Errorf("parse date %q: %w", date, err)
	}
	return t.AddDate(0, 0, n).Format("2006-01-02"), nil
}
