package survey_test

import (
	"cmp"
	"math"
	"slices"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func TestNewInterpolator(t *testing.T) {
	samples := []survey.SampleValue{
		{Point: survey.Point2D{X: 0, Y: 0}, Value: 10},
		{Point: survey.Point2D{X: 100, Y: 100}, Value: 20},
	}

	interp := survey.NewInterpolator(samples)

	if interp.Method != survey.MethodIDW {
		t.Errorf("Expected method IDW, got %s", interp.Method)
	}
	if interp.Power != 2.0 {
		t.Errorf("Expected power 2.0, got %f", interp.Power)
	}
}

func TestInterpolator_Interpolate_Empty(t *testing.T) {
	interp := survey.NewInterpolator([]survey.SampleValue{})

	result := interp.Interpolate(50, 50)
	if result != 0 {
		t.Errorf("Expected 0 for empty samples, got %f", result)
	}
}

func TestInterpolator_Interpolate_IDW(t *testing.T) {
	samples := []survey.SampleValue{
		{Point: survey.Point2D{X: 0, Y: 0}, Value: -70},
		{Point: survey.Point2D{X: 100, Y: 0}, Value: -50},
		{Point: survey.Point2D{X: 0, Y: 100}, Value: -60},
		{Point: survey.Point2D{X: 100, Y: 100}, Value: -40},
	}

	interp := survey.NewInterpolator(samples)
	interp.Method = survey.MethodIDW

	// Test at a sample point - should return exact value.
	result := interp.Interpolate(0, 0)
	if result != -70 {
		t.Errorf("Expected -70 at sample point, got %f", result)
	}

	// Test at center - should be weighted average.
	result = interp.Interpolate(50, 50)
	// All corners are equidistant, so result should be average.
	expected := (-70 + -50 + -60 + -40) / 4.0
	if math.Abs(result-expected) > 0.1 {
		t.Errorf("Expected ~%f at center, got %f", expected, result)
	}

	// Test closer to one corner.
	result = interp.Interpolate(10, 10)
	// Should be closer to -70 (nearest corner).
	if result > -65 || result < -75 {
		t.Errorf("Expected value near -70 at (10,10), got %f", result)
	}
}

func TestInterpolator_Interpolate_Nearest(t *testing.T) {
	samples := []survey.SampleValue{
		{Point: survey.Point2D{X: 0, Y: 0}, Value: -70},
		{Point: survey.Point2D{X: 100, Y: 100}, Value: -40},
	}

	interp := survey.NewInterpolator(samples)
	interp.Method = survey.MethodNearest

	// Test at first sample.
	result := interp.Interpolate(0, 0)
	if result != -70 {
		t.Errorf("Expected -70, got %f", result)
	}

	// Test closer to first sample.
	result = interp.Interpolate(10, 10)
	if result != -70 {
		t.Errorf("Expected -70 (nearest), got %f", result)
	}

	// Test closer to second sample.
	result = interp.Interpolate(90, 90)
	if result != -40 {
		t.Errorf("Expected -40 (nearest), got %f", result)
	}
}

func TestInterpolator_InterpolateGrid(t *testing.T) {
	samples := []survey.SampleValue{
		{Point: survey.Point2D{X: 0, Y: 0}, Value: -70},
		{Point: survey.Point2D{X: 100, Y: 0}, Value: -50},
		{Point: survey.Point2D{X: 0, Y: 100}, Value: -60},
		{Point: survey.Point2D{X: 100, Y: 100}, Value: -40},
	}

	interp := survey.NewInterpolator(samples)

	// 100x100 with 50px cells = 2x2 grid.
	grid := interp.InterpolateGrid(100, 100, 50)

	if len(grid) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(grid))
	}
	if len(grid[0]) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(grid[0]))
	}

	// Values should be reasonable (between -70 and -40).
	for row, rowData := range grid {
		for col, val := range rowData {
			if val > -40 || val < -70 {
				t.Errorf("Value at [%d][%d] = %f out of range [-70, -40]",
					row, col, val)
			}
		}
	}
}

