// SPDX-License-Identifier: BUSL-1.1

//go:build !e2e

package main

import (
	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/internal/throughput"
)

// newThroughputMeter returns the host's iperf3.
//
// It takes the survey's interface per measurement, so a host with ethernet as
// well as Wi-Fi measures the radio rather than the wire.
func newThroughputMeter() survey.ThroughputMeter {
	return throughput.NewMeter()
}
