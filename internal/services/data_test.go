package services

import (
	"path/filepath"
	"testing"
	"time"

	"mikuchrono/internal/store"
)

func TestCSVFieldEscaping(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"学习", "学习"},
		{"", ""},
		{"plain text", "plain text"},
		{"a,b", `"a,b"`},
		{"say \"hi\"", `"say ""hi"""`},
		{"line\r\nbreak", "\"line\r\nbreak\""},
		{"note,with \"quotes\" and,\nnewline", "\"note,with \"\"quotes\"\" and,\nnewline\""},
	}
	for _, c := range cases {
		if got := csvField(c.in); got != c.want {
			t.Fatalf("csvField(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestExportHooks verifies that the injected shell hooks are used as-is:
// the save dialog's chosen path is where the export lands and a missing
// OpenDir hook reports the platform as unsupported instead of failing
// cryptically.
func TestExportHooks(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := &DataService{Store: st}

	dir := t.TempDir()
	got := ""
	s.SaveFile = func(defaultName, filterName, pattern string) (string, error) {
		if defaultName == "" || filterName == "" || pattern == "" {
			t.Fatalf("SaveFile args incomplete: %q %q %q", defaultName, filterName, pattern)
		}
		got = filepath.Join(dir, defaultName)
		return got, nil
	}
	path, err := s.ExportJSON()
	if err != nil {
		t.Fatalf("export json: %v", err)
	}
	if path != got {
		t.Fatalf("export path = %q, want the dialog's choice %q", path, got)
	}

	s2 := &DataService{Store: st}
	if err := s2.OpenDataDir(); err == nil {
		t.Fatal("OpenDataDir without a hook must report the platform as unsupported")
	}
}

// TestCachedStateProjects covers the tray pump's zero-SQL path: a fresh
// snapshot projects elapsed seconds arithmetically, Invalidate forces one
// re-read, and after Stop the projection turns idle.
func TestCachedStateProjects(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := &TimerService{Store: st}

	if _, err := s.Start(1); err != nil {
		t.Fatalf("start: %v", err)
	}
	projected, err := s.CachedState(time.Now().Add(30 * time.Second))
	if err != nil {
		t.Fatalf("cached state: %v", err)
	}
	if !projected.Running || projected.ActivityID != 1 {
		t.Fatalf("projected state: %+v", projected)
	}
	if projected.SessionElapsedSeconds < 28 || projected.SessionElapsedSeconds > 35 {
		t.Fatalf("projected session = %d, want ~30", projected.SessionElapsedSeconds)
	}
	if projected.ElapsedSeconds < projected.SessionElapsedSeconds {
		t.Fatalf("chain total must include the session: %+v", projected)
	}

	// Invalidate (the wiring for out-of-band timer:stopped broadcasts)
	// forces the next read back to the store, which still shows running.
	s.Invalidate()
	fresh, err := s.CachedState(time.Now())
	if err != nil {
		t.Fatalf("post-invalidate state: %v", err)
	}
	if !fresh.Running {
		t.Fatalf("state after invalidate must re-read as running: %+v", fresh)
	}

	// Stop drops the snapshot; the next projection reflects the idle store.
	if _, err := s.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	idle, err := s.CachedState(time.Now())
	if err != nil {
		t.Fatalf("idle state: %v", err)
	}
	if idle.Running {
		t.Fatalf("state after stop must be idle: %+v", idle)
	}
	if idle.LastActivityID != 1 {
		t.Fatalf("idle state must carry the paused chain: %+v", idle)
	}
}
