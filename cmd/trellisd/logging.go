// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/MustardSeedNetworks/trellis/internal/apppaths"
)

// logDirPerm and logFilePerm keep the log readable only by the user who ran the
// survey. It records which Wi-Fi networks were in range of them.
const (
	logDirPerm  os.FileMode = 0o700
	logFilePerm os.FileMode = 0o600
)

// Rotation bounds for the bundled daemon's log: it runs for as long as the
// operator leaves the app open, so an unbounded file grows until the disk
// fills. Ten 10 MB generations, compressed, for at most a month.
const (
	logMaxSizeMB  = 10
	logMaxBackups = 10
	logMaxAgeDays = 30
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

	writer, err := openLogFile(path)
	if err != nil {
		return nil, err
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(writer, nil)))
	slog.Info("logging to file", "path", path)
	return writer.Close, nil
}

// osCreateAppend is how the log file is created at startup.
const osCreateAppend = os.O_CREATE | os.O_WRONLY | os.O_APPEND

// openLogFile creates the log file, owner-only, and returns a writer that
// rotates it. The file is created here rather than on first write so an
// unwritable location fails at startup, not on the first log line.
func openLogFile(path string) (*lumberjack.Logger, error) {
	if err := createLogFile(path); err != nil {
		return nil, err
	}

	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    logMaxSizeMB,
		MaxBackups: logMaxBackups,
		MaxAge:     logMaxAgeDays,
		Compress:   true,
	}, nil
}
