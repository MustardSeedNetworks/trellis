// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/foundation/pkg/securedir"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

// drive runs Execute against a fake Service Control Manager: it feeds the
// requests, collects every reported status, and returns Execute's result.
func drive(t *testing.T, h serviceHandler, requests ...svc.Cmd) (states []svc.State, ssec bool, code uint32) {
	t.Helper()

	reqCh := make(chan svc.ChangeRequest, len(requests))
	statusCh := make(chan svc.Status, 16)
	for _, cmd := range requests {
		reqCh <- svc.ChangeRequest{Cmd: cmd, CurrentStatus: svc.Status{State: svc.Running}}
	}

	type result struct {
		ssec bool
		code uint32
	}
	done := make(chan result, 1)
	go func() {
		ssec, code := h.Execute(nil, reqCh, statusCh)
		done <- result{ssec, code}
	}()

	select {
	case r := <-done:
		close(statusCh)
		for s := range statusCh {
			states = append(states, s.State)
		}
		return states, r.ssec, r.code
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return")
		return nil, false, 0
	}
}

func TestServiceHandlerExecute(t *testing.T) {
	t.Parallel()

	blockUntilCancelled := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	tests := []struct {
		name      string
		preflight func() error
		run       func(context.Context) error
		requests  []svc.Cmd
		want      []svc.State
		wantSpec  bool
		wantCode  uint32
	}{
		{
			name:     "stop cancels the daemon and exits cleanly",
			run:      blockUntilCancelled,
			requests: []svc.Cmd{svc.Stop},
			want:     []svc.State{svc.StartPending, svc.Running, svc.StopPending},
		},
		{
			name:     "shutdown is handled like stop",
			run:      blockUntilCancelled,
			requests: []svc.Cmd{svc.Shutdown},
			want:     []svc.State{svc.StartPending, svc.Running, svc.StopPending},
		},
		{
			name:     "interrogate is answered and the service keeps running",
			run:      blockUntilCancelled,
			requests: []svc.Cmd{svc.Interrogate, svc.Stop},
			want:     []svc.State{svc.StartPending, svc.Running, svc.Running, svc.StopPending},
		},
		{
			name:     "a daemon that fails on its own is reported as a service failure",
			run:      func(context.Context) error { return errors.New("bind refused") },
			want:     []svc.State{svc.StartPending, svc.Running},
			wantSpec: true,
			wantCode: 1,
		},
		{
			name:      "a refused preflight fails the start before Running is reported",
			preflight: func() error { return errors.New("data directory does not exist") },
			run:       func(context.Context) error { t.Error("run must not start after a refused preflight"); return nil },
			want:      []svc.State{svc.StartPending},
			wantSpec:  true,
			wantCode:  1,
		},
		{
			name:     "a failure while stopping is still reported",
			run:      func(ctx context.Context) error { <-ctx.Done(); return errors.New("store close failed") },
			requests: []svc.Cmd{svc.Stop},
			want:     []svc.State{svc.StartPending, svc.Running, svc.StopPending},
			wantSpec: true,
			wantCode: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			states, ssec, code := drive(t, serviceHandler{preflight: tc.preflight, run: tc.run}, tc.requests...)
			if len(states) != len(tc.want) {
				t.Fatalf("states = %v, want %v", states, tc.want)
			}
			for i := range states {
				if states[i] != tc.want[i] {
					t.Fatalf("states = %v, want %v", states, tc.want)
				}
			}
			if ssec != tc.wantSpec || code != tc.wantCode {
				t.Fatalf("Execute() = %v, %d; want %v, %d", ssec, code, tc.wantSpec, tc.wantCode)
			}
		})
	}
}

func TestStateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state svc.State
		want  string
	}{
		{svc.Stopped, "Stopped"},
		{svc.StartPending, "Starting"},
		{svc.Running, "Running"},
		{svc.StopPending, "Stopping"},
		{svc.State(99), "Unknown (99)"},
	}
	for _, tc := range tests {
		if got := stateName(tc.state); got != tc.want {
			t.Errorf("stateName(%d) = %q, want %q", tc.state, got, tc.want)
		}
	}
}

// currentUserTrust trusts the account running the tests, standing in for the
// service account: the runner is neither SYSTEM nor the Trellis virtual account.
func currentUserTrust(t *testing.T) securedir.Options {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("read process token user: %v", err)
	}
	return securedir.Options{ExtraTrustedSIDs: []string{user.User.Sid.String()}}
}

// protect gives dir the descriptor install.ps1 writes: SYSTEM, Administrators
// and the trusted account only, inherited by children.
func protect(t *testing.T, dir string, trust securedir.Options) {
	t.Helper()
	setDACL(t, dir, "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;"+trust.ExtraTrustedSIDs[0]+")")
}

// setDACL replaces path's DACL with the one in sddl, protected.
func setDACL(t *testing.T, path, sddl string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatalf("parse %q: %v", sddl, err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("DACL of %q: %v", sddl, err)
	}
	err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
	if err != nil {
		t.Fatalf("set DACL on %s: %v", path, err)
	}
}

func TestVerifyDataDir(t *testing.T) {
	t.Parallel()

	trust := currentUserTrust(t)
	base := t.TempDir()
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	installed := func(t *testing.T) string {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "data")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		protect(t, dir, trust)
		return dir
	}

	tests := []struct {
		name    string
		dir     func(t *testing.T) string
		wantErr string
		wantIs  error
	}{
		{name: "the installer's layout is accepted", dir: installed},
		{
			name:    "a missing directory is refused",
			dir:     func(*testing.T) string { return filepath.Join(base, "missing") },
			wantErr: "does not exist",
		},
		{name: "a file is refused", dir: func(*testing.T) string { return file }, wantIs: windows.ERROR_DIRECTORY},
		{
			name: "an extra user ACE is refused, not repaired",
			dir: func(t *testing.T) string {
				dir := installed(t)
				setDACL(t, dir, "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;"+trust.ExtraTrustedSIDs[0]+")(A;OICI;FA;;;BU)")
				return dir
			},
			wantIs: securedir.ErrUnsafeAccess,
		},
		{
			name: "a junction inside the directory is refused",
			dir: func(t *testing.T) string {
				dir := installed(t)
				target := filepath.Join(t.TempDir(), "target")
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
				junction := filepath.Join(dir, "junction")
				out, err := exec.CommandContext(t.Context(), "cmd", "/c", "mklink", "/J", junction, target).CombinedOutput()
				if err != nil {
					t.Fatalf("mklink /J: %v: %s", err, out)
				}
				return dir
			},
			wantIs: securedir.ErrReparsePoint,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := verifyDataDir(tc.dir(t), trust)
			switch {
			case tc.wantIs != nil:
				if !errors.Is(err, tc.wantIs) {
					t.Fatalf("verifyDataDir() = %v, want %v", err, tc.wantIs)
				}
			case tc.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("verifyDataDir() = %v, want one containing %q", err, tc.wantErr)
				}
			case err != nil || got == "":
				t.Fatalf("verifyDataDir() = %q, %v; want a canonical path", got, err)
			}
		})
	}
}
