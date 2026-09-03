package survey

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// DeadZone represents a detected area of poor WiFi coverage.
type DeadZone struct {
	ID          string  `json:"id"`
	Center      Point2D `json:"center"`
	RadiusM     float64 `json:"radius_m"`
	MinRSSI     int     `json:"min_rssi"`
	AvgRSSI     int     `json:"avg_rssi"`
	SampleCount int     `json:"sample_count"`
	Severity    string  `json:"severity"` // "minor", "moderate", "severe"
}

// DeadZoneAnalysis contains the results of dead zone detection analysis.
type DeadZoneAnalysis struct {
	SurveyID        string         `json:"survey_id"`
	ThresholdDBm    int            `json:"threshold_dbm"`
	DeadZones       []DeadZone     `json:"dead_zones"`
	CoverageScore   float64        `json:"coverage_score"` // 0-100
	Recommendations []string       `json:"recommendations"`
	Anomalies       []wifi.Anomaly `json:"anomalies"` // Wi-Fi anomalies detected from the survey's passive AP observations
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

// DefaultThreshold is the default threshold for dead zone detection.
const DefaultThreshold = -75

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

	// deadZoneMinSamples is the minimum number of samples needed for reliable dead zone analysis.
	deadZoneMinSamples = 20
)

// DetectDeadZones analyzes a survey and identifies areas with poor WiFi coverage.
//
// The analysis:
//  1. Extracts RSSI values from all survey samples
//  2. Identifies samples below the threshold (default: -75 dBm)
//  3. Groups nearby weak samples into dead zones using distance-based clustering
//  4. Calculates severity based on RSSI levels: minor (-75 to -80), moderate (-80 to -85), severe (< -85)
//  5. Computes overall coverage score (percentage of samples above threshold)
//  6. Generates recommendations based on findings
//
// Parameters:
//   - survey: The WiFi survey to analyze
//   - threshold: RSSI threshold in dBm for identifying weak signals (default: -75)
//
// detector runs the Wi-Fi anomaly rule set over the survey's passive AP
// observations; nil yields no anomalies (see [AnomalyDetector]).
//
// Returns an analysis result with dead zones, coverage score, and recommendations.
func DetectDeadZones(survey *Survey, threshold int, detector AnomalyDetector) (*DeadZoneAnalysis, error) {
	if survey == nil {
		return nil, errors.New("survey is nil")
	}

	points := survey.GetAllSamples()
	if len(points) == 0 {
		return nil, errors.New("survey has no samples")
	}

	return analyzeCoverage(survey.ID, points, floorPlanOf(survey), survey.UpdatedAt, threshold, detector)
}

// DetectFloorDeadZones analyses one floor from that floor's own measurements
// and plan, the counterpart of [GenerateFloorHeatmap]. A survey-wide analysis
// scores every floor's samples together and clusters them on whichever plan is
// active, which is only right when there is one floor.
func DetectFloorDeadZones(
	surveyID string,
	floor *Floor,
	threshold int,
	detector AnomalyDetector,
) (*DeadZoneAnalysis, error) {
	if floor == nil {
		return nil, errors.New("floor is nil")
	}
	if len(floor.Samples) == 0 {
		return nil, errors.New("floor has no samples")
	}

	return analyzeCoverage(surveyID, floor.Samples, floor.FloorPlan, floor.UpdatedAt, threshold, detector)
}

// analyzeCoverage is the analysis itself, once a caller has decided which
// measurements and which plan it means.
func analyzeCoverage(
	surveyID string,
	points []*SamplePoint,
	floorPlan *FloorPlan,
	observedAt time.Time,
	threshold int,
	detector AnomalyDetector,
) (*DeadZoneAnalysis, error) {
	// Apply default threshold if not specified
	if threshold == 0 {
		threshold = DefaultThreshold
	}

	samples := extractSamples(points, "rssi")
	if len(samples) == 0 {
		return nil, errors.New("no RSSI samples found in survey")
	}

	weakSamples := filterWeakSamples(samples, float64(threshold))
	coverageScore := calculateCoverageScore(samples, weakSamples)
	deadZones := clusterDeadZones(weakSamples, floorPlan)
	recommendations := generateRecommendations(deadZones, coverageScore, len(samples))

	// Surface Wi-Fi anomalies (security/RF/standards) from the same passive AP
	// observations, alongside the coverage analysis.
	anomalies, err := analyzeAnomalies(points, observedAt, detector)
	if err != nil {
		return nil, fmt.Errorf("analyze survey anomalies: %w", err)
	}

	return &DeadZoneAnalysis{
		SurveyID:        surveyID,
		ThresholdDBm:    threshold,
		DeadZones:       deadZones,
		CoverageScore:   coverageScore,
		Recommendations: recommendations,
		Anomalies:       anomalies,
	}, nil
}

