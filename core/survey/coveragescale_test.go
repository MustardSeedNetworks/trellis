package survey

import (
	"image/color"
	"math"
	"testing"
)

// The properties the coverage ramp is FOR (UI-TRL-9, trellis#484).
//
// These assert behaviour, not hex: lightness ranks the value on each side of
// the threshold, the threshold is a step rather than a shade, and the legend's
// stops stay apart for a deuteranope. A palette change that keeps those
// properties passes; one that quietly reintroduces a traffic light does not.

// relativeLuminanceToLightness converts sRGB to CIE L*, the perceptual
// lightness. HSL's "L" is not this — it calls #0000ff and #ffff00 equally
// light, which is exactly the judgement this ramp must not make.
func lightness(c color.RGBA) float64 {
	linear := func(channel uint8) float64 {
		v := float64(channel) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	y := 0.2126729*linear(c.R) + 0.7151522*linear(c.G) + 0.0721750*linear(c.B)
	if y > 216.0/24389.0 {
		return 116*math.Cbrt(y) - 16
	}
	return y * 24389 / 27
}

// labOf is CIE L*a*b* under D65, for the CIE76 distances below.
func labOf(c color.RGBA) [3]float64 {
	linear := func(channel uint8) float64 {
		v := float64(channel) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	x := (r*0.4124564 + g*0.3575761 + b*0.1804375) / 0.95047
	y := r*0.2126729 + g*0.7151522 + b*0.0721750
	z := (r*0.0193339 + g*0.1191920 + b*0.9503041) / 1.08883
	f := func(t float64) float64 {
		if t > 216.0/24389.0 {
			return math.Cbrt(t)
		}
		return t*841/108 + 4.0/29.0
	}
	fx, fy, fz := f(x), f(y), f(z)
	return [3]float64{116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)}
}

func deltaE76(a, b color.RGBA) float64 {
	la, lb := labOf(a), labOf(b)
	return math.Sqrt((la[0]-lb[0])*(la[0]-lb[0]) + (la[1]-lb[1])*(la[1]-lb[1]) + (la[2]-lb[2])*(la[2]-lb[2]))
}

// simulateDeuteranopia applies the Machado, Oliveira and Fernandes (2009)
// severity-1.0 matrix — the model browsers and design tools use.
func simulateDeuteranopia(c color.RGBA) color.RGBA {
	m := [3][3]float64{
		{0.367322, 0.860646, -0.227968},
		{0.280085, 0.672501, 0.047413},
		{-0.011820, 0.042940, 0.968881},
	}
	in := [3]float64{float64(c.R), float64(c.G), float64(c.B)}
	var out [3]uint8
	for i := range m {
		v := m[i][0]*in[0] + m[i][1]*in[1] + m[i][2]*in[2]
		out[i] = uint8(math.Max(0, math.Min(255, math.Round(v))))
	}
	return color.RGBA{R: out[0], G: out[1], B: out[2], A: c.A}
}

func TestCoverageScaleLightnessRanksValue(t *testing.T) {
	for _, tc := range []struct {
		name      string
		metric    HeatmapType
		threshold float64
	}{
		{"rssi at its default", HeatmapRSSI, float64(DefaultThreshold)},
		{"rssi at a stricter floor", HeatmapRSSI, -60},
		{"rssi at a lenient floor", HeatmapRSSI, -85},
		{"snr at its default", HeatmapSNR, float64(DefaultSNRThreshold)},
		{"snr at a stricter floor", HeatmapSNR, 35},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scale := CoverageScale(tc.metric, tc.threshold)

			below, above := splitAtThreshold(t, scale, tc.threshold)
			assertLightnessRises(t, "below the threshold", below)
			assertLightnessRises(t, "above the threshold", above)

			// The step: the last stop under the threshold and the first one at
			// it must not read as neighbours on one gradient.
			last, first := below[len(below)-1].Color, above[0].Color
			if drop := lightness(last) - lightness(first); drop < 20 {
				t.Errorf("threshold step is a shade, not a step: L* falls only %.1f", drop)
			}
			if d := deltaE76(last, first); d < 40 {
				t.Errorf("threshold step is too small to see: dE76 %.1f", d)
			}
		})
	}
}

func TestCoverageScaleSurvivesDeuteranopia(t *testing.T) {
	// The audit measured the traffic-light ramp's two ends at dE76 ~47 for a
	// deuteranope — its whole range collapsed into one. Every PAIR here, not
	// just the ends, stays further apart than that.
	const floor = 20.0

	scale := CoverageScale(HeatmapRSSI, float64(DefaultThreshold))
	if len(scale.Stops) != 5 {
		t.Fatalf("the legend is specified as five stops, got %d", len(scale.Stops))
	}

	for i := range scale.Stops {
		for j := i + 1; j < len(scale.Stops); j++ {
			a := simulateDeuteranopia(scale.Stops[i].Color)
			b := simulateDeuteranopia(scale.Stops[j].Color)
			if d := deltaE76(a, b); d < floor {
				t.Errorf("stops %d and %d collapse to dE76 %.1f under deuteranopia (floor %.0f)",
					i, j, d, floor)
			}
		}
	}
}

