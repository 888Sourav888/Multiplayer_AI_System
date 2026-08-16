//go:build !windows

package ui

// enableWindowsVT is a no-op on non-Windows platforms.
func enableWindowsVT() {}
