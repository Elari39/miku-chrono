package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"mikuchrono/internal/services"
	"mikuchrono/internal/store"
)

// Wails uses Go's `embed` package to embed the frontend files into the
// binary. Any files in the frontend/dist folder will be embedded into the
// binary and made available to the frontend.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Open (and migrate/seed) the SQLite database under the user config dir.
	st, err := store.Open(mustDefaultDBPath())
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer func() { _ = st.Close() }()

	// Build the services the frontend binds to.
	categoryService := &services.CategoryService{Store: st}
	activityService := &services.ActivityService{Store: st}
	timerService := &services.TimerService{Store: st}
	entryService := &services.EntryService{Store: st}
	statsService := &services.StatsService{Store: st}
	dataService := &services.DataService{Store: st}
	ballService := &services.BallService{Store: st}

	app := application.New(application.Options{
		Name:        "Miku Chrono",
		Description: "多活动打卡计时",
		Services: []application.Service{
			application.NewService(categoryService),
			application.NewService(activityService),
			application.NewService(timerService),
			application.NewService(entryService),
			application.NewService(statsService),
			application.NewService(dataService),
			application.NewService(ballService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "Miku Chrono",
		Width:     1080,
		Height:    720,
		MinWidth:  920,
		MinHeight: 640,
		URL:       "/",
		// Cream canvas from DESIGN.md so there is no white flash at startup.
		BackgroundColour: application.NewRGB(250, 249, 245),
	})

	// The floating ball: a small transparent frameless window that always
	// stays on top. Its size is fixed at creation — resizing a
	// frameless+transparent window on Windows leaves the grown area
	// click-through, so the ball never changes size at runtime.
	ballWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "ball",
		Title:            "Miku Chrono 悬浮球",
		Width:            240,
		Height:           56,
		DisableResize:    true,
		Frameless:        true,
		AlwaysOnTop:      true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		InitialPosition:  application.WindowXY,
		X:                0,
		Y:                0,
		URL:              "/#/ball",
		StartState:       application.WindowStateNormal,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
			WindowDidMoveDebounceMS:           200,
		},
	})

	// Wire the ball service to the app/windows, then start its helpers.
	ballService.App = app
	ballService.MainWindow = mainWindow
	ballService.BallWindow = ballWindow
	ballService.Timer = timerService
	ballService.StartPositionPersist()
	// Broadcast timer state changes from the shared success paths so every
	// window (ball, tray, other views) refreshes right after start/stop/clear.
	broadcast := func(event string) { app.Event.Emit(event) }
	timerService.Emit = broadcast
	dataService.Emit = broadcast

	// Closing the main window hides it (default) or quits the app, per the
	// stored close_action. The hook cancels the close first; a real quit is
	// flagged via QuitApp so the hook lets it through.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if ballService.IsQuitting() {
			return
		}
		e.Cancel()
		action, _ := ballService.GetCloseAction()
		if action == services.CloseActionQuit {
			ballService.QuitApp()
			return
		}
		mainWindow.Hide()
	})

	// Native right-click mini menu for the ball, opened from the ball page
	// via the CSS property `--custom-contextmenu: ball-menu`. The timer item
	// toggles between 开始计时/停止计时 and the window item between
	// 显示/隐藏主窗口; labels are kept fresh by the tray state pump.
	ballMenu := app.ContextMenu.New()
	ballTimerItem := ballMenu.Add("开始计时").OnClick(func(_ *application.Context) {
		ballService.ToggleTimerFromMenu()
	})
	ballToggleItem := ballMenu.Add("显示主窗口").OnClick(func(_ *application.Context) {
		_, _ = ballService.ToggleMainWindow()
	})
	ballMenu.Add("退出悬浮球").OnClick(func(_ *application.Context) {
		_ = ballService.HideBall()
	})
	app.ContextMenu.Add("ball-menu", ballMenu)

	// Notification-area icon: guarantees the app stays reachable and gives
	// quick access to the main window, the running timer and quitting.
	setupSystemTray(app, ballService, ballTimerItem, ballToggleItem)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func mustDefaultDBPath() string {
	path, err := store.DefaultPath()
	if err != nil {
		log.Fatalf("定位数据目录失败: %v", err)
	}
	return path
}
