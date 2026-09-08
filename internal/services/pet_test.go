package services

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mikuchrono/internal/store"
)

func newPetTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "pet.db"))
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
	var lastSave time.Time
	bump := func() {
		mu.Lock()
		saves++
		lastSave = time.Now()
		mu.Unlock()
	}
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return saves
	}
	lastAt := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return lastSave
	}

	const interval = 60 * time.Millisecond
	// Poll until cond holds instead of assuming fixed sleeps land where the
	// assertions need them: a slow scheduler must not turn the timing into
	// CI flakes.
	waitFor := func(cond func() bool, msg string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatal(msg)
			}
			time.Sleep(interval / 3)
		}
	}

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
	waitFor(func() bool { return count() == 2 }, "trailing save missing")

	// After the burst the throttle is idle again: once a full interval has
	// passed since the trailing save, the next call is a new leading edge.
	waitFor(func() bool { return time.Since(lastAt()) >= interval }, "trailing save never aged past the interval")
	th.trigger(interval, bump)
	if got := count(); got != 3 {
		t.Fatalf("post-burst trigger saves = %d, want 3", got)
	}
}

// TestPetSettings covers the store-backed pet settings: close-action
// round-trip with validation, the goal-notify default (missing = enabled),
// and corrupt pet positions treated as never-saved.
func TestPetSettings(t *testing.T) {
	st := newPetTestStore(t)
	s := &PetService{Store: st}

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
	if err := s.SavePetPosition(12, 34); err != nil {
		t.Fatal(err)
	}
	if pos, err := s.GetPetPosition(); err != nil || !pos.Set || pos.X != 12 || pos.Y != 34 {
		t.Fatalf("pet position = %+v, %v", pos, err)
	}
	if err := st.SetSetting(keyPetX, "not-a-number"); err != nil {
		t.Fatal(err)
	}
	if pos, err := s.GetPetPosition(); err != nil || pos.Set {
		t.Fatalf("corrupt pet position must read as unset: %+v, %v", pos, err)
	}
}

// TestPetPositionLegacyMigration pins the lazy migration from the pre-pet
// floating-ball keys: while pet_x/pet_y are missing, the ball position is
// served; the first save switches to the new keys for good.
func TestPetPositionLegacyMigration(t *testing.T) {
	st := newPetTestStore(t)
	s := &PetService{Store: st}

	if err := st.SetSettings([][2]string{
		{keyLegacyPetX, "100"},
		{keyLegacyPetY, "200"},
	}); err != nil {
		t.Fatal(err)
	}
	pos, err := s.GetPetPosition()
	if err != nil || !pos.Set || pos.X != 100 || pos.Y != 200 {
		t.Fatalf("legacy ball position = %+v, %v", pos, err)
	}

	// The first save writes the new keys, which now take precedence.
	if err := s.SavePetPosition(300, 400); err != nil {
		t.Fatal(err)
	}
	if pos, err := s.GetPetPosition(); err != nil || pos.X != 300 || pos.Y != 400 {
		t.Fatalf("pet position after save = %+v, %v", pos, err)
	}

	// A half-migrated install (only one legacy key) reads as never saved.
	st2 := newPetTestStore(t)
	if err := st2.SetSetting(keyLegacyPetX, "100"); err != nil {
		t.Fatal(err)
	}
	if pos, err := (&PetService{Store: st2}).GetPetPosition(); err != nil || pos.Set {
		t.Fatalf("half legacy position must read as unset: %+v, %v", pos, err)
	}
}
