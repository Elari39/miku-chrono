//go:build !windows

package services

// clampWindowBounds is a no-op on platforms without virtual-screen metrics;
// the saved position is restored as-is.
func clampWindowBounds(x, y, width, height int) (int, int) { return x, y }
