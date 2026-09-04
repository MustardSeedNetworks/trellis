// SPDX-License-Identifier: BUSL-1.1

// Package throughput measures how fast a link actually carries data, by running
// iperf3 against a server the operator names.
//
// It is the active half of a survey. A passive scan says how strong the radio
// hears an AP; it says nothing about what the link does under load, which is
// the number a person standing in a room actually cares about. The two are not
// interchangeable — a strong signal on a congested channel carries very little.
package throughput

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

// Conditions a caller can act on.
var (
	// ErrNotInstalled means iperf3 is not on this host. The one failure an
	// operator fixes by installing something, so it is distinguishable from a
	// network failure, whose remedy is different.
	ErrNotInstalled = errors.New("throughput: iperf3 is not installed")

	// ErrNoAddress means the survey's interface has no address to send from,
	// which on a Wi-Fi adapter means it is not associated.
	ErrNoAddress = errors.New("throughput: interface has no address")
)

// bitsPerMegabit is decimal, as every network rate is. Dividing by 1<<20
// instead would understate every reading by 4.9%.
const bitsPerMegabit = 1e6

// Meter runs iperf3. It implements [survey.ThroughputMeter].
type Meter struct {
	// binary is the iperf3 to run, so a test can point at one that is not
	// there. Empty means "iperf3", found on PATH.
	binary string
}

// NewMeter returns a meter backed by the host's iperf3.
func NewMeter() *Meter { return &Meter{} }

// Measure runs a download and an upload against server and returns them.
//
// Latency, jitter and loss are left at zero: a TCP test does not measure them,
// and filling them with zeros would report a perfect link. They belong to a UDP
// test, which is a different measurement and not one this row makes.
func (m *Meter) Measure(
	ctx context.Context,
	iface, server string,
	durationSec int,
) (survey.ThroughputSample, error) {
	bind, err := bindAddress(iface)
	if err != nil {
		return survey.ThroughputSample{}, err
	}

	download, err := m.run(ctx, server, durationSec, bind, true)
	if err != nil {
		return survey.ThroughputSample{}, err
	}
	upload, err := m.run(ctx, server, durationSec, bind, false)
	if err != nil {
		return survey.ThroughputSample{}, err
	}

	return survey.ThroughputSample{DownloadMbps: download, UploadMbps: upload}, nil
}

// bindAddress is the address on the survey's interface that iperf3 sends from.
//
// A survey that names no interface lets the host route, which is right on a
// laptop with one adapter. On anything multi-homed it is how a Wi-Fi survey
// ends up reporting the speed of an ethernet cable.
func bindAddress(name string) (string, error) {
	if name == "" {
		return "", nil
	}

	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", fmt.Errorf("throughput: interface %s: %w", name, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("throughput: addresses of %s: %w", name, err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.To4() == nil || ipNet.IP.IsLinkLocalUnicast() {
			continue
		}
		return ipNet.IP.String(), nil
	}
	// An associated adapter has an address. One without is either down or not
	// joined to anything, and either way there is no Wi-Fi link to measure.
	return "", fmt.Errorf("%w: %s is not associated", ErrNoAddress, name)
}

// run performs one direction and returns it in Mbps.
func (m *Meter) run(
	ctx context.Context,
	server string,
	durationSec int,
	bind string,
	reverse bool,
) (float64, error) {
	binary := m.binary
	if binary == "" {
		binary = "iperf3"
	}
	if _, err := exec.LookPath(binary); err != nil {
		return 0, fmt.Errorf("%w: install iperf3 to measure throughput", ErrNotInstalled)
	}

	args := []string{"-c", server, "-J", "-t", strconv.Itoa(durationSec)}
	if bind != "" {
		args = append(args, "-B", bind)
	}
	if reverse {
		// The server sends: what the client downloads, which is the direction a
		// survey is usually about.
		args = append(args, "-R")
	}

	// Output, not CombinedOutput: iperf3 writes its report to stdout and
	// diagnostics to stderr, and mixing them makes the JSON unparseable exactly
	// when there is a diagnostic worth reading.
	out, err := exec.CommandContext(ctx, binary, args...).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(out) > 0 {
			// A refused connection, a busy server and an unknown host all
			// arrive as a non-zero exit with the reason in the report.
			if _, parseErr := parseReport(out); parseErr != nil {
				return 0, parseErr
			}
		}
		return 0, fmt.Errorf("throughput: run iperf3 against %s: %w", server, err)
	}
	return parseReport(out)
}

// report is the part of iperf3's JSON a survey reads.
type report struct {
	Error string `json:"error"`
	End   struct {
		SumReceived struct {
			BitsPerSecond *float64 `json:"bits_per_second"`
		} `json:"sum_received"`
	} `json:"end"`
}

// parseReport turns one iperf3 report into a rate in Mbps.
//
// sum_received, not sum_sent: what arrived is what the link carried. The two
// differ by whatever was still in flight when the test ended.
func parseReport(out []byte) (float64, error) {
	var r report
	if err := json.Unmarshal(out, &r); err != nil {
		return 0, fmt.Errorf("throughput: iperf3 said %q", strings.TrimSpace(string(out)))
	}
	if r.Error != "" {
		return 0, fmt.Errorf("throughput: iperf3: %s", r.Error)
	}
	if r.End.SumReceived.BitsPerSecond == nil {
		// Absent is not zero. Reporting 0 Mbps would draw a dead spot at a
		// position where nothing was measured at all.
		return 0, errors.New("throughput: iperf3 reported no rate")
	}
	return *r.End.SumReceived.BitsPerSecond / bitsPerMegabit, nil
}
