// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseServiceAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    serviceAction
		wantErr string
	}{
		{name: "install", args: []string{"install"}, want: actionInstall},
		{name: "start", args: []string{"start"}, want: actionStart},
		{name: "stop", args: []string{"stop"}, want: actionStop},
		{name: "status", args: []string{"status"}, want: actionStatus},
		{name: "uninstall", args: []string{"uninstall"}, want: actionUninstall},
		{name: "run is accepted for the service manager", args: []string{"run"}, want: actionRun},
		{name: "no action", args: nil, wantErr: "missing action"},
		{name: "unknown action", args: []string{"restart"}, wantErr: `unknown action "restart"`},
		{name: "extra argument", args: []string{"start", "now"}, wantErr: `unexpected argument "now"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseServiceAction(tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseServiceAction(%q) error = %v, want one containing %q", tc.args, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("parseServiceAction(%q) = %q, %v; want %q", tc.args, got, err, tc.want)
			}
		})
	}
}

func TestServiceEnvironment(t *testing.T) {
	t.Parallel()

	programData := filepath.Join("C:", "ProgramData")
	defaultData := envDataDir + "=" + filepath.Join(programData, "Trellis", "data")

	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "nothing set puts the data dir under ProgramData",
			want: []string{defaultData},
		},
		{
			name: "an explicit data dir wins",
			env:  map[string]string{envDataDir: `D:\surveys`},
			want: []string{envDataDir + `=D:\surveys`},
		},
		{
			name: "settings are copied in a fixed order",
			env: map[string]string{
				envTLSKey: "k.pem", envTLSCert: "c.pem", captureModeEnv: "monitor", envAddr: "127.0.0.1:9000",
			},
			want: []string{
				defaultData,
				envAddr + "=127.0.0.1:9000",
				captureModeEnv + "=monitor",
				envTLSCert + "=c.pem",
				envTLSKey + "=k.pem",
			},
		},
		{
			name: "the credential is never copied",
			env:  map[string]string{"TRELLIS_AUTH_USERNAME": "op", "TRELLIS_AUTH_PASSWORD": "hunter2"},
			want: []string{defaultData},
		},
		{
			name: "unrelated variables are ignored",
			env:  map[string]string{"PATH": "x", "TRELLIS_UNKNOWN": "y"},
			want: []string{defaultData},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := serviceEnvironment(func(k string) string { return tc.env[k] }, programData)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("serviceEnvironment() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCredentialSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "neither", want: false},
		{name: "username", env: map[string]string{"TRELLIS_AUTH_USERNAME": "op"}, want: true},
		{name: "password", env: map[string]string{"TRELLIS_AUTH_PASSWORD": "x"}, want: true},
		{name: "blank username", env: map[string]string{"TRELLIS_AUTH_USERNAME": "  "}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := credentialSet(func(k string) string { return tc.env[k] }); got != tc.want {
				t.Fatalf("credentialSet() = %v, want %v", got, tc.want)
			}
		})
	}
}

// handleArgs routes `service` before flag parsing, so a service verb can never
// reach the daemon and a bad one is a usage error on every platform.
func TestHandleArgsServiceBadAction(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code, done := handleArgs([]string{"service", "restart"}, &stdout, &stderr)
	if !done || code != exitUsage {
		t.Fatalf("handleArgs(service restart) = %d, %v; want %d, true", code, done, exitUsage)
	}
	if !strings.Contains(stderr.String(), `unknown action "restart"`) {
		t.Fatalf("stderr = %q, want the unknown-action message", stderr.String())
	}
}

func TestValidateServiceInstall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{name: "empty environment installs"},
		{name: "loopback address installs", env: map[string]string{envAddr: "127.0.0.1:9000"}},
		{name: "username is refused", env: map[string]string{"TRELLIS_AUTH_USERNAME": "op"}, wantErr: "readable by every local user"},
		{name: "password is refused", env: map[string]string{"TRELLIS_AUTH_PASSWORD": "x"}, wantErr: "readable by every local user"},
		{name: "routable address is refused", env: map[string]string{envAddr: "0.0.0.0:8446"}, wantErr: "loopback only"},
		{name: "all-interfaces shorthand is refused", env: map[string]string{envAddr: ":8446"}, wantErr: "loopback only"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateServiceInstall(func(k string) string { return tc.env[k] })
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateServiceInstall() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validateServiceInstall() = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}
