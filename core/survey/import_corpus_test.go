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

			points, obs := 0, 0
			for _, floor := range got.Floors {
				points += len(floor.Samples)
				for _, p := range floor.Samples {
					if ps, ok := p.SampleData.(*survey.PassiveSample); ok {
						obs += len(ps.Networks)
					}
				}
			}

			if parts.declaredPoints > 0 && points != parts.declaredPoints {
				t.Errorf("stored %d points, the archive declares %d", points, parts.declaredPoints)
			}
			if len(got.Floors) == 0 || got.Floors[0].FloorPlan == nil {
				t.Error("floor plan did not survive the round trip")
			}
			t.Logf("%d points, %d observations persisted and reloaded", points, obs)
		})
	}
}
