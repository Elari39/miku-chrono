package services

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"mikuchrono/internal/models"
	"mikuchrono/internal/store"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Close-button behaviours for the main window, stored under the settings key
// set via SetCloseAction.
const (
	// CloseActionHide keeps the app running (main window hidden) when the
	// main window's close button is pressed. Default when never chosen.
	CloseActionHide = "hide"
	// CloseActionQuit terminates the whole application.
	CloseActionQuit = "quit"
)

// App-wide event names used by the desktop-pet feature.
const (
	EventTimerStopped = "timer:stopped"
	EventTimerStarted = "timer:started"
	EventPetToast     = "pet:toast"
	// EventGoalAchieved carries an activity name and is broadcast the moment
	// the goal notifier detects that an activity's live daily total (running
	// session included) crossed its daily goal.
	EventGoalAchieved = "goal:achieved"
	// EventPetMoving is emitted on every WindowDidMove of the pet window so
	// the front-end can play the dragging animation: the native drag loop
	// swallows webview mouse events, so the animation cannot be driven from
	// the page itself.
	EventPetMoving = "pet:moving"
)

const (
	keyPetX = "pet_x"
	keyPetY = "pet_y"

	// Legacy position keys written by the pre-pet floating ball. They are
	// only read as a fallback so an upgrade keeps the saved position; the
	// first SavePetPosition writes the new keys.
	keyLegacyPetX = "ball_x"
	keyLegacyPetY = "ball_y"

	keyCloseAction = "close_action"

	// keyGoalNotify gates the daily-goal notification. A missing value means
	// enabled: the notifier defaults to on so the feature works out of the box.
	keyGoalNotify = "goal_notify_enabled"

	// Main-window geometry keys, written throttled on move/resize.
	keyWinX = "win_x"
	keyWinY = "win_y"
	keyWinW = "win_w"
	keyWinH = "win_h"

	// positionPersistInterval throttles pet-position writes during drags.
	positionPersistInterval = time.Second
)

// PetService drives the desktop-pet window and the window-level behaviour
// around it: toggling the main window, hiding/showing the pet, persisting the
// pet position and the close-button behaviour. The App/window/Timer fields
// are injected by main.go after window creation.
type PetService struct {
	Store      *store.Store
	App        *application.App
	MainWindow *application.WebviewWindow
	PetWindow  *application.WebviewWindow
	Timer      *TimerService

	mu       sync.Mutex
	quitting bool

	// Independent throttles: the pet and the main window must not suppress
	// each other's saves (they used to share one lastSaved timestamp).
	petPersist persistThrottle
	winPersist persistThrottle

	// Registration guards: the bound Start* methods are callable from the
	// frontend, and a repeated call must not register the handlers twice.
	positionPersistOnce sync.Once
	windowPersistOnce   sync.Once
}

// --- bound methods (called from the frontend) ---

// ToggleMainWindow shows the main window when it is hidden and hides it when
// it is visible. Returns the resulting visibility.
func (s *PetService) ToggleMainWindow() (bool, error) {
	if s.MainWindow == nil {
		return false, nil
	}
	if s.MainWindow.IsVisible() {
		s.MainWindow.Hide()
		return false, nil
	}
	if s.MainWindow.IsMinimised() {
		s.MainWindow.UnMinimise()
	}
	s.MainWindow.Show()
	s.MainWindow.Focus()
	return true, nil
}

// HidePet hides the desktop pet. When the main window is hidden too, it is
// shown so the app stays reachable (the tray icon can also restore it).
func (s *PetService) HidePet() error {
	if s.PetWindow == nil {
		return nil
	}
	s.PetWindow.Hide()
	if s.MainWindow != nil && !s.MainWindow.IsVisible() {
		s.MainWindow.Show()
	}
	return nil
}

// ShowPet brings the desktop pet back (settings page / restore entry).
func (s *PetService) ShowPet() error {
	if s.PetWindow == nil {
		return nil
	}
	s.PetWindow.Show()
	return nil
}

// IsPetVisible reports whether the desktop pet is currently shown.
func (s *PetService) IsPetVisible() bool {
	return s.PetWindow != nil && s.PetWindow.IsVisible()
}

// GetPetPosition returns the persisted pet position; Set is false when no
// position has ever been saved.
func (s *PetService) GetPetPosition() (models.PetPosition, error) {
	var pos models.PetPosition
	x, y, set, err := s.readPosition(keyPetX, keyPetY)
	if err != nil {
		return pos, err
	}
	if !set {
		// Lazy migration: an install upgraded from the floating ball keeps
		// the ball's saved position until the first save writes pet_x/pet_y.
		x, y, set, err = s.readPosition(keyLegacyPetX, keyLegacyPetY)
		if err != nil {
			return pos, err
		}
	}
	if !set {
		return pos, nil
	}
	pos.X, pos.Y, pos.Set = x, y, true
	return pos, nil
}

