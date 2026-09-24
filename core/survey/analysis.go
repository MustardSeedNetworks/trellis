package survey

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// DeadZone represents a detected area of poor Wi-Fi coverage.
//
// Min and Avg carry the analysed metric's own unit — dBm under [HeatmapRSSI],
// dB under [HeatmapSNR] — which is why they are not named for either.
type DeadZone struct {
	ID          string  `json:"id"`
	Center      Point2D `json:"center"`
	RadiusM     float64 `json:"radius_m"`
	Min         int     `json:"min"`
	Avg         int     `json:"avg"`
	SampleCount int     `json:"sample_count"`
	Severity    string  `json:"severity"` // "minor", "moderate", "severe"
}

// DeadZoneAnalysis contains the results of dead zone detection analysis.
type DeadZoneAnalysis struct {
	SurveyID string `json:"survey_id"`
	// Metric is the layer analysed, and Threshold is in that layer's unit.
	// Both are reported because the same figures mean different things under
	// each, and a caller that shows a number must be able to label it.
	Metric          string         `json:"metric"`
	Threshold       int            `json:"threshold"`
	DeadZones       []DeadZone     `json:"dead_zones"`
	CoverageScore   float64        `json:"coverage_score"` // 0-100
	Recommendations []string       `json:"recommendations"`
	Anomalies       []wifi.Anomaly `json:"anomalies"` // Wi-Fi anomalies detected from the survey's passive AP observations

	// advice is the same findings carrying the urgency each was written with.
	// The wire keeps plain sentences; the PDF prints a priority badge beside
	// each, and used to build its own drifted copies of them to get one.
	advice []Recommendation
}

// Coverage level thresholds in dBm.
const (
	ExcellentSignal = -50 // Excellent: > -50 dBm.
	GoodSignal      = -65 // Good: -50 to -65 dBm.
	FairSignal      = -75 // Fair: -65 to -75 dBm.
	PoorSignal      = -85 // Poor: -75 to -85 dBm (dead zone below).
)

// Severity level constants for dead zone classification.
const (
	SeverityMinor    = "minor"
	SeverityModerate = "moderate"
	SeveritySevere   = "severe"
)

// DefaultThreshold is the default RSSI dead-zone threshold, in dBm.
const DefaultThreshold = -75

// DefaultSNRThreshold is the default signal-to-noise dead-zone threshold, in
// dB. A link needs headroom above the noise, not a signal level: 20 dB is the
// margin voice is normally planned to, and below it retries climb whatever the
// signal strength says.
const DefaultSNRThreshold = 20

// A zone's severity is how far its average falls below the threshold the
// operator chose, not a fixed level. Both metrics used to carry these two
// offsets written out as absolutes — -85 and -80 dBm against the default -75
// threshold, 10 and 15 dB against the default 20 — which is this same pair
// hard-coded at one threshold each, and wrong at every other. They are
// offsets, so they subtract in the metric's own unit: a signal level and a
// noise margin are still never compared to each other.
const (
	severeBandOffset   = 10
	moderateBandOffset = 5
)

// DefaultThresholdFor is the dead-zone threshold to use when a caller names a
// metric but no threshold. There is deliberately no shared default: -75 dB of
// signal-to-noise is not a number, and 20 dBm is not a coverage floor.
func DefaultThresholdFor(metric HeatmapType) int {
	if metric == HeatmapSNR {
		return DefaultSNRThreshold
	}
	return DefaultThreshold
}

// SupportsCoverageAnalysis reports whether the dead-zone analysis has a rule
// for this metric. The heatmap renders more layers than the analysis can speak
// about; answering about RSSI when asked about one of those was the defect
// this parameter exists to close.
func SupportsCoverageAnalysis(metric HeatmapType) bool {
	return metric == HeatmapRSSI || metric == HeatmapSNR
}

// ClusterRadius defines the maximum distance (in pixels) to consider samples as part of the same dead zone.
const ClusterRadius = 50.0

