// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/MustardSeedNetworks/trellis/internal/apppaths"
)

// logDirPerm and logFilePerm keep the log readable only by the user who ran the
// survey. It records which Wi-Fi networks were in range of them.
const (
	logDirPerm  os.FileMode = 0o700
	logFilePerm os.FileMode = 0o600
)

// installLogger points slog at stdout, or at a file when stdout would be
// discarded, and returns a close function for whatever it opened.
//
// A macOS bundle launched through LaunchServices gets /dev/null for stdout and
// stderr (docs/10-WIFI-CAPTURE). Left alone, the bundled daemon — the only
// build that can read network names — would be the one build with no output.
func installLogger() (func() error, error) {
	path, err := apppaths.LogFile()
	if err != nil {
		return nil, err
	}
	if path == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
		return func() error { return nil }, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), logDirPerm); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFilePerm)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(io.Writer(file), nil)))
	slog.Info("logging to file", "path", path)
	return file.Close, nil
}
