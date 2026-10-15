//go:build !unix && !windows

package node

import "errors"

func freeBytes(string) (uint64, error) { return 0, errors.New("desteklenmiyor") }
