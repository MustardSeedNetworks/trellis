// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"encoding/binary"
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

func TestCheckDataDirSecurity(t *testing.T) {
	t.Parallel()

	const (
		self     = "S-1-5-80-111"
		users    = "S-1-5-32-545"
		everyone = "S-1-1-0"
		attacker = "S-1-5-21-1-2-3-1001"
	)
	trusted := []aclEntry{
		{sidSystem, genericAll},
		{sidAdministrators, genericAll},
		{self, fileWriteData | fileAppendData},
	}

	tests := []struct {
		name    string
		owner   string
		hasDACL bool
		entries []aclEntry
		wantErr string
	}{
		{name: "installer's layout", owner: sidAdministrators, hasDACL: true, entries: trusted},
		{name: "owned by SYSTEM", owner: sidSystem, hasDACL: true, entries: trusted},
		{
			name: "read-only grant to Users is fine", owner: sidAdministrators, hasDACL: true,
			entries: append([]aclEntry{{users, 0x001200a9}}, trusted...),
		},
		{name: "squatter owns it", owner: attacker, hasDACL: true, entries: trusted, wantErr: "owned by " + attacker},
		{name: "users owner", owner: users, hasDACL: true, entries: trusted, wantErr: "owned by " + users},
		{name: "null DACL", owner: sidAdministrators, hasDACL: false, wantErr: "no DACL"},
		{
			name: "Users may write", owner: sidAdministrators, hasDACL: true,
			entries: append([]aclEntry{{users, fileWriteData}}, trusted...), wantErr: "write access",
		},
		{
			name: "Everyone may append", owner: sidAdministrators, hasDACL: true,
			entries: append([]aclEntry{{everyone, fileAppendData}}, trusted...), wantErr: "write access",
		},
		{
			name: "a user may take ownership", owner: sidAdministrators, hasDACL: true,
			entries: append([]aclEntry{{attacker, accessWriteOwner}}, trusted...), wantErr: "write access",
		},
		{
			name: "generic all for a user", owner: sidAdministrators, hasDACL: true,
			entries: append([]aclEntry{{attacker, genericAll}}, trusted...), wantErr: "write access",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := checkDataDirSecurity(tc.owner, tc.hasDACL, tc.entries, self)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkDataDirSecurity() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("checkDataDirSecurity() = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestParseACE(t *testing.T) {
	t.Parallel()

	// header: type, flags, size(2), mask(4); then a minimal SID (S-1-5-18).
	sid := []byte{1, 1, 0, 0, 0, 0, 0, 5, 18, 0, 0, 0}
	mk := func(aceType byte, mask uint32, middle ...byte) []byte {
		raw := binary.LittleEndian.AppendUint32([]byte{aceType, 0, 0, 0}, mask)
		raw = append(raw, middle...)
		return append(raw, sid...)
	}
	guid := make([]byte, guidLen)
	flags := func(f byte) []byte { return []byte{f, 0, 0, 0} }

	tests := []struct {
		name    string
		raw     []byte
		want    aceInfo
		wantErr string
	}{
		{name: "allowed", raw: mk(aceTypeAllowed, 2), want: aceInfo{true, 2, 8}},
		{name: "allowed callback", raw: mk(aceTypeAllowedCallback, 4), want: aceInfo{true, 4, 8}},
		{name: "denied is understood and not a grant", raw: mk(aceTypeDenied, 2), want: aceInfo{false, 2, 8}},
		{name: "denied callback", raw: mk(aceTypeDeniedCallback, 2), want: aceInfo{false, 2, 8}},
		{name: "allowed object, no GUIDs", raw: mk(aceTypeAllowedObject, 2, flags(0)...), want: aceInfo{true, 2, 12}},
		{
			name: "allowed object, object GUID", want: aceInfo{true, 2, 28},
			raw: mk(aceTypeAllowedObject, 2, append(flags(aceObjectTypePresent), guid...)...),
		},
		{
			name: "allowed callback object, both GUIDs", want: aceInfo{true, 2, 44},
			raw: mk(aceTypeAllowedCallbackObject, 2, append(append(flags(3), guid...), guid...)...),
		},
		{name: "denied object", raw: mk(aceTypeDeniedObject, 2, flags(0)...), want: aceInfo{false, 2, 12}},
		{name: "audit ACE is refused", raw: mk(2, 2), wantErr: "unrecognised ACE type 2"},
		{name: "unknown type is refused", raw: mk(0x42, 2), wantErr: "unrecognised ACE type 66"},
		{name: "truncated header", raw: []byte{0, 0, 0}, wantErr: "truncated ACE"},
		{name: "no room for a SID", raw: []byte{0, 0, 0, 0, 2, 0, 0, 0, 1}, wantErr: "no room for a SID"},
		{name: "object ACE cut inside its flags", raw: []byte{5, 0, 0, 0, 2, 0, 0, 0, 1}, wantErr: "truncated object ACE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseACE(tc.raw)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseACE() error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("parseACE() = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}
