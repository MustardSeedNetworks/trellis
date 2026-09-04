// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// ErrNoScanner means the manager was built without a capture backend, so it can
// serve imported surveys but cannot take one.
var ErrNoScanner = errors.New("survey: no Wi-Fi capture backend configured")

// CapturePoint scans the airspace and records what it sees as a passive sample
// at (x, y) on the survey's active floor. It is the live equivalent of one
// point in an imported walk.
//
// The scan runs *outside* the manager lock. An active scan blocks for seconds
// on every platform, and holding the lock across it would stall every other
// request on the daemon for the duration of each survey point. The consequence
// is that the survey can change while a scan is in flight — if it is paused or
// completed meanwhile, [Manager.AddSample] rejects the sample, which is the
// correct outcome rather than a race to defend against.
func (m *Manager) CapturePoint(ctx context.Context, surveyID string, x, y int) (*PassiveSample, error) {
	m.mu.RLock()
	scanner := m.scanner
	m.mu.RUnlock()

	if scanner == nil {
		return nil, ErrNoScanner
	}

	networks, err := m.scan(ctx, scanner)
	if err != nil {
		return nil, fmt.Errorf("scan at (%d,%d): %w", x, y, err)
	}

	sample := &PassiveSample{Networks: make([]*wifi.ScannedNetwork, len(networks))}
	for i := range networks {
		sample.Networks[i] = &networks[i]
	}
	// AddSample aggregates the sample on the way in, which is what sorts
	// Networks strongest-first and makes Networks[0] the AP serving this point
	// for the heatmap, coverage analysis and report. It mutates in place, so
	// the value returned here is the aggregated one.
	if err := m.AddSample(surveyID, x, y, sample); err != nil {
		return nil, err
	}
	return sample, nil
}

// Scan reads the airspace without recording it anywhere.
//
// This is what the live view runs on. It is deliberately not a survey
// operation: a reading taken while an operator is looking at a page belongs to
// no floor and no position, and storing it would fill a walk with points nobody
// stood at.
func (m *Manager) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	m.mu.RLock()
	scanner := m.scanner
	m.mu.RUnlock()

	if scanner == nil {
		return nil, ErrNoScanner
	}

	networks, err := m.scan(ctx, scanner)
	if err != nil {
		return nil, err
	}
	// Strongest first, the same order CapturePoint's aggregation puts a stored
	// sample in. A radio reports its cache in whatever order it holds it, and a
	// reader comparing a live view against a walked point should not have to
	// account for the two being ordered differently.
	slices.SortStableFunc(networks, func(a, b wifi.ScannedNetwork) int {
		return b.Signal - a.Signal
	})
	return networks, nil
}

// scan is the single door to the radio.
//
// There is one adapter, and an OS scan is a blocking call into its driver, so a
// live view polling while a walk captures a point would have the two of them
// inside that call together. scanMu makes the second wait for the first instead
// — a live poll behind a survey point is a second of staleness, which is the
// honest cost of one radio, where two concurrent sweeps are a driver's problem.
//
// It is its own lock rather than the manager's: m.mu guards the survey map, and
// holding that across a multi-second driver call would stall every other
// request on the daemon.
func (m *Manager) scan(ctx context.Context, scanner Scanner) ([]wifi.ScannedNetwork, error) {
	m.scanMu.Lock()
	defer m.scanMu.Unlock()

	// Checked under the lock as well as by the backend: a caller that gave up
	// while queueing behind another scan should not then start one.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return scanner.Scan(ctx)
}
