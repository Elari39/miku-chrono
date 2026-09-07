//go:build windows

package main

import (
	"os/exec"

	"github.com/wailsapp/wails/v3/pkg/application"

	"mikuchrono/internal/services"
)

// wireDesktopShell connects the platform-dependent service hooks to their
// Windows implementations: Explorer for "open data directory" and the Wails
// native save dialog for exports.
func wireDesktopShell(app *application.App, data *services.DataService) {
	data.OpenDir = func(dir string) error {
		return exec.Command("explorer", dir).Start()
	}
	data.SaveFile = func(defaultName, filterName, pattern string) (string, error) {
		dialog := app.Dialog.SaveFile()
		dialog.SetFilename(defaultName)
		dialog.AddFilter(filterName, pattern)
		return dialog.PromptForSingleSelection()
	}
}
