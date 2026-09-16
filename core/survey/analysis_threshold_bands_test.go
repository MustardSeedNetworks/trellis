package survey_test

// The severity bands and the sentences that describe them used to be written
// as absolute constants — signal "between -75 and -80 dBm", a margin "at or
// below 10 dB" — while the dead zones themselves were found relative to the
// operator's threshold. The two only agreed at the default threshold. An
// operator who raised the RSSI threshold to -40 dBm was told about a band no
// sample of theirs could be in, and the PDF said "-85 dBm" whatever was asked
// for.
//
// These tests pin the bands to the threshold, which is the only number the
// operator chose.

import (
	"strings"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// bandSurvey places one sample per listed signal level, each beyond
// [survey.ClusterRadius] of the last so that every level becomes its own zone.
func bandSurvey(signals []int) *survey.Survey {
	samples := make([]*survey.SamplePoint, 0, len(signals))
	for i, dbm := range signals {
		samples = append(samples, &survey.SamplePoint{
			X:         100 + i*int(survey.ClusterRadius)*4,
			Y:         100,
			Timestamp: time.Now(),
			SampleData: &survey.PassiveSample{
				Networks: []*wifi.ScannedNetwork{
					{Signal: dbm, NoiseFloor: -95, SNR: dbm + 95},
				},
			},
		})
	}
	return &survey.Survey{
		ID:         "band-survey",
		Name:       "Band survey",
		SurveyType: survey.TypePassive,
		Status:     survey.StatusCompleted,
		Samples:    samples,
		UpdatedAt:  time.Now(),
	}
}

func TestRSSIRecommendationBandsDeriveFromThreshold(t *testing.T) {
	// At a -40 dBm threshold the bands are -50 (severe) and -45 (moderate).
	// Every one of these samples is weak against that threshold.
	s := bandSurvey([]int{-52, -47, -42})

	result, err := survey.DetectDeadZones(s, survey.HeatmapRSSI, -40, nil)
	if err != nil {
		t.Fatalf("DetectDeadZones: %v", err)
	}

	joined := strings.Join(result.Recommendations, "\n")

	// The defect: bands written as absolutes, describing a band that cannot
	// hold a sample taken against this threshold.
	for _, stale := range []string{"-85", "-80", "-75"} {
		if strings.Contains(joined, stale) {
			t.Errorf("recommendation names %s dBm, which no band at threshold -40 covers:\n%s",
				stale, joined)
		}
	}

	// The bands the operator's threshold actually implies.
	for _, want := range []string{"-50", "-45"} {
		if !strings.Contains(joined, want) {
			t.Errorf("recommendation never names the derived band edge %s dBm:\n%s", want, joined)
		}
	}
}

func TestRSSIRecommendationBandsAtTheDefaultThresholdAreUnchanged(t *testing.T) {
	// The fixed constants were the default threshold's bands hard-coded:
	// -75 - 10 = -85 and -75 - 5 = -80. Deriving them must not move them.
	s := bandSurvey([]int{-87, -82, -77})

	result, err := survey.DetectDeadZones(s, survey.HeatmapRSSI, survey.DefaultThreshold, nil)
	if err != nil {
		t.Fatalf("DetectDeadZones: %v", err)
	}

	joined := strings.Join(result.Recommendations, "\n")
	for _, want := range []string{"-85", "-80"} {
		if !strings.Contains(joined, want) {
			t.Errorf("default-threshold band edge %s dBm is missing:\n%s", want, joined)
		}
	}
}

func TestSNRRecommendationBandsDeriveFromThreshold(t *testing.T) {
	// A 30 dB margin threshold puts the bands at 20 dB and 25 dB, not the
	// 10 dB and 15 dB the sentences used to state.
	samples := make([]*survey.SamplePoint, 0, 3)
	for i, snr := range []int{18, 23, 28} {
		samples = append(samples, &survey.SamplePoint{
			X:         100 + i*int(survey.ClusterRadius)*4,
			Y:         100,
			Timestamp: time.Now(),
			SampleData: &survey.PassiveSample{
				Networks: []*wifi.ScannedNetwork{
					{Signal: -55, NoiseFloor: -55 - snr, SNR: snr},
				},
			},
		})
	}
	s := &survey.Survey{
		ID:         "snr-band-survey",
		Name:       "SNR band survey",
		SurveyType: survey.TypePassive,
		Status:     survey.StatusCompleted,
		Samples:    samples,
		UpdatedAt:  time.Now(),
	}

	result, err := survey.DetectDeadZones(s, survey.HeatmapSNR, 30, nil)
	if err != nil {
		t.Fatalf("DetectDeadZones: %v", err)
	}

	joined := strings.Join(result.Recommendations, "\n")
	for _, stale := range []string{"10 dB", "15 dB"} {
		if strings.Contains(joined, stale) {
			t.Errorf("recommendation names %s, a band no sample at threshold 30 falls in:\n%s",
				stale, joined)
		}
	}
	for _, want := range []string{"20 dB", "25 dB"} {
		if !strings.Contains(joined, want) {
			t.Errorf("recommendation never names the derived band edge %s:\n%s", want, joined)
		}
	}
}

// TestReportRecommendationsUseTheOperatorsThreshold covers the PDF half. The
// report used to run a recommendation generator of its own — a second copy of
// these sentences that took no threshold at all and always said "-85 dBm" —
// so raising the threshold changed the Coverage page and left the report
// saying something no measurement in it supported.
func TestReportRecommendationsUseTheOperatorsThreshold(t *testing.T) {
	s := bandSurvey([]int{-52, -47, -42})

	opts := survey.DefaultReportOptions()
	opts.Metric = survey.HeatmapRSSI
	opts.Threshold = -40

	pdf, err := survey.NewReportGenerator(s, opts).Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("Generate produced no PDF")
	}

	// Assert on the findings the report itself will print, not on a separate
	// call to the analysis — otherwise this passes just as well when the
	// report goes back to generating recommendations of its own.
	var texts []string
	for _, rec := range survey.ExportReportRecommendations(s, opts) {
		texts = append(texts, rec.Text)
	}
	if len(texts) == 0 {
		t.Fatal("the report printed no recommendations")
	}

	joined := strings.Join(texts, "\n")
	if strings.Contains(joined, "-85") {
		t.Errorf("report findings still name the hard-coded -85 dBm band:\n%s", joined)
	}
	if !strings.Contains(joined, "-50") {
		t.Errorf("report findings never name the band threshold -40 implies:\n%s", joined)
	}
}
