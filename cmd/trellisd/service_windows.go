// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName        = "Trellis"
	serviceDisplayName = "Trellis Wi-Fi Survey Daemon"
	serviceDescription = "Serves the Trellis survey API on loopback. Live Wi-Fi scanning needs an interactive user session; run trellisd in one for surveys."

	// serviceAccount is the service's virtual account: a per-service identity
	// with no password and no rights of its own, so the data directory can be
	// granted to this service alone rather than to every LocalService process.
	// Not LocalSystem: the daemon serves a browser on a plain-HTTP, credential-
	// less loopback port (ADR-0007), and anything that reaches it through that
	// port must not land in a machine account.
	serviceAccount = `NT SERVICE\` + serviceName

	// serviceRestartDelay matches RestartSec in the Linux user unit.
	serviceRestartDelay = 5 * time.Second
	// serviceFailureWindow is how long before the SCM forgets earlier failures.
	serviceFailureWindow = 24 * time.Hour
	// serviceStateTimeout bounds start and stop. A graceful stop is bounded by
	// shutdownGracePeriod, so this leaves room for the readiness scan too.
	serviceStateTimeout = 30 * time.Second
)

// serviceCommand runs `trellisd service <action>` and returns the exit code.
func serviceCommand(args []string, stdout, stderr io.Writer) int {
	action, err := parseServiceAction(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "trellisd service: %v\n", err)
		return exitUsage
	}

	switch action {
	case actionInstall:
		err = installService(stdout)
	case actionStart:
		err = startService(stdout)
	case actionStop:
		err = stopService(stdout)
	case actionStatus:
		err = serviceStatus(stdout)
	case actionUninstall:
		err = uninstallService(stdout)
	case actionRun:
		err = runService()
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "trellisd service %s: %v\n", action, err)
		return 1
	}
	return 0
}

// serviceHandler adapts run to the Service Control Manager's protocol: report
// Running once started, cancel the daemon's context on Stop or Shutdown, and
// report StopPending while it drains.
type serviceHandler struct {
	run func(context.Context) error
}

func (h serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.run(ctx) }()

	const accepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				status <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				return exitOf(<-done)
			}
		case err := <-done:
			// The daemon ended on its own, a refused bind or a lost store.
			// Reporting the failure is what lets the recovery action restart it.
			return exitOf(err)
		}
	}
}

// exitOf maps run's result to the (service-specific, code) pair Execute returns.
func exitOf(err error) (bool, uint32) {
	if err != nil {
		slog.Error("trellisd service stopped with error", "error", err)
		return true, 1
	}
	return false, 0
}

// runService is what the Service Control Manager executes. The logger is
// installed here because handleArgs returns before main reaches it.
func runService() error {
	if inService, err := svc.IsWindowsService(); err != nil {
		return fmt.Errorf("detect service session: %w", err)
	} else if !inService {
		return errors.New("only the Service Control Manager runs this; use `trellisd service start`")
	}

	closeLog, err := installLogger()
	if err != nil {
		return err
	}
	defer func() { _ = closeLog() }()

	return svc.Run(serviceName, serviceHandler{run: runChecked})
}

// runChecked refuses to start on a data directory the service cannot trust,
// then runs the daemon. It runs inside the handler so the refusal is logged and
// reported to the Service Control Manager as a failed start.
func runChecked(ctx context.Context) error {
	dataDir := os.Getenv(envDataDir)
	if dataDir == "" {
		return errors.New(envDataDir + " is not set in the service environment; reinstall the service")
	}
	if err := ensureDataDir(dataDir); err != nil {
		return fmt.Errorf("data directory %s: %w", dataDir, err)
	}
	return run(ctx)
}

func connectManager() (*mgr.Mgr, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("connect to the service manager (run from an elevated prompt): %w", err)
	}
	return m, nil
}

func openService(m *mgr.Mgr) (*mgr.Service, error) {
	s, err := m.OpenService(serviceName)
	if err != nil {
		return nil, fmt.Errorf("open service %s (is it installed?): %w", serviceName, err)
	}
	return s, nil
}

func installService(out io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	if exe, err = filepath.Abs(exe); err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	programData, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil {
		return fmt.Errorf("resolve ProgramData: %w", err)
	}
	if err := validateServiceInstall(os.Getenv); err != nil {
		return err
	}

	m, err := connectManager()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()

	if existing, err := m.OpenService(serviceName); err == nil {
		_ = existing.Close()
		return fmt.Errorf("service %s is already installed; `trellisd service uninstall` removes it", serviceName)
	}

	// Manual start and not started: the service is the operator's opt-in, as
	// `systemctl --user enable` is on Linux (ADR-0007). `sc config Trellis
	// start= auto` makes it start with the machine.
	s, err := m.CreateService(serviceName, exe, mgr.Config{
		DisplayName:      serviceDisplayName,
		Description:      serviceDescription,
		StartType:        mgr.StartManual,
		ServiceStartName: serviceAccount,
	}, string(actionRun))
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer func() { _ = s.Close() }()

	env := serviceEnvironment(os.Getenv, programData)
	if err := configureService(s, env); err != nil {
		_ = s.Delete()
		return err
	}

	_, _ = fmt.Fprintf(out, "Service %s installed (manual start, as %s).\n  data dir: %s\n", serviceName, serviceAccount, env[0][len(envDataDir)+1:])
	_, _ = fmt.Fprintln(out, "Start it with: trellisd service start")
	return nil
}