// The grid is computed in squared distance, across goroutines (#684) and from
// a bucket index (#691). The first two are speed-ups only and the index only
// finds the neighbours: every cell must still be the textbook IDW of its 12
// nearest samples, at the default power and off it, for walks that cover the
// floor, sit in one corner of it, run along one corridor line or stand still.
func TestInterpolateGrid_MatchesTextbookNearestIDW(t *testing.T) {
	// An R2 low-discrepancy sequence scatters the samples irregularly,
	// deterministically.
	frac := func(v float64) float64 { return v - math.Floor(v) }
	scatter := func(n int, place func(u, v float64) survey.Point2D) []survey.SampleValue {
		samples := make([]survey.SampleValue, n)
		for k := range samples {
			m := float64(k + 1)
			samples[k] = survey.SampleValue{
				Point: place(frac(m*0.7548776662466927), frac(m*0.5698402909980532)),
				Value: -90 + frac(m*0.6180339887498949)*60,
			}
		}
		return samples
	}

	floor := scatter(200, func(u, v float64) survey.Point2D { return survey.Point2D{X: u * 400, Y: v * 310} })
	// 0.005 from cell [0][0]'s centre: close, but not the coincident sample
	// whose value a cell takes outright.
	floor = append(floor, survey.SampleValue{Point: survey.Point2D{X: 5.003, Y: 5.004}, Value: -30})
	corner := scatter(150, func(u, v float64) survey.Point2D { return survey.Point2D{X: 300 + u*80, Y: 240 + v*60} })
	corridor := scatter(90, func(u, _ float64) survey.Point2D { return survey.Point2D{X: 20 + u*360, Y: 155} })
	still := scatter(20, func(_, _ float64) survey.Point2D { return survey.Point2D{X: 133, Y: 77} })

	// textbook sorts every sample by distance, ties by position, and weighs
	// the first 12.
	textbook := func(samples []survey.SampleValue, power, x, y float64) float64 {
		order := make([]int, len(samples))
		for k := range order {
			order[k] = k
		}
		dist := func(k int) float64 { return math.Hypot(x-samples[k].Point.X, y-samples[k].Point.Y) }
		slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(dist(a), dist(b)) })
		if dist(order[0]) < 0.0001 {
			return samples[order[0]].Value
		}
		var weighted, weights float64
		for _, k := range order[:min(12, len(order))] {
			w := 1 / math.Pow(dist(k), power)
			weighted += w * samples[k].Value
			weights += w
		}
		return weighted / weights
	}

	for _, tc := range []struct {
		name    string
		samples []survey.SampleValue
		power   float64
	}{
		{"whole floor, default power", floor, 2},
		{"whole floor, power 3", floor, 3},
		{"whole floor, power 1.5", floor, 1.5},
		{"one corner", corner, 2},
		{"one corridor line", corridor, 2},
		{"standing still", still, 2},
		{"fewer samples than neighbours", floor[:5], 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interp := survey.NewInterpolator(tc.samples)
			interp.Power = tc.power

			const cell = 10
			grid := interp.InterpolateGrid(400, 310, cell)
			if len(grid) != 31 || len(grid[0]) != 40 {
				t.Fatalf("grid is %dx%d, want 31x40", len(grid), len(grid[0]))
			}
			for row, cells := range grid {
				for col, got := range cells {
					want := textbook(tc.samples, tc.power, float64(col*cell+cell/2), float64(row*cell+cell/2))
					if math.Abs(got-want) > 1e-9 {
						t.Fatalf("cell [%d][%d] = %.12f, want %.12f", row, col, got, want)
					}
				}
			}
		})
	}
}

func TestDistance(t *testing.T) {
	tests := []struct {
		name     string
		p1       survey.Point2D
		p2       survey.Point2D
		expected float64
	}{
		{
			name:     "same point",
			p1:       survey.Point2D{X: 0, Y: 0},
			p2:       survey.Point2D{X: 0, Y: 0},
			expected: 0,
		},
		{
			name:     "horizontal",
			p1:       survey.Point2D{X: 0, Y: 0},
			p2:       survey.Point2D{X: 10, Y: 0},
			expected: 10,
		},
		{
			name:     "vertical",
			p1:       survey.Point2D{X: 0, Y: 0},
			p2:       survey.Point2D{X: 0, Y: 10},
			expected: 10,
		},
		{
			name:     "diagonal 3-4-5 triangle",
			p1:       survey.Point2D{X: 0, Y: 0},
			p2:       survey.Point2D{X: 3, Y: 4},
			expected: 5,
		},
		{
			name:     "negative coordinates",
			p1:       survey.Point2D{X: -5, Y: -5},
			p2:       survey.Point2D{X: -5, Y: 5},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := survey.ExportDistance(tt.p1, tt.p2)
			if math.Abs(got-tt.expected) > 0.0001 {
				t.Errorf("distance(%v, %v) = %f, want %f", tt.p1, tt.p2, got, tt.expected)
			}
		})
	}
}

