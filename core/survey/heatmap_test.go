package survey_test

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func TestDefaultHeatmapConfig(t *testing.T) {
	config := survey.DefaultHeatmapConfig()

	if config.Type != survey.HeatmapRSSI {
		t.Errorf("Expected type RSSI, got %s", config.Type)
	}
	if config.CellSize != 10 {
		t.Errorf("Expected cellSize 10, got %d", config.CellSize)
	}
	if config.Opacity != 180 {
		t.Errorf("Expected opacity 180, got %d", config.Opacity)
	}
	if config.Method != survey.MethodIDW {
		t.Errorf("Expected method IDW, got %s", config.Method)
	}
	if config.Power != 2.0 {
		t.Errorf("Expected power 2.0, got %f", config.Power)
	}
	if config.ShowGrid {
		t.Error("Expected ShowGrid false")
	}
	if !config.ShowSamples {
		t.Error("Expected ShowSamples true")
	}
	if !config.BlendWithPlan {
		t.Error("Expected BlendWithPlan true")
	}
}

func TestGenerateHeatmap_NilSurvey(t *testing.T) {
	config := survey.DefaultHeatmapConfig()
	result, err := survey.GenerateHeatmap(nil, config)

	if err == nil {
		t.Error("Expected error for nil survey")
	}
	if result != nil {
		t.Error("Expected nil result for nil survey")
	}
	if err.Error() != "survey is nil" {
		t.Errorf("Expected 'survey is nil' error, got %q", err.Error())
	}
}

func TestGenerateHeatmap_NoFloorPlan(t *testing.T) {
	s := &survey.Survey{
		ID:        "test",
		FloorPlan: nil,
		Samples:   []*survey.SamplePoint{},
	}
	config := survey.DefaultHeatmapConfig()
	result, err := survey.GenerateHeatmap(s, config)

	if err == nil {
		t.Error("Expected error for survey without floor plan or samples")
	}
	if result != nil {
		t.Error("Expected nil result")
	}
}

func TestGenerateHeatmap_NoSamples(t *testing.T) {
	s := &survey.Survey{
		ID: "test",
		FloorPlan: &survey.FloorPlan{
			Width:  100,
			Height: 100,
		},
		Samples: []*survey.SamplePoint{},
	}
	config := survey.DefaultHeatmapConfig()
	result, err := survey.GenerateHeatmap(s, config)

	if err == nil {
		t.Error("Expected error for no samples")
	}
	if result != nil {
		t.Error("Expected nil result")
	}
}

