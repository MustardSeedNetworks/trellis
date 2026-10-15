// SPDX-License-Identifier: BUSL-1.1

package linkliveoracle_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/internal/linkliveoracle"
)

func TestSurveyorNamed(t *testing.T) {
	for label, want := range map[string]bool{
		"WA-EVERETT-AP7_":        true,
		"10.24.23.87":            true,
		"RuckusWi:58b633-c652d9": false,
		"":                       false,
	} {
		if got := (linkliveoracle.Placement{Label: label}).SurveyorNamed(); got != want {
			t.Errorf("SurveyorNamed(%q) = %v, want %v", label, got, want)
		}
	}
}

func TestNormalizeBSSID(t *testing.T) {
	for in, want := range map[string]string{
		"RuckusWi:58b633-c652d9": "58b633c652d9",
		"AA:BB:CC:00:00:01":      "aabbcc000001",
		"7649820fe0d0":           "7649820fe0d0",
	} {
		if got := linkliveoracle.NormalizeBSSID(in); got != want {
			t.Errorf("NormalizeBSSID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCarrierMHz(t *testing.T) {
	for _, tc := range []struct {
		band, channel string
		want          int
	}{
		{"2.4 GHz", "6", 2437},
		{"2.4 GHz", "14", 2484},
		{"5 GHz", "36", 5180},
		{"6 GHz", `"97E"`, 6435},
		{"5 GHz", `""`, 0},
		{"Bluetooth", "2", 0},
	} {
		o := linkliveoracle.Observed{Band: tc.band, Channel: json.RawMessage(tc.channel)}
		if got := o.CarrierMHz(); got != tc.want {
			t.Errorf("CarrierMHz(%s, %s) = %d, want %d", tc.band, tc.channel, got, tc.want)
		}
	}
}

func TestLoadReadsFetchedSurveysOnly(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, body []byte, gzipped bool) {
		t.Helper()
		if gzipped {
			var buf bytes.Buffer
			zw := gzip.NewWriter(&buf)
			if _, err := zw.Write(body); err != nil {
				t.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			body = buf.Bytes()
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("heatmap.json", []byte(`[
		{"_id":"a1","fileName":"object","floorPlanScalePpf":10,"floorPlanWidthPx":800,"floorPlanHeightPx":600},
		{"_id":"b2","fileName":"bare array"},
		{"_id":"c3","fileName":"not fetched"}]`), false)
	write("a1.json.gz", []byte(`{"points":[{"x":1,"y":2,"aps":[{"bssid":"aa","signal":-50}]}],
		"apLocations":[{"x":5,"y":6,"label":"AP1"}]}`), true)
	write("b2.json.gz", []byte(`[{"x":3,"y":4,"aps":[]}]`), true)

	surveys, err := linkliveoracle.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(surveys) != 2 {
		t.Fatalf("loaded %d surveys, want the two fetched", len(surveys))
	}
	a, b := surveys[0], surveys[1]
	if len(a.Points) != 1 || *a.Points[0].APs[0].Signal != -50 || len(a.APLocations) != 1 || a.WidthPx != 800 {
		t.Errorf("object record = %+v", a)
	}
	if math.Abs(a.MetresPerPixel()-0.03048) > 1e-12 || b.MetresPerPixel() != 0 {
		t.Errorf("scales = %v, %v; want 0.03048 and 0", a.MetresPerPixel(), b.MetresPerPixel())
	}
	if len(b.Points) != 1 || b.Points[0].X != 3 {
		t.Errorf("bare-array record = %+v", b)
	}
}

func TestLoadRefusesADirectoryWithNothingFetched(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "heatmap.json"), []byte(`[{"_id":"x"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := linkliveoracle.Load(dir); err == nil {
		t.Error("Load found surveys in a directory with none fetched")
	}
}

func TestDir(t *testing.T) {
	t.Setenv(linkliveoracle.Env, "")
	if _, ok := linkliveoracle.Dir(); ok {
		t.Error("Dir reported a directory with the variable unset")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	t.Setenv(linkliveoracle.Env, "~/oracle")
	if got, _ := linkliveoracle.Dir(); got != filepath.Join(home, "oracle") {
		t.Errorf("Dir = %q, want ~ expanded", got)
	}
}