func TestCalculateGridStats(t *testing.T) {
	tests := []struct {
		name     string
		grid     [][]float64
		expected survey.GridStats
	}{
		{
			name:     "empty grid",
			grid:     [][]float64{},
			expected: survey.GridStats{},
		},
		{
			name:     "single value",
			grid:     [][]float64{{-50}},
			expected: survey.GridStats{Min: -50, Max: -50, Average: -50, Count: 1},
		},
		{
			name: "2x2 grid",
			grid: [][]float64{
				{-70, -50},
				{-60, -40},
			},
			expected: survey.GridStats{Min: -70, Max: -40, Average: -55, Count: 4},
		},
		{
			name: "3x3 grid",
			grid: [][]float64{
				{10, 20, 30},
				{40, 50, 60},
				{70, 80, 90},
			},
			expected: survey.GridStats{Min: 10, Max: 90, Average: 50, Count: 9},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := survey.CalculateGridStats(tt.grid)
			if got.Count != tt.expected.Count {
				t.Errorf("Count = %d, want %d", got.Count, tt.expected.Count)
			}
			if got.Count > 0 {
				if got.Min != tt.expected.Min {
					t.Errorf("Min = %f, want %f", got.Min, tt.expected.Min)
				}
				if got.Max != tt.expected.Max {
					t.Errorf("Max = %f, want %f", got.Max, tt.expected.Max)
				}
				if math.Abs(got.Average-tt.expected.Average) > 0.0001 {
					t.Errorf("Average = %f, want %f", got.Average, tt.expected.Average)
				}
			}
		})
	}
}

func TestExtractSamplesFromSurvey(t *testing.T) {
	s := &survey.Survey{
		Samples: []*survey.SamplePoint{
			{
				X: 10,
				Y: 20,
				SampleData: &survey.PassiveSample{
					Networks: []*wifi.ScannedNetwork{
						{Signal: -55, SNR: 30},
					},
					UniqueBSSIDs: 5,
					CoChannelAPs: 2,
				},
			},
			{
				X: 30,
				Y: 40,
				SampleData: &survey.PassiveSample{
					Networks: []*wifi.ScannedNetwork{
						{Signal: -65, SNR: 25},
					},
					UniqueBSSIDs: 3,
					CoChannelAPs: 1,
				},
			},
		},
	}

	tests := []struct {
		name      string
		valueType string
		expected  []float64
	}{
		{
			name:      "rssi",
			valueType: "rssi",
			expected:  []float64{-55, -65},
		},
		{
			name:      "snr",
			valueType: "snr",
			expected:  []float64{30, 25},
		},
		{
			name:      "density",
			valueType: "density",
			expected:  []float64{5, 3},
		},
		{
			name:      "interference",
			valueType: "interference",
			expected:  []float64{2, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samples := survey.ExtractSamplesFromSurvey(s, tt.valueType)
			if len(samples) != len(tt.expected) {
				t.Fatalf("Expected %d samples, got %d", len(tt.expected), len(samples))
			}
			for i, sample := range samples {
				if sample.Value != tt.expected[i] {
					t.Errorf("Sample[%d].Value = %f, want %f", i, sample.Value, tt.expected[i])
				}
			}
		})
	}
}

