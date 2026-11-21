// SPDX-License-Identifier: BUSL-1.1

package main

// The fail-closed data-directory checks, in the part that needs no Windows API
// so it is table-tested on every OS.
//
// TODO(#1065): replace with foundation/pkg/securedir once it is tagged, and
// delete this file, parseACE and their tests.

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// SIDs a data directory may be owned by, and may grant write access to, besides
// the account the service runs as.
const (
	sidSystem         = "S-1-5-18"
	sidAdministrators = "S-1-5-32-544"
)

// Access-mask bits that let a principal change what is in a directory or who
// controls it. A mask holding any of them is a write for this check's purpose.
const (
	fileWriteData       = 0x00000002
	fileAppendData      = 0x00000004
	fileWriteEA         = 0x00000010
	fileDeleteChild     = 0x00000040
	fileWriteAttributes = 0x00000100
	accessDelete        = 0x00010000
	accessWriteDAC      = 0x00040000
	accessWriteOwner    = 0x00080000
	genericAll          = 0x10000000
	genericWrite        = 0x40000000

	writeMask = fileWriteData | fileAppendData | fileWriteEA | fileDeleteChild |
		fileWriteAttributes | accessDelete | accessWriteDAC | accessWriteOwner |
		genericAll | genericWrite
)

// aclEntry is one access-allowed entry of a directory's DACL.
type aclEntry struct {
	sid  string
	mask uint32
}

// checkDataDirSecurity decides whether a service may trust an existing data
// directory. Anyone can create a folder under %ProgramData% before the
// installer does, which would leave them owning, or able to rewrite, the
// directory a machine account then reads surveys and a SQLite store from. So
// the directory must be owned by SYSTEM or Administrators, must have a DACL,
// and must grant write access to no one but SYSTEM, Administrators and self,
// the service's own SID.
func checkDataDirSecurity(owner string, hasDACL bool, entries []aclEntry, self string) error {
	if owner != sidSystem && owner != sidAdministrators {
		return fmt.Errorf("owned by %s, want SYSTEM or Administrators", owner)
	}
	if !hasDACL {
		return errors.New("has no DACL, which grants everyone full access")
	}
	for _, e := range entries {
		if e.mask&writeMask == 0 {
			continue
		}
		if e.sid != sidSystem && e.sid != sidAdministrators && e.sid != self {
			return fmt.Errorf("grants write access (mask 0x%x) to %s", e.mask, e.sid)
		}
	}
	return nil
}

// ACE types that can appear in a DACL. Every allow variant is a grant; a deny
// is understood but never subtracted, so a deny cannot hide a grant from the
// check. Anything else is refused: a type this does not know could grant access
// by a layout it cannot read.
const (
	aceTypeAllowed               = 0
	aceTypeDenied                = 1
	aceTypeAllowedObject         = 5
	aceTypeDeniedObject          = 6
	aceTypeAllowedCallback       = 9
	aceTypeDeniedCallback        = 10
	aceTypeAllowedCallbackObject = 11
	aceTypeDeniedCallbackObject  = 12

	aceObjectTypePresent          = 0x1
	aceInheritedObjectTypePresent = 0x2
	aceHeaderLen                  = 8 // type, flags, size, mask
	guidLen                       = 16
	minSIDLen                     = 8
)

// aceInfo is what parseACE reads out of one raw ACE.
type aceInfo struct {
	grant     bool
	mask      uint32
	sidOffset int
}

// parseACE locates the mask and the trustee SID in a raw ACE, whatever its
// allow or deny variant, and refuses a type it does not understand or a
// truncated entry.
func parseACE(raw []byte) (aceInfo, error) {
	if len(raw) < aceHeaderLen {
		return aceInfo{}, fmt.Errorf("truncated ACE of %d bytes", len(raw))
	}
	info := aceInfo{mask: binary.LittleEndian.Uint32(raw[4:8]), sidOffset: aceHeaderLen}

	switch raw[0] {
	case aceTypeAllowed, aceTypeAllowedCallback:
		info.grant = true
	case aceTypeDenied, aceTypeDeniedCallback:
	case aceTypeAllowedObject, aceTypeAllowedCallbackObject, aceTypeDeniedObject, aceTypeDeniedCallbackObject:
		info.grant = raw[0] == aceTypeAllowedObject || raw[0] == aceTypeAllowedCallbackObject
		if len(raw) < aceHeaderLen+4 {
			return aceInfo{}, fmt.Errorf("truncated object ACE of %d bytes", len(raw))
		}
		flags := binary.LittleEndian.Uint32(raw[aceHeaderLen : aceHeaderLen+4])
		info.sidOffset = aceHeaderLen + 4
		if flags&aceObjectTypePresent != 0 {
			info.sidOffset += guidLen
		}
		if flags&aceInheritedObjectTypePresent != 0 {
			info.sidOffset += guidLen
		}
	default:
		return aceInfo{}, fmt.Errorf("unrecognised ACE type %d", raw[0])
	}
	if info.sidOffset+minSIDLen > len(raw) {
		return aceInfo{}, fmt.Errorf("ACE type %d has no room for a SID", raw[0])
	}
	return info, nil
}