// Dead zone analysis coverage score thresholds and calculation constants.
const (
	// deadZonePercentMultiplier converts a ratio (0.0-1.0) to a percentage (0-100).
	deadZonePercentMultiplier = 100.0

	// deadZoneThresholdCritical indicates critical coverage issues requiring infrastructure redesign.
	deadZoneThresholdCritical = 50

	// deadZoneThresholdPoor indicates poor coverage needing 2-3 additional access points.
	deadZoneThresholdPoor = 70

	// deadZoneThresholdModerate indicates moderate coverage needing 1-2 additional access points.
	deadZoneThresholdModerate = 85

	// deadZoneThresholdGood indicates good coverage with only minor improvements needed.
	deadZoneThresholdGood = 95

	// deadZoneThresholdClean is the score at or above which a floor with no
	// dead zone at all is reported as clean rather than left without a verdict.
	deadZoneThresholdClean = 90

	// deadZoneMinSamples is the minimum number of samples needed for reliable dead zone analysis.
	deadZoneMinSamples = 20
)

// DetectDeadZones analyzes a survey and identifies areas of poor coverage in
// the named metric.
//
// The analysis:
//  1. Extracts the metric's values from all survey samples
//  2. Identifies samples below the threshold
//  3. Groups nearby weak samples into dead zones using distance-based clustering
//  4. Classifies each zone's severity against that metric's bands
//  5. Computes overall coverage score (percentage of samples above threshold)
//  6. Generates recommendations in that metric's terms
//
// Parameters:
//   - survey: The Wi-Fi survey to analyze
//   - metric: [HeatmapRSSI] or [HeatmapSNR]; see [SupportsCoverageAnalysis]
//   - threshold: in the metric's unit; see [DefaultThresholdFor]
//
// detector runs the Wi-Fi anomaly rule set over the survey's passive AP
// observations; nil yields no anomalies (see [AnomalyDetector]).
//
// Returns an analysis result with dead zones, coverage score, and recommendations.
func DetectDeadZones(
	survey *Survey,
	metric HeatmapType,
	threshold int,
	detector AnomalyDetector,
) (*DeadZoneAnalysis, error) {
	if survey == nil {
		return nil, errors.New("survey is nil")
	}

	points := survey.GetAllSamples()
	if len(points) == 0 {
		return nil, errors.New("survey has no samples")
	}

	return analyzeCoverage(survey.ID, points, floorPlanOf(survey), survey.UpdatedAt, metric, threshold, detector)
}

// DetectFloorDeadZones analyses one floor from that floor's own measurements
// and plan, the counterpart of [GenerateFloorHeatmap]. A survey-wide analysis
// scores every floor's samples together and clusters them on whichever plan is
// active, which is only right when there is one floor.
func DetectFloorDeadZones(
	surveyID string,
	floor *Floor,
	metric HeatmapType,
	threshold int,
	detector AnomalyDetector,
) (*DeadZoneAnalysis, error) {
	if floor == nil {
		return nil, errors.New("floor is nil")
	}
	if len(floor.Samples) == 0 {
		return nil, errors.New("floor has no samples")
	}

	return analyzeCoverage(surveyID, floor.Samples, floor.FloorPlan, floor.UpdatedAt, metric, threshold, detector)
}

