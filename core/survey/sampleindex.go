package survey

import (
	"math"
	"slices"
)

// samplesPerBucket is the average number of samples the index puts in one
// bucket. A nearest-neighbour query then reads a handful of buckets, whatever
// the walk's length.
const samplesPerBucket = 4

// sampleIndex is a uniform bucket grid over the samples' bounding box, stored
// compactly: bucket b holds order[start[b]:start[b+1]], each entry an index
// into samples.
type sampleIndex struct {
	samples    []SampleValue
	minX, minY float64
	side       float64
	cols, rows int
	start      []int
	order      []int
}

// neighbour is one candidate of a nearest-neighbour query.
type neighbour struct {
	distSq float64
	index  int
}

func newSampleIndex(samples []SampleValue) *sampleIndex {
	idx := &sampleIndex{samples: samples}
	if len(samples) == 0 {
		return idx
	}

	minX, minY := samples[0].Point.X, samples[0].Point.Y
	maxX, maxY := minX, minY
	for _, s := range samples[1:] {
		minX, maxX = min(minX, s.Point.X), max(maxX, s.Point.X)
		minY, maxY = min(minY, s.Point.Y), max(maxY, s.Point.Y)
	}
	w, h := maxX-minX, maxY-minY

	// The side gives about samplesPerBucket samples a bucket over the box,
	// and never less than the long edge over the bucket count, so a walk
	// along a single corridor line still makes a bounded number of buckets.
	buckets := float64(max(1, len(samples)/samplesPerBucket))
	side := max(math.Sqrt(w*h/buckets), max(w, h)/buckets)
	if side == 0 {
		side = 1 // every sample at one spot: one bucket
	}

	idx.minX, idx.minY, idx.side = minX, minY, side
	idx.cols = int(w/side) + 1
	idx.rows = int(h/side) + 1

	bucketOf := make([]int, len(samples))
	idx.start = make([]int, idx.cols*idx.rows+1)
	for i, s := range samples {
		b := idx.bucket(idx.cell(s.Point.X, s.Point.Y))
		bucketOf[i] = b
		idx.start[b+1]++
	}
	for b := range idx.cols * idx.rows {
		idx.start[b+1] += idx.start[b]
	}
	next := slices.Clone(idx.start[:len(idx.start)-1])
	idx.order = make([]int, len(samples))
	for i, b := range bucketOf {
		idx.order[next[b]] = i
		next[b]++
	}
	return idx
}

// cell is the bucket column and row holding (x, y), clamped onto the grid: a
// heatmap cell outside the walked area searches from the nearest edge bucket.
func (idx *sampleIndex) cell(x, y float64) (int, int) {
	col := int(math.Floor((x - idx.minX) / idx.side))
	row := int(math.Floor((y - idx.minY) / idx.side))
	return min(max(col, 0), idx.cols-1), min(max(row, 0), idx.rows-1)
}

func (idx *sampleIndex) bucket(col, row int) int {
	return row*idx.cols + col
}

// nearest fills buf with the k samples nearest (x, y), closest first, and
// returns it. Equal distances keep the lower sample index first, so the
// result does not depend on bucket order.
//
// The search widens one ring of buckets at a time around the query's bucket.
// It stops once it holds k candidates and the k-th is no farther than the
// nearest bucket it has not read yet.
func (idx *sampleIndex) nearest(x, y float64, k int, buf []neighbour) []neighbour {
	buf = buf[:0]
	if len(idx.samples) == 0 || k <= 0 {
		return buf
	}

	cx, cy := idx.cell(x, y)
	for r := 0; ; r++ {
		for row := max(cy-r, 0); row <= min(cy+r, idx.rows-1); row++ {
			if row == cy-r || row == cy+r {
				for col := max(cx-r, 0); col <= min(cx+r, idx.cols-1); col++ {
					buf = idx.offerBucket(buf, k, x, y, col, row)
				}
				continue
			}
			// Inside the ring only its left and right buckets are new.
			if cx-r >= 0 {
				buf = idx.offerBucket(buf, k, x, y, cx-r, row)
			}
			if cx+r < idx.cols {
				buf = idx.offerBucket(buf, k, x, y, cx+r, row)
			}
		}

		// The unread buckets lie beyond the block's sides that are not grid
		// edges; the query is never outside such a side, so its distance to
		// each one bounds every unread sample from below.
		gap, open := math.Inf(1), false
		if cx-r > 0 {
			gap, open = min(gap, x-(idx.minX+float64(cx-r)*idx.side)), true
		}
		if cx+r < idx.cols-1 {
			gap, open = min(gap, idx.minX+float64(cx+r+1)*idx.side-x), true
		}
		if cy-r > 0 {
			gap, open = min(gap, y-(idx.minY+float64(cy-r)*idx.side)), true
		}
		if cy+r < idx.rows-1 {
			gap, open = min(gap, idx.minY+float64(cy+r+1)*idx.side-y), true
		}
		if !open {
			return buf
		}
		if len(buf) == k && buf[k-1].distSq <= gap*gap {
			return buf
		}
	}
}

func (idx *sampleIndex) offerBucket(buf []neighbour, k int, x, y float64, col, row int) []neighbour {
	b := idx.bucket(col, row)
	for _, si := range idx.order[idx.start[b]:idx.start[b+1]] {
		buf = idx.offer(buf, k, x, y, si)
	}
	return buf
}

// offer inserts sample si into the sorted candidate list when it is among the
// k nearest so far.
func (idx *sampleIndex) offer(buf []neighbour, k int, x, y float64, si int) []neighbour {
	dx := x - idx.samples[si].Point.X
	dy := y - idx.samples[si].Point.Y
	c := neighbour{distSq: dx*dx + dy*dy, index: si}

	if len(buf) == k && !closer(c, buf[k-1]) {
		return buf
	}
	if len(buf) < k {
		buf = append(buf, c)
	} else {
		buf[k-1] = c
	}
	for j := len(buf) - 1; j > 0 && closer(buf[j], buf[j-1]); j-- {
		buf[j], buf[j-1] = buf[j-1], buf[j]
	}
	return buf
}

func closer(a, b neighbour) bool {
	return a.distSq < b.distSq || (a.distSq == b.distSq && a.index < b.index)
}
