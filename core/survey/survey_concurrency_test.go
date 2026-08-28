// Persistence across manager restarts, and concurrent access.
package survey_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	// Create a survey.
	s, err := mgr.CreateSurvey("Test Survey", "Description", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey() failed: %v", err)
	}

	// Verify the store exists. A survey is a row now, not a file, so this
	// asserts the database was created rather than looking for <id>.json —
	// the round-trip below is what proves the survey itself persisted.
	dbPath := filepath.Join(tmpDir, "surveys.db")
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		t.Errorf("survey store not created at %s", dbPath)
	}

	// Create new manager to load surveys.
	mgr2 := mustManager(t, tmpDir, nil, nil, nil, nil)

	// Load surveys from disk.
	err = mgr2.LoadSurveys()
	if err != nil {
		t.Fatalf("LoadSurveys() failed: %v", err)
	}

	// Verify survey was loaded.
	loaded, err := mgr2.GetSurvey(s.ID)
	if err != nil {
		t.Errorf("Failed to load survey: %v", err)
	}

	if loaded == nil {
		t.Fatal("Loaded survey is nil")
	}

	if loaded.Name != s.Name {
		t.Errorf("Loaded survey Name = %v, want %v", loaded.Name, s.Name)
	}

	if loaded.Description != s.Description {
		t.Errorf("Loaded survey Description = %v, want %v", loaded.Description, s.Description)
	}

	// Delete, then prove it is gone from the store rather than from the
	// filesystem: a third manager reloads from SQLite and must not find it.
	if err = mgr2.DeleteSurvey(s.ID); err != nil {
		t.Errorf("DeleteSurvey() failed: %v", err)
	}

	mgr3 := mustManager(t, tmpDir, nil, nil, nil, nil)
	if err := mgr3.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys() after delete: %v", err)
	}
	if _, err := mgr3.GetSurvey(s.ID); err == nil {
		t.Error("deleted survey still present after reload")
	}
}

func TestConcurrentOperations(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := mustManager(t, tmpDir, nil, nil, nil, nil)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent creates.
	for i := range numGoroutines {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			_, err := mgr.CreateSurvey("Survey", "Desc", "wlan0", survey.TypePassive)
			if err != nil {
				t.Errorf("Concurrent CreateSurvey() failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Should have created all surveys.
	surveys := mgr.ListSurveys()
	if len(surveys) != numGoroutines {
		t.Errorf("Expected %d surveys, got %d", numGoroutines, len(surveys))
	}
}

func TestConcurrentSampleAddition(t *testing.T) {
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

	var wg sync.WaitGroup
	numSamples := 50

	sampleData := map[string]any{
		"networks": []any{
			map[string]any{
				"ssid": "Test",
				"rssi": -60,
			},
		},
	}

	// Concurrent sample additions.
	for i := range numSamples {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			addErr := mgr.AddSample(s.ID, n, n, sampleData)
			if addErr != nil {
				t.Errorf("Concurrent AddSample() failed: %v", addErr)
			}
		}(i)
	}

	wg.Wait()

	// Verify all samples added (now on the active floor).
	result, err := mgr.GetSurvey(s.ID)
	if err != nil {
		t.Fatalf("GetSurvey() failed: %v", err)
	}

	samples := result.GetAllSamples()
	if len(samples) != numSamples {
		t.Errorf("Expected %d samples, got %d", numSamples, len(samples))
	}
}
