package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mikuchrono/internal/models"
	"mikuchrono/internal/store"
)

// seedExportStore plants a category plus a few entries — including one with
// CSV-hostile note content — and returns the store.
func seedExportStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	now := time.Date(2025, 9, 5, 12, 0, 0, 0, time.Local)
	if _, err := st.CreateCategory(models.Category{Name: "日常"}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		act        int64
		start, end string
		note       string
	}{
		{1, "2025-09-04T09:00:00+08:00", "2025-09-04T10:00:00+08:00", "plain"},
		{2, "2025-09-05T09:00:00+08:00", "2025-09-05T10:30:00+08:00", "note, with \"quotes\" and,\nnewline"},
		{1, "2025-09-03T23:50:00+08:00", "2025-09-04T00:10:00+08:00", "跨午夜"},
	} {
		if _, err := st.CreateManualEntry(f.act, f.start, f.end, f.note, now); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

// TestExportJSONStreamsFullDocument verifies the streamed export still
// parses as the canonical models.ExportData payload (version 2, all
// collections, entries newest first) on both a seeded and an empty database.
func TestExportJSONStreamsFullDocument(t *testing.T) {
	st := seedExportStore(t)
	s := &DataService{Store: st}
	path, err := s.ExportJSON()
	if err != nil {
		t.Fatalf("export json: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload models.ExportData
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("exported JSON must parse: %v\n%s", err, raw)
	}
	if payload.Version != 2 {
		t.Fatalf("version = %d, want 2", payload.Version)
	}
	if payload.ExportedAt == "" {
		t.Fatal("exportedAt must be set")
	}
	if len(payload.Categories) != 1 || payload.Categories[0].Name != "日常" {
		t.Fatalf("categories = %+v", payload.Categories)
	}
	if len(payload.Activities) != 3 {
		t.Fatalf("activities = %d, want the 3 seeded ones", len(payload.Activities))
	}
	if len(payload.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(payload.Entries))
	}
	// Newest first, activity info joined, notes byte-exact.
	if payload.Entries[0].ActivityID != 2 || payload.Entries[0].Note != "note, with \"quotes\" and,\nnewline" {
		t.Fatalf("first entry = %+v", payload.Entries[0])
	}
	if payload.Entries[0].ActivityName == "" {
		t.Fatalf("activity name must be joined: %+v", payload.Entries[0])
	}
	if payload.Entries[2].Note != "跨午夜" {
		t.Fatalf("third entry = %+v", payload.Entries[2])
	}

	// An empty database still yields a valid document with an empty array.
	empty, err := store.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = empty.Close() })
	path2, err := (&DataService{Store: empty}).ExportJSON()
	if err != nil {
		t.Fatalf("export empty: %v", err)
	}
	raw2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatal(err)
	}
	var emptyPayload models.ExportData
	if err := json.Unmarshal(raw2, &emptyPayload); err != nil {
		t.Fatalf("empty export must parse: %v\n%s", err, raw2)
	}
	if emptyPayload.Entries == nil || len(emptyPayload.Entries) != 0 {
		t.Fatalf("entries must be an empty array, got %s", raw2)
	}
}

// TestExportCSVStreamsRows verifies BOM, header, CRLF line endings, quoting
// and the row count of the streamed CSV export.
func TestExportCSVStreamsRows(t *testing.T) {
	st := seedExportStore(t)
	s := &DataService{Store: st}
	path, err := s.ExportCSV()
	if err != nil {
		t.Fatalf("export csv: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.HasPrefix(text, "\xEF\xBB\xBF") {
		t.Fatal("CSV must start with a UTF-8 BOM")
	}
	text = strings.TrimPrefix(text, "\xEF\xBB\xBF")
	lines := strings.Split(strings.TrimSuffix(text, "\r\n"), "\r\n")
	if len(lines) != 4 { // header + 3 entries
		t.Fatalf("line count = %d, want 4:\n%s", len(lines), text)
	}
	if lines[0] != "activity,started_at,ended_at,duration_seconds,note,source" {
		t.Fatalf("header = %q", lines[0])
	}
	// Newest first: the 09-05 session (quoted note), the 09-04 plain one,
	// then the 09-03 cross-midnight start.
	if !strings.Contains(lines[1], "\"note, with \"\"quotes\"\" and,\nnewline\"") {
		t.Fatalf("quoted note missing in %q", lines[1])
	}
	if !strings.Contains(lines[3], "跨午夜") {
		t.Fatalf("unicode note missing in %q", lines[3])
	}
	if strings.Contains(text, "\nactivity,") || strings.Count(text, "\r\n") != 4 { // header + 3 rows
		t.Fatalf("rows must use CRLF endings:\n%s", text)
	}
}
