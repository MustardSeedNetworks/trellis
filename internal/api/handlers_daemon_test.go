// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"regexp"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	daemonv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/daemon/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

func TestDaemonGetConfigReportsEffectiveConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  api.DaemonConfig
		want *daemonv1.GetConfigResponse
	}{
		{
			name: "loopback desktop daemon",
			cfg: api.DaemonConfig{
				ListenAddr: "127.0.0.1:8446", DataDir: "/data/trellis", CaptureMode: "scan",
			},
			want: &daemonv1.GetConfigResponse{
				ListenAddr: "127.0.0.1:8446", DataDir: "/data/trellis", CaptureMode: "scan",
			},
		},
		{
			name: "protected daemon with a log file",
			cfg: api.DaemonConfig{
				ListenAddr: "0.0.0.0:8446", DataDir: "/srv/trellis", CaptureMode: "monitor",
				TLSEnabled: true, AuthRequired: true, LogFile: "/var/log/trellis.log",
			},
			want: &daemonv1.GetConfigResponse{
				ListenAddr: "0.0.0.0:8446", DataDir: "/srv/trellis", CaptureMode: "monitor",
				TlsEnabled: true, AuthRequired: true, LogFile: "/var/log/trellis.log",
			},
		},
		{
			name: "host with no capture backend",
			cfg:  api.DaemonConfig{ListenAddr: "127.0.0.1:8447", DataDir: "/d", CaptureMode: "unavailable"},
			want: &daemonv1.GetConfigResponse{ListenAddr: "127.0.0.1:8447", DataDir: "/d", CaptureMode: "unavailable"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := api.NewDaemonServiceHandler(tt.cfg).GetConfig(t.Context(), &daemonv1.GetConfigRequest{})
			if err != nil {
				t.Fatalf("GetConfig: %v", err)
			}
			if !proto.Equal(got, tt.want) {
				t.Errorf("GetConfig = %v, want %v", got, tt.want)
			}
		})
	}
}

// secretish matches a field name that would carry a credential, key material
// or token. The response message is closed over what an operator may read off
// their own launch environment, so a name like this appearing is a contract
// change that has to be argued for, not slipped in.
var secretish = regexp.MustCompile(`(?i)pass|secret|token|credential|private|key|cert|hash|salt|csrf|session|cookie`)

func TestDaemonConfigResponseHasNoSecretField(t *testing.T) {
	fields := (&daemonv1.GetConfigResponse{}).ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		if name := string(fields.Get(i).Name()); secretish.MatchString(name) {
			t.Errorf("GetConfigResponse field %q looks like a secret; the Daemon tab must stay non-secret", name)
		}
	}
}

func TestDaemonGetConfigNeverEchoesSecretsFromTheEnvironment(t *testing.T) {
	const (
		password = "hunter2-operator-password"
		keyPath  = "/etc/trellis/tls-private.key"
	)
	t.Setenv("TRELLIS_AUTH_USERNAME", "surveyor")
	t.Setenv("TRELLIS_AUTH_PASSWORD", password)
	t.Setenv("TRELLIS_TLS_KEY", keyPath)

	resp, err := api.NewDaemonServiceHandler(api.DaemonConfig{
		ListenAddr: "0.0.0.0:8446", DataDir: "/srv/trellis", CaptureMode: "scan",
		TLSEnabled: true, AuthRequired: true,
	}).GetConfig(t.Context(), &daemonv1.GetConfigRequest{})
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}

	wire, err := protojson.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, secret := range []string{password, keyPath, "surveyor"} {
		if strings.Contains(string(wire), secret) {
			t.Errorf("response %s contains %q", wire, secret)
		}
	}
}
