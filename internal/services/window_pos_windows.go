//go:build windows

package services

import "golang.org/x/sys/windows"

var procGetSystemMetrics = windows.NewLazySystemDLL("user32.dll").NewProc("GetSystemMetrics")

// Virtual-screen metrics (span all monitors in a multi-monitor setup).
const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
)

// clampWindowBounds keeps a window fully inside the virtual screen so a
// saved position from a now-disconnected monitor cannot leave it
// unreachable. Falls back to the given position when the metrics are
// unavailable.
func clampWindowBounds(x, y, width, height int) (int, int) {
	sx, _, _ := procGetSystemMetrics.Call(smXVirtualScreen)
	sy, _, _ := procGetSystemMetrics.Call(smYVirtualScreen)
	sw, _, _ := procGetSystemMetrics.Call(smCXVirtualScreen)
	sh, _, _ := procGetSystemMetrics.Call(smCYVirtualScreen)
	if sw == 0 || sh == 0 {
		return x, y
	}
	left, top := int(sx), int(sy)
	return clampToRect(x, y, width, height, left, top, left+int(sw), top+int(sh))
}
