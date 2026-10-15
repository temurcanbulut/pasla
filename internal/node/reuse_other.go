//go:build !unix

package node

import "syscall"

// On Windows SO_REUSEADDR would let another program steal the port, so the
// discovery socket stays exclusive there.
func reuseAddr(network, address string, c syscall.RawConn) error { return nil }