// analyzeCoverage is the analysis itself, once a caller has decided which
// measurements, which plan and which metric it means.
func analyzeCoverage(
	surveyID string,
	points []*SamplePoint,
	floorPlan *FloorPlan,
	observedAt time.Time,
	metric HeatmapType,
	threshold int,
	detector AnomalyDetector,
) (*DeadZoneAnalysis, error) {
	if !SupportsCoverageAnalysis(metric) {
		return nil, fmt.Errorf("no dead-zone rule for metric %q", metric)
	}

	samples := extractSamples(points, string(metric))
	if len(samples) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrMetricUnmeasured, metric)
	}

	weakSamples := filterWeakSamples(samples, float64(threshold))
	coverageScore := calculateCoverageScore(samples, weakSamples)
	edges := edgesFor(threshold)
	deadZones := clusterDeadZones(weakSamples, floorPlan, edges)
	recommendations := generateRecommendations(metric, deadZones, coverageScore, len(samples), edges)

	// Surface Wi-Fi anomalies (security/RF/standards) from the same passive AP
	// observations, alongside the coverage analysis.
	anomalies, err := analyzeAnomalies(points, observedAt, detector)
	if err != nil {
		return nil, fmt.Errorf("analyze survey anomalies: %w", err)
	}

	return &DeadZoneAnalysis{
		SurveyID:        surveyID,
		Metric:          string(metric),
		Threshold:       threshold,
		DeadZones:       deadZones,
		CoverageScore:   coverageScore,
		Recommendations: recommendationTexts(recommendations),
		Anomalies:       anomalies,
		advice:          recommendations,
	}, nil
}

// recommendationTexts is the wire form: the sentences without their urgency.
func recommendationTexts(recs []Recommendation) []string {
	texts := make([]string, 0, len(recs))
	for _, rec := range recs {
		texts = append(texts, rec.Text)
	}
	return texts
}

// DetectDeadZones provides dead zone detection through the survey manager,
// using the manager's configured [AnomalyDetector] (nil if none was wired).
func (m *Manager) DetectDeadZones(
	surveyID string,
	metric HeatmapType,
	threshold int,
) (*DeadZoneAnalysis, error) {
	survey, err := m.GetSurvey(surveyID)
	if err != nil {
		return nil, err
	}

	return DetectDeadZones(survey, metric, threshold, m.anomalyDetector)
}

// filterWeakSamples returns samples with RSSI below the threshold.
func filterWeakSamples(samples []SampleValue, threshold float64) []SampleValue {
	weak := make([]SampleValue, 0)
	for _, sample := range samples {
		if sample.Value < threshold {
			weak = append(weak, sample)
		}
	}
	return weak
}

// calculateCoverageScore computes the percentage of samples with acceptable signal.
func calculateCoverageScore(allSamples, weakSamples []SampleValue) float64 {
	if len(allSamples) == 0 {
		return 0.0
	}

	goodSamples := len(allSamples) - len(weakSamples)
	return (float64(goodSamples) / float64(len(allSamples))) * deadZonePercentMultiplier
}

// clusterDeadZones groups nearby weak samples into dead zones using distance-based clustering.
func clusterDeadZones(weakSamples []SampleValue, floorPlan *FloorPlan, edges bandEdges) []DeadZone {
	if len(weakSamples) == 0 {
		return []DeadZone{}
	}

	// Track which samples have been clustered
	clustered := make([]bool, len(weakSamples))
	deadZones := make([]DeadZone, 0)
	zoneID := 1

	for i, sample := range weakSamples {
		if clustered[i] {
			continue
		}

		// Start a new cluster
		cluster := []SampleValue{sample}
		clustered[i] = true

		// Find nearby weak samples
		for j := i + 1; j < len(weakSamples); j++ {
			if clustered[j] {
				continue
			}

			dist := distance(sample.Point, weakSamples[j].Point)
			if dist <= ClusterRadius {
				cluster = append(cluster, weakSamples[j])
				clustered[j] = true
			}
		}

		// Create dead zone from cluster
		deadZone := createDeadZone(cluster, zoneID, floorPlan, edges)
		deadZones = append(deadZones, deadZone)
		zoneID++
	}

	return deadZones
}

