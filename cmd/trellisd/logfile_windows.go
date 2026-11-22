// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package main

import (
	"fmt"

	"github.com/MustardSeedNetworks/foundation/pkg/securedir"
)

// createLogFile is the only Windows log file this daemon opens: the service's,
// under the directory install.ps1 secured. The service does not create that
// directory, so a missing one fails the start rather than being built under
// whatever the parent inherits.
func createLogFile(path string) error {
	trust, err := serviceTrust()
	if err != nil {
		return err
	}
	return createLogFileWith(path, trust)
}

// createLogFileWith creates the file through securedir, which refuses a link in
// the path or at the file and checks the directory and its ancestors, so a
// planted link cannot redirect the service's writes. lumberjack then reopens it
// by name: that handle and the rotated files are created by the standard
// library and take their DACL from the directory's inheritable entries.
func createLogFileWith(path string, trust securedir.Options) error {
	file, err := securedir.OpenFile(path, osCreateAppend, trust)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	return nil
}