func TestExtractPassiveValue(t *testing.T) {
	sample := &survey.PassiveSample{
		Networks: []*wifi.ScannedNetwork{
			{Signal: -55, SNR: 30},
		},
		UniqueBSSIDs: 5,
		CoChannelAPs: 2,
		APCount2_4:   3,
		APCount5:     2,
		APCount6:     1,
	}

	tests := []struct {
		name      string
		valueType string
		expected  float64
	}{
		{"rssi", "rssi", -55},
		{"signal alias", "signal", -55},
		{"snr", "snr", 30},
		{"density", "density", 5},
		{"ap_count alias", "ap_count", 5},
		{"interference", "interference", 2},
		{"cochannel alias", "cochannel", 2},
		{"ap_2_4", "ap_2_4", 3},
		{"ap_5", "ap_5", 2},
		{"ap_6", "ap_6", 1},
		// A metric this sample kind cannot answer is absent, not the signal.
		// ParseHeatmapType and mapHeatmapTypeToValueType already funnel an
		// unrecognised *string* to RSSI, so the case that actually reaches here
		// is a known metric asked of the wrong sample kind — a passive scan
		// asked for a download speed — and answering that with dBm put a
		// plausible number in the wrong unit onto the layer.
		{"a metric this sample never measured", "download", math.NaN()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := survey.ExportExtractPassiveValue(sample, tt.valueType)
			if !sameValue(got, tt.expected) {
				t.Errorf("extractPassiveValue(%q) = %f, want %f", tt.valueType, got, tt.expected)
			}
		})
	}
}

func TestExtractPassiveValue_Empty(t *testing.T) {
	// Nil sample.
	result := survey.ExportExtractPassiveValue(nil, "rssi")
	if !math.IsNaN(result) {
		t.Errorf("Expected NaN for nil sample, got %f", result)
	}

	// Empty networks.
	sample := &survey.PassiveSample{Networks: []*wifi.ScannedNetwork{}}
	result = survey.ExportExtractPassiveValue(sample, "rssi")
	if !math.IsNaN(result) {
		t.Errorf("Expected NaN for empty networks, got %f", result)
	}
}

func TestExtractActiveValue(t *testing.T) {
	sample := &survey.ActiveSample{
		RSSI:     -60,
		DataRate: 100.5,
	}

	tests := []struct {
		name      string
		valueType string
		expected  float64
	}{
		{"rssi", "rssi", -60},
		{"signal alias", "signal", -60},
		{"datarate", "datarate", 100.5},
		{"speed alias", "speed", 100.5},
		{"a metric this sample never measured", "download", math.NaN()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := survey.ExportExtractActiveValue(sample, tt.valueType)
			if !sameValue(got, tt.expected) {
				t.Errorf("extractActiveValue(%q) = %f, want %f", tt.valueType, got, tt.expected)
			}
		})
	}
}

func TestExtractActiveValue_Nil(t *testing.T) {
	result := survey.ExportExtractActiveValue(nil, "rssi")
	if !math.IsNaN(result) {
		t.Errorf("Expected NaN for nil sample, got %f", result)
	}
}

func TestExtractThroughputValue(t *testing.T) {
	sample := &survey.ThroughputSample{
		RSSI:         -65,
		DownloadMbps: 100.0,
		UploadMbps:   50.0,
		Latency:      10.5,
		Jitter:       2.3,
	}

	tests := []struct {
		name      string
		valueType string
		expected  float64
	}{
		{"rssi", "rssi", -65},
		{"signal alias", "signal", -65},
		{"download", "download", 100.0},
		{"upload", "upload", 50.0},
		{"latency", "latency", 10.5},
		{"jitter", "jitter", 2.3},
		{"a metric this sample never measured", "density", math.NaN()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := survey.ExportExtractThroughputValue(sample, tt.valueType)
			if !sameValue(got, tt.expected) {
				t.Errorf("extractThroughputValue(%q) = %f, want %f", tt.valueType, got, tt.expected)
			}
		})
	}
}

// sameValue compares two readings, treating "absent" as equal to itself: NaN is
// how an extractor says a sample never measured this metric, and NaN != NaN.
func sameValue(got, want float64) bool {
	if math.IsNaN(want) {
		return math.IsNaN(got)
	}
	return got == want
}

func TestExtractThroughputValue_Nil(t *testing.T) {
	result := survey.ExportExtractThroughputValue(nil, "rssi")
	if !math.IsNaN(result) {
		t.Errorf("Expected NaN for nil sample, got %f", result)
	}
}