// readPosition parses one position key pair; set is false when either key is
// missing or corrupt, mirroring the "never saved" semantics of the other
// settings readers.
func (s *PetService) readPosition(xKey, yKey string) (int, int, bool, error) {
	x, _, err := s.Store.GetSetting(xKey)
	if err != nil {
		return 0, 0, false, err
	}
	y, _, err := s.Store.GetSetting(yKey)
	if err != nil {
		return 0, 0, false, err
	}
	xi, errX := strconv.Atoi(x)
	yi, errY := strconv.Atoi(y)
	if len(x) == 0 || len(y) == 0 || errX != nil || errY != nil {
		return 0, 0, false, nil
	}
	return xi, yi, true, nil
}

// SavePetPosition persists the pet position across restarts, in one
// transaction so the pair can never be half-written by a crash mid-drag.
func (s *PetService) SavePetPosition(x, y int) error {
	return s.Store.SetSettings([][2]string{
		{keyPetX, strconv.Itoa(x)},
		{keyPetY, strconv.Itoa(y)},
	})
}

// GetCloseAction returns the stored close-button behaviour. The empty string
// means "never chosen yet" (treated as CloseActionHide everywhere; the main
// window prompts the user once in that case).
func (s *PetService) GetCloseAction() (string, error) {
	v, found, err := s.Store.GetSetting(keyCloseAction)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return v, nil
}

// SetCloseAction stores the close-button behaviour ("hide" or "quit").
func (s *PetService) SetCloseAction(action string) error {
	if action != CloseActionHide && action != CloseActionQuit {
		return fmt.Errorf("关闭行为必须是 hide 或 quit")
	}
	return s.Store.SetSetting(keyCloseAction, action)
}

// GetAutostart reports whether the app is registered to launch at boot
// (Windows Run key; false and nil on unsupported platforms).
func (s *PetService) GetAutostart() (bool, error) {
	return autostartEnabled()
}

// SetAutostart registers or removes the boot launch entry. Enabling writes
// the --minimized flag too, so a boot launch starts silently.
func (s *PetService) SetAutostart(enabled bool) error {
	return setAutostart(enabled)
}

// GetGoalNotifyEnabled reports whether the daily-goal notification is on.
// A missing setting means enabled (the default); only an explicit "0"
// disables, matching how the notifier itself reads the key.
func (s *PetService) GetGoalNotifyEnabled() (bool, error) {
	v, found, err := s.Store.GetSetting(keyGoalNotify)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	return v != "0", nil
}

// SetGoalNotifyEnabled toggles the daily-goal notification.
func (s *PetService) SetGoalNotifyEnabled(enabled bool) error {
	v := "0"
	if enabled {
		v = "1"
	}
	return s.Store.SetSetting(keyGoalNotify, v)
}

// GetMainWindowBounds returns the persisted main-window geometry. Set is
// false when the window has never been moved or resized.
func (s *PetService) GetMainWindowBounds() (models.WindowBounds, error) {
	var b models.WindowBounds
	x, _, err := s.Store.GetSetting(keyWinX)
	if err != nil {
		return b, err
	}
	y, _, err := s.Store.GetSetting(keyWinY)
	if err != nil {
		return b, err
	}
	w, _, err := s.Store.GetSetting(keyWinW)
	if err != nil {
		return b, err
	}
	h, _, err := s.Store.GetSetting(keyWinH)
	if err != nil {
		return b, err
	}
	xi, errX := strconv.Atoi(x)
	yi, errY := strconv.Atoi(y)
	wi, errW := strconv.Atoi(w)
	hi, errH := strconv.Atoi(h)
	if len(x) == 0 || len(y) == 0 || len(w) == 0 || len(h) == 0 ||
		errX != nil || errY != nil || errW != nil || errH != nil {
		// Missing or corrupt: treat as never saved.
		return b, nil
	}
	b.X, b.Y, b.Width, b.Height, b.Set = xi, yi, wi, hi, true
	// Clamp to the visible screen here so a position saved while a monitor
	// was connected cannot put the window out of reach after it is gone.
	b.X, b.Y = clampWindowBounds(b.X, b.Y, b.Width, b.Height)
	return b, nil
}

// SaveMainWindowBounds persists the main-window geometry across restarts, in
// one transaction so a crash mid-drag can never leave a mixed geometry (new X
// with old height).
func (s *PetService) SaveMainWindowBounds(x, y, width, height int) error {
	return s.Store.SetSettings([][2]string{
		{keyWinX, strconv.Itoa(x)},
		{keyWinY, strconv.Itoa(y)},
		{keyWinW, strconv.Itoa(width)},
		{keyWinH, strconv.Itoa(height)},
	})
}

// --- internal helpers (menu callbacks / window hooks) ---

