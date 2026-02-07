//go:build windows

package main

import (
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
	procGetStdHandle   = kernel32.NewProc("GetStdHandle")
)

const (
	stdOutputHandle                 = ^uintptr(0) - 10 + 1 // STD_OUTPUT_HANDLE = -11
	enableVirtualTerminalProcessing = 0x0004
)

// enableANSI enables ANSI escape code processing on Windows 10+.
func enableANSI() {
	handle, _, _ := procGetStdHandle.Call(stdOutputHandle)
	if handle == 0 {
		return
	}
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return
	}
	procSetConsoleMode.Call(handle, uintptr(mode|enableVirtualTerminalProcessing))
}

func registerSignals(ch chan<- os.Signal) {
	// Windows only supports SIGINT (Ctrl+C); SIGTERM is not available.
	signal.Notify(ch, syscall.SIGINT)
}
