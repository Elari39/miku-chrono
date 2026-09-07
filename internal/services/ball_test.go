package services

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mikuchrono/internal/store"
)

func newBallTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "ball.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// TestClampToRect pins the pure geometry behind the window-position restore:
// in-rect positions pass through, off-screen ones are pulled back inside,
// and oversized windows anchor to the top-left corner.
func TestClampToRect(t *testing.T) {
	cases := []struct {
		name             string
		x, y, w, h       int
		wantX, wantY     int
		left, top, right int
		bottom           int
	}{
		{"inside stays", 100, 200, 400, 300, 100, 200, 0, 0, 1920, 1080},
		{"off left/top pulled in", -50, -10, 400, 300, 0, 0, 0, 0, 1920, 1080},
		{"off right/bottom pulled in", 1800, 900, 400, 300, 1520, 780, 0, 0, 1920, 1080},
		{"multi-monitor negative origin", -2000, -100, 400, 300, -1920, -50, -1920, -50, 100, 950},
		{"window larger than screen anchors top-left", 100, 100, 3000, 2000, 0, 0, 0, 0, 1920, 1080},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gx, gy := clampToRect(c.x, c.y, c.w, c.h, c.left, c.top, c.right, c.bottom)
			if gx != c.wantX || gy != c.wantY {
				t.Fatalf("clampToRect = (%d,%d), want (%d,%d)", gx, gy, c.wantX, c.wantY)
			}
		})
	}
}

// TestPersistThrottleLeadingAndTrailing pins the throttle contract: the
// first call saves immediately, a burst inside the interval schedules
// exactly one trailing save, and the trailing save fires so the latest
// geometry is not dropped.
func TestPersistThrottleLeadingAndTrailing(t *testing.T) {
	var th persistThrottle
	var mu sync.Mutex
	saves := 0
	bump := func() { mu.Lock(); saves++; mu.Unlock() }
	count := func() int { mu.Lock(); defer mu.Unlock(); return saves }

	const interval = 60 * time.Millisecond
	th.trigger(interval, bump) // leading edge: immediate
	if got := count(); got != 1 {
		t.Fatalf("after first trigger saves = %d, want 1", got)
	}

	// Burst inside the interval: no immediate save, one trailing scheduled.
	th.trigger(interval, bump)
	th.trigger(interval, bump)
	th.trigger(interval, bump)
	time.Sleep(interval / 3)
	if got := count(); got != 1 {
		t.Fatalf("burst must not save before the interval: saves = %d", got)
	}

	// The trailing save fires once the interval elapses.
	time.Sleep(2 * interval)
	if got := count(); got != 2 {
		t.Fatalf("trailing save missing: saves = %d, want 2", got)
	}

	// After the burst, the throttle is idle again: the next call is a new
	// leading edge.
	th.trigger(interval, bump)
	if got := count(); got != 3 {
		t.Fatalf("post-burst trigger saves = %d, want 3", got)
	}
}

// TestBallSettings covers the store-backed ball settings: close-action
// round-trip with validation, the goal-notify default (missing = enabled),
// and corrupt ball positions treated as never-saved.
func TestBallSettings(t *testing.T) {
	st := newBallTestStore(t)
	s := &BallService{Store: st}

	// Close action: unset means empty (the frontend then prompts once).
	if v, err := s.GetCloseAction(); err != nil || v != "" {
		t.Fatalf("unset close action = %q, %v", v, err)
	}
	if err := s.SetCloseAction("somewhere"); err == nil {
		t.Fatal("invalid close action must be rejected")
	}
	if err := s.SetCloseAction(CloseActionQuit); err != nil {
		t.Fatal(err)
	}
	if v, err := s.GetCloseAction(); err != nil || v != CloseActionQuit {
		t.Fatalf("close action = %q, %v", v, err)
	}

	// Goal notify defaults to enabled.
	if on, err := s.GetGoalNotifyEnabled(); err != nil || !on {
		t.Fatalf("goal notify default = %v, %v", on, err)
	}
	if err := s.SetGoalNotifyEnabled(false); err != nil {
		t.Fatal(err)
	}
	if on, err := s.GetGoalNotifyEnabled(); err != nil || on {
		t.Fatalf("goal notify after disable = %v, %v", on, err)
	}

	// Corrupt values fall back to "never saved" instead of erroring.
	if err := s.SaveBallPosition(12, 34); err != nil {
		t.Fatal(err)
	}
	if pos, err := s.GetBallPosition(); err != nil || !pos.Set || pos.X != 12 || pos.Y != 34 {
		t.Fatalf("ball position = %+v, %v", pos, err)
	}
	if err := st.SetSetting(keyBallX, "not-a-number"); err != nil {
		t.Fatal(err)
	}
	if pos, err := s.GetBallPosition(); err != nil || pos.Set {
		t.Fatalf("corrupt ball position must read as unset: %+v, %v", pos, err)
	}
}
