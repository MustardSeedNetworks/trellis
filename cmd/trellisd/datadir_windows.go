// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package main

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/MustardSeedNetworks/foundation/pkg/securedir"
	"golang.org/x/sys/windows"
)

// serviceTrust is what the service trusts besides SYSTEM and Administrators:
// its own virtual account. install.ps1 creates the data directory with that
// account in its DACL, so a directory owned or writable by anyone else was not
// made by the installer.
func serviceTrust() (securedir.Options, error) {
	sid, _, _, err := windows.LookupSID("", serviceAccount)
	if err != nil {
		return securedir.Options{}, fmt.Errorf("look up %s (is the service installed?): %w", serviceAccount, err)
	}
	return securedir.Options{ExtraTrustedSIDs: []string{sid.String()}}, nil
}

// verifyDataDir fails closed on the data directory and returns its canonical
// path. The service never creates the directory: install.ps1 does, with a
// complete protected descriptor, so a missing one means the install did not
// happen or was tampered with, and a service must not create it under whatever
// the parent inherits. securedir.Ensure is the installer's call for that
// reason; the service only verifies.
func verifyDataDir(dir string, trust securedir.Options) (string, error) {
	canonical, err := securedir.Canonical(dir, trust)
	if err == nil {
		err = securedir.Verify(canonical, trust)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return "", errors.New("does not exist; run install.ps1")
	}
	return canonical, err
}
