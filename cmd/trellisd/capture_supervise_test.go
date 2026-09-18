// SPDX-License-Identifier: BUSL-1.1

package main

// capture_supervise_test.go pins the readiness scan's half of D-TRL-6. Run bare
// it was a `go` statement away from the process: the scan reaches a Wi-Fi driver
// through an OS permission check, and a fault there took the daemon down before
// it had answered a request, with the runtime's stack trace as the only account
// an operator got.

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// panickingScanner panics on its first remaining sweeps, then answers like
// fixedScanner. A driver that faults on one call and not the next is the shape
// this guards.
type panickingScanner struct {
	remaining atomic.Int32
	calls     atomic.Int32
	networks  []wifi.ScannedNetwork
}

func (s *panickingScanner) Scan(context.Context) ([]wifi.ScannedNetwork, error) {
	s.calls.Add(1)
	if s.remaining.Add(-1) >= 0 {
		panic("radio driver faulted during the readiness scan")
	}
	return s.networks, nil
}

func TestReadinessScanSurvivesAPanickingRadio(t *testing.T) {
	scanner := &panickingScanner{networks: []wifi.ScannedNetwork{{SSID: "lab", BSSID: "aa:bb:cc:00:00:01"}}}
	scanner.remaining.Store(1)

	handler := handlerForTest(t, scanner)
	group := superviseCaptureReadiness(t.Context(), scanner, handler)
	t.Cleanup(func() { _ = group.Stop(context.Background()) })

	// The assertion is the second scan, not the reported capability: Available
	// is what the handler answers before any readiness scan has run at all
	// (internal/api falls back to Manager.HasScanner), so a capability check
	// here passes on the unfixed daemon too. A scanner asked twice can only
	// mean the worker that panicked was started again.
	waitFor(t, "the readiness scan to be restarted", func() bool {
		return scanner.calls.Load() >= 2
	})
	if capability := capabilityOf(t, handler); !capability.GetAvailable() {
		t.Errorf("after the retry got through, capability = %+v, want available", capability)
	}
}

func TestReadinessScanThatKeepsPanickingIsReportedNotFatal(t *testing.T) {
	scanner := &panickingScanner{}
	scanner.remaining.Store(1000)

	handler := handlerForTest(t, scanner)
	group := superviseCaptureReadiness(t.Context(), scanner, handler)
	t.Cleanup(func() { _ = group.Stop(context.Background()) })

	// The daemon keeps serving imported surveys; what changes is that the client
	// is told this host has no capture backend, with a reason and no stack.
	waitFor(t, "the supervisor to give up and say so", func() bool {
		c := capabilityOf(t, handler)
		return !c.GetAvailable() && c.GetReason() != ""
	})
	capability := capabilityOf(t, handler)
	if strings.Contains(capability.GetReason(), "goroutine ") {
		t.Errorf("the reason shown to an operator carries a stack trace: %q", capability.GetReason())
	}
}

// waitFor blocks until done reports true. The budget is generous: the readiness
// scan asks the OS for capture permission first, and on macOS that call alone
// takes several seconds, twice over when the worker is restarted.
func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()

	deadline := time.Now().Add(60 * time.Second)
	for {
		if done() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