func TestCoverageScalePaintsTheStep(t *testing.T) {
	scale := CoverageScale(HeatmapRSSI, -70)

	// A cell one whole dB short of the requirement is warm; a cell that meets
	// it exactly is cool. This is the property an operator reads off the map,
	// and it is asserted as warm-vs-cool rather than as two hex values: b* is
	// positive toward yellow and negative toward blue, so its sign is the
	// question "which side of the threshold is this" with no palette in it.
	if b := labOf(scale.GetColor(-71))[2]; b <= 0 {
		t.Errorf("-71 dBm is below the threshold and must read warm, got b*=%.1f", b)
	}
	if b := labOf(scale.GetColor(-70))[2]; b >= 0 {
		t.Errorf("-70 dBm meets the threshold and must read cool, got b*=%.1f", b)
	}
	if got := scale.GetColor(-70); got != colorCoverageMeets {
		t.Errorf("the threshold itself is the first cool stop exactly, got %v", got)
	}

	// And no value between the two stops that make the step may come back as
	// a NaN channel — the interpolator divides by the span between stops.
	for _, v := range []float64{-70.005, -70.001, -70.0001} {
		c := scale.GetColor(v)
		if c.A != colorChannelOpaque {
			t.Errorf("GetColor(%v) returned %v, which is not an opaque colour", v, c)
		}
	}
}

func TestCoverageScaleClampsAThresholdOffTheScale(t *testing.T) {
	// An operator can ask for a floor outside the range the metric renders.
	// The scale still has to have a step in it.
	for _, threshold := range []float64{-200, -100, -30, 40} {
		scale := CoverageScale(HeatmapRSSI, threshold)
		if !sortedByValue(scale.Stops) {
			t.Errorf("threshold %v produced out-of-order stops: %v", threshold, scale.Stops)
		}
		if scale.GetColor(scale.MinVal) != colorCoverageWorst {
			t.Errorf("threshold %v lost the bottom of the range", threshold)
		}
		if scale.GetColor(scale.MaxVal) != colorCoverageStrong {
			t.Errorf("threshold %v lost the top of the range", threshold)
		}
	}
}

func splitAtThreshold(t *testing.T, scale ColorScale, threshold float64) (below, above []ColorStop) {
	t.Helper()
	for _, stop := range scale.Stops {
		if stop.Value < threshold {
			below = append(below, stop)
		} else {
			above = append(above, stop)
		}
	}
	if len(below) < 2 || len(above) < 2 {
		t.Fatalf("a diverging ramp needs at least two stops each side, got %d/%d", len(below), len(above))
	}
	return below, above
}

func assertLightnessRises(t *testing.T, side string, stops []ColorStop) {
	t.Helper()
	for i := 1; i < len(stops); i++ {
		previous, current := lightness(stops[i-1].Color), lightness(stops[i].Color)
		if current <= previous {
			t.Errorf("%s: L* must rise with the value, but stop %d is %.1f after %.1f",
				side, i, current, previous)
		}
	}
}

func sortedByValue(stops []ColorStop) bool {
	for i := 1; i < len(stops); i++ {
		if stops[i].Value <= stops[i-1].Value {
			return false
		}
	}
	return true
}

func TestHeatmapPaintsTheRequestedThreshold(t *testing.T) {
	// The operator's floor has to reach the pixels, not just the findings. A
	// report whose map steps at -70 while its recommendations are written
	// against -60 disagrees with itself about which rooms failed — the same
	// defect the report's own parallel generator had (core/survey/sections.go).
	const strict = -60.0

	config := DefaultHeatmapConfig()
	config.Type = HeatmapRSSI
	strictScale := getColorScaleForType(config.Type, strict)
	defaultScale := getColorScaleForType(config.Type, 0)

	if got := strictScale.GetColor(strict); got != colorCoverageMeets {
		t.Errorf("a cell exactly at the requested threshold should be the first cool stop, got %v", got)
	}
	// -65 passes the default floor of -70 and fails a floor of -60, so it is
	// the value that proves the step moved rather than the palette changing.
	if b := labOf(strictScale.GetColor(-65))[2]; b <= 0 {
		t.Errorf("at a -60 floor, -65 dBm fails and must read warm, got b*=%.1f", b)
	}
	if b := labOf(defaultScale.GetColor(-65))[2]; b >= 0 {
		t.Errorf("at the default -70 floor, -65 dBm passes and must read cool, got b*=%.1f", b)
	}
}
