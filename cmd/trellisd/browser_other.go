// SPDX-License-Identifier: BUSL-1.1

//go:build !darwin

package main

import (
	"context"
	"errors"
)

// openBrowser is unsupported off macOS: only a macOS application bundle opens
// a browser, and a bundle layout elsewhere is not a Finder launch.
func openBrowser(context.Context, string) error {
	return errors.ErrUnsupported
}
