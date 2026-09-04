// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Conditions a caller of MeasureThroughput can act on. They are separate
// because the remedies are: one is a survey that names no server, the other a
// host with no way to reach one.
var (
	// ErrNoThroughputTarget means the survey names no server to measure
	// against.
	ErrNoThroughputTarget = errors.New("survey: no throughput target set")

	// ErrNoThroughputMeter means the manager was built without a way to run a
	// throughput test.
	ErrNoThroughputMeter = errors.New("survey: no throughput meter configured")
)

// defaultTestDuration is how long one direction runs when a survey names no
// duration. Long enough for TCP to leave slow start and settle, short enough
// that an operator standing at a point is not there for a minute — the test
// runs twice, once each way.
const defaultTestDuration = 5

// SetThroughputTarget names the server a survey's active measurements run
// against, and how long each direction runs.
//
// It is survey state rather than a per-measurement argument because it is the
// same server for every point on a walk: comparing two positions only means
// something if both were measured against the same thing.
func (m *Manager) SetThroughputTarget(surveyID, server string, durationSec int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	survey, exists := m.surveys[surveyID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}

	survey.IperfServer = server
	survey.TestDuration = durationSec
	survey.UpdatedAt = time.Now()
	return m.persistSurvey(survey)
}

// MeasureThroughput runs a throughput test at (x, y) and stores what it
// measured as a point on the survey's active floor.
//
// This is the active half of a survey, and it is stop-and-go by nature: the
// test takes seconds in each direction and means nothing if the operator moves
// during it. There is deliberately no continuous equivalent.
//
// The reading is tagged with the AP the link ran over, read from a scan taken
// alongside it. Without that the throughput layer is a set of numbers with no
// radio behind them, and a slow point cannot be told from a distant one.
func (m *Manager) MeasureThroughput(
	ctx context.Context,
	surveyID string,
	x, y int,
) (*ThroughputSample, error) {
	m.mu.RLock()
	meter := m.throughputMeter
	survey, exists := m.surveys[surveyID]
	var server, iface string
	duration := defaultTestDuration
	if exists {
		server, iface = survey.IperfServer, survey.Interface
		if survey.TestDuration > 0 {
			duration = survey.TestDuration
		}
	}
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}
	if server == "" {
		return nil, ErrNoThroughputTarget
	}
	if meter == nil {
		return nil, ErrNoThroughputMeter
	}

	// Tagged before the test rather than after: the association is what the
	// measurement is about, and reading it first means a roam mid-test is
	// visible as a mismatch rather than silently attributed to the new AP.
	ssid, bssid, rssi := m.association(ctx)

	// Outside the radio lock. A test runs for seconds in each direction, and
	// holding the scanner across it would stall a live view and any walk for
	// the whole of it — for a measurement that does not use the scanner.
	sample, err := meter.Measure(ctx, iface, server, duration)
	if err != nil {
		// Nothing is stored. A point saved for a failed test is a zero-speed
		// reading at a position where nothing was measured — a dead spot the
		// survey invented.
		return nil, fmt.Errorf("measure at (%d,%d): %w", x, y, err)
	}

	sample.SSID, sample.BSSID, sample.RSSI = ssid, bssid, rssi
	if err := m.AddSample(surveyID, x, y, &sample); err != nil {
		return nil, err
	}
	return &sample, nil
}

// association is the AP this host is joined to, from a scan.
//
// A scan that fails, or a host that is not associated, leaves the reading
// untagged rather than failing the measurement: the throughput number is real
// either way, and refusing to store it because the tag is missing would lose a
// measurement over its label. Windows never reports an association (#294).
func (m *Manager) association(ctx context.Context) (ssid, bssid string, rssi int) {
	networks, err := m.Scan(ctx)
	if err != nil {
		return "", "", 0
	}
	for i := range networks {
		if networks[i].Associated {
			return networks[i].SSID, networks[i].BSSID, networks[i].Signal
		}
	}
	return "", "", 0
}
