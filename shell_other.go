//go:build !windows

package main

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"mikuchrono/internal/services"
)

// wireDesktopShell stubs the platform-dependent hooks on non-Windows
// desktops; the app itself is Windows-only today (see README), so these
// exist only to keep portable builds compiling.
func wireDesktopShell(app *application.App, data *services.DataService) {
	_ = app
	data.OpenDir = func(string) error { return errors.New("当前平台不支持打开数据目录") }
}
