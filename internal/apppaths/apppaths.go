// SPDX-License-Identifier: BUSL-1.1

// Package apppaths resolves where trellisd keeps its data and its log on the
// host it is running on.
//
// It exists because trellisd is launched two very different ways on macOS. From
// a terminal it behaves like any daemon. Launched as the signed application
// bundle — which is the only way macOS will let it read Wi-Fi network names
// (docs/10-WIFI-CAPTURE) — LaunchServices sets the working directory to "/" and
// connects stdout and stderr to /dev/null. A relative data directory then
// resolves under the root of the filesystem, and every log line is discarded.
package apppaths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// appName is the directory these paths are grouped under, in the capitalised
// form macOS expects under Application Support and Logs.
const appName = "Trellis"

// DataDir returns the default location for the survey store: a per-user,
// absolute path in the platform's conventional place for application data.
//
// TRELLIS_DATA_DIR overrides it. Nothing here reads the working directory —
// see the package comment for why that is not an option.
func DataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", appName), nil

	case "windows":
		// %AppData%. os.UserConfigDir is the only Go API that reads the
		// known-folder path rather than assuming a layout under the profile.
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve application data directory: %w", err)
		}
		return filepath.Join(dir, appName), nil

	default:
		// XDG base directories: surveys are data, not configuration, so this
		// is $XDG_DATA_HOME rather than $XDG_CONFIG_HOME.
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return filepath.Join(dir, "trellis"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".local", "share", "trellis"), nil
	}
}

// LogFile returns the file trellisd should write its log to, or "" when writing
// to stdout is enough.
//
// Only a bundled macOS launch needs a file: LaunchServices gives the process
// /dev/null for stdout and stderr, so a bundled daemon that logs to stdout is
// silent — including the warning that says a survey will record no network
// names. ~/Library/Logs is where the Console app looks.
func LogFile() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	if !inBundle(exe) {
		return "", nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Logs", appName, "trellisd.log"), nil
}

// inBundle reports whether an executable path sits at the
// <name>.app/Contents/MacOS/<binary> position that makes LaunchServices, and
// therefore macOS privacy permissions, treat the process as that bundle.
func inBundle(exe string) bool {
	macOS := filepath.Dir(exe)
	contents := filepath.Dir(macOS)
	bundle := filepath.Dir(contents)

	return filepath.Base(macOS) == "MacOS" &&
		filepath.Base(contents) == "Contents" &&
		strings.HasSuffix(filepath.Base(bundle), ".app")
}
