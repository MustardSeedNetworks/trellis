// SPDX-License-Identifier: BUSL-1.1

//go:build !darwin

// Command trellis-capture reads the host Wi-Fi adapter. Only macOS has a
// host-NIC backend today; Linux and Windows are planned (docs/06-ROADMAP).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "trellis-capture has no host-NIC backend on this platform yet")
	os.Exit(1)
}
