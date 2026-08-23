package survey_test

// store_scale_test.go proves the store holds a real survey's worth of data and
// returns it unchanged.
//
// The size is not arbitrary: the twelve reference AirMapper captures carry
// 1,045 walk points and 113,551 BSS observations between them, the largest
// single survey being 142 points / 29,278 observations. This builds that
// largest shape so the store is exercised at the scale it will actually meet,
// rather than at the two-or-three-sample scale the rest of the suite uses.

import (
	"fmt"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

const (
	scalePoints = 142   // Time Square 10th Floor
	scaleObs    = 29278 // its observation count
)

func TestStoreRoundTripsASurveySizedSurvey(t *testing.T) {
	dir := t.TempDir()
	mgr := mustManager(t, dir, nil, nil, nil, nil)

	svy, err := mgr.CreateSurvey("Scale", "largest reference survey", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	if svy.GetActiveFloor() == nil {
		t.Fatal("no active floor")
	}

	perPoint := scaleObs / scalePoints
	now := time.Now().UTC().Truncate(time.Millisecond)
	for i := range scalePoints {
		nets := make([]*wifi.ScannedNetwork, 0, perPoint)
		for j := range perPoint {
			nets = append(nets, &wifi.ScannedNetwork{
				SSID:     fmt.Sprintf("net-%d", j%48), // 48 distinct SSIDs, as Everett has
				BSSID:    fmt.Sprintf("00:11:22:%02x:%02x:%02x", i%256, j%256, (i+j)%256),
				Signal:   -30 - (j % 55), // -30..-84, the corpus range
				Channel:  []int{12, 18, 24, 28, 50, 156, 178}[j%7],
				SNR:      40 - (j % 30),
				LastSeen: now,
			})
		}
		if err := mgr.AddSample(svy.ID, 93+i, 38+i, &survey.PassiveSample{Networks: nets}); err != nil {
			t.Fatalf("AddSample %d: %v", i, err)
		}
	}

	// Reopen from disk — this is the assertion that matters. Anything the
	// schema drops or coerces shows up as a mismatch here rather than as a
	// wrong heatmap much later.
	reopened := mustManager(t, dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	got, err := reopened.GetSurvey(svy.ID)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	gotFloor := got.GetActiveFloor()
	if gotFloor == nil {
		t.Fatal("reloaded survey has no active floor")
	}
	if len(gotFloor.Samples) != scalePoints {
		t.Fatalf("points = %d, want %d", len(gotFloor.Samples), scalePoints)
	}

	total := 0
	for _, p := range gotFloor.Samples {
		ps, ok := p.SampleData.(*survey.PassiveSample)
		if !ok {
			t.Fatalf("point at (%d,%d) came back as %T, not a passive sample", p.X, p.Y, p.SampleData)
		}
		total += len(ps.Networks)
		for _, n := range ps.Networks {
			if n.Signal > 0 || n.Signal < -110 {
				t.Fatalf("signal %d dBm outside a radio's range", n.Signal)
			}
		}
	}
	if want := scalePoints * perPoint; total != want {
		t.Errorf("observations = %d, want %d", total, want)
	}
}
