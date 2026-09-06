//go:build !windows

package services

import "errors"

// errAutostartUnsupported is returned when enabling boot autostart on a
// platform for which no implementation exists yet. Disabling stays a no-op
// so settings pages can toggle off without friction.
var errAutostartUnsupported = errors.New("当前平台暂不支持开机自启动")

// autostartEnabled reports false on platforms without autostart support.
func autostartEnabled() (bool, error) { return false, nil }

// setAutostart only errors when enabling is attempted on an unsupported
// platform; disabling an unregistered entry is harmless.
func setAutostart(enabled bool) error {
	if enabled {
		return errAutostartUnsupported
	}
	return nil
}
