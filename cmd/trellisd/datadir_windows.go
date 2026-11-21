// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ensureDataDir is the seam the service starts through. The service never
// creates its data directory: install.ps1 does, with a complete protected
// descriptor, so "ensure" here is "verify, and refuse if it is not already
// right". There is no writePrivateFile seam yet because nothing in trellisd
// writes a secret file.
//
// TODO(#1065): swap for foundation/pkg/securedir.Ensure and Verify, passing
// the NT SERVICE\Trellis SID as an extra trusted SID.
func ensureDataDir(dir string) error { return verifyDataDir(dir) }

// verifyDataDir fails closed on the data directory and its parent. The
// installer creates the directory with a complete protected descriptor, so a
// missing one means the install did not happen or was tampered with, and a
// service must not create it under whatever the parent inherits. Both
// directories must pass checkDataDirSecurity, which also covers who may delete
// or replace the data directory by way of its parent.
func verifyDataDir(dir string) error {
	if err := verifyDirectory(dir); err != nil {
		return err
	}
	parent := filepath.Dir(dir)
	if parent == dir {
		return nil
	}
	if err := verifyDirectory(parent); err != nil {
		return fmt.Errorf("parent %s: %w", parent, err)
	}
	return nil
}

func verifyDirectory(dir string) error {
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return fmt.Errorf("path: %w", err)
	}
	attrs, err := windows.GetFileAttributes(path)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		return errors.New("does not exist; run install.ps1")
	}
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("is a junction or symbolic link")
	}
	if attrs&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return errors.New("is not a directory")
	}

	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("read security descriptor: %w", err)
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return fmt.Errorf("read owner: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil && !errors.Is(err, windows.ERROR_OBJECT_NOT_FOUND) {
		return fmt.Errorf("read DACL: %w", err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("read process token user: %w", err)
	}

	var entries []aclEntry
	if dacl != nil {
		for i := range uint32(dacl.AceCount) {
			var ace *windows.ACCESS_ALLOWED_ACE
			if err := windows.GetAce(dacl, i, &ace); err != nil {
				return fmt.Errorf("read DACL entry %d: %w", i, err)
			}
			raw := unsafe.Slice((*byte)(unsafe.Pointer(ace)), ace.Header.AceSize)
			info, err := parseACE(raw)
			if err != nil {
				return fmt.Errorf("DACL entry %d: %w", i, err)
			}
			if !info.grant {
				continue
			}
			sid := (*windows.SID)(unsafe.Pointer(&raw[info.sidOffset]))
			entries = append(entries, aclEntry{sid: sid.String(), mask: info.mask})
		}
	}
	return checkDataDirSecurity(owner.String(), dacl != nil, entries, user.User.Sid.String())
}
