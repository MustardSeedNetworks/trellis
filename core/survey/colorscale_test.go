package survey_test

import (
	"image/color"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func TestColorScale_GetColor(t *testing.T) {
	// RSSI and SNR moved to the diverging coverage ramp and are asserted in
	// coveragescale_test.go, by the properties that ramp exists for rather
	// than by its hex. These two have no operator threshold to diverge
	// around and keep their own sequential scales.
	apDensityScale := survey.GetAPDensityColorScale()
	interferenceScale := survey.GetInterferenceColorScale()

	tests := []struct {
		name     string
		scale    *survey.ColorScale
		value    float64
		expected color.RGBA
	}{
		{
			name:     "AP density at zero",
			scale:    &apDensityScale,
			value:    0,
			expected: color.RGBA{R: 240, G: 240, B: 255, A: 255}, // Very light blue
		},
		{
			name:     "AP density below minimum (clamped)",
			scale:    &apDensityScale,
			value:    -5,
			expected: color.RGBA{R: 240, G: 240, B: 255, A: 255},
		},
		{
			name:     "Interference at zero",
			scale:    &interferenceScale,
			value:    0,
			expected: color.RGBA{R: 40, G: 167, B: 69, A: 255}, // Green
		},
		{
			name:  "Interference interpolated between stops",
			scale: &interferenceScale,
			value: 3,
			// No exact expectation: the assertion is that a value between two
			// stops still comes back as an opaque colour.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scale.GetColor(tt.value)

			// For exact matches, compare exactly
			if tt.expected != (color.RGBA{}) {
				if got != tt.expected {
					t.Errorf("GetColor(%v) = %v, want %v", tt.value, got, tt.expected)
				}
			} else {
				// For interpolated values, just check it's valid
				if got.A != 255 {
					t.Errorf("GetColor(%v) alpha = %d, want 255", tt.value, got.A)
				}
			}
		})
	}
}

func TestInterpolateColor(t *testing.T) {
	stop1 := survey.ColorStop{Value: 0, Color: color.RGBA{R: 0, G: 0, B: 0, A: 255}}
	stop2 := survey.ColorStop{Value: 100, Color: color.RGBA{R: 100, G: 200, B: 50, A: 255}}

	tests := []struct {
		name     string
		value    float64
		expected color.RGBA
	}{
		{
			name:     "at start",
			value:    0,
			expected: color.RGBA{R: 0, G: 0, B: 0, A: 255},
		},
		{
			name:     "at end",
			value:    100,
			expected: color.RGBA{R: 100, G: 200, B: 50, A: 255},
		},
		{
			name:     "midpoint",
			value:    50,
			expected: color.RGBA{R: 50, G: 100, B: 25, A: 255},
		},
		{
			name:     "quarter",
			value:    25,
			expected: color.RGBA{R: 25, G: 50, B: 12, A: 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := survey.ExportInterpolateColor(stop1, stop2, tt.value)
			if got != tt.expected {
				t.Errorf("interpolateColor(%v) = %v, want %v", tt.value, got, tt.expected)
			}
		})
	}
}

func TestColorScaleProperties(t *testing.T) {
	rssiScale := survey.CoverageScale(survey.HeatmapRSSI, float64(survey.DefaultThreshold))
	snrScale := survey.CoverageScale(survey.HeatmapSNR, float64(survey.DefaultSNRThreshold))
	apDensityScale := survey.GetAPDensityColorScale()
	interferenceScale := survey.GetInterferenceColorScale()

	scales := []*survey.ColorScale{
		&rssiScale,
		&snrScale,
		&apDensityScale,
		&interferenceScale,
	}

	for _, scale := range scales {
		t.Run(scale.Name, func(t *testing.T) {
			// Check that stops are sorted by value
			for i := 1; i < len(scale.Stops); i++ {
				if scale.Stops[i].Value <= scale.Stops[i-1].Value {
					t.Errorf("Stops not sorted: %v at %d, %v at %d",
						scale.Stops[i-1].Value, i-1, scale.Stops[i].Value, i)
				}
			}

			// Check min/max match first/last stops
			if scale.MinVal > scale.Stops[0].Value {
				t.Errorf("MinVal %v > first stop %v", scale.MinVal, scale.Stops[0].Value)
			}
			if scale.MaxVal < scale.Stops[len(scale.Stops)-1].Value {
				t.Errorf(
					"MaxVal %v < last stop %v",
					scale.MaxVal,
					scale.Stops[len(scale.Stops)-1].Value,
				)
			}
		})
	}
}
