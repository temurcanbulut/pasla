//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// Windows release builds use the GUI subsystem (-H windowsgui) so that
// double-clicking pasla.exe does not flash a black console window, which
// people unfamiliar with consoles easily take for malware. When the program
// is started from a terminal (cmd, PowerShell, Windows Terminal), it attaches
// to that terminal so `pasla send` and `pasla version` still print there.
//
// hasConsole reports whether output has somewhere visible to go.
var hasConsole = attachParentConsole()

func attachParentConsole() bool {
	// A console build already has its own console: nothing to do.
	if _, err := os.Stdout.Stat(); err == nil {
		return true
	}
	attach := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	if r, _, _ := attach.Call(attachParentProcess); r == 0 {
		return false
	}
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	os.Stdout, os.Stderr = out, out
	if in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = in
	}
	return true
}

// alert shows a message box when Pasla cannot start and there is no
// console to print the reason to.
func alert(text string) {
	if hasConsole {
		return
	}
	box := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString("Pasla")
	const mbIconWarning = 0x30
	box.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), mbIconWarning)
}
