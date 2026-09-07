package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mikuchrono/internal/applog"
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
	// OpenDir opens a directory in the platform's file manager (wired to
	// Explorer by wireDesktopShell in shell_windows.go). Nil reports the
	// operation as unsupported — it is meaningless outside the desktop
	// shell (tests).
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
	// The data dir normally exists (store.Open created it), but it can be
	// removed while the app runs (fresh profile, manual cleanup) — recreate
	// it so the fallback export cannot die on an opaque missing-path error.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	return filepath.Join(dir, defaultName), nil
}

// exportHeader is the entries-less prefix of models.ExportData: the small
// collections marshal whole while entries stream row by row behind them.
type exportHeader struct {
	Version    int               `json:"version"`
	ExportedAt string            `json:"exportedAt"`
	Categories []models.Category `json:"categories"`
	Activities []models.Activity `json:"activities"`
}

// ExportJSON writes the full database (activities + entries) as JSON.
// Returns the written path, or "" when the user cancelled the dialog.
// Failures are logged via applog before propagating to the frontend toast.
func (s *DataService) ExportJSON() (string, error) {
	path, err := s.exportJSON()
	if err != nil {
		applog.Printf("json export: %v", err)
	}
	return path, err
}

func (s *DataService) exportJSON() (string, error) {
	// Entries stream from the store cursor straight to the file so memory
	// stays flat on large histories (the old path marshaled the whole
	// payload at once). The document keeps models.ExportData's key order
	// and meaning; only the indentation differs (entries render one compact
	// object per line).
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
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create export: %w", err)
	}
	w := bufio.NewWriter(f)
	hdr, err := json.MarshalIndent(exportHeader{
		Version:    2,
		ExportedAt: store.FormatTime(time.Now()),
		Categories: cats,
		Activities: acts,
	}, "", "  ")
	if err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("marshal export: %w", err)
	}
	// hdr is a complete JSON object; slice off its closing brace and
	// continue with the streamed entries array inside it.
	if _, err = w.Write(bytes.TrimRight(hdr, "} \n\t")); err != nil {
		err = fmt.Errorf("write export: %w", err)
	}
	if err == nil {
		_, err = w.WriteString(",\n  \"entries\": [")
	}
	first := true
	if err == nil {
		err = s.Store.IterateAllEntries(func(e models.Entry) error {
			row, mErr := json.Marshal(e)
			if mErr != nil {
				return fmt.Errorf("marshal export entry %d: %w", e.ID, mErr)
			}
			if first {
				first = false
				_, wErr := w.WriteString("\n    ")
				if wErr != nil {
					return wErr
				}
			} else if _, wErr := w.WriteString(",\n    "); wErr != nil {
				return wErr
			}
			_, wErr := w.Write(row)
			return wErr
		})
	}
	if err == nil {
		tail := "\n  ]"
		if first {
			tail = "]"
		}
		_, err = w.WriteString(tail + "\n}\n")
	}
	if wErr := w.Flush(); err == nil {
		err = wErr
	}
	if err == nil {
		err = f.Close()
	} else {
		_ = f.Close()
	}
	if err != nil {
		_ = os.Remove(path)
		if strings.Contains(err.Error(), "marshal") {
			err = fmt.Errorf("marshal export: %w", err)
		}
		return "", err
	}
	return path, nil
}

// ExportCSV writes every entry as CSV (Excel-friendly, with BOM).
// Returns the written path, or "" when the user cancelled the dialog.
// Failures are logged via applog before propagating to the frontend toast.
func (s *DataService) ExportCSV() (string, error) {
	path, err := s.exportCSV()
	if err != nil {
		applog.Printf("csv export: %v", err)
	}
	return path, err
}

func (s *DataService) exportCSV() (string, error) {
	// Rows stream from the store cursor to a buffered file writer — the
	// whole export no longer sits in memory before hitting disk.
	path, err := s.savePath("miku-chrono-export.csv", "CSV", "*.csv")
	if err != nil || path == "" {
		return path, err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create export: %w", err)
	}
	w := bufio.NewWriter(f)
	_, err = w.WriteString("\xEF\xBB\xBF") // UTF-8 BOM so Excel opens Chinese text correctly
	if err == nil {
		_, err = w.WriteString("activity,started_at,ended_at,duration_seconds,note,source\r\n")
	}
	// CSV is read by humans/spreadsheets, so timestamps render in the local
	// wall clock even though storage is UTC since schema v3. The JSON export
	// stays canonical UTC for machine import.
	local := func(iso string) string {
		if t, err := store.ParseTime(iso); err == nil {
			return t.Local().Format(time.RFC3339)
		}
		return iso
	}
	if err == nil {
		err = s.Store.IterateAllEntries(func(e models.Entry) error {
			line := []string{
				csvField(e.ActivityName),
				csvField(local(e.StartedAt)),
				csvField(local(e.EndedAt)),
				strconv.FormatInt(e.DurationSeconds, 10),
				csvField(e.Note),
				e.Source,
			}
			_, wErr := w.WriteString(strings.Join(line, ",") + "\r\n")
			return wErr
		})
	}
	if wErr := w.Flush(); err == nil {
		err = wErr
	}
	if err == nil {
		err = f.Close()
	} else {
		_ = f.Close()
	}
	if err != nil {
		_ = os.Remove(path)
		return "", err
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