// ToggleTimerFromMenu stops the running timer, or resumes the last one when
// idle, from the system tray. TimerService broadcasts
// timer:started / timer:stopped itself after each successful change, so this
// wrapper only surfaces failures as a transient toast.
func (s *PetService) ToggleTimerFromMenu() {
	if s.Timer == nil || s.App == nil {
		return
	}
	st, err := s.Timer.GetState()
	if err != nil {
		s.App.Event.Emit(EventPetToast, "读取计时状态失败")
		return
	}
	if st.Running {
		if _, err := s.Timer.Stop(); err != nil {
			s.App.Event.Emit(EventPetToast, err.Error())
		}
		return
	}
	if _, err := s.Timer.StartLast(); err != nil {
		s.App.Event.Emit(EventPetToast, err.Error())
	}
}

// IsQuitting reports whether a real app quit is in progress (used by the
// close hook to let the quit through without cancelling it).
func (s *PetService) IsQuitting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quitting
}

// QuitApp terminates the whole application; the close hook no longer cancels.
func (s *PetService) QuitApp() {
	s.mu.Lock()
	s.quitting = true
	s.mu.Unlock()
	if s.App != nil {
		s.App.Quit()
	}
}

// clampToRect pulls a window's top-left back inside the virtual-screen rect
// [left,right)x[top,bottom) so a position saved while a monitor was
// connected cannot put the window out of reach after it is gone. Pure
// geometry, split out of clampWindowBounds for testing.
func clampToRect(x, y, width, height, left, top, right, bottom int) (int, int) {
	x = max(x, left)
	y = max(y, top)
	if x+width > right {
		x = right - width
		x = max(x, left)
	}
	if y+height > bottom {
		y = bottom - height
		y = max(y, top)
	}
	return x, y
}

// persistThrottle coalesces rapid move/resize events into bounded saves.
// The first event after an idle interval saves immediately; events arriving
// during the interval schedule exactly one trailing save that reads the
// window geometry when it fires — so the final position of a drag is always
// persisted instead of being silently dropped (a leading-edge-only throttle
// loses the last update whenever it lands inside the interval).
type persistThrottle struct {
	mu       sync.Mutex
	last     time.Time
	trailing *time.Timer
}

// trigger runs save at most once per interval, plus one trailing save after
// a burst of calls so the latest geometry always lands on disk.
func (t *persistThrottle) trigger(interval time.Duration, save func()) {
	t.mu.Lock()
	if t.trailing != nil {
		// The scheduled trailing save reads the geometry at its fire time and
		// therefore already covers this event; nothing to do.
		t.mu.Unlock()
		return
	}
	now := time.Now()
	if now.Sub(t.last) >= interval {
		t.last = now
		t.mu.Unlock()
		save()
		return
	}
	t.trailing = time.AfterFunc(interval-now.Sub(t.last), func() {
		t.mu.Lock()
		t.trailing = nil
		t.last = time.Now()
		t.mu.Unlock()
		save()
	})
	t.mu.Unlock()
}

// StartPositionPersist throttled-saves the pet position whenever the pet
// window moves (drag or programmatic restore), and emits pet:moving on every
// move so the front-end can keep its dragging animation in sync. Safe to
// call before Run; repeated calls register the handler only once.
func (s *PetService) StartPositionPersist() {
	if s.PetWindow == nil {
		return
	}
	s.positionPersistOnce.Do(func() {
		s.PetWindow.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) {
			if s.App != nil {
				s.App.Event.Emit(EventPetMoving)
			}
			s.persistPositionSoon()
		})
	})
}

// StartMainWindowPersist throttled-saves the main-window bounds whenever it
// moves or resizes. Safe to call before Run; repeated calls register the
// handlers only once.
func (s *PetService) StartMainWindowPersist() {
	if s.MainWindow == nil {
		return
	}
	s.windowPersistOnce.Do(func() {
		s.MainWindow.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) {
			s.persistMainWindowSoon()
		})
		s.MainWindow.OnWindowEvent(events.Common.WindowDidResize, func(*application.WindowEvent) {
			s.persistMainWindowSoon()
		})
	})
}

func (s *PetService) persistMainWindowSoon() {
	if s.MainWindow == nil {
		return
	}
	s.winPersist.trigger(positionPersistInterval, func() {
		if s.MainWindow == nil {
			return
		}
		x, y := s.MainWindow.Position()
		w, h := s.MainWindow.Size()
		_ = s.SaveMainWindowBounds(x, y, w, h)
	})
}

func (s *PetService) persistPositionSoon() {
	if s.PetWindow == nil {
		return
	}
	s.petPersist.trigger(positionPersistInterval, func() {
		if s.PetWindow == nil {
			return
		}
		x, y := s.PetWindow.Position()
		_ = s.SavePetPosition(x, y)
	})
}
