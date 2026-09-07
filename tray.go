package main

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"mikuchrono/internal/applog"
	"mikuchrono/internal/services"

	_ "embed"
)

//go:embed build/appicon.png
var appIcon []byte

// setupSystemTray creates the notification-area icon with a control menu:
// show/hide the main window, start/stop the timer and quit. Unlike the
// floating ball it is always discoverable, so the app can never become
// unreachable even when both windows are hidden. The ball menu items are
// passed in so the same state pump can keep their labels in sync.
func setupSystemTray(app *application.App, ball *services.BallService, ballTimerItem, ballToggleItem *application.MenuItem) *application.SystemTray {
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetTooltip("Miku Chrono")

	menu := application.NewMenu()
	showItem := menu.Add("显示主窗口")
	timerItem := menu.Add("开始计时")
	menu.AddSeparator()
	quitItem := menu.Add("退出应用")

	showItem.OnClick(func(_ *application.Context) { _, _ = ball.ToggleMainWindow() })
	timerItem.OnClick(func(_ *application.Context) { ball.ToggleTimerFromMenu() })
	quitItem.OnClick(func(_ *application.Context) { ball.QuitApp() })
	tray.SetMenu(menu)

	// Left click brings the main window forward (idempotent, so a
	// double-click is harmless); right click shows the menu above.
	tray.OnClick(func() { showMainWindow(ball) })

	go pumpMenuState(tray, ball, showItem, timerItem, ballTimerItem, ballToggleItem)
	return tray
}

// showMainWindow shows and focuses the main window; a visible window is only
// focused, never hidden again.
func showMainWindow(ball *services.BallService) {
	if ball.MainWindow == nil {
		return
	}
	if !ball.MainWindow.IsVisible() {
		ball.MainWindow.Show()
	}
	if ball.MainWindow.IsMinimised() {
		ball.MainWindow.UnMinimise()
	}
	ball.MainWindow.Focus()
}

// pumpMenuState keeps the tray tooltip and the tray/ball menu labels in sync
// with the timer state. Each tick projects elapsed seconds from the timer
// service's in-memory snapshot (CachedState) instead of re-reading SQLite
// every second; the snapshot re-syncs on state changes and at most once per
// cache TTL, so out-of-band changes converge without per-tick queries.
func pumpMenuState(tray *application.SystemTray, ball *services.BallService, trayShowItem, trayTimerItem, ballTimerItem, ballToggleItem *application.MenuItem) {
	last := ""
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// The pump must survive its own bugs: a panic here used to take the
		// whole app down. Recover, log, keep ticking.
		func() {
			defer func() {
				if r := recover(); r != nil {
					applog.Printf("tray pump panic: %v\n%s", r, debug.Stack())
				}
			}()
			if ball.Timer == nil {
				return
			}
			st, err := ball.Timer.CachedState(time.Now())
			if err != nil {
				return
			}
			var tooltip, timerLabel string
			switch {
			case st.Running:
				elapsed := fmt.Sprintf("%s %s", st.ActivityName, formatElapsed(st.ElapsedSeconds))
				tooltip = "Miku Chrono · " + elapsed
				timerLabel = "停止计时 · " + elapsed
			case st.LastActivityID != 0:
				elapsed := fmt.Sprintf("%s %s", st.LastActivityName, formatElapsed(st.LastElapsedSeconds))
				tooltip = "Miku Chrono · 上次：" + elapsed
				timerLabel = "开始计时 · " + elapsed
			default:
				tooltip = "Miku Chrono · 未在计时"
				timerLabel = "开始计时"
			}
			showLabel := "显示主窗口"
			if ball.MainWindow != nil && ball.MainWindow.IsVisible() {
				showLabel = "隐藏主窗口"
			}
			sig := tooltip + "|" + timerLabel + "|" + showLabel
			if sig == last {
				return
			}
			last = sig
			application.InvokeSync(func() {
				tray.SetTooltip(tooltip)
				trayTimerItem.SetLabel(timerLabel)
				trayShowItem.SetLabel(showLabel)
				ballTimerItem.SetLabel(timerLabel)
				ballToggleItem.SetLabel(showLabel)
			})
		}()
	}
}

// formatElapsed renders seconds as MM:SS or H:MM:SS.
func formatElapsed(s int64) string {
	if s < 0 {
		s = 0
	}
	h, rem := s/3600, s%3600
	m, sec := rem/60, rem%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", m, sec)
}
