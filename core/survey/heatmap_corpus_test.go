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
	// It has to be one that recorded a noise floor, though — six of the 48
	// reference archives did not, and on those the SNR half of this test can
	// only watch the analysis refuse, which proves nothing about the numbers.
	var candidates []string
	for _, f := range files {
		parts := readAMP(t, f)
		if parts.surveyResult != nil && parts.declaredPoints > 50 {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) == 0 {
		t.Skip("no capture with enough points in the corpus")
	}

	var (
		chosen string
		mgr    *survey.Manager
		svy    *survey.Survey
	)
	for _, f := range candidates {
		raw, readErr := os.ReadFile(filepath.Clean(f))
		if readErr != nil {
			t.Fatalf("read archive: %v", readErr)
		}
		m := mustManager(t, t.TempDir(), nil, nil, nil, nil)
		imported, importErr := m.ImportAirMapper("analysis", raw)
		if importErr != nil {
			t.Fatalf("ImportAirMapper(%s): %v", filepath.Base(f), importErr)
		}
		chosen, mgr, svy = f, m, imported
		if _, err := m.DetectDeadZones(imported.ID, survey.HeatmapSNR, survey.DefaultSNRThreshold); err == nil {
			break
		}
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

	analysis, dzErr := mgr.DetectDeadZones(svy.ID, survey.HeatmapRSSI, -75)
	if dzErr != nil {
		t.Fatalf("DetectDeadZones: %v", dzErr)
	}
	if analysis == nil {
		t.Fatal("DetectDeadZones returned nothing for a survey with measurements")
	}

	if analysis.SurveyID != svy.ID {
		t.Errorf("analysis is for survey %q, want %q", analysis.SurveyID, svy.ID)
	}
	if analysis.Threshold != -75 {
		t.Errorf("analysis threshold = %d, want the -75 that was requested", analysis.Threshold)
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
		if zone.Min > zone.Avg {
			t.Errorf("dead zone %d: min RSSI %d exceeds average %d", i, zone.Min, zone.Avg)
		}
		// It is a dead zone at -75, so its average must actually be below that.
		if zone.Avg > -75 {
			t.Errorf("dead zone %d has average RSSI %d, which is above the -75 threshold",
				i, zone.Avg)
		}
		switch zone.Severity {
		case "minor", "moderate", "severe":
		default:
			t.Errorf("dead zone %d has severity %q, which is not one of minor/moderate/severe",
				i, zone.Severity)
		}
	}

	if analysis.Metric != string(survey.HeatmapRSSI) {
		t.Errorf("analysis metric = %q, want %q", analysis.Metric, survey.HeatmapRSSI)
	}

	t.Logf("%s: %dx%d heatmap, %d samples, RSSI %.1f..%.1f dBm, %d dead zones, coverage %.1f%%",
		filepath.Base(chosen), heatmap.Width, heatmap.Height, heatmap.SampleCount,
		stats.Min, stats.Max, len(analysis.DeadZones), analysis.CoverageScore)

	// The SNR analysis over the same measurements. It is a different question
	// asked of the same walk — a real capture carries a noise floor, and the
	// margin above it is not the signal level — so a run that agreed with the
	// RSSI verdict in every number would mean the metric never reached the
	// analysis, which is the defect #344 closes.
	snr, snrErr := mgr.DetectDeadZones(svy.ID, survey.HeatmapSNR, survey.DefaultSNRThreshold)
	if snrErr != nil {
		// Not every archive records a noise floor; the parser leaves SNR unset
		// then and the analysis has nothing to read. That is a legitimate
		// outcome and it must be an error, not a floor of severe dead zones.
		t.Logf("%s: no SNR analysis (%v)", filepath.Base(chosen), snrErr)
		return
	}

	if snr.Metric != string(survey.HeatmapSNR) {
		t.Errorf("SNR analysis reports metric %q", snr.Metric)
	}
	if snr.Threshold != survey.DefaultSNRThreshold {
		t.Errorf("SNR analysis ran at threshold %d, want %d dB", snr.Threshold, survey.DefaultSNRThreshold)
	}
	for i, zone := range snr.DeadZones {
		// dB of margin, not dBm of signal. Any negative here means the RSSI
		// values were read under the SNR heading.
		if zone.Avg < 0 || zone.Min < 0 {
			t.Errorf("SNR dead zone %d reports min=%d avg=%d, which are signal levels, not margins",
				i, zone.Min, zone.Avg)
		}
		if zone.Avg > survey.DefaultSNRThreshold {
			t.Errorf("SNR dead zone %d averages %d dB, above the %d dB threshold",
				i, zone.Avg, survey.DefaultSNRThreshold)
		}
	}
	// A well-covered floor legitimately scores 100% under both metrics, so
	// equal scores prove nothing. What discriminates is a threshold no signal
	// level could ever clear: at 60 dB of required margin every real reading
	// is a finding, and every finding must be a margin. Read the RSSI values
	// here and the averages come back around -40.
	demanding, demErr := mgr.DetectDeadZones(svy.ID, survey.HeatmapSNR, 60)
	if demErr != nil {
		t.Fatalf("SNR analysis at 60 dB: %v", demErr)
	}
	if len(demanding.DeadZones) == 0 {
		t.Error("no SNR finding at a 60 dB margin, which no measured floor clears")
	}
	for i, zone := range demanding.DeadZones {
		if zone.Avg <= 0 || zone.Avg > 60 {
			t.Errorf("SNR finding %d averages %d, which is not a margin in dB", i, zone.Avg)
		}
	}
	// Not zero: a strong signal over a quiet floor really does clear 60 dB in
	// places — 9.6% of this walk does. What cannot happen is a clean sweep.
	if demanding.CoverageScore >= 100 {
		t.Errorf("SNR coverage at a 60 dB margin = %.1f%%, want most of the floor below it",
			demanding.CoverageScore)
	}
	if len(analysis.DeadZones) == len(demanding.DeadZones) && analysis.CoverageScore == demanding.CoverageScore {
		t.Error("the RSSI verdict and the demanding SNR one are identical — the metric did not reach the analysis")
	}
	// The advice must be the noise vocabulary, not the coverage one.
	for _, rec := range snr.Recommendations {
		for _, other := range analysis.Recommendations {
			if rec == other {
				t.Errorf("SNR analysis borrows the RSSI sentence %q", rec)
			}
		}
	}

	t.Logf("%s: SNR %d dead zones, coverage %.1f%% at %d dB",
		filepath.Base(chosen), len(snr.DeadZones), snr.CoverageScore, snr.Threshold)
}
