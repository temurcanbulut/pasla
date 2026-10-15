//go:build !windows

package main

import "os"

// hasConsole reports whether output has somewhere visible to go. On Linux a
// program started from the file manager has no terminal; stdout is then not
// a character device and logs go to a file instead.
var hasConsole = func() bool {
	st, err := os.Stderr.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}()

// startTray is a no-op outside Windows.
func startTray(open, quit func(), labels trayLabels) func() { return func() {} }

// alert is only needed where a program can run without any console.
func alert(string) {}

func trayNotify(title, text string) {}
