package survey_test

// import_corpus_test.go imports real AirMapper archives end to end and checks
// that the measurements survive the whole path: zip -> protobuf -> domain ->
// SQLite -> reload.
//
// The corpus is not committed; this repository is public and those are real
// site surveys of named third parties. Point the test at a directory:
//
//	TRELLIS_AMP_CORPUS=~/Documents/AirMapper-Surveys go test ./core/survey/
//
// The oracle is again the archive's own declared surveyPointCount, so the
// assertion is arithmetic rather than a fixture someone tuned until it passed.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func TestImportAirMapperLandsMeasurementsInTheStore(t *testing.T) {
	dir := os.Getenv(corpusEnv)
	if dir == "" {
		t.Skipf("set %s to a directory of .amp files to run this", corpusEnv)
	}
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.amp"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no .amp files under %s (err=%v)", dir, err)
	}

	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			parts := readAMP(t, path)
			// The path comes from filepath.Glob over an operator-supplied
			// directory, not from anything untrusted; Clean satisfies the
			// taint check without pretending the input is hostile.
			raw, readErr := os.ReadFile(filepath.Clean(path))
			if readErr != nil {
				t.Fatalf("read archive: %v", readErr)
			}

			// What the reader recovers, to compare against what the store
			// keeps. Anything the persistence layer silently drops shows up as
			// a difference between these two.
			parsed, parseErr := survey.ParseSurveyResult(parts.surveyResult)
			if parseErr != nil {
				t.Fatalf("ParseSurveyResult: %v", parseErr)
			}

			storeDir := t.TempDir()
			mgr := mustManager(t, storeDir, nil, nil, nil, nil)
			imported, importErr := mgr.ImportAirMapper("corpus", raw)
			if importErr != nil {
				t.Fatalf("ImportAirMapper: %v", importErr)
			}

			// Reload from SQLite. Asserting on the in-memory value would pass
			// even if nothing were persisted, which is the failure this whole
			// change is about.
			reopened := mustManager(t, storeDir, nil, nil, nil, nil)
			if loadErr := reopened.LoadSurveys(); loadErr != nil {
				t.Fatalf("LoadSurveys: %v", loadErr)
			}
			got, getErr := reopened.GetSurvey(imported.ID)
			if getErr != nil {
				t.Fatalf("GetSurvey after reload: %v", getErr)
			}

			points, obs, active := 0, 0, 0
			for _, floor := range got.Floors {
				points += len(floor.Samples)
				for _, p := range floor.Samples {
					switch sd := p.SampleData.(type) {
					case *survey.PassiveSample:
						obs += len(sd.Networks)
					case *survey.ActiveSample:
						active++
					}
				}
			}

			if parts.declaredPoints > 0 && points != parts.declaredPoints {
				t.Errorf("stored %d points, the archive declares %d", points, parts.declaredPoints)
			}

			// Compare against what the parser recovered, not against a number
			// written here. Counting points alone let a whole survey type
			// through: the parser recovered 245 active associations, the store
			// dropped every one, and this test passed anyway.
			wantObs, wantActive := 0, 0
			for _, p := range parsed {
				wantObs += len(p.Networks)
				if p.Active != nil {
					wantActive++
				}
			}
			if obs != wantObs {
				t.Errorf("stored %d passive observations, the parser recovered %d", obs, wantObs)
			}
			if active != wantActive {
				t.Errorf("stored %d active associations, the parser recovered %d", active, wantActive)
			}
			if len(got.Floors) == 0 || got.Floors[0].FloorPlan == nil {
				t.Error("floor plan did not survive the round trip")
			}
			t.Logf("%d points, %d passive observations, %d active associations persisted and reloaded",
				points, obs, active)
		})
	}
}
