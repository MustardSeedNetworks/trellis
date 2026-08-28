// Manager construction and survey CRUD.
package survey_test

import (
	"context"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// fakeScanner is a no-op survey.Scanner used to exercise Manager plumbing
// without a real capture backend.
type fakeScanner struct{}

func (fakeScanner) Scan(context.Context) ([]wifi.ScannedNetwork, error) { return nil, nil }

// fakeConnMonitor is a no-op survey.ConnectionMonitor.

// fakeConnMonitor is a no-op survey.ConnectionMonitor.
type fakeConnMonitor struct{}

func (fakeConnMonitor) ConnectionInfo(context.Context) (string, string, int, error) {
	return "", "", 0, nil
}

// fakeThroughputMeter is a no-op survey.ThroughputMeter.

// fakeThroughputMeter is a no-op survey.ThroughputMeter.
type fakeThroughputMeter struct{}

func (fakeThroughputMeter) Measure(context.Context, string, int) (survey.ThroughputSample, error) {
	return survey.ThroughputSample{}, nil
}

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()

	mgr := mustManager(t, tmpDir, fakeScanner{}, fakeConnMonitor{}, fakeThroughputMeter{}, nil)
	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}

	if mgr.GetStoragePath() != tmpDir {
		t.Errorf("storagePath = %v, want %v", mgr.GetStoragePath(), tmpDir)
	}

	if mgr.GetSurveys() == nil {
		t.Error("surveys map is nil")
	}

	if mgr.GetScanner() == nil {
		t.Error("scanner is nil")
	}

	if mgr.GetConnMonitor() == nil {
		t.Error("connMonitor is nil")
	}

	if mgr.GetThroughputMeter() == nil {
		t.Error("throughputMeter is nil")
	}
}

// createSurveyTestCase defines a test case for TestCreateSurvey.

// createSurveyTestCase defines a test case for TestCreateSurvey.
type createSurveyTestCase struct {
	name        string
	surveyName  string
	description string
	iface       string
	surveyType  survey.Type
	wantErr     bool
}

// assertSurveyFields validates basic survey fields match expected values.

// assertSurveyFields validates basic survey fields match expected values.
func assertSurveyFields(t *testing.T, s *survey.Survey, tc createSurveyTestCase) {
	t.Helper()

	if s.ID == "" {
		t.Error("Survey ID is empty")
	}
	if s.Name != tc.surveyName {
		t.Errorf("Survey Name = %v, want %v", s.Name, tc.surveyName)
	}
	if s.Description != tc.description {
		t.Errorf("Survey Description = %v, want %v", s.Description, tc.description)
	}
	if s.Interface != tc.iface {
		t.Errorf("Survey Interface = %v, want %v", s.Interface, tc.iface)
	}
	if s.SurveyType != tc.surveyType {
		t.Errorf("Survey Type = %v, want %v", s.SurveyType, tc.surveyType)
	}
	if s.Status != survey.StatusCreated {
		t.Errorf("Survey Status = %v, want %v", s.Status, survey.StatusCreated)
	}
}

// assertSurveyFloors validates survey has proper floor structure.

// assertSurveyFloors validates survey has proper floor structure.
func assertSurveyFloors(t *testing.T, s *survey.Survey) {
	t.Helper()

	if len(s.Floors) == 0 {
		t.Error("Survey has no floors")
	}

	activeFloor := s.GetActiveFloor()
	if activeFloor == nil {
		t.Fatal("Survey has no active floor")
	}
	if activeFloor.Samples == nil {
		t.Error("Active floor Samples is nil")
	}
}

// assertSurveyTimestamps validates survey timestamps are set.

// assertSurveyTimestamps validates survey timestamps are set.
func assertSurveyTimestamps(t *testing.T, s *survey.Survey) {
	t.Helper()

	if s.CreatedAt.IsZero() {
		t.Error("Survey CreatedAt is zero")
	}
	if s.UpdatedAt.IsZero() {
		t.Error("Survey UpdatedAt is zero")
	}
}

