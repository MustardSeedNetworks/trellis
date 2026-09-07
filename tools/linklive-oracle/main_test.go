// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// The two sides write the same address differently: Link-Live prefixes the
// vendor and splits the OUI from the NIC with a hyphen, the archive writes
// colon-separated octets. A comparison that missed this would report every
// BSSID as seen by only one side and call it a defect.
func TestNormalizeBSSID(t *testing.T) {
	for _, tc := range []struct {
		name  string
		given string
		want  string
	}{
		{"Link-Live form", "RuckusWi:58b633-c652d9", "58b633c652d9"},
		{"archive form", "58:B6:33:C6:52:D9", "58b633c652d9"},
		{"vendor name holding a hyphen", "Link-Sys:58b633-c652d9", "58b633c652d9"},
		{"neither form", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeBSSID(tc.given); got != tc.want {
				t.Errorf("normalizeBSSID(%q) = %q, want %q", tc.given, got, tc.want)
			}
		})
	}
}

// TestDiffCountsEveryDisagreement is the check on the checker. Each case
// perturbs exactly one member of an otherwise identical pair, so a comparison
// that silently stopped looking at that member would report agreement.
func TestDiffCountsEveryDisagreement(t *testing.T) {
	const bssid = "58:b6:33:c6:52:d9"
	base := func() *processed {
		return &processed{Points: []llPoint{{
			X: 1532, Y: 3980,
			APs: []llAP{{BSSID: "RuckusWi:58b633-c652d9", Signal: -76, Noise: -90, ChannelPrimary: 8}},
		}}}
	}
	trellis := func() []survey.SurveyPointRecord {
		return []survey.SurveyPointRecord{{
			X: 1532, Y: 3980,
			Networks: []*wifi.ScannedNetwork{
				{BSSID: bssid, Signal: -76, NoiseFloor: -90, Channel: 8},
			},
		}}
	}

	for _, tc := range []struct {
		name   string
		mutate func([]survey.SurveyPointRecord)
		agrees bool
	}{
		{"identical", func([]survey.SurveyPointRecord) {}, true},
		{"position", func(p []survey.SurveyPointRecord) { p[0].X = 1533 }, false},
		{"signal", func(p []survey.SurveyPointRecord) { p[0].Networks[0].Signal = -75 }, false},
		{"noise", func(p []survey.SurveyPointRecord) { p[0].Networks[0].NoiseFloor = -89 }, false},
		{"channel", func(p []survey.SurveyPointRecord) { p[0].Networks[0].Channel = 28 }, false},
		{"a BSSID only one side saw", func(p []survey.SurveyPointRecord) {
			p[0].Networks[0].BSSID = "00:00:5e:00:53:01"
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			points := trellis()
			tc.mutate(points)
			var got comparison
			diff(&got, base(), points, &survey.AirMapperImportResult{})
			if got.agrees() != tc.agrees {
				t.Fatalf("agrees() = %v, want %v (%+v)", got.agrees(), tc.agrees, got)
			}
		})
	}
}

// TestRenderReportTotals: the figures quoted in docs and PR bodies come from
// this row, so it is worth an assertion of its own.
func TestRenderReportTotals(t *testing.T) {
	report := renderReport([]comparison{
		{analysisID: "a", points: [2]int{73, 73}, signal: [2]int{4789, 4789}},
		{analysisID: "b", points: [2]int{36, 36}, signal: [2]int{2260, 2260}},
	}, []string{"c"})

	for _, want := range []string{"**2 surveys**", "109/109", "7049/7049", "matches 1 Link-Live records: c"} {
		if !strings.Contains(report, want) {
			t.Errorf("report does not contain %q:\n%s", want, report)
		}
	}
}