// createDeadZone creates a dead zone from a cluster of weak samples.
func createDeadZone(cluster []SampleValue, id int, floorPlan *FloorPlan, edges bandEdges) DeadZone {
	// Calculate center point
	var sumX, sumY, sumValue float64
	minValue := math.MaxFloat64

	for _, sample := range cluster {
		sumX += sample.Point.X
		sumY += sample.Point.Y
		sumValue += sample.Value
		if sample.Value < minValue {
			minValue = sample.Value
		}
	}

	centerX := sumX / float64(len(cluster))
	centerY := sumY / float64(len(cluster))
	avgValue := sumValue / float64(len(cluster))

	// Calculate radius (maximum distance from center to any point in cluster)
	var maxDist float64
	center := Point2D{X: centerX, Y: centerY}
	for _, sample := range cluster {
		dist := distance(center, sample.Point)
		if dist > maxDist {
			maxDist = dist
		}
	}

	// Convert radius to meters if floor plan scale is available
	radiusM := maxDist
	if floorPlan != nil && floorPlan.ScaleM > 0 {
		radiusM = maxDist * floorPlan.ScaleM
	}

	return DeadZone{
		ID:          fmt.Sprintf("zone-%d", id),
		Center:      center,
		RadiusM:     radiusM,
		Min:         int(minValue),
		Avg:         int(avgValue),
		SampleCount: len(cluster),
		Severity:    determineSeverity(avgValue, edges),
	}
}

// bandEdges are the severity boundaries one threshold implies, in that
// threshold's own unit.
type bandEdges struct {
	severe    int
	moderate  int
	threshold int
}

// edgesFor derives the bands from the only level the operator chose.
func edgesFor(threshold int) bandEdges {
	return bandEdges{
		severe:    threshold - severeBandOffset,
		moderate:  threshold - moderateBandOffset,
		threshold: threshold,
	}
}

// determineSeverity classifies a zone from its average against the bands its
// own metric's threshold implies. A signal level and a noise margin are not
// comparable numbers, so neither metric may read the other's threshold.
func determineSeverity(avg float64, edges bandEdges) string {
	switch {
	case avg <= float64(edges.severe):
		return SeveritySevere
	case avg <= float64(edges.moderate):
		return SeverityModerate
	default:
		return SeverityMinor
	}
}

// coverageAdvice is one metric's vocabulary for the findings panel. The shape
// of the advice is the same for both — a verdict on the score, then a sentence
// per severity band — but the sentences are not translations of each other: a
// coverage gap is answered with another access point and a noise problem is
// made worse by one.
type coverageAdvice struct {
	critical  string
	poor      string
	moderate  string
	good      string
	excellent string
	// Each takes the zone count.
	severeZones   string
	moderateZones string
	minorZones    string
	clean         string
}

var rssiAdvice = coverageAdvice{
	critical:  "Critical coverage issues detected. Consider a complete Wi-Fi infrastructure redesign with additional access points.",
	poor:      "Poor overall coverage. Add 2-3 additional access points in strategic locations.",
	moderate:  "Moderate coverage. Consider adding 1-2 access points to improve coverage in weak areas.",
	good:      "Good coverage overall. Minor improvements may be beneficial in identified weak spots.",
	excellent: "Excellent coverage. Maintain current access point placement and configuration.",
	severeZones: "Found %d severe dead zone(s) with %s. " +
		"Prioritize these areas for immediate AP placement.",
	moderateZones: "Found %d moderate dead zone(s) with %s. " +
		"These areas need attention to ensure reliable connectivity.",
	minorZones: "Found %d minor weak area(s) with %s. " +
		"Monitor these areas during peak usage times.",
	clean: "No significant dead zones detected. Wi-Fi coverage meets quality standards.",
}

var snrAdvice = coverageAdvice{
	critical: "Critical noise problem: most of this floor has too little margin above the noise floor to hold a link. " +
		"Find the source before adding radios — another access point on a noisy channel makes it worse.",
	poor: "Poor signal-to-noise over much of the floor. Sweep the band for non-Wi-Fi emitters " +
		"and re-plan the channel assignment before changing AP placement.",
	moderate: "Moderate signal-to-noise. Check for co-channel neighbours and move the affected radios " +
		"onto a quieter channel.",
	good:      "Good signal-to-noise. A few areas sit close to the margin; re-check them under load.",
	excellent: "Excellent signal-to-noise. The noise floor stays well clear of the signal across this floor.",
	severeZones: "Found %d area(s) with %s. " +
		"A link this close to the noise floor drops even where the signal is strong — locate the interferer.",
	moderateZones: "Found %d area(s) with %s. " +
		"Voice and video will suffer here and data will retry; identify what shares the channel.",
	minorZones: "Found %d area(s) with %s. " +
		"Watch these during the hours the interference appears.",
	clean: "No significant noise problems detected. The signal-to-noise margin holds across this floor.",
}

