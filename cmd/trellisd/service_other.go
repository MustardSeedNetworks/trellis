// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package main

import (
	"fmt"
	"io"
)

// serviceCommand refuses on every platform but Windows. Linux has a systemd
// user unit and macOS an app bundle (ADR-0007); neither is managed from here.
func serviceCommand(args []string, _, stderr io.Writer) int {
	if _, err := parseServiceAction(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "trellisd service: %v\n", err)
		return exitUsage
	}
	_, _ = fmt.Fprintln(stderr, "trellisd service: only available on Windows; on Linux use systemctl --user, on macOS open the app")
	return exitUsage
}
