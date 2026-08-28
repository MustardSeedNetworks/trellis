// Sample capture, floor-plan updates and passive-scan aggregation.
package survey_test

import (
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// updateFloorPlanTestCase defines a test case for TestUpdateFloorPlan.
type updateFloorPlanTestCase struct {
	name      string
	id        string
	floorPlan *survey.FloorPlan
	wantErr   bool
}

// assertFloorPlanUpdated verifies that the floor plan was correctly updated on the survey.

// assertFloorPlanUpdated verifies that the floor plan was correctly updated on the survey.
func assertFloorPlanUpdated(
	t *testing.T,
	mgr *survey.Manager,
	surveyID string,
	expected *survey.FloorPlan,
) {
	t.Helper()

	result, getErr := mgr.GetSurvey(surveyID)
	if getErr != nil {
		t.Fatalf("GetSurvey() failed: %v", getErr)
	}

	activeFloor := result.GetActiveFloor()
	if activeFloor == nil {
		t.Fatal("No active floor found")
	}

	if activeFloor.FloorPlan == nil {
		t.Fatal("FloorPlan is nil after update")
	}

	if activeFloor.FloorPlan.Width != expected.Width {
		t.Errorf("FloorPlan Width = %v, want %v", activeFloor.FloorPlan.Width, expected.Width)
	}

	if activeFloor.FloorPlan.Height != expected.Height {
		t.Errorf("FloorPlan Height = %v, want %v", activeFloor.FloorPlan.Height, expected.Height)
	}
}

// runUpdateFloorPlanTest executes a single update floor plan test case.

// runUpdateFloorPlanTest executes a single update floor plan test case.
func runUpdateFloorPlanTest(
	t *testing.T,
	mgr *survey.Manager,
	tc updateFloorPlanTestCase,
) {
	t.Helper()

	updateErr := mgr.UpdateFloorPlan(tc.id, tc.floorPlan)

	if tc.wantErr {
		if updateErr == nil {
			t.Error("UpdateFloorPlan() error = nil, want error")
		}
		return
	}

	if updateErr != nil {
		t.Errorf("UpdateFloorPlan() error = %v, want nil", updateErr)
		return
	}

	assertFloorPlanUpdated(t, mgr, tc.id, tc.floorPlan)
}

func TestUpdateFloorPlan(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	floorPlan := &survey.FloorPlan{
		ImageData: "base64encodeddata",
		Width:     1000,
		Height:    800,
	}

	tests := []updateFloorPlanTestCase{
		{
			name:      "update with valid floor plan",
			id:        s.ID,
			floorPlan: floorPlan,
			wantErr:   false,
		},
		{
			name:      "update non-existent survey",
			id:        "non-existent-id",
			floorPlan: floorPlan,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runUpdateFloorPlanTest(t, mgr, tt)
		})
	}
}

// addSampleTestCase defines test parameters for TestAddSample.

// addSampleTestCase defines test parameters for TestAddSample.
type addSampleTestCase struct {
	name       string
	id         string
	x          int
	y          int
	sampleData map[string]any
	wantErr    bool
}

// addSampleTestFixture holds shared test resources for AddSample tests.

// addSampleTestFixture holds shared test resources for AddSample tests.
type addSampleTestFixture struct {
	mgr         *survey.Manager
	validID     string
	passiveData map[string]any
}

// setupAddSampleTest creates a test fixture with a started survey.

// setupAddSampleTest creates a test fixture with a started survey.
func setupAddSampleTest(t *testing.T) *addSampleTestFixture {
	t.Helper()
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	err = mgr.StartSurvey(s.ID)
	if err != nil {
		t.Fatalf("StartSurvey() failed: %v", err)
	}

	passiveData := map[string]any{
		"networks": []any{
			map[string]any{
				"ssid":  "TestNetwork",
				"bssid": "00:11:22:33:44:55",
				"rssi":  -50,
			},
		},
	}

	return &addSampleTestFixture{
		mgr:         mgr,
		validID:     s.ID,
		passiveData: passiveData,
	}
}

// assertSampleAdded verifies that a sample was correctly added to the survey.

// assertSampleAdded verifies that a sample was correctly added to the survey.
func assertSampleAdded(t *testing.T, mgr *survey.Manager, surveyID string, wantX, wantY int) {
	t.Helper()
	result, err := mgr.GetSurvey(surveyID)
	if err != nil {
		t.Fatalf("GetSurvey() failed: %v", err)
	}

	samples := result.GetAllSamples()
	if len(samples) == 0 {
		t.Fatal("No samples found after AddSample()")
	}

	lastSample := samples[len(samples)-1]
	if lastSample.X != wantX {
		t.Errorf("Sample X = %v, want %v", lastSample.X, wantX)
	}
	if lastSample.Y != wantY {
		t.Errorf("Sample Y = %v, want %v", lastSample.Y, wantY)
	}
	if lastSample.Timestamp.IsZero() {
		t.Error("Sample Timestamp is zero")
	}
}

