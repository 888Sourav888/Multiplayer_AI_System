//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode       = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode       = kernel32.NewProc("SetConsoleMode")
	procGetStdHandle         = kernel32.NewProc("GetStdHandle")
)

const (
	stdOutputHandle              = ^uintptr(10) // (DWORD)(-11) = 0xFFFFFFF5
	stdErrorHandle               = ^uintptr(11) // (DWORD)(-12) = 0xFFFFFFF4
	enableVirtualTerminalProc    = 0x0004
)

func getStdHandle(nStdHandle uintptr) uintptr {
	r, _, _ := procGetStdHandle.Call(nStdHandle)
	return r
}

func enableVTForHandle(handle uintptr) {
	var mode uint32
	procGetConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	procSetConsoleMode.Call(handle, uintptr(mode|enableVirtualTerminalProc))
}

// enableWindowsVT enables ANSI/VT100 escape processing on Windows consoles.
func enableWindowsVT() {
	enableVTForHandle(getStdHandle(stdOutputHandle))
	enableVTForHandle(getStdHandle(stdErrorHandle))
}
