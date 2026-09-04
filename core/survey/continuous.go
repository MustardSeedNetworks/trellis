// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// captureGap is how long the loop waits between asking the radio.
//
// It is not the rate points are stored at. Each platform decides that for
// itself, and they differ by an order of magnitude: nl80211 triggers a real
// sweep on every call, three to four seconds each, while CoreWLAN sweeps about
// every fourteen seconds and answers anything asked in between from its cache
// in a tenth of a second. Measured on this Mac, 2026-09-04.
//
// So the loop polls, and stores only what is new (see freshSweep). The gap is
// short enough to notice a refresh promptly and long enough to leave the one
// radio reachable — a live view polling it, or a stop-and-go CapturePoint,
// waits at most one sweep behind this.
const captureGap = 2 * time.Second

// ErrNotWalking means the survey is not in the state that accepts samples, so
// there is nothing for a capture loop to write into.
var ErrNotWalking = errors.New("survey: not in progress")

// Position is where the operator says they are, in the floor plan's pixel
// space — the same space imported samples use.
type Position struct {
	X, Y int
}

// CaptureStatus is what a client needs to draw a walk it did not start: a
// reload mid-survey, or a second window on the same daemon.
//
// A stopped capture stays reportable until the next start, and carries the
// reason it stopped. A walk whose pins simply cease, with nothing on screen
// saying why, is the worst of the three possible outcomes — worse than an error
// banner and worse than a walk that kept trying.
type CaptureStatus struct {
	Position
	Running bool
	// LastError is the radio's own message from the sweep that ended the walk,
	// empty while it is running or when it was stopped deliberately.
	LastError string
}

// captureFailureLimit is how many consecutive failed sweeps end a walk.
//
// Some failures heal — an adapter that was busy for one sweep — and ending a
// building walk on the first of those would lose everything after it. Others
// never do: a macOS binary outside its entitled bundle, a host with no adapter,
// a Linux process without CAP_NET_ADMIN. core/survey cannot tell them apart
// without importing the capture package it is deliberately independent of, so
// it counts instead. Three sweeps is long enough that a transient is not
// mistaken for a dead radio and short enough that a dead one is not warned
// about once a second until somebody reads the log.
const captureFailureLimit = 3

// continuousCapture is one survey's running capture loop.
type continuousCapture struct {
	cancel context.CancelFunc
	// done closes when the goroutine has returned, so a stop is observable
	// rather than merely requested.
	done chan struct{}

	mu        sync.Mutex
	pos       Position
	markedAt  time.Time
	running   bool
	lastError string
	// lastSweep identifies the airspace the previous stored point was taken
	// from, so a cached repeat of it is not stored as a second measurement.
	lastSweep string
}

func (c *continuousCapture) position() Position {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pos
}

// mark records the operator's new position and returns the segment they just
// walked: where they were, when they said so, and now.
func (c *continuousCapture) mark(p Position, now time.Time) (from Position, markedAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	from, markedAt = c.pos, c.markedAt
	c.pos, c.markedAt = p, now
	return from, markedAt
}

func (c *continuousCapture) status() CaptureStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return CaptureStatus{Position: c.pos, Running: c.running, LastError: c.lastError}
}

// freshSweep reports whether these networks are a new measurement rather than
// the radio repeating what it last said, and remembers them if they are.
//
// A real two-minute walk on a Mac stored ninety-four points holding two
// distinct readings, fifty-two of them consecutively identical, because
// CoreWLAN answers from its cache between sweeps. Interpolating a heatmap over
// ninety points that were nine measurements is not a more detailed survey; it
// is the same survey claiming more than it measured.
//
// The comparison is the whole airspace, not the strongest BSS: two sweeps that
// agree on the nearest AP and differ on everything else are genuinely different
// observations of the room.
func (c *continuousCapture) freshSweep(networks []wifi.ScannedNetwork) bool {
	var b strings.Builder
	for i := range networks {
		fmt.Fprintf(&b, "%s@%d;", networks[i].BSSID, networks[i].Signal)
	}
	sweep := b.String()

	c.mu.Lock()
	defer c.mu.Unlock()
	if sweep == c.lastSweep {
		return false
	}
	c.lastSweep = sweep
	return true
}

// stopped records why the loop ended, so CapturingAt can answer it.
func (c *continuousCapture) stopped(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	c.lastError = reason
}

// StartContinuousCapture samples repeatedly at (x, y) until it is stopped.
//
// Calling it again on a running survey moves the capture rather than starting a
// second loop: it is the operator saying "I am here now", which is the whole
// interaction in a walking survey. Two loops would double the radio's load and
// interleave two positions into one walk.
//
// The loop is the continuous half of the two survey modes. CapturePoint remains
// the stop-and-go half — stand still, take one reading, move — and the two share
// the radio through the same door, so neither has to know about the other.
func (m *Manager) StartContinuousCapture(surveyID string, x, y int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.surveys[surveyID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}
	if s.Status != StatusInProgress {
		return fmt.Errorf("%w: %s is %s", ErrNotWalking, surveyID, s.Status)
	}
	if m.scanner == nil {
		return ErrNoScanner
	}

	// A capture that stopped itself is left in the map so its reason survives
	// for a client to read; starting again replaces it rather than marking it.
	if existing, ok := m.captures[surveyID]; ok && existing.status().Running {
		now := time.Now()
		from, markedAt := existing.mark(Position{X: x, Y: y}, now)
		// The readings taken since the last mark were taken on the way here.
		// Placed under the same lock that guards the sample slices, so a reader
		// sees the segment before or after, never mid-rewrite.
		s.placeWalkedSegment(from, Position{X: x, Y: y}, markedAt, now)
		s.UpdatedAt = now
		return m.persistSurvey(s)
	}

	ctx, cancel := context.WithCancel(context.Background())
	capture := &continuousCapture{
		cancel:   cancel,
		done:     make(chan struct{}),
		pos:      Position{X: x, Y: y},
		markedAt: time.Now(),
		running:  true,
	}
	if m.captures == nil {
		m.captures = make(map[string]*continuousCapture)
	}
	m.captures[surveyID] = capture

	go m.captureLoop(ctx, surveyID, capture)
	return nil
}

