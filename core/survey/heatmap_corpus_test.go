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

	// Assert the values, not just that a result came back. Every heatmap
	// read-path defect found so far (#114 and the three before it) produced a
	// fully-populated result with the wrong numbers in it, and a nil check
	// passes all four.
	if heatmap.Width <= 0 || heatmap.Height <= 0 {
		t.Errorf("heatmap has no extent: %dx%d", heatmap.Width, heatmap.Height)
	}
	if len(heatmap.Image) == 0 {
		t.Error("heatmap carries no PNG image")
	}
	// A PNG, not an empty buffer or some other format: the first eight bytes
	// are the signature.
	if len(heatmap.Image) >= 8 && string(heatmap.Image[1:4]) != "PNG" {
		t.Errorf("image is not a PNG: % x", heatmap.Image[:8])
	}
	if heatmap.SampleCount <= 0 {
		t.Error("heatmap reports no samples despite a survey with measurements")
	}
	if heatmap.Type != string(survey.HeatmapRSSI) {
		t.Errorf("heatmap type = %q, want %q", heatmap.Type, survey.HeatmapRSSI)
	}

	// RSSI values must be physically plausible. A unit mix-up or a zeroed grid
	// lands outside this range, which is the class of defect the old nil check
	// could not see.
	stats := heatmap.Stats
	if stats.Count <= 0 {
		t.Error("grid stats report no cells")
	}
	if stats.Min < -120 || stats.Max > 0 {
		t.Errorf("RSSI grid outside a plausible range: min=%.1f max=%.1f", stats.Min, stats.Max)
	}
	if stats.Min > stats.Max {
		t.Errorf("grid min %.1f exceeds max %.1f", stats.Min, stats.Max)
	}
	if stats.Average < stats.Min || stats.Average > stats.Max {
		t.Errorf("grid average %.1f outside [%.1f, %.1f]", stats.Average, stats.Min, stats.Max)
	}
	// A real survey covers a range of signal levels. An entirely flat grid means
	// interpolation collapsed, which renders as a plausible single-colour plate.
	if stats.Min == stats.Max {
		t.Errorf("grid is entirely flat at %.1f dBm — interpolation produced no variation", stats.Min)
	}

	analysis, dzErr := mgr.DetectDeadZones(svy.ID, -75)
	if dzErr != nil {
		t.Fatalf("DetectDeadZones: %v", dzErr)
	}
	if analysis == nil {
		t.Fatal("DetectDeadZones returned nothing for a survey with measurements")
	}

	if analysis.SurveyID != svy.ID {
		t.Errorf("analysis is for survey %q, want %q", analysis.SurveyID, svy.ID)
	}
	if analysis.ThresholdDBm != -75 {
		t.Errorf("analysis threshold = %d, want the -75 that was requested", analysis.ThresholdDBm)
	}
	if analysis.CoverageScore < 0 || analysis.CoverageScore > 100 {
		t.Errorf("coverage score %.1f is outside 0-100", analysis.CoverageScore)
	}

	// Each reported dead zone must describe a real region below the threshold.
	for i, zone := range analysis.DeadZones {
		if zone.SampleCount <= 0 {
			t.Errorf("dead zone %d (%s) is backed by no samples", i, zone.ID)
		}
		if zone.RadiusM <= 0 {
			t.Errorf("dead zone %d has radius %.2f m", i, zone.RadiusM)
		}
		if zone.MinRSSI > zone.AvgRSSI {
			t.Errorf("dead zone %d: min RSSI %d exceeds average %d", i, zone.MinRSSI, zone.AvgRSSI)
		}
		// It is a dead zone at -75, so its average must actually be below that.
		if zone.AvgRSSI > -75 {
			t.Errorf("dead zone %d has average RSSI %d, which is above the -75 threshold",
				i, zone.AvgRSSI)
		}
		switch zone.Severity {
		case "minor", "moderate", "severe":
		default:
			t.Errorf("dead zone %d has severity %q, which is not one of minor/moderate/severe",
				i, zone.Severity)
		}
	}

	t.Logf("%s: %dx%d heatmap, %d samples, RSSI %.1f..%.1f dBm, %d dead zones, coverage %.1f%%",
		filepath.Base(chosen), heatmap.Width, heatmap.Height, heatmap.SampleCount,
		stats.Min, stats.Max, len(analysis.DeadZones), analysis.CoverageScore)
}
