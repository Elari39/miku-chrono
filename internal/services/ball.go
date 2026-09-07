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

// App-wide event names used by the ball feature.
const (
	EventTimerStopped = "timer:stopped"
	EventTimerStarted = "timer:started"
	EventBallToast    = "ball:toast"
)

const (
	keyBallX       = "ball_x"
	keyBallY       = "ball_y"
	keyCloseAction = "close_action"

	// keyGoalNotify gates the daily-goal notification. A missing value means
	// enabled: the notifier defaults to on so the feature works out of the box.
	keyGoalNotify = "goal_notify_enabled"

	// Main-window geometry keys, written throttled on move/resize.
	keyWinX = "win_x"
	keyWinY = "win_y"
	keyWinW = "win_w"
	keyWinH = "win_h"

	// positionPersistInterval throttles ball-position writes during drags.
	positionPersistInterval = time.Second
)

// BallService drives the floating ball window and the window-level behaviour
// around it: toggling the main window, hiding/showing the ball, persisting the
// ball position and the close-button behaviour. The App/window/Timer fields
// are injected by main.go after window creation.
type BallService struct {
	Store      *store.Store
	App        *application.App
	MainWindow *application.WebviewWindow
	BallWindow *application.WebviewWindow
	Timer      *TimerService

	mu       sync.Mutex
	quitting bool

	// Independent throttles: the ball and the main window must not suppress
	// each other's saves (they used to share one lastSaved timestamp).
	ballPersist persistThrottle
	winPersist  persistThrottle

	// Registration guards: the bound Start* methods are callable from the
	// frontend, and a repeated call must not register the handlers twice.
	positionPersistOnce sync.Once
	windowPersistOnce   sync.Once
}

// --- bound methods (called from the frontend) ---

