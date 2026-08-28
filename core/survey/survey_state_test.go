// Survey state machine: start, pause, complete and the legal transitions
// between them.
package survey_test

import (
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func TestStartSurvey(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	tests := []struct {
		name        string
		id          string
		setupStatus survey.Status
		wantErr     bool
	}{
		{
			name:        "start created survey",
			id:          s.ID,
			setupStatus: survey.StatusCreated,
			wantErr:     false,
		},
		{
			name:        "start non-existent survey",
			id:          "non-existent-id",
			setupStatus: survey.StatusCreated,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startErr := mgr.StartSurvey(tt.id)

			if tt.wantErr {
				if startErr == nil {
					t.Error("StartSurvey() error = nil, want error")
				}
				return
			}

			if startErr != nil {
				t.Errorf("StartSurvey() error = %v, want nil", startErr)
				return
			}

			// Verify status changed.
			result, getErr := mgr.GetSurvey(tt.id)
			if getErr != nil {
				t.Fatalf("GetSurvey() failed: %v", getErr)
			}

			if result.Status != survey.StatusInProgress {
				t.Errorf("Survey Status = %v, want %v", result.Status, survey.StatusInProgress)
			}
		})
	}
}

func TestPauseSurvey(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	// Start survey first.
	err = mgr.StartSurvey(s.ID)
	if err != nil {
		t.Fatalf("StartSurvey() failed: %v", err)
	}

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "pause in-progress survey",
			id:      s.ID,
			wantErr: false,
		},
		{
			name:    "pause non-existent survey",
			id:      "non-existent-id",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pauseErr := mgr.PauseSurvey(tt.id)

			if tt.wantErr {
				if pauseErr == nil {
					t.Error("PauseSurvey() error = nil, want error")
				}
				return
			}

			if pauseErr != nil {
				t.Errorf("PauseSurvey() error = %v, want nil", pauseErr)
				return
			}

			// Verify status changed.
			result, getErr := mgr.GetSurvey(tt.id)
			if getErr != nil {
				t.Fatalf("GetSurvey() failed: %v", getErr)
			}

			if result.Status != survey.StatusPaused {
				t.Errorf("Survey Status = %v, want %v", result.Status, survey.StatusPaused)
			}
		})
	}
}

func TestCompleteSurvey(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	// Start survey first.
	err = mgr.StartSurvey(s.ID)
	if err != nil {
		t.Fatalf("StartSurvey() failed: %v", err)
	}

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "complete in-progress survey",
			id:      s.ID,
			wantErr: false,
		},
		{
			name:    "complete non-existent survey",
			id:      "non-existent-id",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completeErr := mgr.CompleteSurvey(tt.id)

			if tt.wantErr {
				if completeErr == nil {
					t.Error("CompleteSurvey() error = nil, want error")
				}
				return
			}

			if completeErr != nil {
				t.Errorf("CompleteSurvey() error = %v, want nil", completeErr)
				return
			}

			// Verify status changed.
			result, getErr := mgr.GetSurvey(tt.id)
			if getErr != nil {
				t.Fatalf("GetSurvey() failed: %v", getErr)
			}

			if result.Status != survey.StatusCompleted {
				t.Errorf("Survey Status = %v, want %v", result.Status, survey.StatusCompleted)
			}
		})
	}
}

func TestStateTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	// Test valid state transitions.
	// Created -> InProgress.
	err = mgr.StartSurvey(s.ID)
	if err != nil {
		t.Errorf("Created -> InProgress failed: %v", err)
	}

	// InProgress -> Paused.
	err = mgr.PauseSurvey(s.ID)
	if err != nil {
		t.Errorf("InProgress -> Paused failed: %v", err)
	}

	// Paused -> InProgress (resume).
	err = mgr.StartSurvey(s.ID)
	if err != nil {
		t.Errorf("Paused -> InProgress failed: %v", err)
	}

	// InProgress -> Completed.
	err = mgr.CompleteSurvey(s.ID)
	if err != nil {
		t.Errorf("InProgress -> Completed failed: %v", err)
	}

	// Verify final state.
	result, _ := mgr.GetSurvey(s.ID)
	if result.Status != survey.StatusCompleted {
		t.Errorf("Final status = %v, want %v", result.Status, survey.StatusCompleted)
	}
}

// updateFloorPlanTestCase defines a test case for TestUpdateFloorPlan.
