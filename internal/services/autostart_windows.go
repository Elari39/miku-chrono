//go:build windows

package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// autostartValueName is the value name under the per-user Run key. Renaming
// or moving the product may leave a stale entry; SetAutostart(false) removes
// it, and enabling always rewrites the current executable path.
const autostartValueName = "Miku Chrono"

// autostartRunKey is the HKCU Run key: no admin rights needed, launches the
// app at logon for the current user only.
const autostartRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// autostartCommand builds the Run value for this executable: the quoted
// absolute path plus --minimized so a boot launch starts silently (desktop
// pet + tray only, no main window).
func autostartCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("定位可执行文件失败: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("解析可执行文件路径失败: %w", err)
	}
	return fmt.Sprintf(`"%s" --minimized`, abs), nil
}

// autostartEnabled reports whether the app is registered in the Run key.
func autostartEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, autostartRunKey, registry.QUERY_VALUE)
	if err != nil {
		// The Run key always exists on a normal system; treat a missing key
		// as "not configured" rather than an error.
		return false, nil
	}
	defer k.Close()
	if _, _, err := k.GetStringValue(autostartValueName); err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// setAutostart registers (enabled) or removes (disabled) the boot launch
// entry. Disabling an entry that is not present is a no-op.
func setAutostart(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, autostartRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开开机启动注册表失败: %w", err)
	}
	defer k.Close()
	if enabled {
		cmd, err := autostartCommand()
		if err != nil {
			return err
		}
		if err := k.SetStringValue(autostartValueName, cmd); err != nil {
			return fmt.Errorf("写入开机启动注册表失败: %w", err)
		}
		return nil
	}
	if err := k.DeleteValue(autostartValueName); err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("清除开机启动注册表失败: %w", err)
	}
	return nil
}