func TestCreateSurvey(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	tests := []createSurveyTestCase{
		{
			name:        "valid passive survey",
			surveyName:  "Test Passive",
			description: "Test passive survey",
			iface:       "wlan0",
			surveyType:  survey.TypePassive,
			wantErr:     false,
		},
		{
			name:        "valid active survey",
			surveyName:  "Test Active",
			description: "Test active survey",
			iface:       "wlan0",
			surveyType:  survey.TypeActive,
			wantErr:     false,
		},
		{
			name:        "valid throughput survey",
			surveyName:  "Test Throughput",
			description: "Test throughput survey",
			iface:       "wlan0",
			surveyType:  survey.TypeThroughput,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := mgr.CreateSurvey(tt.surveyName, tt.description, tt.iface, tt.surveyType)

			if tt.wantErr {
				if err == nil {
					t.Error("CreateSurvey() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Errorf("CreateSurvey() error = %v, want nil", err)
				return
			}

			if s == nil {
				t.Fatal("CreateSurvey() returned nil survey")
			}

			assertSurveyFields(t, s, tt)
			assertSurveyFloors(t, s)
			assertSurveyTimestamps(t, s)
		})
	}
}

func TestGetSurvey(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	// Create a survey first.
	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "existing survey",
			id:      s.ID,
			wantErr: false,
		},
		{
			name:    "non-existent survey",
			id:      "non-existent-id",
			wantErr: true,
		},
		{
			name:    "empty id",
			id:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, getErr := mgr.GetSurvey(tt.id)

			if tt.wantErr {
				if getErr == nil {
					t.Error("GetSurvey() error = nil, want error")
				}
				return
			}

			if getErr != nil {
				t.Errorf("GetSurvey() error = %v, want nil", getErr)
				return
			}

			if result == nil {
				t.Fatal("GetSurvey() returned nil")
			}

			if result.ID != tt.id {
				t.Errorf("Survey ID = %v, want %v", result.ID, tt.id)
			}
		})
	}
}

func TestListSurveys(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	// Initially empty.
	surveys := mgr.ListSurveys()
	if len(surveys) != 0 {
		t.Errorf("ListSurveys() returned %d surveys, want 0", len(surveys))
	}

	// Create surveys.
	_, err := mgr.CreateSurvey("Survey 1", "Desc 1", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	_, err = mgr.CreateSurvey("Survey 2", "Desc 2", "wlan0", survey.TypeActive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	_, err = mgr.CreateSurvey("Survey 3", "Desc 3", "wlan0", survey.TypeThroughput)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	// Assert what came back, not how much. Counting alone passes if ListSurveys
	// returns the same survey three times, or three surveys carrying the wrong
	// names and types -- the exact shape of the heatmap defects in this package,
	// which held a correct count while every value was wrong.
	want := map[string]survey.Type{
		"Survey 1": survey.TypePassive,
		"Survey 2": survey.TypeActive,
		"Survey 3": survey.TypeThroughput,
	}

	surveys = mgr.ListSurveys()
	if len(surveys) != len(want) {
		t.Fatalf("ListSurveys() returned %d surveys, want %d", len(surveys), len(want))
	}

	seenIDs := make(map[string]bool, len(surveys))
	for i, s := range surveys {
		if s == nil {
			t.Fatalf("survey at index %d is nil", i)
		}
		if seenIDs[s.ID] {
			t.Errorf("survey %q returned more than once (ID %s)", s.Name, s.ID)
		}
		seenIDs[s.ID] = true

		wantType, ok := want[s.Name]
		if !ok {
			t.Errorf("unexpected survey %q in listing", s.Name)
			continue
		}
		if s.SurveyType != wantType {
			t.Errorf("survey %q has type %v, want %v", s.Name, s.SurveyType, wantType)
		}
		delete(want, s.Name)
	}

	for name := range want {
		t.Errorf("survey %q was created but not returned by ListSurveys()", name)
	}
}

func TestDeleteSurvey(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "delete existing survey",
			id:      s.ID,
			wantErr: false,
		},
		{
			name:    "delete non-existent survey",
			id:      "non-existent-id",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteErr := mgr.DeleteSurvey(tt.id)

			if tt.wantErr {
				if deleteErr == nil {
					t.Error("DeleteSurvey() error = nil, want error")
				}
				return
			}

			if deleteErr != nil {
				t.Errorf("DeleteSurvey() error = %v, want nil", deleteErr)
			}

			// Verify survey is deleted.
			_, getErr := mgr.GetSurvey(tt.id)
			if getErr == nil {
				t.Error("GetSurvey() after delete succeeded, want error")
			}
		})
	}
}
