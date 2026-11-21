// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package apppaths

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

// underService reports whether this process was started by the Service
// Control Manager.
func underService() bool {
	inService, err := svc.IsWindowsService()
	return err == nil && inService
}

// serviceLogFile is where the Windows service logs. A service has no console,
// and its profile (LocalService) is not somewhere an operator looks, so the log
// sits beside the data directory install.ps1 creates under %ProgramData%.
func serviceLogFile() (string, error) {
	programData, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil {
		return "", fmt.Errorf("resolve ProgramData: %w", err)
	}
	return filepath.Join(programData, appName, "logs", "trellisd.log"), nil
}
