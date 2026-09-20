package survey

import (
	"image/color"
	"math"
)

// The coverage ramp: orange below the operator's threshold, teal above it,
// with a hard step where the two meet (owner decision 2026-09-15, UI-TRL-9).
//
// A traffic-light ramp answers the wrong question. It ranks signal on a scale
// an operator has to read off a legend, when the question in front of them is
// binary: does this room meet the requirement or not. Worse, its "very poor"
// red and "excellent" green collapse toward each other under deuteranopia —
// measured at a CIE76 distance of about 47 in the 2026-09-15 audit, which for
// the two ends of a scale is no distance at all.
//
// So the ramp diverges around the threshold rather than running through it:
//
//   - Below it, orange, getting darker as the signal gets worse.
//   - Above it, teal, getting lighter as the signal gets better.
//   - At it, a step in both hue and lightness, so the boundary is a line on
//     the map and not a shade an operator has to judge.
//
// Lightness carries the value on each side, so the ranking survives when the
// hue does not: the warm and cool families stay apart for a deuteranope
// (CIE76 ≥ 23 between every pair of legend stops under the Machado 2009
// severity-1.0 simulation, asserted in coveragescale_test.go), and within one
// family L* alone still says which of two cells is worse.
//
// Orange and teal specifically because they sit on the yellow–blue axis that
// deuteranopia leaves largely intact, and because teal is the product hue —
// "good" reads as the product's own colour.
var (
	// colorCoverageWorst is the deepest orange: the far end of the dead zone.
	colorCoverageWorst = color.RGBA{R: 0x7a, G: 0x3c, B: 0x06, A: colorChannelOpaque}
	// colorCoverageWeak is mid orange, halfway from the worst to the threshold.
	colorCoverageWeak = color.RGBA{R: 0xc2, G: 0x68, B: 0x0f, A: colorChannelOpaque}
	// colorCoverageMarginal is light orange: just short of the threshold.
	colorCoverageMarginal = color.RGBA{R: 0xf0, G: 0xa8, B: 0x60, A: colorChannelOpaque}
	// colorCoverageMeets is mid teal: the first value that meets the threshold.
	colorCoverageMeets = color.RGBA{R: 0x10, G: 0x64, B: 0x6b, A: colorChannelOpaque}
	// colorCoverageStrong is light teal: the top of the range.
	colorCoverageStrong = color.RGBA{R: 0x8f, G: 0xd6, B: 0xd8, A: colorChannelOpaque}
)

// thresholdStepEpsilon separates the two stops that make the step at the
// threshold. Zero would divide by zero in the interpolator; this is small
// enough that no value a radio reports falls between them — a hundredth of a
// decibel — and large enough to stay ordered in float64.
const thresholdStepEpsilon = 0.01

// thresholdEdgeMargin is how far from either end of the range the step is kept,
// as a fraction of the span. A step against an end is not a diverging scale:
// one side would be a single stop wide, the mid stop would collide with the
// step, and the map would be one colour with a sliver of the other.
const thresholdEdgeMargin = 0.05

// CoverageScale builds the ramp for a coverage metric around a threshold.
//
// The threshold is the operator's, not ours: it is the same number the
// dead-zone analysis uses to decide what counts as a dead zone, so the map
// and the findings under it agree about which areas fail. A threshold at or
// outside the range is pulled back inside it by thresholdEdgeMargin, because a
// scale whose step falls off its own ends is a scale with no step.
func CoverageScale(metric HeatmapType, threshold float64) ColorScale {
	minVal, maxVal := coverageRange(metric)

	// Keep room for a stop on each side of the step.
	margin := (maxVal - minVal) * thresholdEdgeMargin
	threshold = math.Max(minVal+margin, math.Min(maxVal-margin, threshold))

	return ColorScale{
		Name:   string(metric),
		MinVal: minVal,
		MaxVal: maxVal,
		Stops: []ColorStop{
			{Value: minVal, Color: colorCoverageWorst},
			{Value: (minVal + threshold) / 2, Color: colorCoverageWeak},
			{Value: threshold - thresholdStepEpsilon, Color: colorCoverageMarginal},
			{Value: threshold, Color: colorCoverageMeets},
			{Value: maxVal, Color: colorCoverageStrong},
		},
	}
}

// coverageRange is the span a metric's ramp covers, in the metric's own unit.
func coverageRange(metric HeatmapType) (minVal, maxVal float64) {
	if metric == HeatmapSNR {
		return snrMinimum, snrExcellent
	}
	return rssiNoSignal, rssiExcellent
}
