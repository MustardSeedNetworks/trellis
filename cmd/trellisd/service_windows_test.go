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

	"golang.org/x/sys/windows/svc"
)

// drive runs Execute against a fake Service Control Manager: it feeds the
// requests, collects every reported status, and returns Execute's result.
func drive(t *testing.T, run func(context.Context) error, requests ...svc.Cmd) (states []svc.State, ssec bool, code uint32) {
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
		ssec, code := serviceHandler{run: run}.Execute(nil, reqCh, statusCh)
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
		name     string
		run      func(context.Context) error
		requests []svc.Cmd
		want     []svc.State
		wantSpec bool
		wantCode uint32
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
			states, ssec, code := drive(t, tc.run, tc.requests...)
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

func TestVerifyDataDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(base, "junction")
	if out, err := exec.CommandContext(t.Context(), "cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v: %s", err, out)
	}

	tests := []struct {
		name    string
		dir     string
		wantErr string
	}{
		{name: "missing directory is created later", dir: filepath.Join(base, "missing")},
		{name: "a file is refused", dir: file, wantErr: "not a directory"},
		{name: "a junction is refused", dir: junction, wantErr: "junction"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := verifyDataDir(tc.dir)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("verifyDataDir() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("verifyDataDir() = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}
