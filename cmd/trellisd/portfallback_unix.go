// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package main

import (
	"errors"
	"syscall"
)

// isAddrInUse reports whether a bind failed because something else already
// holds the port.
func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
