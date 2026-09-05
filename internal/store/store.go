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

// migrations holds every schema version in order. Index i+1 is applied when
// the stored schema_version equals i.
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
}

// seedActivities are created on first launch so the check-in page is never
// empty. Colors come from the DESIGN.md palette.
var seedActivities = []struct {
	name  string
	color string
}{
	{"学习", "#cc785c"},
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

// Open opens (creating if needed) the database at path, runs pending
// migrations and seeds default activities on first run.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// One connection serializes writes; no SQLITE_BUSY juggling needed.
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply pragma: %w", err)
		}
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store migrate: %w", err)
	}
	if err := s.seed(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store seed: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create meta: %w", err)
	}
	var version int
	row := s.db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`)
	switch err := row.Scan(&version); {
	case errors.Is(err, sql.ErrNoRows):
		version = 0
	case err != nil:
		return fmt.Errorf("read schema_version: %w", err)
	}
	for v := version; v < len(migrations); v++ {
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", v+1, err)
		}
		if _, err := tx.Exec(migrations[v]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", v+1, err)
		}
		if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES('schema_version',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, fmt.Sprint(v+1)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("bump schema_version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", v+1, err)
		}
	}
	return nil
}

func (s *Store) seed() error {
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

// NowString renders t as a local RFC3339 timestamp, the canonical storage
// format for every time column.
func NowString() string { return time.Now().Format(time.RFC3339) }

// FormatTime renders t as a local RFC3339 timestamp.
func FormatTime(t time.Time) string { return t.Format(time.RFC3339) }

// ParseTime parses a stored RFC3339 timestamp.
func ParseTime(v string) (time.Time, error) { return time.Parse(time.RFC3339, v) }

// DayOf extracts the local "YYYY-MM-DD" date part of a stored timestamp.
func DayOf(stored string) string {
	if len(stored) >= 10 {
		return stored[:10]
	}
	return stored
}

// Today returns the local date string for t.
func Today(t time.Time) string { return t.Format("2006-01-02") }

// AddDays returns the date string n days from date (date is "YYYY-MM-DD").
func AddDays(date string, n int) (string, error) {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return "", fmt.Errorf("parse date %q: %w", date, err)
	}
	return t.AddDate(0, 0, n).Format("2006-01-02"), nil
}
