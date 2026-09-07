package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrintfWritesTimestampedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	l, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	l.Printf("goal reached for %s", "学习")
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// After Close, Printf must be a silent no-op, not a panic.
	l.Printf("discarded")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := string(raw)
	if !strings.Contains(line, "goal reached for 学习") {
		t.Fatalf("message missing: %q", line)
	}
	if _, err := time.Parse("2006-01-02 15:04:05 ", line[:20]); err != nil {
		t.Fatalf("line must start with a timestamp: %q", line)
	}
	if !strings.HasSuffix(line, "\n") {
		t.Fatalf("line must be newline-terminated: %q", line)
	}
}

func TestRotationKeepsOldGeneration(t *testing.T) {
	oldCap := maxLogBytes
	maxLogBytes = 200
	t.Cleanup(func() { maxLogBytes = oldCap })

	path := filepath.Join(t.TempDir(), "app.log")
	l, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = l.Close() }()
	for i := 0; i < 20; i++ {
		l.Printf("line %d of the rotation test", i)
	}

	oldRaw, err := os.ReadFile(path + ".old")
	if err != nil {
		t.Fatalf(".old generation must exist after overflow: %v", err)
	}
	freshRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// .old holds the previous generation (it is replaced on every rotation);
	// the fresh file holds the tail. The contract is a bounded footprint
	// with the newest lines always present — not full history.
	if !strings.Contains(string(freshRaw), "line 19") {
		t.Fatalf("late lines must land in the fresh file: %q", freshRaw)
	}
	if !strings.Contains(string(oldRaw), "line ") {
		t.Fatalf(".old generation must hold earlier lines: %q", oldRaw)
	}
	if int64(len(freshRaw)) > 200+60 {
		t.Fatalf("fresh file far exceeds the cap: %d bytes", len(freshRaw))
	}
}

func TestNilSafety(t *testing.T) {
	// A nil *Logger and an unset Default must both discard silently.
	var l *Logger
	l.Printf("no panic")
	saved := Default
	Default = nil
	t.Cleanup(func() { Default = saved })
	Printf("no panic")
}
