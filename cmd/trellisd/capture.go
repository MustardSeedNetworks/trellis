// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"errors"
	"log/slog"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/internal/api"
	"github.com/MustardSeedNetworks/trellis/internal/capture"
)

// reportCaptureReadiness asks the OS for capture permission and then performs
// one scan, so an operator learns whether this host can contribute measured
// survey points before walking a building rather than after.
//
// The scan is the check. On macOS a survey without Location Services
// authorization does not fail — it records the right number of networks with
// every name and BSSID emptied — so [capture.Scanner] reports that state as
// [capture.ErrPermission] and a successful scan is proof that identifiers came
// back. That makes the count below meaningful and keeps identifiers, which say
// where the operator is, out of the log.
//
// It runs in its own goroutine: an active scan takes three to four seconds and
// nothing else should wait on it.
// It also reports the outcome to the API handler, so a client can say what
// this host can do before an operator walks a building. The log line alone was
// only ever visible to whoever started the daemon, which on a packaged install
// is nobody.
func reportCaptureReadiness(ctx context.Context, scanner survey.Scanner, h *api.SurveyServiceHandler) {
	if err := capture.Authorize(); err != nil {
		slog.Warn("capture permission incomplete", "error", err)
	}

	networks, err := scanner.Scan(ctx)
	switch {
	case errors.Is(err, capture.ErrPermission):
		slog.Error("capture cannot read network names; a survey would record nameless BSSIDs",
			"error", err,
			"fix", capture.PermissionRemedy)
		h.SetCaptureCapability(api.CaptureCapability{
			Reason: err.Error(),
			Remedy: capture.PermissionRemedy,
		})
	case err != nil:
		slog.Error("capture scan failed", "error", err)
		h.SetCaptureCapability(api.CaptureCapability{Reason: err.Error()})
	default:
		slog.Info("capture ready", "networks", len(networks))
		h.SetCaptureCapability(api.CaptureCapability{Available: true})
	}
}
