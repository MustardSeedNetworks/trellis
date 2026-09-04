// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"math"
	"time"
)

// placeAlongSegment gives the readings taken between two marks a position.
//
// A walking survey has one position anybody actually recorded: where the
// operator said they were, when they said it. Everything the radio sampled
// between two marks was taken somewhere along the way, and nobody wrote down
// where. Spreading those readings across the segment by the fraction of its
// elapsed time each one falls at is the honest approximation — honest because
// the points it moves are marked Interpolated, so a reader can tell a claim
// about a position from a record of one. A survey that presented them as pinned
// would be asserting a precision nobody measured.
//
// It is a straight line at a constant pace, which is what makes it an
// approximation rather than a measurement: an operator who paused halfway or
// walked around a pillar leaves readings placed where they were not. More marks
// is the remedy, and it is the operator's to apply — which is why the walk's
// interaction is "mark where you are", not "start and stop".
//
// Points are replaced, not moved. The manager hands out survey snapshots that
// share sample pointers, on the argument that a stored point never changes;
// mutating one here would break that quietly.
func placeAlongSegment(
	samples []*SamplePoint,
	from, to Position,
	markedAt, now time.Time,
) []*SamplePoint {
	elapsed := now.Sub(markedAt)
	placed := make([]*SamplePoint, len(samples))

	for i, sample := range samples {
		if sample == nil || !sample.Timestamp.After(markedAt) || sample.Timestamp.After(now) {
			// Before the previous mark, or after this one: another segment's
			// reading, or one that has not been walked yet.
			placed[i] = sample
			continue
		}

		// elapsed is always positive here: the interval above is half-open, so
		// reaching this line means the reading is strictly after markedAt and
		// at or before now.
		fraction := float64(sample.Timestamp.Sub(markedAt)) / float64(elapsed)

		moved := *sample
		moved.X = lerp(from.X, to.X, fraction)
		moved.Y = lerp(from.Y, to.Y, fraction)
		moved.Interpolated = true
		placed[i] = &moved
	}
	return placed
}

// lerp is one axis of the segment, rounded to the integer pixel space the floor
// plan and every stored point use.
func lerp(from, to int, fraction float64) int {
	return int(math.Round(float64(from) + float64(to-from)*fraction))
}
