// Package applog is the app's minimal file logger: timestamped plain lines
// appended to a log file in the data directory, rotated to "<name>.old" once
// the active file outgrows its cap. It exists because the background
// goroutines (goal notifier, tray pump, notifications) used to fail
// silently — invisible when a user reports "reminders stopped working".
//
// The process-wide Default is nil until main installs one; every entry
// point is nil-safe, so tests and headless shells simply discard.
package applog

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// maxLogBytes caps the active log before it rotates; one .old generation
// keeps the on-disk footprint at roughly twice this. A var (not a const) so
// the rotation test can shrink it.
var maxLogBytes = int64(1 << 20)

// Default is the process-wide sink; nil until SetDefault. The package-level
// Printf discards while it is nil.
var Default *Logger

// SetDefault installs the process-wide sink.
func SetDefault(l *Logger) { Default = l }

// Printf writes one timestamped line to Default, discarding while unset.
func Printf(format string, args ...any) { Default.Printf(format, args...) }

// Logger appends timestamped lines to a file. Safe for concurrent use from
// every background goroutine.
type Logger struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
	// closed latches Close: afterwards Printf stays a silent no-op instead of
	// reopening the file behind the caller's back.
	closed bool
	// rotateAfter postpones the next rotation attempt: when rotation fails
	// (e.g. path+".old" held open by an editor or antivirus on Windows) the
	// file keeps growing, and retrying on every line would churn a
	// close/rename/open per write while dropping every line in between.
	// Rotation is retried only once the file outgrows this threshold (zero
	// means "no failed attempt pending").
	rotateAfter int64
}

// Open returns a Logger appending to path; the parent directory must exist.
func Open(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat log: %w", err)
	}
	return &Logger{path: path, f: f, size: info.Size()}, nil
}

// Close releases the underlying file; the Logger discards afterwards.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

// Printf writes one timestamped line. Write failures are swallowed by
// design: diagnostics must never take the app down — the next write or
// rotation gets a fresh chance.
func (l *Logger) Printf(format string, args ...any) {
	if l == nil {
		return
	}
	line := time.Now().Format("2006-01-02 15:04:05 ") + fmt.Sprintf(format, args...) + "\n"
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	if l.f == nil {
		l.reopenLocked()
		if l.f == nil {
			return
		}
	}
	if l.size > maxLogBytes && (l.rotateAfter == 0 || l.size > l.rotateAfter) {
		l.rotateLocked()
		if l.f == nil {
			return
		}
	}
	n, err := l.f.WriteString(line)
	l.size += int64(n)
	if err != nil || n < len(line) {
		// The handle may have gone bad (file moved, share violation): one
		// reopen-and-retry beats losing the line silently.
		l.reopenLocked()
		if l.f != nil {
			if n, err := l.f.WriteString(line); err == nil {
				l.size += int64(n)
			}
		}
	}
}

// reopenLocked opens path in append mode, re-syncing size from the file so
// the byte count stays honest after any external change.
func (l *Logger) reopenLocked() {
	if l.f != nil {
		_ = l.f.Close()
		l.f = nil
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	l.f = f
	if info, err := f.Stat(); err == nil {
		l.size = info.Size()
	}
}

// rotateLocked swaps the active file for path+".old" and opens a fresh one.
// On any failure it falls back to appending to the original handle — losing
// rotation beats losing logs entirely — and postpones the next attempt until
// the file outgrows the cap again, so a persistently locked ".old" cannot
// turn every subsequent write into a failing rotation cycle.
func (l *Logger) rotateLocked() {
	_ = l.f.Close()
	l.f = nil
	l.rotateAfter = 0
	if err := os.Rename(l.path, l.path+".old"); err == nil {
		f, err := os.OpenFile(l.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err == nil {
			l.f = f
			l.size = 0
			return
		}
	}
	l.reopenLocked()
	if l.f != nil {
		l.rotateAfter = l.size + maxLogBytes
	}
}
