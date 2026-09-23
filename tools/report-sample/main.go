// Command report-sample writes the documentation's synthetic coverage report to stdout.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func main() {
	data, err := sampleReport()
	if err == nil {
		_, err = os.Stdout.Write(data)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func sampleReport() ([]byte, error) {
	stamp := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	floor := &survey.Floor{ID: "sample-floor", Name: "Synthetic floor", Level: 1}
	// The report-map test uses these positions and readings. No customer data
	// or real building geometry is represented by this documentation fixture.
	for i, point := range [][2]int{{15, 15}, {60, 45}, {100, 70}} {
		sample := &survey.PassiveSample{Networks: []*wifi.ScannedNetwork{{
			SSID: "sample-ap", BSSID: "00:00:00:00:00:01",
			Signal: -45 - i*12, Channel: 36, Frequency: 5180, LastSeen: stamp,
		}}}
		sample.CalculateAggregations()
		floor.Samples = append(floor.Samples, &survey.SamplePoint{
			X: point[0], Y: point[1], Timestamp: stamp, SampleData: sample,
		})
	}
	s := &survey.Survey{
		ID: "coverage-ramp-sample", Name: "Coverage ramp acceptance sample",
		Description: "Synthetic readings; English UI; RSSI threshold -60 dBm.",
		Status:      survey.StatusCompleted, CreatedAt: stamp, UpdatedAt: stamp,
		Floors: []*survey.Floor{floor}, ActiveFloorID: floor.ID,
	}
	opts := survey.DefaultReportOptions()
	opts.Threshold = -60
	opts.IncludeExecutiveSummary = false
	opts.IncludeRecommendations = false
	return survey.NewReportGenerator(s, opts).Generate()
}
