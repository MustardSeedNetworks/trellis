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

// serviceEnvNames are the settings `service install` copies from the
// installing shell into the service's environment.
//
// TRELLIS_AUTH_USERNAME and TRELLIS_AUTH_PASSWORD are deliberately not on the
// list: a service's Environment lives in the registry under HKLM, which every
// local user can read, so a credential put there is published to the machine.
// The service therefore runs in ADR-0010's default mode, loopback without a
// login, until the credential has somewhere safe to be read from.
var serviceEnvNames = []string{envAddr, captureModeEnv, envTLSCert, envTLSKey}

// serviceEnvironment builds the service's environment block. The data
// directory is always explicit: the service runs as LocalService, whose
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
