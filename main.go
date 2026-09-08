package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"mikuchrono/internal/applog"
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
	// Boot autostart (registry) launches with --minimized: the main window is
	// created hidden so only the desktop pet and tray appear; a manual
	// launch without the flag shows the window normally.
	startHidden := slices.Contains(os.Args[1:], "--minimized")

	// Open (and migrate/seed) the SQLite database under the user config dir.
	dbPath := mustDefaultDBPath()
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer func() { _ = st.Close() }()

	// File log next to the database for everything the user cannot see:
	// background goroutine errors and panics. Before it exists (or if it
	// fails to open) diagnostics stay on stderr only.
	if l, err := applog.Open(filepath.Join(filepath.Dir(dbPath), "app.log")); err == nil {
		defer func() { _ = l.Close() }()
		applog.SetDefault(l)
	} else {
		log.Printf("打开应用日志失败: %v", err)
	}

	// Daily-goal notifications: half-minute background check, stopped when
	// the app exits (the process is about to terminate anyway). Started only
	// after the Emit callback is wired below: the first check runs on its
	// own goroutine and reads Emit, so the field must be assigned before
	// that goroutine exists.
	goalNotifier := services.NewGoalNotifier(st)
	defer goalNotifier.Stop()

	// Build the services the frontend binds to.
	categoryService := &services.CategoryService{Store: st}
	activityService := &services.ActivityService{Store: st}
	timerService := &services.TimerService{Store: st}
	entryService := &services.EntryService{Store: st}
	statsService := &services.StatsService{Store: st}
	dataService := &services.DataService{Store: st}
	petService := &services.PetService{Store: st}

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
			application.NewService(petService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// A second launch (e.g. boot autostart racing a manual start) resignals
		// the running instance instead of creating a second tray + timer.
		// The callback reads petService.MainWindow only inside InvokeSync:
		// wails starts the callback goroutine during application.New (before
		// the field is assigned below), but the closure executes on the main
		// thread — the same thread that assigns the field before app.Run —
		// so the accesses are same-thread ordered, not racy.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "mikuchrono",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				application.InvokeSync(func() { showMainWindow(petService) })
			},
		},
	})

	// Goal achievement reaches every window the moment it is detected (the
	// notifier already counts the live running session): the system balloon
	// is shown by the notifier itself, and the desktop pet listens for the
	// same event to play its celebration easter egg.
	goalNotifier.Emit = func(event string, data ...any) { app.Event.Emit(event, data...) }
	goalNotifier.Start()

	// Restore the main window's saved geometry when one exists; the position
	// is clamped to the visible screen by GetMainWindowBounds.
	mainWindowOptions := application.WebviewWindowOptions{
		Name:      "main",
		Title:     "Miku Chrono",
		Width:     1080,
		Height:    720,
		MinWidth:  920,
		MinHeight: 640,
		Hidden:    startHidden,
		URL:       "/",
		// Cream canvas from DESIGN.md so there is no white flash at startup.
		BackgroundColour: application.NewRGB(250, 249, 245),
	}
	if bounds, err := petService.GetMainWindowBounds(); err == nil && bounds.Set {
		// WindowXY is required: the InitialPosition zero value is WindowCentered,
		// which makes Wails ignore the restored X/Y and center the window instead.
		mainWindowOptions.InitialPosition = application.WindowXY
		mainWindowOptions.X, mainWindowOptions.Y = bounds.X, bounds.Y
		mainWindowOptions.Width, mainWindowOptions.Height = bounds.Width, bounds.Height
	}
	mainWindow := app.Window.NewWithOptions(mainWindowOptions)

	// The desktop pet: a small transparent frameless window hosting a chibi
	// Hatsune Miku that reacts to the timer state. Its size is fixed at
	// creation and snugly fits the sprite shown 140px wide (the normalized
	// 660x900 source canvas scales down) — resizing a frameless+transparent
	// window on Windows leaves the grown area click-through, so the pet never
	// changes size at runtime.
	petWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "pet",
		Title:            "Miku Chrono 桌宠",
		Width:            180,
		Height:           240,
		DisableResize:    true,
		Frameless:        true,
		AlwaysOnTop:      true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		InitialPosition:  application.WindowXY,
		X:                0,
		Y:                0,
		URL:              "/#/pet",
		StartState:       application.WindowStateNormal,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
			WindowDidMoveDebounceMS:           200,
		},
	})

	// Wire the pet service to the app/windows, then start its helpers.
	petService.App = app
	petService.MainWindow = mainWindow
	petService.PetWindow = petWindow
	petService.Timer = timerService
	petService.StartPositionPersist()
	petService.StartMainWindowPersist()
	// Platform-dependent hooks (Explorer, native save dialog).
	wireDesktopShell(app, dataService)

	// Broadcast timer state changes from the shared success paths so every
	// window (pet, tray, other views) refreshes right after start/stop/clear.
	// A timer:stopped can also originate outside TimerService (a
	// settings-page clear, a stop-and-delete), so the tray pump's snapshot
	// cache is dropped on that event to keep its projection honest.
	broadcast := func(event string) { app.Event.Emit(event) }
	notifyTimer := func(event string) {
		broadcast(event)
		if event == services.EventTimerStopped {
			timerService.Invalidate()
		}
	}
	activityService.Emit = notifyTimer
	timerService.Emit = notifyTimer
	dataService.Emit = notifyTimer

	// Closing the main window hides it (default) or quits the app, per the
	// stored close_action. The hook cancels the close first; a real quit is
	// flagged via QuitApp so the hook lets it through.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if petService.IsQuitting() {
			return
		}
		e.Cancel()
		action, _ := petService.GetCloseAction()
		if action == services.CloseActionQuit {
			petService.QuitApp()
			return
		}
		mainWindow.Hide()
	})

	// Notification-area icon: guarantees the app stays reachable and gives
	// quick access to the main window, the running timer and quitting. The
	// pet itself has no context menu — its interactions are drag, click,
	// double-click and hover.
	setupSystemTray(app, petService)

	if err := app.Run(); err != nil {
		// log.Fatal would os.Exit past the defers above, skipping the WAL
		// checkpoint on st.Close and the notifier stop — clean up by hand.
		goalNotifier.Stop()
		_ = st.Close()
		applog.Printf("app run: %v", err)
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
