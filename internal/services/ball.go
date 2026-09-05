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

	mu        sync.Mutex
	quitting  bool
	lastSaved time.Time
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

// --- internal helpers (menu callbacks / window hooks) ---

// ToggleTimerFromMenu stops the running timer, or resumes the last one when
// idle, from the ball's context menu / system tray. The change is broadcast
// so every window refreshes; failures surface as a transient toast.
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
			return
		}
		s.App.Event.Emit(EventTimerStopped)
		return
	}
	if _, err := s.Timer.StartLast(); err != nil {
		s.App.Event.Emit(EventBallToast, err.Error())
		return
	}
	s.App.Event.Emit(EventTimerStarted)
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

// StartPositionPersist throttled-saves the ball position whenever the ball
// window moves (drag or programmatic restore). Safe to call before Run.
func (s *BallService) StartPositionPersist() {
	if s.BallWindow == nil {
		return
	}
	s.BallWindow.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) {
		s.persistPositionSoon()
	})
}

func (s *BallService) persistPositionSoon() {
	s.mu.Lock()
	now := time.Now()
	if now.Sub(s.lastSaved) < positionPersistInterval {
		s.mu.Unlock()
		return
	}
	s.lastSaved = now
	s.mu.Unlock()
	if s.BallWindow == nil {
		return
	}
	x, y := s.BallWindow.Position()
	_ = s.SaveBallPosition(x, y)
}
