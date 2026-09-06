//go:build !windows

package services

// showNotification is a no-op on platforms without a notification
// implementation; the daily-goal check still runs and simply stays silent.
func showNotification(title, body string) {}
