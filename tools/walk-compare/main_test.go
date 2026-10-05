// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func floorWith(w, h int, scaleM float64, points ...*survey.SamplePoint) *survey.Floor {
	return &survey.Floor{
		FloorPlan: &survey.FloorPlan{Width: w, Height: h, ScaleM: scaleM},
		Samples:   points,
	}
}

func at(x, y int, nets ...*wifi.ScannedNetwork) *survey.SamplePoint {
	return &survey.SamplePoint{X: x, Y: y, SampleData: &survey.PassiveSample{Networks: nets}}
}

func bss(bssid string, mhz, dbm int) *wifi.ScannedNetwork {
	return &wifi.ScannedNetwork{BSSID: bssid, Frequency: mhz, Signal: dbm}
}

// At 0.1 m per pixel the pairing radius is 20 px.
func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		name       string
		ref, walk  *survey.Floor
		wantErr    string
		wantPaired int
		wantRows   []string
	}{
		{
			name: "offset adapter scores bias, not error",
			ref: floorWith(100, 100, 0.1,
				at(10, 10, bss("AA:00:00:00:00:01", 2437, -50), bss("aa:00:00:00:00:02", 5180, -60)),
				at(60, 60, bss("aa:00:00:00:00:01", 2437, -70))),
			walk: floorWith(100, 100, 0.1,
				at(12, 10, bss("aa:00:00:00:00:01", 2437, -44), bss("AA:00:00:00:00:02", 5180, -54)),
				at(60, 65, bss("aa:00:00:00:00:01", 2437, -64))),
			wantPaired: 2,
			wantRows: []string{
				"| 2.4 GHz | 2 | +6.00 | 6.00 | 0.00 | 0.00 | 0/2 (0%) |",
				"| 5 GHz | 1 | +6.00 | 6.00 | 0.00 | 0.00 | 0/1 (0%) |",
			},
		},
		{
			// A 2.4 GHz-only adapter hears nothing on 5 GHz: that is a miss
			// against a clear reading, and the band has no pairs to score.
			name: "missed clear AP counts, weak one does not",
			ref: floorWith(100, 100, 0.1,
				at(10, 10, bss("aa:00:00:00:00:01", 2437, -50), bss("aa:00:00:00:00:02", 5180, -60),
					bss("aa:00:00:00:00:03", 5200, -85))),
			walk:       floorWith(100, 100, 0.1, at(10, 10, bss("aa:00:00:00:00:01", 2437, -52))),
			wantPaired: 1,
			wantRows: []string{
				"| 2.4 GHz | 1 | -2.00 | 2.00 | 0.00 | 0.00 | 0/1 (0%) |",
				"| 5 GHz | 0 | - | - | - | - | 1/1 (100%) |",
			},
		},
		{
			name:       "point beyond the radius is not paired",
			ref:        floorWith(100, 100, 0.1, at(10, 10, bss("aa:00:00:00:00:01", 2437, -50))),
			walk:       floorWith(100, 100, 0.1, at(10, 31, bss("aa:00:00:00:00:01", 2437, -50))),
			wantPaired: 0,
		},
		{
			name:    "different images are refused",
			ref:     floorWith(100, 100, 0.1),
			walk:    floorWith(100, 120, 0.1),
			wantErr: "floor plans differ",
		},
		{
			name:    "uncalibrated reference is refused",
			ref:     floorWith(100, 100, 0),
			walk:    floorWith(100, 100, 0.1),
			wantErr: "carries no scale",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := compare(tc.ref, tc.walk)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("compare error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("compare: %v", err)
			}
			if got.pairedPoints != tc.wantPaired {
				t.Errorf("paired = %d, want %d", got.pairedPoints, tc.wantPaired)
			}
			report := got.String()
			for _, row := range tc.wantRows {
				if !strings.Contains(report, row) {
					t.Errorf("report is missing %q:\n%s", row, report)
				}
			}
		})
	}
}

// TestRunScoresAStoredWalkAgainstAnArchive drives the command the way the
// walk script does: an AirMapper archive on one side, a trellisd store on the
// other, both on the same 200x100 plan.
func TestRunScoresAStoredWalkAgainstAnArchive(t *testing.T) {
	dir := t.TempDir()
	ampPath := filepath.Join(dir, "floor.amp")
	if err := os.WriteFile(ampPath, buildAMP(t, 40, 50, 6, 60), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	store := filepath.Join(dir, "store")
	mgr, err := survey.NewManager(store, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	walk, err := mgr.CreateSurvey("T-KILL floor", "", "", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.UpdateFloorPlan(walk.ID, &survey.FloorPlan{Width: 200, Height: 100, ScaleM: 0.05}); err != nil {
		t.Fatalf("UpdateFloorPlan: %v", err)
	}
	if err := mgr.StartSurvey(walk.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	sample := &survey.PassiveSample{Networks: []*wifi.ScannedNetwork{bss("58:B6:33:C6:52:D9", 2437, -55)}}
	if err := mgr.AddSample(walk.ID, 42, 50, sample); err != nil {
		t.Fatalf("AddSample: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	report, err := run(ampPath, store, "T-KILL floor")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{
		"Trellis points: 1, paired with an AirMapper point within 2.0 m: 1",
		"| 2.4 GHz | 1 | +5.00 | 5.00 | 0.00 | 0.00 | 0/1 (0%) |",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report is missing %q:\n%s", want, report)
		}
	}

	if _, err := run(ampPath, store, "no such walk"); err == nil {
		t.Error("run found a survey that is not in the store")
	}
}

// buildAMP writes an archive holding one walk position with one observation
// of -minusDBm dBm on a 200x100 plan, in the shape ParseAirMapperFile and
// ParseSurveyResult expect (field numbers as tools/linklive-oracle's test
// builds them).
func buildAMP(t *testing.T, x, y, channel, minusDBm uint64) []byte {
	t.Helper()
	observation := field(40, 2, bytes.Join([][]byte{
		bytesField(1, "58:b6:33:c6:52:d9"),
		bytesField(3, "corp"),
		varintField(6, channel),
		varintField(21, ^minusDBm+1),
		varintField(22, ^uint64(90)+1),
		varintField(24, 1788004800000),
	}, nil))
	point := field(40, 2, bytes.Join([][]byte{
		varintField(10, x),
		varintField(20, y),
		varintField(30, 1788004800000),
		observation,
	}, nil))

	var plan bytes.Buffer
	if err := png.Encode(&plan, image.NewRGBA(image.Rect(0, 0, 200, 100))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	for name, body := range map[string][]byte{
		"1920020.serial": []byte(`{"fileName":"walk","floorPlanFilename":"plan.png",` +
			`"floorPlanScalePpf":6.673,"surveyPointCount":1}`),
		"walk.SurveyResult": point,
		"plan.png":          plan.Bytes(),
	} {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := f.Write(body); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return archive.Bytes()
}

func varint(v uint64) []byte {
	var out []byte
	for v >= 0x80 {
		out = append(out, byte(v)|0x80)
		v >>= 7
	}
	return append(out, byte(v))
}

func field(number, wire uint64, payload []byte) []byte {
	out := varint(number<<3 | wire)
	if wire == 2 {
		out = append(out, varint(uint64(len(payload)))...)
	}
	return append(out, payload...)
}

func varintField(number, value uint64) []byte { return field(number, 0, varint(value)) }

func bytesField(number uint64, s string) []byte { return field(number, 2, []byte(s)) }