func TestAddSample(t *testing.T) {
	fixture := setupAddSampleTest(t)

	tests := []addSampleTestCase{
		{
			name:       "add valid sample",
			id:         fixture.validID,
			x:          100,
			y:          200,
			sampleData: fixture.passiveData,
			wantErr:    false,
		},
		{
			name:       "add sample to non-existent survey",
			id:         "non-existent-id",
			x:          100,
			y:          200,
			sampleData: fixture.passiveData,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fixture.mgr.AddSample(tt.id, tt.x, tt.y, tt.sampleData)

			if tt.wantErr {
				if err == nil {
					t.Error("AddSample() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("AddSample() error = %v, want nil", err)
			}

			assertSampleAdded(t, fixture.mgr, tt.id, tt.x, tt.y)
		})
	}
}

func TestSurveyTimestamps(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	beforeCreate := time.Now()
	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}
	afterCreate := time.Now()

	// Verify CreatedAt is within expected time range.
	if s.CreatedAt.Before(beforeCreate) || s.CreatedAt.After(afterCreate) {
		t.Error("CreatedAt timestamp out of expected range")
	}

	// Verify UpdatedAt is set.
	if s.UpdatedAt.Before(beforeCreate) || s.UpdatedAt.After(afterCreate) {
		t.Error("UpdatedAt timestamp out of expected range")
	}

	// Complete survey.
	err = mgr.StartSurvey(s.ID)
	if err != nil {
		t.Fatalf("StartSurvey() failed: %v", err)
	}

	err = mgr.CompleteSurvey(s.ID)
	if err != nil {
		t.Fatalf("CompleteSurvey() failed: %v", err)
	}

	result, _ := mgr.GetSurvey(s.ID)
	if result.Status != survey.StatusCompleted {
		t.Error("Survey not marked as completed")
	}
}

func TestSampleCount(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	err = mgr.StartSurvey(s.ID)
	if err != nil {
		t.Fatalf("StartSurvey() failed: %v", err)
	}

	sampleData := map[string]any{
		"networks": []any{
			map[string]any{"ssid": "Test", "rssi": -60},
		},
	}

	// Add multiple samples.
	for i := range 5 {
		err = mgr.AddSample(s.ID, i*10, i*10, sampleData)
		if err != nil {
			t.Errorf("AddSample() failed: %v", err)
		}
	}

	result, err := mgr.GetSurvey(s.ID)
	if err != nil {
		t.Fatalf("GetSurvey() failed: %v", err)
	}

	// With multi-floor support, samples are on the active floor.
	samples := result.GetAllSamples()
	if len(samples) != 5 {
		t.Errorf("Sample count = %d, want 5", len(samples))
	}
}

