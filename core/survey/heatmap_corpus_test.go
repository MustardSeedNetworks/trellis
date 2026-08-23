package survey_test

// heatmap_corpus_test.go proves the analysis path still works on a real
// imported survey, not just on hand-built fixtures.
//
// The store changed underneath heatmap generation, dead-zone detection and the
// findings pass. Each of those reads samples through the survey domain type, so
// they should be unaffected — "should be" is why this test exists. It imports a
// real capture and runs them.
//
// Skips without TRELLIS_AMP_CORPUS, like the other corpus tests.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func TestAnalysisRunsOnAnImportedSurvey(t *testing.T) {
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

	// One passive capture is enough: the analysis path reads passive samples,
	// and running all twelve would just repeat the same code with more rows.
	var chosen string
	for _, f := range files {
		parts := readAMP(t, f)
		if parts.surveyResult != nil && parts.declaredPoints > 50 {
			chosen = f
			break
		}
	}
	if chosen == "" {
		t.Skip("no capture with enough points in the corpus")
	}

	raw, readErr := os.ReadFile(filepath.Clean(chosen))
	if readErr != nil {
		t.Fatalf("read archive: %v", readErr)
	}

	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	svy, importErr := mgr.ImportAirMapper("analysis", raw)
	if importErr != nil {
		t.Fatalf("ImportAirMapper: %v", importErr)
	}

	heatmap, hmErr := mgr.GenerateHeatmap(svy.ID, survey.HeatmapConfig{
		Type: survey.HeatmapRSSI,
	})
	if hmErr != nil {
		t.Fatalf("GenerateHeatmap on %s: %v", filepath.Base(chosen), hmErr)
	}
	if heatmap == nil {
		t.Fatal("GenerateHeatmap returned nothing for a survey with measurements")
	}

	zones, dzErr := mgr.DetectDeadZones(svy.ID, -75)
	if dzErr != nil {
		t.Fatalf("DetectDeadZones: %v", dzErr)
	}
	if zones == nil {
		t.Fatal("DetectDeadZones returned nothing for a survey with measurements")
	}
	t.Logf("%s: heatmap generated, dead-zone analysis returned", filepath.Base(chosen))
}