func TestGenerateHeatmap_Success(t *testing.T) {
	s := createTestSurvey()
	config := survey.DefaultHeatmapConfig()

	result, err := survey.GenerateHeatmap(s, config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check result properties.
	if result.Width != 100 {
		t.Errorf("Expected width 100, got %d", result.Width)
	}
	if result.Height != 100 {
		t.Errorf("Expected height 100, got %d", result.Height)
	}
	if result.Type != string(survey.HeatmapRSSI) {
		t.Errorf("Expected type rssi, got %s", result.Type)
	}
	if result.SampleCount != 4 {
		t.Errorf("Expected 4 samples, got %d", result.SampleCount)
	}
	if result.Generated.IsZero() {
		t.Error("Expected non-zero generated time")
	}

	// Check image data.
	if len(result.Image) == 0 {
		t.Error("Expected non-empty image data")
	}
	if result.ImageBase64 == "" {
		t.Error("Expected non-empty base64 image")
	}

	// Verify PNG is valid.
	_, err = png.Decode(bytes.NewReader(result.Image))
	if err != nil {
		t.Errorf("Invalid PNG data: %v", err)
	}

	// Check stats.
	if result.Stats.Count == 0 {
		t.Error("Expected non-zero stats count")
	}
	if result.Stats.Min > result.Stats.Max {
		t.Error("Stats min > max")
	}
}

func TestGenerateHeatmap_DefaultsApplied(t *testing.T) {
	s := createTestSurvey()
	config := survey.HeatmapConfig{
		Type:     survey.HeatmapRSSI,
		CellSize: 0, // Should default to 10.
		Opacity:  0, // Should default to 180.
		Power:    0, // Should default to 2.0.
	}

	result, err := survey.GenerateHeatmap(s, config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	// If we got here, defaults were applied successfully.
}

func TestGenerateHeatmap_WithGrid(t *testing.T) {
	s := createTestSurvey()
	config := survey.DefaultHeatmapConfig()
	config.ShowGrid = true

	result, err := survey.GenerateHeatmap(s, config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestGenerateHeatmap_WithoutSamples(t *testing.T) {
	s := createTestSurvey()
	config := survey.DefaultHeatmapConfig()
	config.ShowSamples = false

	result, err := survey.GenerateHeatmap(s, config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestGenerateHeatmap_AllTypes(t *testing.T) {
	s := createTestSurveyWithAll()

	types := []survey.HeatmapType{
		survey.HeatmapRSSI,
		survey.HeatmapSNR,
		survey.HeatmapDensity,
		survey.HeatmapInterference,
	}

	for _, ht := range types {
		t.Run(string(ht), func(t *testing.T) {
			config := survey.DefaultHeatmapConfig()
			config.Type = ht

			result, err := survey.GenerateHeatmap(s, config)
			if err != nil {
				t.Fatalf("Unexpected error for type %s: %v", ht, err)
			}
			if result == nil {
				t.Fatalf("Expected non-nil result for type %s", ht)
			}
			if result.Type != string(ht) {
				t.Errorf("Expected type %s, got %s", ht, result.Type)
			}
		})
	}
}

func TestGenerateHeatmap_ThroughputTypes(t *testing.T) {
	s := createTestSurveyWithThroughput()

	types := []survey.HeatmapType{
		survey.HeatmapDownload,
		survey.HeatmapUpload,
	}

	for _, ht := range types {
		t.Run(string(ht), func(t *testing.T) {
			config := survey.DefaultHeatmapConfig()
			config.Type = ht

			result, err := survey.GenerateHeatmap(s, config)
			if err != nil {
				t.Fatalf("Unexpected error for type %s: %v", ht, err)
			}
			if result == nil {
				t.Fatalf("Expected non-nil result for type %s", ht)
			}
		})
	}
}

func TestParseHeatmapType(t *testing.T) {
	tests := []struct {
		input    string
		expected survey.HeatmapType
	}{
		{"rssi", survey.HeatmapRSSI},
		{"RSSI", survey.HeatmapRSSI},
		{"signal", survey.HeatmapRSSI},
		{"snr", survey.HeatmapSNR},
		{"SNR", survey.HeatmapSNR},
		{"density", survey.HeatmapDensity},
		{"ap_density", survey.HeatmapDensity},
		{"interference", survey.HeatmapInterference},
		{"cochannel", survey.HeatmapInterference},
		{"download", survey.HeatmapDownload},
		{"upload", survey.HeatmapUpload},
		{"unknown", survey.HeatmapRSSI}, // Default.
		{"", survey.HeatmapRSSI},        // Default.
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := survey.ParseHeatmapType(tt.input)
			if got != tt.expected {
				t.Errorf("ParseHeatmapType(%q) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetColorScaleForType(t *testing.T) {
	tests := []struct {
		heatmapType survey.HeatmapType
		scaleName   string
	}{
		{survey.HeatmapRSSI, "rssi"},
		{survey.HeatmapSNR, "snr"},
		{survey.HeatmapDensity, "ap_density"},
		{survey.HeatmapInterference, "interference"},
		{survey.HeatmapDownload, "throughput"},
		{survey.HeatmapUpload, "throughput"},
		{"unknown", "rssi"}, // Default.
	}

	for _, tt := range tests {
		t.Run(string(tt.heatmapType), func(t *testing.T) {
			scale := survey.ExportGetColorScaleForType(tt.heatmapType)
			if scale.Name != tt.scaleName {
				t.Errorf("getColorScaleForType(%s) = %s, want %s",
					tt.heatmapType, scale.Name, tt.scaleName)
			}
		})
	}
}

func TestMapHeatmapTypeToValueType(t *testing.T) {
	tests := []struct {
		input    survey.HeatmapType
		expected string
	}{
		{survey.HeatmapRSSI, "rssi"},
		{survey.HeatmapSNR, "snr"},
		{survey.HeatmapDensity, "density"},
		{survey.HeatmapInterference, "interference"},
		{survey.HeatmapDownload, "download"},
		{survey.HeatmapUpload, "upload"},
		{"unknown", "rssi"}, // Default.
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := survey.ExportMapHeatmapTypeToValueType(tt.input)
			if got != tt.expected {
				t.Errorf("mapHeatmapTypeToValueType(%s) = %s, want %s",
					tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetHeatmapDimensions_FloorPlan(t *testing.T) {
	s := &survey.Survey{
		FloorPlan: &survey.FloorPlan{
			Width:  500,
			Height: 300,
		},
	}

	width, height := survey.ExportGetHeatmapDimensions(s)
	if width != 500 {
		t.Errorf("Expected width 500, got %d", width)
	}
	if height != 300 {
		t.Errorf("Expected height 300, got %d", height)
	}
}

func TestGetHeatmapDimensions_FromSamples(t *testing.T) {
	s := &survey.Survey{
		FloorPlan: nil,
		Samples: []*survey.SamplePoint{
			{X: 100, Y: 50},
			{X: 200, Y: 150},
			{X: 50, Y: 200},
		},
	}

	width, height := survey.ExportGetHeatmapDimensions(s)
	// Should be max + padding (50).
	if width != 250 {
		t.Errorf("Expected width 250 (200+50), got %d", width)
	}
	if height != 250 {
		t.Errorf("Expected height 250 (200+50), got %d", height)
	}
}

func TestGetHeatmapDimensions_NoData(t *testing.T) {
	s := &survey.Survey{
		FloorPlan: nil,
		Samples:   []*survey.SamplePoint{},
	}

	width, height := survey.ExportGetHeatmapDimensions(s)
	if width != 0 || height != 0 {
		t.Errorf("Expected 0x0, got %dx%d", width, height)
	}
}

func TestRenderHeatmapToImage(t *testing.T) {
	// This is a simple smoke test.
	grid := [][]float64{
		{-70, -60},
		{-65, -55},
	}
	img := survey.CreateTestImage(40, 40)
	rssiScale := survey.GetRSSIColorScale()
	scale := &rssiScale

	// Should not panic.
	survey.ExportRenderHeatmapToImage(img, grid, 20, scale, 180)

	// Verify some pixels were set.
	c := img.At(5, 5)
	r, g, b, a := c.RGBA()
	if a == 0 {
		t.Error("Expected non-zero alpha at (5,5)")
	}
	// Just check we got some color.
	if r == 0 && g == 0 && b == 0 {
		t.Error("Expected non-black color at (5,5)")
	}
}

func TestRenderHeatmapToImage_EmptyGrid(t *testing.T) {
	img := survey.CreateTestImage(100, 100)
	grid := [][]float64{}
	rssiScale := survey.GetRSSIColorScale()
	scale := &rssiScale

	// Should not panic.
	survey.ExportRenderHeatmapToImage(img, grid, 10, scale, 180)

	// Verify image was not modified (still transparent).
	c := img.At(50, 50).(color.RGBA)
	if c.A != 0 {
		t.Errorf("Expected transparent pixel for empty grid, got alpha %d", c.A)
	}
}

func TestRenderSamplePoints(t *testing.T) {
	img := survey.CreateTestImage(100, 100)
	samples := []survey.SampleValue{
		{Point: survey.Point2D{X: 50, Y: 50}, Value: -60},
	}

	// Should not panic.
	survey.ExportRenderSamplePoints(img, samples)

	// Check center is white (marker center).
	c := img.At(50, 50).(color.RGBA)
	if c.R != 255 || c.G != 255 || c.B != 255 {
		t.Errorf("Expected white center at (50,50), got %v", c)
	}
}

func TestRenderSamplePoints_EdgeCases(t *testing.T) {
	img := survey.CreateTestImage(100, 100)
	samples := []survey.SampleValue{
		{Point: survey.Point2D{X: 0, Y: 0}, Value: -60},       // Corner.
		{Point: survey.Point2D{X: 99, Y: 99}, Value: -60},     // Other corner.
		{Point: survey.Point2D{X: 1000, Y: 1000}, Value: -60}, // Outside bounds.
	}

	// Should not panic.
	survey.ExportRenderSamplePoints(img, samples)

	// Verify corner markers were drawn (white center).
	c := img.At(0, 0).(color.RGBA)
	if c.R != 255 || c.G != 255 || c.B != 255 {
		t.Errorf("Expected white center at (0,0), got %v", c)
	}
}

func TestRenderGrid(t *testing.T) {
	img := survey.CreateTestImage(100, 100)

	// Should not panic.
	survey.ExportRenderGrid(img, 20)

	// Check that vertical line exists at x=0.
	c := img.At(0, 50).(color.RGBA)
	if c.R != 200 || c.G != 200 || c.B != 200 {
		t.Errorf("Expected grid color at (0,50), got %v", c)
	}

	// Check that horizontal line exists at y=0.
	c = img.At(50, 0).(color.RGBA)
	if c.R != 200 || c.G != 200 || c.B != 200 {
		t.Errorf("Expected grid color at (50,0), got %v", c)
	}
}

// Helper functions.

func createTestSurvey() *survey.Survey {
	return &survey.Survey{
		ID:   "test-survey",
		Name: "Test Survey",
		FloorPlan: &survey.FloorPlan{
			Width:  100,
			Height: 100,
		},
		Samples: []*survey.SamplePoint{
			{
				X:         10,
				Y:         10,
				Timestamp: time.Now(),
				SampleData: &survey.PassiveSample{
					Networks: []*wifi.ScannedNetwork{
						{Signal: -55, SNR: 30},
					},
					UniqueBSSIDs: 5,
					CoChannelAPs: 2,
				},
			},
			{
				X:         90,
				Y:         10,
				Timestamp: time.Now(),
				SampleData: &survey.PassiveSample{
					Networks: []*wifi.ScannedNetwork{
						{Signal: -65, SNR: 25},
					},
					UniqueBSSIDs: 4,
					CoChannelAPs: 3,
				},
			},
			{
				X:         10,
				Y:         90,
				Timestamp: time.Now(),
				SampleData: &survey.PassiveSample{
					Networks: []*wifi.ScannedNetwork{
						{Signal: -60, SNR: 28},
					},
					UniqueBSSIDs: 3,
					CoChannelAPs: 1,
				},
			},
			{
				X:         90,
				Y:         90,
				Timestamp: time.Now(),
				SampleData: &survey.PassiveSample{
					Networks: []*wifi.ScannedNetwork{
						{Signal: -70, SNR: 20},
					},
					UniqueBSSIDs: 6,
					CoChannelAPs: 4,
				},
			},
		},
	}
}

func createTestSurveyWithAll() *survey.Survey {
	s := createTestSurvey()
	// Add more diverse data for comprehensive testing.
	return s
}

func createTestSurveyWithThroughput() *survey.Survey {
	return &survey.Survey{
		ID:   "test-throughput",
		Name: "Throughput Test",
		FloorPlan: &survey.FloorPlan{
			Width:  100,
			Height: 100,
		},
		Samples: []*survey.SamplePoint{
			{
				X:         10,
				Y:         10,
				Timestamp: time.Now(),
				SampleData: &survey.ThroughputSample{
					RSSI:         -55,
					DownloadMbps: 100,
					UploadMbps:   50,
				},
			},
			{
				X:         90,
				Y:         90,
				Timestamp: time.Now(),
				SampleData: &survey.ThroughputSample{
					RSSI:         -70,
					DownloadMbps: 50,
					UploadMbps:   25,
				},
			},
		},
	}
}

// A floor walked before any plan was imported has samples and no plan, which
// is every live survey until floorplan import exists. Sizing that floor from
// its own measurements is what keeps its heatmap renderable; requiring a plan
// here silently broke the walk when per-floor rendering was introduced.
func TestGenerateFloorHeatmap_NoPlanSizesFromSamples(t *testing.T) {
	t.Parallel()

	floor := &survey.Floor{
		ID:   "floor-1",
		Name: "Floor 1",
		Samples: []*survey.SamplePoint{
			{X: 40, Y: 30, Timestamp: time.Now(), SampleData: &survey.PassiveSample{
				Networks: []*wifi.ScannedNetwork{{Signal: -55, SNR: 30}},
			}},
			{X: 160, Y: 120, Timestamp: time.Now(), SampleData: &survey.PassiveSample{
				Networks: []*wifi.ScannedNetwork{{Signal: -72, SNR: 18}},
			}},
		},
	}

	result, err := survey.GenerateFloorHeatmap(floor, survey.DefaultHeatmapConfig())
	if err != nil {
		t.Fatalf("GenerateFloorHeatmap on a plan-less floor: %v", err)
	}
	// The bounding box of the samples plus the renderer's padding, not a
	// default canvas: a fixed size would crop or float the measurements.
	if result.Width != 210 || result.Height != 170 {
		t.Errorf("size = %dx%d, want 210x170 (max sample + padding)", result.Width, result.Height)
	}
	if result.SampleCount != 2 {
		t.Errorf("SampleCount = %d, want 2", result.SampleCount)
	}
	if len(result.Image) == 0 {
		t.Error("no PNG bytes for a floor that has measurements")
	}
}

func TestGenerateFloorHeatmap_NoPlanNoSamples(t *testing.T) {
	t.Parallel()

	_, err := survey.GenerateFloorHeatmap(&survey.Floor{ID: "empty"}, survey.DefaultHeatmapConfig())
	if err == nil {
		t.Fatal("GenerateFloorHeatmap on an empty floor: want an error, got none")
	}
}

// Each floor is drawn from its own measurements. Before per-floor rendering
// every floor's samples were interpolated onto whichever plan was active, so
// the two floors below would have produced the same picture.
func TestGenerateFloorHeatmap_IsPerFloor(t *testing.T) {
	t.Parallel()

	plan := &survey.FloorPlan{Width: 200, Height: 200}
	ground := &survey.Floor{ID: "g", Name: "Ground", FloorPlan: plan, Samples: []*survey.SamplePoint{
		{X: 50, Y: 50, Timestamp: time.Now(), SampleData: &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{Signal: -50, SNR: 40}},
		}},
	}}
	upper := &survey.Floor{ID: "u", Name: "Upper", FloorPlan: plan, Samples: []*survey.SamplePoint{
		{X: 50, Y: 50, Timestamp: time.Now(), SampleData: &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{Signal: -85, SNR: 8}},
		}},
		{X: 150, Y: 150, Timestamp: time.Now(), SampleData: &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{Signal: -80, SNR: 10}},
		}},
	}}

	groundMap, err := survey.GenerateFloorHeatmap(ground, survey.DefaultHeatmapConfig())
	if err != nil {
		t.Fatalf("ground floor: %v", err)
	}
	upperMap, err := survey.GenerateFloorHeatmap(upper, survey.DefaultHeatmapConfig())
	if err != nil {
		t.Fatalf("upper floor: %v", err)
	}

	if groundMap.SampleCount != 1 || upperMap.SampleCount != 2 {
		t.Errorf("sample counts = %d and %d, want 1 and 2 — each floor's own",
			groundMap.SampleCount, upperMap.SampleCount)
	}
	// -50 dBm against -85/-80: the strong floor's own minimum must not be
	// dragged down by the weak floor's measurements.
	if groundMap.Stats.Min < -60 {
		t.Errorf("ground floor min = %.1f dBm, want its own (~-50), not the upper floor's",
			groundMap.Stats.Min)
	}
	if upperMap.Stats.Max > -70 {
		t.Errorf("upper floor max = %.1f dBm, want its own (~-80)", upperMap.Stats.Max)
	}
}

// The same split for coverage: a per-floor score is about that storey.
func TestDetectFloorDeadZones_ScoresOneFloor(t *testing.T) {
	t.Parallel()

	strong := &survey.Floor{ID: "g", Name: "Ground", Samples: []*survey.SamplePoint{
		{X: 10, Y: 10, Timestamp: time.Now(), SampleData: &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{Signal: -50, SNR: 40}},
		}},
		{X: 90, Y: 90, Timestamp: time.Now(), SampleData: &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{Signal: -55, SNR: 35}},
		}},
	}}
	weak := &survey.Floor{ID: "u", Name: "Upper", Samples: []*survey.SamplePoint{
		{X: 10, Y: 10, Timestamp: time.Now(), SampleData: &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{Signal: -88, SNR: 6}},
		}},
		{X: 90, Y: 90, Timestamp: time.Now(), SampleData: &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{Signal: -86, SNR: 7}},
		}},
	}}

	strongAnalysis, err := survey.DetectFloorDeadZones("svy", strong, -75, nil)
	if err != nil {
		t.Fatalf("strong floor: %v", err)
	}
	weakAnalysis, err := survey.DetectFloorDeadZones("svy", weak, -75, nil)
	if err != nil {
		t.Fatalf("weak floor: %v", err)
	}

	if strongAnalysis.CoverageScore != 100 {
		t.Errorf("strong floor score = %.1f, want 100 — no sample is below -75",
			strongAnalysis.CoverageScore)
	}
	if len(strongAnalysis.DeadZones) != 0 {
		t.Errorf("strong floor dead zones = %d, want 0", len(strongAnalysis.DeadZones))
	}
	if weakAnalysis.CoverageScore != 0 {
		t.Errorf("weak floor score = %.1f, want 0 — every sample is below -75",
			weakAnalysis.CoverageScore)
	}
	if len(weakAnalysis.DeadZones) == 0 {
		t.Error("weak floor dead zones = 0, want at least one")
	}
	if weakAnalysis.SurveyID != "svy" {
		t.Errorf("SurveyID = %q, want the survey's", weakAnalysis.SurveyID)
	}
}

func TestDetectFloorDeadZones_EmptyFloor(t *testing.T) {
	t.Parallel()

	if _, err := survey.DetectFloorDeadZones("svy", &survey.Floor{ID: "e"}, -75, nil); err == nil {
		t.Fatal("DetectFloorDeadZones on a floor with no samples: want an error, got none")
	}
	if _, err := survey.DetectFloorDeadZones("svy", nil, -75, nil); err == nil {
		t.Fatal("DetectFloorDeadZones(nil): want an error, got none")
	}
}
