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
// TRELLIS_CAPTURE_MODE is spelled out because scanner.go, which owns its constant,
// is not part of the e2e build that also compiles this file.
var serviceEnvNames = []string{envAddr, "TRELLIS_CAPTURE_MODE", envTLSCert, envTLSKey}

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
