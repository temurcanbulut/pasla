//go:build !windows

package node

// markFromNetwork is a no-op outside Windows; Linux file managers have no
// equivalent of the Windows "Mark of the Web".
func markFromNetwork(string) {}
