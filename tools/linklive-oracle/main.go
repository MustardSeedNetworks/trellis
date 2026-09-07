// SPDX-License-Identifier: BUSL-1.1

// Command linklive-oracle compares Trellis's read of an AirMapper archive with
// NetAlly's own read of the same archive.
//
// Link-Live keeps, for every survey uploaded to it, the decoded measurements
// its own pipeline produced. That makes it an oracle for this repo's AirMapper
// importer in the strict sense: same input, two independent analyzers, so a
// disagreement is a defect in one of them rather than a difference of opinion.
// The `.SurveyResult` member is protobuf with no schema shipped alongside it
// and its field numbers were recovered by inspection, which is exactly the
// kind of reading an oracle can check and a unit test cannot.
//
// It reads files, never the API: the fetch is scripts/linklive-fetch-oracle.sh,
// which holds the credentials. Usage:
//
//	linklive-oracle -index /tmp/ll-oracle/index.json \
//	    -processed /tmp/ll-oracle -corpus ~/AirMapper-Surveys
//
// Exit status is non-zero when any paired survey disagrees.
package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

// record is one Link-Live heatmap entry: the metadata Link-Live states about a
// survey it holds. Only the members this comparison needs are modelled.
type record struct {
	FileName          string `json:"fileName"`
	FloorPlanFilename string `json:"plan"`
	SurveyPointCount  int    `json:"pts"`
	UnitType          string `json:"unit"`
	SurveyMode        string `json:"mode"`
}

// processed is Link-Live's decode of the archive's measurements.
type processed struct {
	Points      []llPoint `json:"points"`
	APLocations []struct {
		Label string `json:"label"`
	} `json:"apLocations"`
}

// llPoint is one walk position as Link-Live decoded it.
type llPoint struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	APs []llAP  `json:"aps"`
}

// llAP is one BSS observed from a walk position.
type llAP struct {
	BSSID          string `json:"bssid"`
	Signal         int    `json:"signal"`
	Noise          int    `json:"noise"`
	ChannelPrimary int    `json:"channelPrimary"`
}

// comparison is one survey's result, in the order the report prints it.
type comparison struct {
	analysisID string
	unit       string
	mode       string
	points     [2]int // Link-Live, Trellis
	placements [2]int
	positions  [2]int // agreeing, compared
	bssids     [2]int // common, disjoint
	signal     [2]int // agreeing, compared
	noise      [2]int
	channel    [2]int
}

// agrees leaves placements out deliberately. Link-Live groups an AP's radios
// into one placement and lets an operator add placements in its own web UI
// after the upload, so its count is legitimately not the archive's; the
// numbers are reported side by side and read by a person.
func (c comparison) agrees() bool {
	return c.points[0] == c.points[1] &&
		c.positions[0] == c.positions[1] &&
		c.bssids[1] == 0 &&
		c.signal[0] == c.signal[1] &&
		c.noise[0] == c.noise[1] &&
		c.channel[0] == c.channel[1]
}

// add accumulates one survey into a totals row, so the figures quoted
// elsewhere come out of the tool rather than out of somebody's arithmetic.
func (c *comparison) add(other comparison) {
	for i := range 2 {
		c.points[i] += other.points[i]
		c.placements[i] += other.placements[i]
		c.positions[i] += other.positions[i]
		c.bssids[i] += other.bssids[i]
		c.signal[i] += other.signal[i]
		c.noise[i] += other.noise[i]
		c.channel[i] += other.channel[i]
	}
}

func main() {
	index := flag.String("index", "", "Link-Live heatmap index JSON (analysis id -> metadata)")
	dir := flag.String("processed", "", "directory of <analysis id>.json[.gz] decoded surveys")
	corpus := flag.String("corpus", "", "directory of .amp archives, searched recursively")
	out := flag.String("out", "", "write the markdown report here instead of stdout")
	flag.Parse()

	if *index == "" || *dir == "" || *corpus == "" {
		fmt.Fprintln(os.Stderr, "-index, -processed and -corpus are all required")
		os.Exit(2)
	}

	report, agreed, err := run(*index, *dir, *corpus)
	if err != nil {
		fatal(err)
	}
	if *out == "" {
		fmt.Print(report)
	} else if writeErr := os.WriteFile(*out, []byte(report), 0o600); writeErr != nil {
		fatal(writeErr)
	}
	if !agreed {
		os.Exit(1)
	}
}

