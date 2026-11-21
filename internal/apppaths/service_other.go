// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package apppaths

// underService is always false off Windows: Linux runs a systemd user unit that
// logs to the journal, and macOS runs the app bundle.
func underService() bool { return false }

func serviceLogFile() (string, error) { return "", nil }
