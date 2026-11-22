// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// createLogFile makes the log directory and the file, owner-only.
func createLogFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), logDirPerm); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(path, osCreateAppend, logFilePerm)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	return nil
}
