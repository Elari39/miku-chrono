package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

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
}

// DataDir returns the directory holding the SQLite database.
func (s *DataService) DataDir() (string, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

// OpenDataDir opens the data directory in Windows Explorer.
func (s *DataService) OpenDataDir() error {
	dir, err := s.DataDir()
	if err != nil {
		return err
	}
	return exec.Command("explorer", dir).Start()
}

// savePath asks the user where to save a file via the native save dialog.
// When no dialog is available (e.g. tests) it falls back to the data dir.
func (s *DataService) savePath(defaultName, filterName, pattern string) (string, error) {
	if app := application.Get(); app != nil {
		dialog := app.Dialog.SaveFile()
		dialog.SetFilename(defaultName)
		dialog.AddFilter(filterName, pattern)
		path, err := dialog.PromptForSingleSelection()
		if err != nil {
			return "", fmt.Errorf("save dialog: %w", err)
		}
		if path != "" {
			return path, nil
		}
		return "", nil // user cancelled
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
	for _, e := range entries {
		buf.WriteString(csvField(e.ActivityName))
		buf.WriteByte(',')
		buf.WriteString(csvField(e.StartedAt))
		buf.WriteByte(',')
		buf.WriteString(csvField(e.EndedAt))
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
