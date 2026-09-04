// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"testing"
	"time"
)

// TestPlaceAlongSegment covers the claim a continuous walk makes about where
// its readings were taken.
//
// A walking operator marks where they are now and again; between two marks the
// radio kept sampling and nobody recorded a position for those readings. Placing
// them by the fraction of the segment's time each one falls at is the honest
// approximation — and it is an approximation, which is why the points it moves
// are marked as interpolated rather than passed off as pinned.
func TestPlaceAlongSegment(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	at := func(seconds int) time.Time { return base.Add(time.Duration(seconds) * time.Second) }
	point := func(seconds int) *SamplePoint {
		return &SamplePoint{X: 10, Y: 10, Timestamp: at(seconds)}
	}

	from, to := Position{X: 0, Y: 0}, Position{X: 400, Y: 200}

	t.Run("places each reading at its share of the segment", func(t *testing.T) {
		t.Parallel()

		got := placeAlongSegment(
			[]*SamplePoint{point(10), point(20), point(30)},
			from, to, at(0), at(40),
		)

		want := []Position{{X: 100, Y: 50}, {X: 200, Y: 100}, {X: 300, Y: 150}}
		for i, p := range got {
			if p.X != want[i].X || p.Y != want[i].Y {
				t.Errorf("point %d at (%d,%d), want (%d,%d)", i, p.X, p.Y, want[i].X, want[i].Y)
			}
			if !p.Interpolated {
				t.Errorf("point %d is not marked interpolated", i)
			}
		}
	})

	t.Run("replaces rather than mutates", func(t *testing.T) {
		t.Parallel()

		original := point(10)
		got := placeAlongSegment([]*SamplePoint{original}, from, to, at(0), at(20))

		// The manager hands snapshots out that share sample pointers, on the
		// argument that a stored point never changes. Moving one in place would
		// break that quietly, and only under -race.
		if got[0] == original {
			t.Fatal("the same pointer came back; a reader holding it would see the point move")
		}
		if original.X != 10 || original.Y != 10 || original.Interpolated {
			t.Errorf("the original point was modified: %+v", original)
		}
	})

	t.Run("a reading taken at the mark lands on it", func(t *testing.T) {
		t.Parallel()

		got := placeAlongSegment([]*SamplePoint{point(20)}, from, to, at(0), at(20))
		if got[0].X != to.X || got[0].Y != to.Y {
			t.Errorf("got (%d,%d), want the segment's end (%d,%d)", got[0].X, got[0].Y, to.X, to.Y)
		}
	})

	t.Run("two marks in one instant place nothing", func(t *testing.T) {
		t.Parallel()

		// The interval a reading has to fall in is (markedAt, now]. Two marks in
		// the same instant make that empty, so there is nothing to spread and no
		// elapsed time to divide by — the guard is the interval, not a special
		// case inside the arithmetic.
		original := point(0)
		got := placeAlongSegment([]*SamplePoint{original}, from, to, at(0), at(0))
		if got[0] != original {
			t.Errorf("a point was placed across a segment of no duration: %+v", got[0])
		}
	})

	t.Run("readings outside the segment are left alone", func(t *testing.T) {
		t.Parallel()

		// A point from before the previous mark belongs to an earlier segment,
		// which was placed when that mark arrived; one from after this mark has
		// not happened yet.
		before, after := point(-5), point(50)
		got := placeAlongSegment([]*SamplePoint{before, point(10), after}, from, to, at(0), at(20))

		if got[0] != before || got[2] != after {
			t.Error("a point outside the segment was replaced")
		}
		if !got[1].Interpolated {
			t.Error("the point inside the segment was not placed")
		}
	})

	t.Run("no readings is not an error", func(t *testing.T) {
		t.Parallel()

		if got := placeAlongSegment(nil, from, to, at(0), at(20)); len(got) != 0 {
			t.Errorf("got %d points from none", len(got))
		}
	})
}