// The fixture is the shape a stored sample actually has: these are wifi.
// ScannedNetwork's JSON tags, as written by the survey store. The previous
// fixture used an "rssi" key, which nothing marshals — so it agreed with the
// extractor and neither noticed that a reloaded survey read as no samples.
func TestExtractMapValue(t *testing.T) {
	data := map[string]any{
		"networks": []any{
			map[string]any{
				"signal": float64(-55),
				"snr":    float64(40),
			},
		},
		"uniqueBSSIDs": float64(5),
		"coChannelAPs": float64(2),
	}

	tests := []struct {
		name      string
		valueType string
		expected  float64
	}{
		{"rssi from networks", "rssi", -55},
		{"signal alias", "signal", -55},
		{"snr from networks", "snr", 40},
		{"density", "density", 5},
		{"interference", "interference", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := survey.ExportExtractMapValue(data, tt.valueType)
			if got != tt.expected {
				t.Errorf("extractMapValue(%q) = %f, want %f", tt.valueType, got, tt.expected)
			}
		})
	}
}

func TestExtractMapValue_IntValue(t *testing.T) {
	data := map[string]any{
		"rssi": int(-60),
	}

	result := survey.ExportExtractMapValue(data, "rssi")
	if result != -60 {
		t.Errorf("Expected -60 for int value, got %f", result)
	}
}

func TestExtractMapValue_Missing(t *testing.T) {
	data := map[string]any{}

	result := survey.ExportExtractMapValue(data, "rssi")
	if !math.IsNaN(result) {
		t.Errorf("Expected NaN for missing key, got %f", result)
	}
}

func TestExtractValue_UnsupportedType(t *testing.T) {
	result := survey.ExportExtractValue("string data", "rssi")
	if !math.IsNaN(result) {
		t.Errorf("Expected NaN for unsupported type, got %f", result)
	}
}

func TestExtractValue_PassiveSampleDirect(t *testing.T) {
	// Test non-pointer PassiveSample.
	sample := survey.PassiveSample{
		Networks: []*wifi.ScannedNetwork{
			{Signal: -55},
		},
	}

	result := survey.ExportExtractValue(sample, "rssi")
	if result != -55 {
		t.Errorf("Expected -55, got %f", result)
	}
}

func TestExtractValue_ActiveSampleDirect(t *testing.T) {
	// Test non-pointer ActiveSample.
	sample := survey.ActiveSample{
		RSSI: -60,
	}

	result := survey.ExportExtractValue(sample, "rssi")
	if result != -60 {
		t.Errorf("Expected -60, got %f", result)
	}
}

func TestExtractValue_ThroughputSampleDirect(t *testing.T) {
	// Test non-pointer ThroughputSample.
	sample := survey.ThroughputSample{
		RSSI: -65,
	}

	result := survey.ExportExtractValue(sample, "rssi")
	if result != -65 {
		t.Errorf("Expected -65, got %f", result)
	}
}

// TestExtractValueRefusesAMetricASampleDoesNotHave covers the defect a
// throughput layer would otherwise be built on.
//
// Every extractor fell through to the signal strength for a metric it did not
// recognise. A download heatmap over a passive point therefore rendered -50 as
// if it were 50 Mbps: not a zero that reads as a gap, but a plausible number in
// the wrong unit, blended into the same interpolated field as the real ones.
func TestExtractValueRefusesAMetricASampleDoesNotHave(t *testing.T) {
	t.Parallel()

	passive := &survey.PassiveSample{
		Networks: []*wifi.ScannedNetwork{{Signal: -50, SNR: 45}},
	}
	throughput := &survey.ThroughputSample{RSSI: -50, DownloadMbps: 220}
	active := &survey.ActiveSample{RSSI: -50, DataRate: 866}

	tests := []struct {
		name   string
		sample any
		metric string
	}{
		{"a passive scan measured no download", passive, "download"},
		{"a passive scan measured no upload", passive, "upload"},
		{"a passive scan measured no latency", passive, "latency"},
		{"a throughput test counted no access points", throughput, "density"},
		{"an active sample has no throughput", active, "download"},
		{"nothing answers a metric that does not exist", passive, "not-a-metric"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// NaN is what the heatmap skips. Any real number here is a point
			// drawn on a layer it was never measured for.
			if got := survey.ExportExtractValue(tc.sample, tc.metric); !math.IsNaN(got) {
				t.Errorf("%s = %v, want NaN so the point is left out of the layer",
					tc.metric, got)
			}
		})
	}
}

