//go:build unix

package node

import "syscall"

// reuseAddr lets several Pasla instances on one machine (other users, or
// tests) share the discovery port; the kernel delivers broadcasts to all.
func reuseAddr(network, address string, c syscall.RawConn) error {
	var serr error
	err := c.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	})
	if err != nil {
		return err
	}
	return serr
}