// ToggleMainWindow shows the main window when it is hidden and hides it when
// it is visible. Returns the resulting visibility.
func (s *BallService) ToggleMainWindow() (bool, error) {
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

// HideBall hides the floating ball. When the main window is hidden too, it is
// shown so the app stays reachable (the tray icon can also restore it).
func (s *BallService) HideBall() error {
	if s.BallWindow == nil {
		return nil
	}
	s.BallWindow.Hide()
	if s.MainWindow != nil && !s.MainWindow.IsVisible() {
		s.MainWindow.Show()
	}
	return nil
}

// ShowBall brings the floating ball back (settings page / restore entry).
func (s *BallService) ShowBall() error {
	if s.BallWindow == nil {
		return nil
	}
	s.BallWindow.Show()
	return nil
}

// IsBallVisible reports whether the floating ball is currently shown.
func (s *BallService) IsBallVisible() bool {
	return s.BallWindow != nil && s.BallWindow.IsVisible()
}

// GetBallPosition returns the persisted ball position; Set is false when no
// position has ever been saved.
func (s *BallService) GetBallPosition() (models.BallPosition, error) {
	var pos models.BallPosition
	x, _, err := s.Store.GetSetting(keyBallX)
	if err != nil {
		return pos, err
	}
	y, _, err := s.Store.GetSetting(keyBallY)
	if err != nil {
		return pos, err
	}
	xi, errX := strconv.Atoi(x)
	yi, errY := strconv.Atoi(y)
	if len(x) == 0 || len(y) == 0 || errX != nil || errY != nil {
		// Missing or corrupt: treat as never saved.
		return pos, nil
	}
	pos.X, pos.Y, pos.Set = xi, yi, true
	return pos, nil
}

// SaveBallPosition persists the ball position across restarts.
func (s *BallService) SaveBallPosition(x, y int) error {
	if err := s.Store.SetSetting(keyBallX, strconv.Itoa(x)); err != nil {
		return err
	}
	return s.Store.SetSetting(keyBallY, strconv.Itoa(y))
}

// GetCloseAction returns the stored close-button behaviour. The empty string
// means "never chosen yet" (treated as CloseActionHide everywhere; the main
// window prompts the user once in that case).
func (s *BallService) GetCloseAction() (string, error) {
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
func (s *BallService) SetCloseAction(action string) error {
	if action != CloseActionHide && action != CloseActionQuit {
		return fmt.Errorf("关闭行为必须是 hide 或 quit")
	}
	return s.Store.SetSetting(keyCloseAction, action)
}

// GetAutostart reports whether the app is registered to launch at boot
// (Windows Run key; false and nil on unsupported platforms).
func (s *BallService) GetAutostart() (bool, error) {
	return autostartEnabled()
}

// SetAutostart registers or removes the boot launch entry. Enabling writes
// the --minimized flag too, so a boot launch starts silently.
func (s *BallService) SetAutostart(enabled bool) error {
	return setAutostart(enabled)
}

// GetGoalNotifyEnabled reports whether the daily-goal notification is on.
// A missing setting means enabled (the default).
func (s *BallService) GetGoalNotifyEnabled() (bool, error) {
	v, found, err := s.Store.GetSetting(keyGoalNotify)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	return v == "1", nil
}

// SetGoalNotifyEnabled toggles the daily-goal notification.
func (s *BallService) SetGoalNotifyEnabled(enabled bool) error {
	v := "0"
	if enabled {
		v = "1"
	}
	return s.Store.SetSetting(keyGoalNotify, v)
}

// GetMainWindowBounds returns the persisted main-window geometry. Set is
// false when the window has never been moved or resized.
func (s *BallService) GetMainWindowBounds() (models.WindowBounds, error) {
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

// SaveMainWindowBounds persists the main-window geometry across restarts.
func (s *BallService) SaveMainWindowBounds(x, y, width, height int) error {
	for _, kv := range [][2]string{
		{keyWinX, strconv.Itoa(x)},
		{keyWinY, strconv.Itoa(y)},
		{keyWinW, strconv.Itoa(width)},
		{keyWinH, strconv.Itoa(height)},
	} {
		if err := s.Store.SetSetting(kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}

// --- internal helpers (menu callbacks / window hooks) ---

// ToggleTimerFromMenu stops the running timer, or resumes the last one when
// idle, from the ball's context menu / system tray. TimerService broadcasts
// timer:started / timer:stopped itself after each successful change, so this
// wrapper only surfaces failures as a transient toast.
func (s *BallService) ToggleTimerFromMenu() {
	if s.Timer == nil || s.App == nil {
		return
	}
	st, err := s.Timer.GetState()
	if err != nil {
		s.App.Event.Emit(EventBallToast, "读取计时状态失败")
		return
	}
	if st.Running {
		if _, err := s.Timer.Stop(); err != nil {
			s.App.Event.Emit(EventBallToast, err.Error())
		}
		return
	}
	if _, err := s.Timer.StartLast(); err != nil {
		s.App.Event.Emit(EventBallToast, err.Error())
	}
}

// IsQuitting reports whether a real app quit is in progress (used by the
// close hook to let the quit through without cancelling it).
func (s *BallService) IsQuitting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quitting
}

// QuitApp terminates the whole application; the close hook no longer cancels.
func (s *BallService) QuitApp() {
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

// StartPositionPersist throttled-saves the ball position whenever the ball
// window moves (drag or programmatic restore). Safe to call before Run;
// repeated calls register the handler only once.
func (s *BallService) StartPositionPersist() {
	if s.BallWindow == nil {
		return
	}
	s.positionPersistOnce.Do(func() {
		s.BallWindow.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) {
			s.persistPositionSoon()
		})
	})
}

// StartMainWindowPersist throttled-saves the main-window bounds whenever it
// moves or resizes. Safe to call before Run; repeated calls register the
// handlers only once.
func (s *BallService) StartMainWindowPersist() {
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

func (s *BallService) persistMainWindowSoon() {
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

func (s *BallService) persistPositionSoon() {
	if s.BallWindow == nil {
		return
	}
	s.ballPersist.trigger(positionPersistInterval, func() {
		if s.BallWindow == nil {
			return
		}
		x, y := s.BallWindow.Position()
		_ = s.SaveBallPosition(x, y)
	})
}
