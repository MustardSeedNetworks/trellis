// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

// utf16le encodes s the way AirMagnet writes an .svd: a UTF-16 little-endian
// byte-order mark, then UTF-16LE code units, with CRLF line endings.
func utf16le(t *testing.T, s string) []byte {
	t.Helper()
	units := utf16.Encode([]rune(strings.ReplaceAll(s, "\n", "\r\n")))
	out := []byte{0xff, 0xfe}
	for _, u := range units {
		out = binary.LittleEndian.AppendUint16(out, u)
	}
	return out
}

// svdPassiveExport is a passive export as AirMagnet Survey Pro 8.0 writes it,
// trimmed to the columns the importer reads: two positions, three
// observations. It lives in testdata rather than in a Go literal because
// gosec reads "#Type: passive" as a hardcoded credential — "pass" is in its
// pattern — and a test fixture is not worth a suppression comment.
//
// The file is stored as UTF-8 with LF endings; utf16le above converts it to
// the UTF-16LE and CRLF that AirMagnet actually writes, so the encoding under
// test is produced by the test rather than committed as opaque bytes.
func svdPassiveExport(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "airmagnet-passive.svd.txt"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(body)
}

func TestParseAirMagnetReadsAPassiveSurvey(t *testing.T) {
	file, err := survey.ParseAirMagnetSVD(utf16le(t, svdPassiveExport(t)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if file.Type != "passive" {
		t.Errorf("type = %q, want passive", file.Type)
	}
	if file.AppVersion != "8.0" {
		t.Errorf("app version = %q, want 8.0", file.AppVersion)
	}
	if file.Width != 292 || file.Height != 268 {
		t.Errorf("dimensions = %dx%d, want 292x268", file.Width, file.Height)
	}

	// Two positions, because the first two rows share a timestamp and a
	// position: one stop that heard two BSSes, not two stops.
	if len(file.Points) != 2 {
		t.Fatalf("points = %d, want 2", len(file.Points))
	}

	first := file.Points[0]
	if first.X != 246 || first.Y != 43 {
		t.Errorf("first point at (%d,%d), want (246,43)", first.X, first.Y)
	}
	if len(first.Networks) != 2 {
		t.Fatalf("first point heard %d networks, want 2", len(first.Networks))
	}
	if got := first.Networks[0]; got.SSID != "cisco-3500" ||
		got.BSSID != "00:14:F1:AF:1B:96" || got.Signal != -48 ||
		got.NoiseFloor != -90 || got.Channel != 1 {
		t.Errorf("first network = %+v", got)
	}
	if got := first.Networks[0].SNR; got != 42 {
		t.Errorf("SNR = %d, want 42 (-48 less -90)", got)
	}
	// 5 GHz channel 36 must not be read back as 2.4 GHz.
	if got := first.Networks[1].Frequency; got != 5180 {
		t.Errorf("channel 36 frequency = %d, want 5180", got)
	}

	// An SSID containing a comma survives the split, because the fields are
	// quoted and a naive comma split would cut this name in half.
	if got := file.Points[1].Networks[0].SSID; got != "guest, wifi" {
		t.Errorf("second point SSID = %q, want %q", got, "guest, wifi")
	}
}

func TestParseAirMagnetRefusesWhatItCannotRead(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"not an AirMagnet file": {
			body: "Xpos,Ypos\n1,2\n",
			want: "not an AirMagnet survey export",
		},
		"a planner simulation rather than a walk": {
			body: strings.Replace(svdPassiveExport(t), "#Type: passive", "#Type: virtual", 1),
			want: "virtual",
		},
		"a layout this build does not describe": {
			body: strings.Replace(svdPassiveExport(t),
				"#Time,Xpos,Ypos,Channel,SSID,AP,SignalDBM", "#Time,Channel,SSID,AP,SignalDBM", 1),
			want: "Xpos",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := survey.ParseAirMagnetSVD(utf16le(t, tc.body))
			if err == nil {
				t.Fatal("want an error, got none")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

func TestParseAirMagnetBoundsTheInput(t *testing.T) {
	huge := make([]byte, survey.AirMagnetMaxBytes+1)
	copy(huge, []byte{0xff, 0xfe})
	if _, err := survey.ParseAirMagnetSVD(huge); err == nil ||
		!strings.Contains(err.Error(), "too large") {
		t.Fatalf("oversized input error = %v, want a size refusal", err)
	}
}

// svdWithSections is a voice survey: measurements, then a roaming-event log,
// then a phone list, then the AP placements. It is trimmed from a real export
// and exists because the sections are what a one-section reading gets wrong.
func svdWithSections(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "airmagnet-sections.svd.txt"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(body)
}

// TestParseAirMagnetReadsSectionsByRowPrefix pins the rule that tells a
// measurement from everything else in the file.
//
// Rows carry a one-character prefix: none for a measurement, "$" for an AP
// placement, "%" for a roaming event. Reading them positionally instead put an
// event's timestamp in the X column — the store's coordinate constraint refused
// it and took the whole import down — and shifted every AP row by one field, so
// every placement in the reference corpus parsed to nothing and was stored as
// an empty list without complaint.
func TestParseAirMagnetReadsSectionsByRowPrefix(t *testing.T) {
	file, err := survey.ParseAirMagnetSVD(utf16le(t, svdWithSections(t)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(file.Points) != 2 {
		t.Fatalf("points = %d, want 2 — the event and phone rows are not measurements", len(file.Points))
	}
	for _, p := range file.Points {
		if p.X > 1000 {
			t.Errorf("point at (%d,%d): a timestamp was read as a position", p.X, p.Y)
		}
	}

	if len(file.APs) != 2 {
		t.Fatalf("AP placements = %d, want 2", len(file.APs))
	}
	first := file.APs[0]
	if first.X != 151 || first.Y != 103 {
		t.Errorf("first AP at (%d,%d), want (151,103) — the row prefix took a column", first.X, first.Y)
	}
	if second := file.APs[1]; second.Name != "ap-two" || second.BSSID != "00:0F:34:A7:78:1F" {
		t.Errorf("second AP = %+v", second)
	}
}