// TestExtractValueStillAnswersTheMetricsASampleHas is the other half: refusing
// too much would empty every layer.
func TestExtractValueStillAnswersTheMetricsASampleHas(t *testing.T) {
	t.Parallel()

	passive := &survey.PassiveSample{
		Networks: []*wifi.ScannedNetwork{{Signal: -50, SNR: 45}},
	}
	throughput := &survey.ThroughputSample{RSSI: -60, DownloadMbps: 220, UploadMbps: 88}

	for _, tc := range []struct {
		metric string
		sample any
		want   float64
	}{
		{"rssi", passive, -50},
		{"signal", passive, -50},
		{"snr", passive, 45},
		{"download", throughput, 220},
		{"upload", throughput, 88},
		{"rssi", throughput, -60},
	} {
		if got := survey.ExportExtractValue(tc.sample, tc.metric); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.metric, got, tc.want)
		}
	}
}

// Weighing only the 12 nearest samples changes the map (#691). This bounds the
// change against weighing every sample, on a floor whose true field is known:
// six APs, log-distance path loss, 1,000 points scattered over a 2000×1500 px
// plan at 0.05 m/px. Full IDW at power 2 drags every cell toward the floor's
// mean, flattening the peak under each AP, so the cutoff is the closer of the
// two to the truth; the deviation bounds are the measured figures recorded in
// docs/13-PERFORMANCE.md, with a little room.
func TestInterpolateGrid_NearestCutoffDeviation(t *testing.T) {
	aps := []survey.Point2D{{X: 300, Y: 300}, {X: 1000, Y: 250}, {X: 1700, Y: 400}, {X: 400, Y: 1200}, {X: 1100, Y: 1100}, {X: 1750, Y: 1250}}
	truth := func(x, y float64) float64 {
		best := math.Inf(-1)
		for _, ap := range aps {
			metres := max(math.Hypot(x-ap.X, y-ap.Y)*0.05, 1)
			best = max(best, -40-30*math.Log10(metres))
		}
		return best
	}
	frac := func(v float64) float64 { return v - math.Floor(v) }
	samples := make([]survey.SampleValue, 1000)
	for k := range samples {
		m := float64(k + 1)
		x, y := frac(m*0.7548776662466927)*2000, frac(m*0.5698402909980532)*1500
		samples[k] = survey.SampleValue{Point: survey.Point2D{X: x, Y: y}, Value: truth(x, y)}
	}
	full := func(x, y float64) float64 {
		var weighted, weights float64
		for _, s := range samples {
			d2 := (x-s.Point.X)*(x-s.Point.X) + (y-s.Point.Y)*(y-s.Point.Y)
			weighted += s.Value / d2
			weights += 1 / d2
		}
		return weighted / weights
	}

	const cell = 10
	grid := survey.NewInterpolator(samples).InterpolateGrid(2000, 1500, cell)
	var maxDev, sumDev, sumErrNearest, sumErrFull float64
	var cells int
	for row, values := range grid {
		for col, got := range values {
			x, y := float64(col*cell+cell/2), float64(row*cell+cell/2)
			all := full(x, y)
			maxDev = max(maxDev, math.Abs(got-all))
			sumDev += math.Abs(got - all)
			sumErrNearest += math.Abs(got - truth(x, y))
			sumErrFull += math.Abs(all - truth(x, y))
			cells++
		}
	}
	meanDev := sumDev / float64(cells)
	errNearest, errFull := sumErrNearest/float64(cells), sumErrFull/float64(cells)
	t.Logf("vs full IDW: max %.2f dB, mean %.3f dB; mean error vs truth: nearest 12 %.3f dB, full %.3f dB",
		maxDev, meanDev, errNearest, errFull)

	if maxDev > 12.5 || meanDev > 1.75 {
		t.Errorf("deviation from full IDW max %.2f dB, mean %.3f dB; want at most 12.5 and 1.75", maxDev, meanDev)
	}
	if errNearest > 0.5 || errNearest >= errFull {
		t.Errorf("mean error vs truth %.3f dB (full IDW %.3f dB); want at most 0.5 and below full IDW's", errNearest, errFull)
	}
}
