// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"errors"
	"log/slog"

	"github.com/MustardSeedNetworks/foundation/pkg/supervise"
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

// superviseCaptureReadiness runs the readiness scan under foundation's
// supervisor and returns the group so the caller can stop it.
//
// The scan reaches a Wi-Fi driver through an OS permission check, which is
// exactly the kind of call that faults rather than returning an error. Run bare
// it was one panic away from taking the daemon down before it had served a
// request (D-TRL-6).
//
// Giving up is not fatal. A host with no backend still serves imported surveys,
// which is why a missing backend is already reported and not fatal above; a
// backend that panicked is the same answer for the operator, said the same way.
func superviseCaptureReadiness(
	ctx context.Context,
	scanner survey.Scanner,
	h *api.SurveyServiceHandler,
) *supervise.Group {
	group := supervise.New(nil)
	group.Add("capture-readiness", supervise.RestartN(captureReadinessRestarts),
		func(ctx context.Context) error {
			reportCaptureReadiness(ctx, scanner, h)
			return nil
		})
	group.Start(ctx)

	go func() {
		if err := group.Wait(); err != nil {
			slog.Error("capture readiness gave up; this host reports no capture backend", "error", err)
			h.SetCaptureCapability(api.CaptureCapability{Reason: err.Error()})
		}
	}()
	return group
}
