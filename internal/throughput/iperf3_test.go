// SPDX-License-Identifier: BUSL-1.1

package throughput

import (
	"errors"
	"strings"
	"testing"
)

func TestParseReport(t *testing.T) {
	t.Parallel()

	t.Run("reads the rate in the unit the survey stores", func(t *testing.T) {
		t.Parallel()

		// iperf3 reports bits per second. Mbps is 1e6 bits, not 1<<20 — the
		// megabit is decimal, and dividing by 1048576 would understate every
		// reading by 4.9%.
		mbps, err := parseReport([]byte(`{"end":{"sum_received":{"bits_per_second":221000000}}}`))
		if err != nil {
			t.Fatalf("parseReport: %v", err)
		}
		if mbps != 221 {
			t.Errorf("rate = %v Mbps, want 221", mbps)
		}
	})

	t.Run("reports iperf3's own message rather than an exit code", func(t *testing.T) {
		t.Parallel()

		// A busy server, a refused connection and an unknown host all arrive
		// this way. "exit status 1" tells the operator nothing they can act on.
		_, err := parseReport([]byte(`{"error":"the server is busy running a test"}`))
		if err == nil {
			t.Fatal("want an error")
		}
		if !strings.Contains(err.Error(), "busy running a test") {
			t.Errorf("error = %q, want iperf3's own message", err)
		}
	})

	t.Run("a report with no rate is not a zero-speed link", func(t *testing.T) {
		t.Parallel()

		// Reporting 0 Mbps would draw a dead spot on the throughput layer at a
		// position where nothing was measured at all.
		if _, err := parseReport([]byte(`{"end":{}}`)); err == nil {
			t.Fatal("want an error for a report carrying no rate")
		}
	})

	t.Run("output that is not a report says so", func(t *testing.T) {
		t.Parallel()

		if _, err := parseReport([]byte("iperf3: command not found")); err == nil {
			t.Fatal("want an error for output that is not JSON")
		}
	})
}

func TestMeasureWithoutIperf3(t *testing.T) {
	t.Parallel()

	// The one failure an operator fixes by installing something. It has to be
	// distinguishable from a network failure, because the remedies differ.
	meter := Meter{binary: "iperf3-that-is-not-installed"}
	_, err := meter.Measure(t.Context(), "", "10.0.0.1", 1)
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Measure without iperf3 = %v, want ErrNotInstalled", err)
	}
	if !strings.Contains(err.Error(), "iperf3") {
		t.Errorf("error %q does not name what is missing", err)
	}
}