// DetectDeadZones provides dead zone detection through the survey manager,
// using the manager's configured [AnomalyDetector] (nil if none was wired).
func (m *Manager) DetectDeadZones(surveyID string, threshold int) (*DeadZoneAnalysis, error) {
	survey, err := m.GetSurvey(surveyID)
	if err != nil {
		return nil, err
	}

	return DetectDeadZones(survey, threshold, m.anomalyDetector)
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
func clusterDeadZones(weakSamples []SampleValue, floorPlan *FloorPlan) []DeadZone {
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
		deadZone := createDeadZone(cluster, zoneID, floorPlan)
		deadZones = append(deadZones, deadZone)
		zoneID++
	}

	return deadZones
}

// createDeadZone creates a dead zone from a cluster of weak samples.
func createDeadZone(cluster []SampleValue, id int, floorPlan *FloorPlan) DeadZone {
	// Calculate center point
	var sumX, sumY, sumRSSI float64
	minRSSI := math.MaxFloat64
	maxRSSI := -math.MaxFloat64

	for _, sample := range cluster {
		sumX += sample.Point.X
		sumY += sample.Point.Y
		sumRSSI += sample.Value
		if sample.Value < minRSSI {
			minRSSI = sample.Value
		}
		if sample.Value > maxRSSI {
			maxRSSI = sample.Value
		}
	}

	centerX := sumX / float64(len(cluster))
	centerY := sumY / float64(len(cluster))
	avgRSSI := sumRSSI / float64(len(cluster))

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

	// Determine severity based on average RSSI
	severity := determineSeverity(avgRSSI)

	return DeadZone{
		ID:          fmt.Sprintf("zone-%d", id),
		Center:      center,
		RadiusM:     radiusM,
		MinRSSI:     int(minRSSI),
		AvgRSSI:     int(avgRSSI),
		SampleCount: len(cluster),
		Severity:    severity,
	}
}

// determineSeverity classifies dead zone severity based on average RSSI.
func determineSeverity(avgRSSI float64) string {
	switch {
	case avgRSSI <= -85:
		return SeveritySevere
	case avgRSSI <= -80:
		return SeverityModerate
	default:
		return SeverityMinor
	}
}

// generateRecommendations provides actionable recommendations based on analysis results.
func generateRecommendations(
	deadZones []DeadZone,
	coverageScore float64,
	totalSamples int,
) []string {
	recommendations := make([]string, 0)

	// Coverage-based recommendations
	switch {
	case coverageScore < deadZoneThresholdCritical:
		recommendations = append(
			recommendations,
			"Critical coverage issues detected. Consider a complete WiFi infrastructure redesign with additional access points.",
		)
	case coverageScore < deadZoneThresholdPoor:
		recommendations = append(
			recommendations,
			"Poor overall coverage. Add 2-3 additional access points in strategic locations.",
		)
	case coverageScore < deadZoneThresholdModerate:
		recommendations = append(
			recommendations,
			"Moderate coverage. Consider adding 1-2 access points to improve coverage in weak areas.",
		)
	case coverageScore < deadZoneThresholdGood:
		recommendations = append(
			recommendations,
			"Good coverage overall. Minor improvements may be beneficial in identified weak spots.",
		)
	default:
		recommendations = append(
			recommendations,
			"Excellent coverage. Maintain current access point placement and configuration.",
		)
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
		recommendations = append(
			recommendations,
			fmt.Sprintf(
				"Found %d severe dead zone(s) with signal below -85 dBm. Prioritize these areas for immediate AP placement.",
				severeCount,
			),
		)
	}

	if moderateCount > 0 {
		recommendations = append(
			recommendations,
			fmt.Sprintf(
				"Found %d moderate dead zone(s) with signal between -80 and -85 dBm. These areas need attention to ensure reliable connectivity.",
				moderateCount,
			),
		)
	}

	if minorCount > 0 {
		recommendations = append(
			recommendations,
			fmt.Sprintf(
				"Found %d minor weak area(s) with signal between -75 and -80 dBm. Monitor these areas during peak usage times.",
				minorCount,
			),
		)
	}

	// Sample density recommendations
	if totalSamples < deadZoneMinSamples {
		recommendations = append(
			recommendations,
			"Limited sample data. Collect more samples for accurate analysis, especially in edge areas.",
		)
	}

	// No dead zones found
	if len(deadZones) == 0 && coverageScore >= 90 {
		recommendations = append(
			recommendations,
			"No significant dead zones detected. WiFi coverage meets quality standards.",
		)
	}

	return recommendations
}