// run pairs every Link-Live record with an archive and compares the two reads.
// It reports disagreement as a bool rather than an error: a survey the two
// sides read differently is the finding, not a failure of the comparison.
//
// A run that pairs nothing does not agree either. Silence from a comparison
// that compared nothing is the one result that must never read as a pass.
func run(indexPath, processedDir, corpusDir string) (report string, agreed bool, err error) {
	records, err := readIndex(indexPath)
	if err != nil {
		return "", false, err
	}
	archives, err := readCorpus(corpusDir)
	if err != nil {
		return "", false, err
	}

	var (
		results  []comparison
		unpaired []string
	)
	for _, id := range sortedKeys(records) {
		rec := records[id]
		amp, ok := archives[key{rec.FloorPlanFilename, rec.SurveyPointCount}]
		if !ok {
			unpaired = append(unpaired, id)
			continue
		}
		result, compareErr := compare(id, rec, filepath.Join(processedDir, id+".json.gz"), amp)
		if compareErr != nil {
			return "", false, fmt.Errorf("%s: %w", id, compareErr)
		}
		results = append(results, result)
	}

	agreed = len(results) > 0
	for _, r := range results {
		if !r.agrees() {
			agreed = false
		}
	}
	return renderReport(results, unpaired), agreed, nil
}

// key pairs a survey with its archive. The plan filename alone is not unique —
// several walks of one floor share it — and neither is the point count.
type key struct {
	plan   string
	points int
}

func readIndex(path string) (map[string]record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var records map[string]record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// readCorpus indexes every readable archive under root by plan and point
// count. An archive without `.serial` states neither, so it cannot be paired.
func readCorpus(root string) (map[key]string, error) {
	found := map[key]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".amp") {
			return nil
		}
		k, ok := archiveKey(path)
		if !ok {
			return nil
		}
		if _, seen := found[k]; !seen {
			found[k] = path
		}
		return nil
	})
	return found, err
}

// archiveKey reads an archive's plan name and point count. A file this parser
// refuses, or one carrying no `.serial`, states neither and cannot be paired
// with a Link-Live record; that is a property of the corpus, not a failure.
func archiveKey(path string) (key, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return key{}, false
	}
	file, err := survey.ParseAirMapperFile(data)
	if err != nil || file.Serial == nil {
		return key{}, false
	}
	plan := file.Serial.FloorPlanFilename
	if plan == "" {
		plan = file.FloorPlanFilename
	}
	return key{plan, file.Serial.SurveyPointCount}, true
}

func compare(id string, rec record, processedPath, ampPath string) (comparison, error) {
	result := comparison{analysisID: id, unit: rec.UnitType, mode: rec.SurveyMode}

	ll, err := readProcessed(processedPath)
	if err != nil {
		return result, err
	}
	data, err := os.ReadFile(ampPath)
	if err != nil {
		return result, err
	}
	file, err := survey.ParseAirMapperFile(data)
	if err != nil {
		return result, err
	}
	points, err := survey.ParseSurveyResult(file.SurveyResult)
	if err != nil {
		return result, err
	}
	imported, err := file.ToImportResult()
	if err != nil {
		return result, err
	}

	diff(&result, ll, points, imported)
	return result, nil
}