// TestReadProcessedAcceptsBothShapes: Link-Live's older exports are a bare
// array of points and the newer ones wrap that array alongside the AP
// placements. Both are served from the same field, and both are gzipped by S3
// some of the time and not others.
func TestReadProcessedAcceptsBothShapes(t *testing.T) {
	dir := t.TempDir()
	const wrapped = `{"points":[{"x":1,"y":2,"aps":[{"bssid":"a","signal":-70}]}],"apLocations":[{"label":"ap"}]}`
	const bare = `[{"x":1,"y":2,"aps":[{"bssid":"a","signal":-70}]}]`

	write := func(name, body string, compress bool) string {
		path := filepath.Join(dir, name)
		var buf bytes.Buffer
		if compress {
			gz := gzip.NewWriter(&buf)
			if _, err := gz.Write([]byte(body)); err != nil {
				t.Fatalf("gzip write: %v", err)
			}
			if err := gz.Close(); err != nil {
				t.Fatalf("gzip close: %v", err)
			}
		} else {
			buf.WriteString(body)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}

	for _, tc := range []struct {
		name       string
		path       string
		placements int
	}{
		{"wrapped, gzipped", write("wrapped.json.gz", wrapped, true), 1},
		{"wrapped, plain", write("wrapped.json", wrapped, false), 1},
		{"bare array", write("bare.json.gz", bare, false), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readProcessed(tc.path)
			if err != nil {
				t.Fatalf("readProcessed: %v", err)
			}
			if len(got.Points) != 1 {
				t.Fatalf("points = %d, want 1", len(got.Points))
			}
			if len(got.APLocations) != tc.placements {
				t.Errorf("placements = %d, want %d", len(got.APLocations), tc.placements)
			}
		})
	}
}