// StopContinuousCapture ends a survey's capture loop and waits for it to
// return.
//
// Waiting matters: the caller's next act is usually to complete the survey or
// close the store, and a sweep still in flight would write into it afterwards.
// Stopping a survey that is not capturing is not an error — pause, complete and
// delete all call this without knowing.
func (m *Manager) StopContinuousCapture(surveyID string) {
	m.mu.Lock()
	capture, ok := m.captures[surveyID]
	if ok {
		delete(m.captures, surveyID)
	}
	m.mu.Unlock()

	if !ok {
		return
	}
	capture.cancel()
	<-capture.done
}

// CapturingAt reports a survey's capture loop — where it is sampling, whether
// it is still running, and why it stopped if it is not. nil means the survey has
// never had one. A client that reloads mid-walk needs this to know the walk is
// still running without having started it.
func (m *Manager) CapturingAt(surveyID string) *CaptureStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	capture, ok := m.captures[surveyID]
	if !ok {
		return nil
	}
	status := capture.status()
	return &status
}

// stopEveryCapture ends every running loop. Close and each terminal state
// change go through it.
func (m *Manager) stopEveryCapture() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.captures))
	for id := range m.captures {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		m.StopContinuousCapture(id)
	}
}

// captureLoop scans, stores, waits, repeats.
//
// A scan failure does not end the walk. An operator carrying a laptop through a
// building hits transient radio failures — an adapter that was busy, a driver
// that returned nothing for one sweep — and ending the survey on the first of
// them would lose the rest of the walk over a recoverable condition. A rejected
// *sample* is different: it means the survey stopped accepting them, and there
// is nothing left for the loop to do.
func (m *Manager) captureLoop(ctx context.Context, surveyID string, capture *continuousCapture) {
	defer close(capture.done)

	failures := 0
	for {
		at := capture.position()
		err := m.captureIfFresh(ctx, surveyID, capture, at)
		switch {
		case err == nil:
			failures = 0

		case ctx.Err() != nil:
			// Stopped deliberately. StopContinuousCapture has already taken the
			// entry out of the map and owns what happens next.
			return

		case errors.Is(err, ErrSurveyNotFound), errors.Is(err, ErrNotWalking):
			m.captureEnded(capture, "")
			return

		default:
			failures++
			if failures >= captureFailureLimit {
				slog.Error("continuous capture stopped: the radio kept failing",
					"survey", surveyID, "sweeps", failures, "error", err)
				m.captureEnded(capture, err.Error())
				return
			}
			slog.Warn("continuous capture: sweep failed, walk continues",
				"survey", surveyID, "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(m.captureGap()):
		}
	}
}

// captureIfFresh takes one reading and stores it only if the radio has moved on
// since the last one.
//
// A repeated cache entry is not a failure — the walk is fine, there is simply
// nothing new to record — so it returns nil and the loop carries on. That is
// also why this does not go through CapturePoint: the decision has to happen
// between the scan and the store, and CapturePoint does both.
func (m *Manager) captureIfFresh(
	ctx context.Context,
	surveyID string,
	capture *continuousCapture,
	at Position,
) error {
	networks, err := m.Scan(ctx)
	if err != nil {
		return fmt.Errorf("sweep for %s: %w", surveyID, err)
	}
	if !capture.freshSweep(networks) {
		return nil
	}

	sample := &PassiveSample{Networks: make([]*wifi.ScannedNetwork, len(networks))}
	for i := range networks {
		sample.Networks[i] = &networks[i]
	}
	return m.AddSample(surveyID, at.X, at.Y, sample)
}

// captureEnded records why a loop stopped itself.
//
// The entry stays in the map, marked not running and carrying the reason, so a
// client can read it. Deleting it instead would make a walk whose pins stop
// appearing say nothing at all about why.
//
// Under the manager lock so a start cannot read Running while it is being
// cleared. A start that wins the race by microseconds moves a loop that is
// already exiting; the next CapturingAt then reports it stopped, with the
// reason, which is the same answer the operator would have got a moment later.
func (m *Manager) captureEnded(capture *continuousCapture, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	capture.stopped(reason)
}

// captureGap is the loop's cadence, overridable only by the package's own tests
// so they do not have to sleep out a real one.
func (m *Manager) captureGap() time.Duration {
	if m.testCaptureGap > 0 {
		return m.testCaptureGap
	}
	return captureGap
}