// band describes one severity's measured range in the metric's own unit. The
// sentences take it as a parameter rather than stating a range of their own,
// which is how they used to name a band no sample could be in.
func (e bandEdges) band(metric HeatmapType, severity string) string {
	noun, unit := "signal", "dBm"
	if metric == HeatmapSNR {
		noun, unit = "a margin", "dB"
	}

	switch severity {
	case SeveritySevere:
		return fmt.Sprintf("%s at or below %d %s", noun, e.severe, unit)
	case SeverityModerate:
		return fmt.Sprintf("%s between %d and %d %s", noun, e.severe, e.moderate, unit)
	default:
		return fmt.Sprintf("%s between %d and %d %s", noun, e.moderate, e.threshold, unit)
	}
}

func adviceFor(metric HeatmapType) coverageAdvice {
	if metric == HeatmapSNR {
		return snrAdvice
	}
	return rssiAdvice
}

// generateRecommendations provides actionable recommendations based on analysis results.
func generateRecommendations(
	metric HeatmapType,
	deadZones []DeadZone,
	coverageScore float64,
	totalSamples int,
	edges bandEdges,
) []Recommendation {
	advice := adviceFor(metric)
	recommendations := make([]Recommendation, 0)

	// Coverage-based recommendations
	switch {
	case coverageScore < deadZoneThresholdCritical:
		recommendations = append(recommendations, Recommendation{advice.critical, PriorityHigh})
	case coverageScore < deadZoneThresholdPoor:
		recommendations = append(recommendations, Recommendation{advice.poor, PriorityHigh})
	case coverageScore < deadZoneThresholdModerate:
		recommendations = append(recommendations, Recommendation{advice.moderate, PriorityMedium})
	case coverageScore < deadZoneThresholdGood:
		recommendations = append(recommendations, Recommendation{advice.good, PriorityLow})
	default:
		recommendations = append(recommendations, Recommendation{advice.excellent, PriorityLow})
	}

	// Dead zone-specific recommendations
	severeCount := 0
	moderateCount := 0
	minorCount := 0

	for _, zone := range deadZones {
		switch zone.Severity {
		case SeveritySevere:
			severeCount++
		case SeverityModerate:
			moderateCount++
		case SeverityMinor:
			minorCount++
		}
	}

	if severeCount > 0 {
		recommendations = append(recommendations, Recommendation{
			fmt.Sprintf(advice.severeZones, severeCount, edges.band(metric, SeveritySevere)),
			PriorityHigh,
		})
	}

	if moderateCount > 0 {
		recommendations = append(recommendations, Recommendation{
			fmt.Sprintf(advice.moderateZones, moderateCount, edges.band(metric, SeverityModerate)),
			PriorityMedium,
		})
	}

	if minorCount > 0 {
		recommendations = append(recommendations, Recommendation{
			fmt.Sprintf(advice.minorZones, minorCount, edges.band(metric, SeverityMinor)),
			PriorityLow,
		})
	}

	// Sample density recommendations
	if totalSamples < deadZoneMinSamples {
		recommendations = append(recommendations, Recommendation{
			"Limited sample data. Collect more samples for accurate analysis, especially in edge areas.",
			PriorityLow,
		})
	}

	// No dead zones found
	if len(deadZones) == 0 && coverageScore >= deadZoneThresholdClean {
		recommendations = append(recommendations, Recommendation{advice.clean, PriorityLow})
	}

	return recommendations
}