// TestReadCorpusSkipsWhatItCannotPair: a directory of surveys holds plenty
// that is not an archive, and an archive without `.serial` states neither a
// plan name nor a point count. Neither is an error; both must be skipped
// rather than pairing something arbitrary with a Link-Live record.
func TestReadCorpusSkipsWhatItCannotPair(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"notes.txt":      "not an archive",
		"broken.amp":     "PK\x03\x04 truncated",
		"nested/two.amp": "PK\x03\x04 truncated",
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	found, err := readCorpus(dir)
	if err != nil {
		t.Fatalf("readCorpus: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("paired %d archives, want none: %v", len(found), found)
	}

	if _, err := readCorpus(filepath.Join(dir, "absent")); err == nil {
		t.Error("a corpus directory that does not exist reported no error")
	}
}

func TestReadIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.json")
	body := `{"615ddd08":{"plan":"DIA Main Hall.jpg","pts":36,"unit":"EtherScopeXG","mode":"passive"}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	records, err := readIndex(path)
	if err != nil {
		t.Fatalf("readIndex: %v", err)
	}
	if got := records["615ddd08"]; got.FloorPlanFilename != "DIA Main Hall.jpg" || got.SurveyPointCount != 36 {
		t.Errorf("record = %+v, want the plan and point count that pair it", got)
	}
	if keys := sortedKeys(records); len(keys) != 1 || keys[0] != "615ddd08" {
		t.Errorf("sortedKeys = %v", keys)
	}
}

// varint and field write the two pieces of protobuf wire format a
// `.SurveyResult` fixture needs. Building the archive rather than committing
// one keeps the vendor's surveys out of a public repository and keeps the
// fixture readable.
func varint(v uint64) []byte {
	var out []byte
	for v >= 0x80 {
		out = append(out, byte(v)|0x80)
		v >>= 7
	}
	return append(out, byte(v))
}

func field(number uint64, wire uint64, payload []byte) []byte {
	out := varint(number<<3 | wire)
	if wire == 2 {
		out = append(out, varint(uint64(len(payload)))...)
	}
	return append(out, payload...)
}

func varintField(number, value uint64) []byte { return field(number, 0, varint(value)) }
func bytesField(number uint64, s string) []byte {
	return field(number, 2, []byte(s))
}

// buildAMP writes an archive holding one walk position with one observation,
// in the shape ParseAirMapperFile and ParseSurveyResult expect.
func buildAMP(t *testing.T, plan string, x, y, channel uint64) []byte {
	t.Helper()
	observation := field(40, 2, bytes.Join([][]byte{
		bytesField(1, "58:b6:33:c6:52:d9"),
		bytesField(3, "corp"),
		varintField(6, channel),
		varintField(21, ^uint64(76)+1),
		varintField(22, ^uint64(90)+1),
		varintField(24, 1788004800000),
	}, nil))
	point := field(40, 2, bytes.Join([][]byte{
		varintField(10, x),
		varintField(20, y),
		varintField(30, 1788004800000),
		observation,
	}, nil))

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	write := func(name string, body []byte) {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := f.Write(body); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	serial := fmt.Sprintf(`{"fileName":"walk","floorPlanFilename":%q,"floorPlanScalePpf":6.673,`+
		`"surveyPointCount":1,"apLocations":[{"x":10,"y":20,"label":"RuckusWi:58b633-c652d9"}]}`, plan)
	write("1920020.serial", []byte(serial))
	write("walk.SurveyResult", point)
	// The parser refuses an archive with no plan image, and reads the plan's
	// own name out of `.serial` rather than off this member.
	write("plan.png", planPNG(t))
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return archive.Bytes()
}

// planPNG is the smallest image the parser will accept as a floor plan.
func planPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// TestRunPairsAndCompares drives the whole harness the way the operator does,
// over a corpus and an index it builds itself: one survey that matches
// Link-Live's read and one that does not.
func TestRunPairsAndCompares(t *testing.T) {
	dir := t.TempDir()
	corpus := filepath.Join(dir, "corpus")
	if err := os.MkdirAll(corpus, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile := func(path string, body []byte) {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	llBody := `{"points":[{"x":100,"y":200,"aps":[` +
		`{"bssid":"RuckusWi:58b633-c652d9","signal":-76,"noise":-90,"channelPrimary":8}]}],` +
		`"apLocations":[{"label":"grouped"}]}`

	writeFile(filepath.Join(corpus, "agree.amp"), buildAMP(t, "agree.jpg", 100, 200, 8))
	writeFile(filepath.Join(corpus, "differ.amp"), buildAMP(t, "differ.jpg", 100, 200, 28))
	writeFile(filepath.Join(dir, "agree.json.gz"), []byte(llBody))
	writeFile(filepath.Join(dir, "differ.json.gz"), []byte(llBody))
	writeFile(filepath.Join(dir, "index.json"), []byte(
		`{"agree":{"plan":"agree.jpg","pts":1,"unit":"EtherScopeXG","mode":"passive"},`+
			`"differ":{"plan":"differ.jpg","pts":1,"unit":"EtherScopeXG","mode":"passive"},`+
			`"absent":{"plan":"nothing.jpg","pts":9}}`))

	report, agreed, err := run(filepath.Join(dir, "index.json"), dir, corpus)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if agreed {
		t.Error("agreed = true, but one survey disagrees on the channel")
	}
	for _, want := range []string{
		"| `agree` | EtherScopeXG | passive | 1/1 | 1/1 | 1/1 | 1/0 | 1/1 | 1/1 | 1/1 |",
		"| `differ` | EtherScopeXG | passive | 1/1 | 1/1 | 1/1 | 1/0 | 1/1 | 1/1 | 0/1 |",
		"matches 1 Link-Live records: absent",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report is missing %q:\n%s", want, report)
		}
	}

	// A corpus that pairs with nothing must not read as a pass.
	if _, _, err = run(filepath.Join(dir, "index.json"), dir, filepath.Join(dir, "empty")); err == nil {
		t.Error("a missing corpus directory reported no error")
	}
	empty := filepath.Join(dir, "empty")
	if err := os.MkdirAll(empty, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, agreed, err = run(filepath.Join(dir, "index.json"), dir, empty); err != nil || agreed {
		t.Errorf("empty corpus: agreed=%v err=%v, want agreed=false and no error", agreed, err)
	}
}