// configureService restarts the daemon after a failure and gives it its
// environment. The environment is the service's own registry value, which the
// SCM hands to the process at start, so `trellisd` reads it exactly as it reads
// a shell's.
func configureService(s *mgr.Service, env []string) error {
	if err := s.SetRecoveryActions(
		[]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: serviceRestartDelay}},
		uint32(serviceFailureWindow/time.Second),
	); err != nil {
		return fmt.Errorf("set recovery actions: %w", err)
	}
	// Without this a failed bind exits non-zero but is not a crash, so the
	// recovery action never fires.
	if err := s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return fmt.Errorf("set recovery on non-crash failures: %w", err)
	}

	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Services\`+serviceName, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open service registry key: %w", err)
	}
	defer func() { _ = key.Close() }()
	if err := key.SetStringsValue("Environment", env); err != nil {
		return fmt.Errorf("set service environment: %w", err)
	}
	return nil
}

func startService(out io.Writer) error {
	m, err := connectManager()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := openService(m)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	if err := s.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	if err := waitForState(s, svc.Running); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Service %s started.\n", serviceName)
	return nil
}

func stopService(out io.Writer) error {
	m, err := connectManager()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := openService(m)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	if err := stopAndWait(s); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Service %s stopped.\n", serviceName)
	return nil
}

func serviceStatus(out io.Writer) error {
	m, err := connectManager()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := openService(m)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	st, err := s.Query()
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Service: %s\nStatus:  %s\n", serviceDisplayName, stateName(st.State))
	return nil
}

func uninstallService(out io.Writer) error {
	m, err := connectManager()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := openService(m)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	if err := stopAndWait(s); err != nil {
		return err
	}
	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Service %s removed. Surveys in its data directory were left in place.\n", serviceName)
	return nil
}

// stopAndWait stops the service if it is running and waits for it to finish.
func stopAndWait(s *mgr.Service) error {
	st, err := s.Query()
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	if st.State == svc.Stopped {
		return nil
	}
	if _, err := s.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	return waitForState(s, svc.Stopped)
}

func waitForState(s *mgr.Service, want svc.State) error {
	deadline := time.Now().Add(serviceStateTimeout)
	for {
		st, err := s.Query()
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		if st.State == want {
			return nil
		}
		// A start that fell back to Stopped has failed; waiting out the
		// timeout would only hide why.
		if want == svc.Running && st.State == svc.Stopped {
			return errors.New("service stopped while starting; see the log in %ProgramData%\\Trellis\\logs")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service did not reach %s within %s", stateName(want), serviceStateTimeout)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func stateName(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "Stopped"
	case svc.StartPending:
		return "Starting"
	case svc.StopPending:
		return "Stopping"
	case svc.Running:
		return "Running"
	case svc.ContinuePending:
		return "Continuing"
	case svc.PausePending:
		return "Pausing"
	case svc.Paused:
		return "Paused"
	}
	return fmt.Sprintf("Unknown (%d)", state)
}
