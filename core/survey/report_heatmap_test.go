package survey_test

// report_heatmap_test.go pins the coverage map into the PDF report.
//
// A site survey report is read for the map; the statistics are its caption.
// The report generator carried an IncludeHeatmaps option that was honoured by
// printing the sentence "Heatmap visualization available in the web
// interface." where the map belonged, so a report could look complete, pass
// every existing test, and still be the one thing a surveyor cannot hand to a
// customer.
//
// The assertions are on the PDF's own structure rather than on its size alone,
// because a report that merely grew proves nothing about what it grew by.

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// imageXObject matches a PDF image XObject declaration, whose spacing fpdf may
// vary. Note that a bare "/Image" appears in the procset of a PDF with no
// images at all, so matching that alone would pass on an empty report.
var imageXObject = regexp.MustCompile(`/Subtype\s*/Image`)

func surveyWithPlan(t *testing.T, name string) *survey.Manager {
	t.Helper()
	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey(name, "report map", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	floor := svy.GetActiveFloor()
	if floor == nil {
		t.Fatal("no active floor")
	}
	floor.FloorPlan = &survey.FloorPlan{
		ImageData: solidPlanDataURI(t, planWhite, 120, 90),
		Width:     120, Height: 90, ScaleM: 0.05,
	}
	now := time.Now().UTC()
	for i, pt := range [][2]int{{15, 15}, {60, 45}, {100, 70}} {
		nets := []*wifi.ScannedNetwork{{
			SSID: "ap", BSSID: "00:00:00:00:00:01",
			Signal: -45 - i*12, Channel: 36, Frequency: 5180, LastSeen: now,
		}}
		if err := mgr.AddSample(svy.ID, pt[0], pt[1], &survey.PassiveSample{Networks: nets}); err != nil {
			t.Fatalf("AddSample %d: %v", i, err)
		}
	}
	t.Cleanup(func() {})
	return mgr
}

func onlySurveyID(t *testing.T, mgr *survey.Manager) string {
	t.Helper()
	all := mgr.ListSurveys()
	if len(all) != 1 {
		t.Fatalf("surveys = %d, want 1", len(all))
	}
	return all[0].ID
}

func TestReportEmbedsTheCoverageMap(t *testing.T) {
	mgr := surveyWithPlan(t, "Mapped")
	id := onlySurveyID(t, mgr)

	withMap, err := mgr.GenerateReport(id, survey.DefaultReportOptions())
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}
	if !bytes.HasPrefix(withMap, []byte("%PDF-")) {
		t.Fatalf("not a PDF: %q", withMap[:min(8, len(withMap))])
	}
	if !imageXObject.Match(withMap) {
		t.Error("the report contains no image — the coverage map is missing")
	}

	// The option has to mean something. Turning it off is the control: same
	// survey, same statistics, no map.
	opts := survey.DefaultReportOptions()
	opts.IncludeHeatmaps = false
	withoutMap, err := mgr.GenerateReport(id, opts)
	if err != nil {
		t.Fatalf("GenerateReport(IncludeHeatmaps=false): %v", err)
	}
	if imageXObject.Match(withoutMap) {
		t.Error("IncludeHeatmaps=false still produced an image")
	}
	if len(withMap) <= len(withoutMap) {
		t.Errorf("report with the map (%d bytes) is not larger than without it (%d)", len(withMap), len(withoutMap))
	}

	// The placeholder this replaces must be gone, or a reader is told to go
	// look elsewhere while the map sits on the next page.
	if bytes.Contains(withMap, []byte("available in the web interface")) {
		t.Error("the report still refers the reader to the web interface for the map")
	}
}

func TestReportSurvivesAFloorItCannotMap(t *testing.T) {
	// No plan means no canvas to draw on. The measurements are still worth
	// reporting, so this must produce a PDF rather than an error.
	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey("Planless", "no floor plan", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	nets := []*wifi.ScannedNetwork{{SSID: "ap", BSSID: "00:00:00:00:00:01", Signal: -50, Channel: 36, Frequency: 5180}}
	if err := mgr.AddSample(svy.ID, 5, 5, &survey.PassiveSample{Networks: nets}); err != nil {
		t.Fatalf("AddSample: %v", err)
	}
	pdf, err := mgr.GenerateReport(svy.ID, survey.DefaultReportOptions())
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("not a PDF")
	}
}
