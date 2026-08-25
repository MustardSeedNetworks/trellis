// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"context"
	"errors"
	"fmt"

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

	networks, err := scanner.Scan(ctx)
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
