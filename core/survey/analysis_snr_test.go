package survey_test

// The dead-zone analysis used to be RSSI-only: it read the strongest signal of
// every sample and scored it against a dBm floor, whichever layer the operator
// was looking at. A floor can carry a strong signal and still be unusable when
// the noise floor rises to meet it, and that floor scored 100 out of 100.
//
// These tests are the discriminator: the same measurements, analysed twice,
// must disagree.

import (
	"strings"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// noisySurvey is strong signal over a high noise floor: -55 dBm against a
// -63 dBm noise floor is 8 dB of margin, which no link works well in.
func noisySurvey(points int) *survey.Survey {
	samples := make([]*survey.SamplePoint, 0, points)
	for i := range points {
		samples = append(samples, &survey.SamplePoint{
			X:         100 + i*10,
			Y:         100,
			Timestamp: time.Now(),
			SampleData: &survey.PassiveSample{
				Networks: []*wifi.ScannedNetwork{
					{Signal: -55, NoiseFloor: -63, SNR: 8},
				},
			},
		})
	}
	return &survey.Survey{
		ID:         "noisy-floor",
		Name:       "Noisy floor",
		SurveyType: survey.TypePassive,
		Status:     survey.StatusCompleted,
		Samples:    samples,
	}
}

func TestDetectDeadZones_SNRFindsWhatRSSICannot(t *testing.T) {
	s := noisySurvey(25)

	rssi, err := survey.DetectDeadZones(s, survey.HeatmapRSSI, survey.DefaultThreshold, nil)
	if err != nil {
		t.Fatalf("RSSI analysis: %v", err)
	}
	if len(rssi.DeadZones) != 0 {
		t.Errorf("RSSI analysis found %d dead zone(s) at -55 dBm against a -75 dBm floor, want none",
			len(rssi.DeadZones))
	}
	if rssi.CoverageScore != 100 {
		t.Errorf("RSSI coverage score = %.1f, want 100", rssi.CoverageScore)
	}

	snr, err := survey.DetectDeadZones(s, survey.HeatmapSNR, survey.DefaultSNRThreshold, nil)
	if err != nil {
		t.Fatalf("SNR analysis: %v", err)
	}
	if len(snr.DeadZones) == 0 {
		t.Fatal("SNR analysis found no dead zone at 8 dB against a 20 dB floor")
	}
	if snr.CoverageScore != 0 {
		t.Errorf("SNR coverage score = %.1f, want 0 — every sample is below the margin", snr.CoverageScore)
	}
	if snr.Metric != string(survey.HeatmapSNR) {
		t.Errorf("analysis metric = %q, want %q", snr.Metric, survey.HeatmapSNR)
	}
	if snr.Threshold != survey.DefaultSNRThreshold {
		t.Errorf("analysis threshold = %d, want %d", snr.Threshold, survey.DefaultSNRThreshold)
	}

	// 8 dB of margin is severe, and the figures carried are dB, not the dBm
	// the same cluster would have reported under the RSSI analysis.
	zone := snr.DeadZones[0]
	if zone.Severity != survey.SeveritySevere {
		t.Errorf("zone severity = %q at 8 dB SNR, want %q", zone.Severity, survey.SeveritySevere)
	}
	if zone.Avg != 8 || zone.Min != 8 {
		t.Errorf("zone reports min=%d avg=%d, want 8 dB of SNR — not the -55 dBm signal", zone.Min, zone.Avg)
	}
}

func TestDetectDeadZones_SNRAdviceIsAboutNoise(t *testing.T) {
	s := noisySurvey(25)

	rssi, err := survey.DetectDeadZones(s, survey.HeatmapRSSI, survey.DefaultThreshold, nil)
	if err != nil {
		t.Fatalf("RSSI analysis: %v", err)
	}
	snr, err := survey.DetectDeadZones(s, survey.HeatmapSNR, survey.DefaultSNRThreshold, nil)
	if err != nil {
		t.Fatalf("SNR analysis: %v", err)
	}

	joined := strings.ToLower(strings.Join(snr.Recommendations, " "))
	if !strings.Contains(joined, "noise") && !strings.Contains(joined, "interfer") {
		t.Errorf("SNR advice says nothing about noise or interference: %q", snr.Recommendations)
	}
	if strings.Contains(joined, "dbm") {
		t.Errorf("SNR advice quotes a dBm level, which is the RSSI analysis's unit: %q", snr.Recommendations)
	}
	for _, rec := range snr.Recommendations {
		for _, other := range rssi.Recommendations {
			if rec == other {
				t.Errorf("SNR advice borrows the RSSI sentence %q", rec)
			}
		}
	}
}

func TestDefaultThresholdFor_NeitherMetricBorrowsTheOther(t *testing.T) {
	if got := survey.DefaultThresholdFor(survey.HeatmapRSSI); got != survey.DefaultThreshold {
		t.Errorf("RSSI default = %d, want %d dBm", got, survey.DefaultThreshold)
	}
	if got := survey.DefaultThresholdFor(survey.HeatmapSNR); got != survey.DefaultSNRThreshold {
		t.Errorf("SNR default = %d, want %d dB", got, survey.DefaultSNRThreshold)
	}
	if survey.DefaultThreshold == survey.DefaultSNRThreshold {
		t.Fatal("the two defaults are the same number, so nothing here discriminates")
	}
}

func TestDetectDeadZones_UnsupportedMetricIsRefused(t *testing.T) {
	// The heatmap renders download throughput; the dead-zone analysis has no
	// rule for it. Falling back to RSSI is exactly the defect #344 closes.
	if _, err := survey.DetectDeadZones(noisySurvey(3), "download", -75, nil); err == nil {
		t.Fatal("analysing a metric with no dead-zone rule: want an error, got none")
	}
}

func TestDetectDeadZones_SNRWithoutANoiseFloorIsAbsentNotZero(t *testing.T) {
	// The parser leaves SNR at zero when the capture carried no usable noise
	// floor (see parseNetwork): a receiver that reports 0 dB of margin on a
	// -55 dBm signal is not a measurement, it is a field that was not there.
	// Read as a value, every such point is a severe dead zone and the whole
	// floor scores 0 — an alarming picture drawn from nothing.
	samples := make([]*survey.SamplePoint, 0, 10)
	for i := range 10 {
		samples = append(samples, &survey.SamplePoint{
			X:         100 + i*10,
			Y:         100,
			Timestamp: time.Now(),
			SampleData: &survey.PassiveSample{
				Networks: []*wifi.ScannedNetwork{{Signal: -55}},
			},
		})
	}
	s := &survey.Survey{ID: "no-noise-floor", SurveyType: survey.TypePassive, Samples: samples}

	if _, err := survey.DetectDeadZones(s, survey.HeatmapSNR, survey.DefaultSNRThreshold, nil); err == nil {
		t.Fatal("SNR analysis of a survey that recorded no noise floor: want an error, got an analysis")
	}

	// The same survey still has signal to analyse.
	rssi, err := survey.DetectDeadZones(s, survey.HeatmapRSSI, survey.DefaultThreshold, nil)
	if err != nil {
		t.Fatalf("RSSI analysis of the same survey: %v", err)
	}
	if len(rssi.DeadZones) != 0 {
		t.Errorf("RSSI analysis found %d dead zone(s) at -55 dBm, want none", len(rssi.DeadZones))
	}
}