// diff fills in everything the two sides can be held to. Both preserve the
// archive's point order, so the index joins a walk position, and a position
// plus a BSSID joins one reading.
func diff(into *comparison, ll *processed, points []survey.SurveyPointRecord, imported *survey.AirMapperImportResult) {
	into.points = [2]int{len(ll.Points), len(points)}
	into.placements = [2]int{len(ll.APLocations), len(imported.APLocations)}

	type reading struct{ signal, noise, channel int }
	truth := map[[2]string]reading{}
	llBSSIDs := map[string]bool{}
	for i, p := range ll.Points {
		for _, ap := range p.APs {
			b := normalizeBSSID(ap.BSSID)
			if b == "" {
				continue
			}
			llBSSIDs[b] = true
			truth[[2]string{strconv.Itoa(i), b}] = reading{ap.Signal, ap.Noise, ap.ChannelPrimary}
		}
	}

	trBSSIDs := map[string]bool{}
	for i, p := range points {
		if i < len(ll.Points) {
			into.positions[1]++
			if int(ll.Points[i].X) == p.X && int(ll.Points[i].Y) == p.Y {
				into.positions[0]++
			}
		}
		for _, n := range p.Networks {
			b := normalizeBSSID(n.BSSID)
			trBSSIDs[b] = true
			want, ok := truth[[2]string{strconv.Itoa(i), b}]
			if !ok {
				continue
			}
			into.signal[1]++
			into.noise[1]++
			into.channel[1]++
			if want.signal == n.Signal {
				into.signal[0]++
			}
			if want.noise == n.NoiseFloor {
				into.noise[0]++
			}
			if want.channel == n.Channel {
				into.channel[0]++
			}
		}
	}

	for b := range llBSSIDs {
		if trBSSIDs[b] {
			into.bssids[0]++
		} else {
			into.bssids[1]++
		}
	}
	for b := range trBSSIDs {
		if !llBSSIDs[b] {
			into.bssids[1]++
		}
	}
}

// normalizeBSSID reduces both sides to bare lowercase hex. Link-Live writes
// "RuckusWi:58b633-c652d9"; the archive writes "58:b6:33:c6:52:d9".
func normalizeBSSID(s string) string {
	if _, address, found := strings.Cut(s, ":"); found && strings.Contains(address, "-") {
		s = address
	}
	return strings.ToLower(strings.NewReplacer(":", "", "-", "").Replace(s))
}

func readProcessed(path string) (*processed, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only

	var reader io.Reader = f
	head := make([]byte, 2)
	if _, err := io.ReadFull(f, head); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if head[0] == 0x1f && head[1] == 0x8b {
		gz, gzErr := gzip.NewReader(f)
		if gzErr != nil {
			return nil, gzErr
		}
		defer gz.Close() //nolint:errcheck // read-only
		reader = gz
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	// Older exports are a bare array of points; newer ones wrap that array
	// alongside the AP placements.
	var out processed
	if len(body) > 0 && body[0] == '[' {
		return &out, json.Unmarshal(body, &out.Points)
	}
	return &out, json.Unmarshal(body, &out)
}

func renderReport(results []comparison, unpaired []string) string {
	var b strings.Builder
	b.WriteString("| Link-Live analysis | Unit | Mode | Points LL/TR | Placements LL/TR | Positions agree | BSSIDs common/disjoint | Signal agree | Noise agree | Channel agree |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	var totals comparison
	for _, r := range results {
		b.WriteString(row(fmt.Sprintf("`%s`", r.analysisID), r.unit, r.mode, r))
		totals.add(r)
	}
	b.WriteString(row(fmt.Sprintf("**%d surveys**", len(results)), "", "", totals))
	if len(unpaired) > 0 {
		fmt.Fprintf(&b, "\nNo archive in the corpus matches %d Link-Live records: %s\n",
			len(unpaired), strings.Join(unpaired, ", "))
	}
	return b.String()
}

func row(label, unit, mode string, c comparison) string {
	return fmt.Sprintf("| %s | %s | %s | %d/%d | %d/%d | %d/%d | %d/%d | %d/%d | %d/%d | %d/%d |\n",
		label, unit, mode,
		c.points[0], c.points[1],
		c.placements[0], c.placements[1],
		c.positions[0], c.positions[1],
		c.bssids[0], c.bssids[1],
		c.signal[0], c.signal[1],
		c.noise[0], c.noise[1],
		c.channel[0], c.channel[1])
}

func sortedKeys(m map[string]record) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "linklive-oracle:", err)
	os.Exit(2)
}
