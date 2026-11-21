// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

// serviceAction is one verb of `trellisd service`.
type serviceAction string

const (
	actionInstall   serviceAction = "install"
	actionStart     serviceAction = "start"
	actionStop      serviceAction = "stop"
	actionStatus    serviceAction = "status"
	actionUninstall serviceAction = "uninstall"
	// actionRun is what the Service Control Manager executes; operators never
	// type it.
	actionRun serviceAction = "run"
)

// parseServiceAction reads the arguments after `trellisd service`. Like the
// rest of the command line it refuses anything it does not recognise, so a
// typo cannot fall through to starting a daemon.
func parseServiceAction(args []string) (serviceAction, error) {
	if len(args) == 0 {
		return "", errors.New("missing action: want one of install, start, stop, status, uninstall")
	}
	if len(args) > 1 {
		return "", fmt.Errorf("unexpected argument %q", args[1])
	}
	switch action := serviceAction(args[0]); action {
	case actionInstall, actionStart, actionStop, actionStatus, actionUninstall, actionRun:
		return action, nil
	}
	return "", fmt.Errorf("unknown action %q: want one of install, start, stop, status, uninstall", args[0])
}

// validateServiceInstall refuses an installing shell whose settings the
// service could not honour safely. Both refusals are loud on purpose: dropping
// them would install a service that quietly behaves differently from the shell
// it was installed from.
//
//   - An operator credential cannot be given to the service. Its Environment is
//     a registry value every local user can read, so storing the credential
//     there would publish it, and the service has no other place to read it
//     from. The credential is what lifts the loopback gate (ADR-0010), so
//     without it a routable TRELLIS_ADDR would be refused at start anyway.
//   - A routable TRELLIS_ADDR is refused here for the same reason, one step
//     earlier, rather than leaving a service that can never start.
func validateServiceInstall(lookup func(string) string) error {
	if credentialSet(lookup) {
		return fmt.Errorf("%s/%s are set, and a service environment is readable by every local user; "+
			"unset them to install the loopback service, or run trellisd from your own session to use a credential",
			auth.EnvUsername, auth.EnvPassword)
	}
	if addr := lookup(envAddr); addr != "" {
		if err := requireBindAllowed(addr, false); err != nil {
			return fmt.Errorf("%s for the service: %w", envAddr, err)
		}
	}
	return nil
}

// serviceEnvNames are the settings `service install` copies from the
// installing shell into the service's environment.
//
// TRELLIS_AUTH_USERNAME and TRELLIS_AUTH_PASSWORD are not on the list because
// a service's Environment lives in the registry under HKLM, which every local
// user can read. validateServiceInstall refuses an installing shell that sets
// them rather than dropping them.
var serviceEnvNames = []string{envAddr, captureModeEnv, envTLSCert, envTLSKey}

// serviceEnvironment builds the service's environment block. The data
// directory is always explicit: the service runs as a virtual account whose
// profile is not where an operator expects surveys, and the default
// apppaths.DataDir resolves under it. Without a TRELLIS_DATA_DIR in the
// installing shell it is %ProgramData%\Trellis\data.
func serviceEnvironment(lookup func(string) string, programData string) []string {
	dataDir := lookup(envDataDir)
	if dataDir == "" {
		dataDir = filepath.Join(programData, "Trellis", "data")
	}

	env := []string{envDataDir + "=" + dataDir}
	for _, name := range serviceEnvNames {
		if value := lookup(name); value != "" {
			env = append(env, name+"="+value)
		}
	}
	return env
}

// credentialSet reports whether the installing shell carries an operator
// credential that serviceEnvironment will not copy.
func credentialSet(lookup func(string) string) bool {
	return strings.TrimSpace(lookup(auth.EnvUsername)) != "" || lookup(auth.EnvPassword) != ""
}

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
