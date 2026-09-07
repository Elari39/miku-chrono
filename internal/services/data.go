package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mikuchrono/internal/models"
	"mikuchrono/internal/store"
)

// DataService handles backup export and destructive data operations.
type DataService struct {
	Store *store.Store
	// Emit, when set (wired in main.go), notifies the app that the timer
	// state may have changed after a destructive clear so every window
	// refreshes. Nil in tests.
	Emit func(event string)
	// OpenDir opens a directory in the platform's file manager (Windows
	// wiring in main.go). Nil reports the platform as unsupported — the
	// operation is meaningless outside a desktop shell (tests, a future
	// mobile shell).
	OpenDir func(dir string) error
	// SaveFile asks the user where to save a file and returns the chosen
	// path ("" when cancelled). Nil falls back to the data directory so
	// exports stay usable in tests and headless shells.
	SaveFile func(defaultName, filterName, pattern string) (string, error)
}

// DataDir returns the directory holding the SQLite database.
func (s *DataService) DataDir() (string, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

// OpenDataDir opens the data directory in the platform's file manager.
func (s *DataService) OpenDataDir() error {
	dir, err := s.DataDir()
	if err != nil {
		return err
	}
	if s.OpenDir == nil {
		return fmt.Errorf("当前平台不支持打开数据目录")
	}
	return s.OpenDir(dir)
}

// savePath asks the user where to save a file via the injected SaveFile
// hook. When no hook is available (e.g. tests) it falls back to the data dir.
func (s *DataService) savePath(defaultName, filterName, pattern string) (string, error) {
	if s.SaveFile != nil {
		path, err := s.SaveFile(defaultName, filterName, pattern)
		if err != nil {
			return "", fmt.Errorf("save dialog: %w", err)
		}
		return path, nil
	}
	dir, err := s.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, defaultName), nil
}

// ExportJSON writes the full database (activities + entries) as JSON.
// Returns the written path, or "" when the user cancelled the dialog.
func (s *DataService) ExportJSON() (string, error) {
	path, err := s.savePath("miku-chrono-export.json", "JSON", "*.json")
	if err != nil || path == "" {
		return path, err
	}
	acts, err := s.Store.ListActivities(true)
	if err != nil {
		return "", err
	}
	cats, err := s.Store.ListCategories()
	if err != nil {
		return "", err
	}
	// Full export: bypass the 200-per-page cap of ListEntries.
	entries, err := s.Store.ListAllEntries()
	if err != nil {
		return "", err
	}
	payload := models.ExportData{
		Version:    2,
		ExportedAt: store.FormatTime(time.Now()),
		Categories: cats,
		Activities: acts,
		Entries:    entries,
	}
	buf, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal export: %w", err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return "", fmt.Errorf("write export: %w", err)
	}
	return path, nil
}

// ExportCSV writes every entry as CSV (Excel-friendly, with BOM).
// Returns the written path, or "" when the user cancelled the dialog.
func (s *DataService) ExportCSV() (string, error) {
	path, err := s.savePath("miku-chrono-export.csv", "CSV", "*.csv")
	if err != nil || path == "" {
		return path, err
	}
	// Full export: bypass the 200-per-page cap of ListEntries.
	entries, err := s.Store.ListAllEntries()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM so Excel opens Chinese text correctly
	buf.WriteString("activity,started_at,ended_at,duration_seconds,note,source\r\n")
	// CSV is read by humans/spreadsheets, so timestamps render in the local
	// wall clock even though storage is UTC since schema v3. The JSON export
	// stays canonical UTC for machine import.
	local := func(iso string) string {
		if t, err := store.ParseTime(iso); err == nil {
			return t.Local().Format(time.RFC3339)
		}
		return iso
	}
	for _, e := range entries {
		buf.WriteString(csvField(e.ActivityName))
		buf.WriteByte(',')
		buf.WriteString(csvField(local(e.StartedAt)))
		buf.WriteByte(',')
		buf.WriteString(csvField(local(e.EndedAt)))
		buf.WriteByte(',')
		fmt.Fprintf(&buf, "%d", e.DurationSeconds)
		buf.WriteByte(',')
		buf.WriteString(csvField(e.Note))
		buf.WriteByte(',')
		buf.WriteString(e.Source)
		buf.WriteString("\r\n")
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("write export: %w", err)
	}
	return path, nil
}

// csvField quotes a CSV field when needed and escapes embedded quotes.
func csvField(v string) string {
	if !strings.ContainsAny(v, ",\"\r\n") {
		return v
	}
	return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
}

// ClearEntries deletes every entry and any running timer state. Activities
// are kept. A success broadcast lets every window drop its local running
// state immediately (the cleared timer must not keep ticking in the UI).
func (s *DataService) ClearEntries() error {
	if err := s.Store.ClearEntries(); err != nil {
		return err
	}
	if s.Emit != nil {
		s.Emit(EventTimerStopped)
	}
	return nil
}
