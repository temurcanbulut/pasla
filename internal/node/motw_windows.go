//go:build windows

package node

import "os"

// markFromNetwork adds the "Mark of the Web" to a received file, the same
// tag browsers add to downloads. Windows then shows its SmartScreen or
// Office "protected view" warning before running or opening it, so a file
// from another computer is never trusted like one created locally.
func markFromNetwork(path string) {
	_ = os.WriteFile(path+":Zone.Identifier", []byte("[ZoneTransfer]\r\nZoneId=3\r\n"), 0o644)
}