func TestPassiveSampleAggregations(t *testing.T) {
	tests := []struct {
		name     string
		networks []*wifi.ScannedNetwork
		want     survey.PassiveSample
	}{
		{
			name:     "empty networks",
			networks: []*wifi.ScannedNetwork{},
			want: survey.PassiveSample{
				Networks:      []*wifi.ScannedNetwork{},
				UniqueSSIDs:   0,
				UniqueBSSIDs:  0,
				APCount2_4:    0,
				APCount5:      0,
				APCount6:      0,
				CoChannelAPs:  0,
				AdjChannelAPs: 0,
			},
		},
		{
			name:     "nil networks",
			networks: nil,
			want: survey.PassiveSample{
				Networks:      nil,
				UniqueSSIDs:   0,
				UniqueBSSIDs:  0,
				APCount2_4:    0,
				APCount5:      0,
				APCount6:      0,
				CoChannelAPs:  0,
				AdjChannelAPs: 0,
			},
		},
		{
			name: "single 2.4GHz network",
			networks: []*wifi.ScannedNetwork{
				{
					SSID:      "TestNet",
					BSSID:     "00:11:22:33:44:55",
					Channel:   6,
					Frequency: 2437,
					Signal:    -50,
				},
			},
			want: survey.PassiveSample{
				UniqueSSIDs:   1,
				UniqueBSSIDs:  1,
				APCount2_4:    1,
				APCount5:      0,
				APCount6:      0,
				CoChannelAPs:  1, // Same as strongest (itself).
				AdjChannelAPs: 0,
			},
		},
		{
			name: "multiple bands and channels",
			networks: []*wifi.ScannedNetwork{
				// Strongest AP on channel 36 (5GHz).
				{
					SSID:      "Net5G",
					BSSID:     "00:11:22:33:44:55",
					Channel:   36,
					Frequency: 5180,
					Signal:    -40,
				},
				// Co-channel AP.
				{
					SSID:      "Net5G-2",
					BSSID:     "00:11:22:33:44:66",
					Channel:   36,
					Frequency: 5180,
					Signal:    -50,
				},
				// Adjacent channel (+-1).
				{
					SSID:      "Net5G-3",
					BSSID:     "00:11:22:33:44:77",
					Channel:   37,
					Frequency: 5185,
					Signal:    -55,
				},
				// Adjacent channel (+-2).
				{
					SSID:      "Net5G-4",
					BSSID:     "00:11:22:33:44:88",
					Channel:   38,
					Frequency: 5190,
					Signal:    -60,
				},
				// 2.4GHz networks.
				{
					SSID:      "Net2.4",
					BSSID:     "AA:BB:CC:DD:EE:FF",
					Channel:   1,
					Frequency: 2412,
					Signal:    -65,
				},
				{
					SSID:      "Net2.4-2",
					BSSID:     "AA:BB:CC:DD:EE:AA",
					Channel:   6,
					Frequency: 2437,
					Signal:    -70,
				},
				// 6GHz network.
				{
					SSID:      "Net6G",
					BSSID:     "FF:EE:DD:CC:BB:AA",
					Channel:   1,
					Frequency: 5955,
					Signal:    -45,
				},
			},
			want: survey.PassiveSample{
				UniqueSSIDs:   7,
				UniqueBSSIDs:  7,
				APCount2_4:    2,
				APCount5:      4,
				APCount6:      1,
				CoChannelAPs:  2, // Two APs on channel 36.
				AdjChannelAPs: 2, // Channels 37 and 38.
			},
		},
		{
			name: "duplicate SSIDs different BSSIDs",
			networks: []*wifi.ScannedNetwork{
				{
					SSID:      "SameNet",
					BSSID:     "00:11:22:33:44:55",
					Channel:   1,
					Frequency: 2412,
					Signal:    -50,
				},
				{
					SSID:      "SameNet",
					BSSID:     "00:11:22:33:44:66",
					Channel:   1,
					Frequency: 2412,
					Signal:    -55,
				},
				{
					SSID:      "SameNet",
					BSSID:     "00:11:22:33:44:77",
					Channel:   1,
					Frequency: 2412,
					Signal:    -60,
				},
			},
			want: survey.PassiveSample{
				UniqueSSIDs:   1, // Only one unique SSID.
				UniqueBSSIDs:  3, // Three different APs.
				APCount2_4:    3,
				APCount5:      0,
				APCount6:      0,
				CoChannelAPs:  3, // All on channel 1.
				AdjChannelAPs: 0,
			},
		},
		{
			name: "hidden SSID handling",
			networks: []*wifi.ScannedNetwork{
				{SSID: "", BSSID: "00:11:22:33:44:55", Channel: 6, Frequency: 2437, Signal: -50},
				{
					SSID:      "VisibleNet",
					BSSID:     "00:11:22:33:44:66",
					Channel:   6,
					Frequency: 2437,
					Signal:    -55,
				},
			},
			want: survey.PassiveSample{
				UniqueSSIDs:   1, // Hidden SSID not counted.
				UniqueBSSIDs:  2, // Both BSSIDs counted.
				APCount2_4:    2,
				APCount5:      0,
				APCount6:      0,
				CoChannelAPs:  2,
				AdjChannelAPs: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := survey.PassiveSample{
				Networks: tt.networks,
			}
			sample.CalculateAggregations()
			assertPassiveSampleAggregations(t, sample, tt.want)
		})
	}
}

// assertPassiveSampleAggregations compares the aggregation fields of a PassiveSample
// and reports any mismatches. This helper reduces cognitive complexity in the test.

// assertPassiveSampleAggregations compares the aggregation fields of a PassiveSample
// and reports any mismatches. This helper reduces cognitive complexity in the test.
func assertPassiveSampleAggregations(t *testing.T, got, want survey.PassiveSample) {
	t.Helper()

	if got.UniqueSSIDs != want.UniqueSSIDs {
		t.Errorf("UniqueSSIDs = %d, want %d", got.UniqueSSIDs, want.UniqueSSIDs)
	}
	if got.UniqueBSSIDs != want.UniqueBSSIDs {
		t.Errorf("UniqueBSSIDs = %d, want %d", got.UniqueBSSIDs, want.UniqueBSSIDs)
	}
	if got.APCount2_4 != want.APCount2_4 {
		t.Errorf("APCount2_4 = %d, want %d", got.APCount2_4, want.APCount2_4)
	}
	if got.APCount5 != want.APCount5 {
		t.Errorf("APCount5 = %d, want %d", got.APCount5, want.APCount5)
	}
	if got.APCount6 != want.APCount6 {
		t.Errorf("APCount6 = %d, want %d", got.APCount6, want.APCount6)
	}
	if got.CoChannelAPs != want.CoChannelAPs {
		t.Errorf("CoChannelAPs = %d, want %d", got.CoChannelAPs, want.CoChannelAPs)
	}
	if got.AdjChannelAPs != want.AdjChannelAPs {
		t.Errorf("AdjChannelAPs = %d, want %d", got.AdjChannelAPs, want.AdjChannelAPs)
	}
}
