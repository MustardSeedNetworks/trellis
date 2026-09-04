// SPDX-License-Identifier: BUSL-1.1

package throughput

import (
	"errors"
	"net"
	"os"
	"path/filepath"
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

// TestBindAddress covers the check that keeps a Wi-Fi survey from measuring an
// ethernet cable.
//
// On a host with both — dev-srv-ubuntu has ens18 beside its USB adapter — an
// iperf3 that let the kernel route goes out the wire, and the throughput layer
// then reports a number that has nothing to do with the radio.
func TestBindAddress(t *testing.T) {
	t.Parallel()

	t.Run("no interface lets the host route", func(t *testing.T) {
		t.Parallel()

		// Right on a laptop with one adapter, which is why it is not an error.
		got, err := bindAddress("")
		if err != nil || got != "" {
			t.Fatalf("bindAddress(\"\") = %q, %v; want an empty bind and no error", got, err)
		}
	})

	t.Run("an interface that does not exist says so", func(t *testing.T) {
		t.Parallel()

		if _, err := bindAddress("not-an-interface0"); err == nil {
			t.Fatal("want an error naming the interface")
		}
	})

	t.Run("an interface with an address binds to it", func(t *testing.T) {
		t.Parallel()

		// Loopback is the one interface every platform has with an IPv4
		// address, so it stands in for an associated adapter.
		name := loopbackName(t)
		got, err := bindAddress(name)
		if err != nil {
			t.Fatalf("bindAddress(%q): %v", name, err)
		}
		if got != "127.0.0.1" {
			t.Errorf("bind = %q, want the interface's IPv4 address", got)
		}
	})
}

// loopbackName finds the loopback interface, whose name differs per platform.
func loopbackName(t *testing.T) string {
	t.Helper()

	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("list interfaces: %v", err)
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			return iface.Name
		}
	}
	t.Skip("no loopback interface on this host")
	return ""
}

// TestMeasureRunsIperf3AndReadsBothDirections drives the exec path against a
// stub that records how it was called, which is the only way to check the
// arguments a real run depends on.
func TestMeasureRunsIperf3AndReadsBothDirections(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	stub := filepath.Join(dir, "iperf3-stub")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> " + argsFile + "\n" +
		`echo '{"end":{"sum_received":{"bits_per_second":221000000}}}'` + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	meter := Meter{binary: stub}
	got, err := meter.Measure(t.Context(), "", "10.44.30.30", 4)
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if got.DownloadMbps != 221 || got.UploadMbps != 221 {
		t.Errorf("sample = %+v, want 221 Mbps each way", got)
	}
	// Latency, jitter and loss stay at zero deliberately: a TCP test does not
	// measure them, and filling them in would report a perfect link.
	if got.Latency != 0 || got.Jitter != 0 || got.PacketLoss != 0 {
		t.Errorf("sample carries unmeasured figures: %+v", got)
	}

	recorded, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read recorded args: %v", err)
	}
	runs := strings.Split(strings.TrimSpace(string(recorded)), "\n")
	if len(runs) != 2 {
		t.Fatalf("iperf3 ran %d times, want once per direction", len(runs))
	}
	for _, run := range runs {
		if !strings.Contains(run, "-c 10.44.30.30") || !strings.Contains(run, "-t 4") {
			t.Errorf("run %q does not carry the server and duration", run)
		}
	}
	// -R is the server sending: what the client downloads. Exactly one of the
	// two runs is reversed, or both directions report the same thing.
	reversed := 0
	for _, run := range runs {
		if strings.Contains(run, "-R") {
			reversed++
		}
	}
	if reversed != 1 {
		t.Errorf("%d of 2 runs were reversed, want exactly 1", reversed)
	}
}

func TestMeasureSurfacesIperf3sOwnFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	stub := filepath.Join(dir, "iperf3-stub")
	// A refused connection: iperf3 exits non-zero with the reason in the
	// report, which is what the operator needs rather than "exit status 1".
	script := "#!/bin/sh\n" +
		`echo '{"error":"unable to connect to server - Connection refused"}'` + "\nexit 1\n"
	if err := os.WriteFile(stub, []byte(script), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	meter := Meter{binary: stub}
	_, err := meter.Measure(t.Context(), "", "10.44.30.30", 1)
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "Connection refused") {
		t.Errorf("error = %q, want iperf3's own reason", err)
	}
}
