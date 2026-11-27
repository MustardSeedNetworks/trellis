// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"context"

	daemonv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/daemon/v1"
)

// DaemonConfig is the effective, non-secret configuration the daemon reports
// about itself. It is a closed struct of plain facts rather than a view of the
// environment, so nothing secret can reach the wire by being added to the
// environment: the credential, the TLS key and the session secrets have no
// field to travel in.
type DaemonConfig struct {
	ListenAddr   string
	DataDir      string
	CaptureMode  string
	TLSEnabled   bool
	AuthRequired bool
	LogFile      string
}

// DaemonServiceHandler serves the daemon's own configuration, read-only.
type DaemonServiceHandler struct {
	cfg DaemonConfig
}

// NewDaemonServiceHandler reports cfg, which is fixed for the life of the
// process: nothing about it changes after the listener is bound.
func NewDaemonServiceHandler(cfg DaemonConfig) *DaemonServiceHandler {
	return &DaemonServiceHandler{cfg: cfg}
}

// GetConfig returns the daemon's effective non-secret configuration.
func (h *DaemonServiceHandler) GetConfig(
	_ context.Context,
	_ *daemonv1.GetConfigRequest,
) (*daemonv1.GetConfigResponse, error) {
	return &daemonv1.GetConfigResponse{
		ListenAddr:   h.cfg.ListenAddr,
		DataDir:      h.cfg.DataDir,
		CaptureMode:  h.cfg.CaptureMode,
		TlsEnabled:   h.cfg.TLSEnabled,
		AuthRequired: h.cfg.AuthRequired,
		LogFile:      h.cfg.LogFile,
	}, nil
}
