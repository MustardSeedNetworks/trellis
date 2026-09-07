// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// AirMagnet Survey writes an .svd as UTF-16LE text with CRLF line endings: a
// banner line, `#`-prefixed metadata, a `&` line carrying the survey's
// dimensions, one `#`-prefixed column header, and then one comma-separated row
// per observation. The column header is what makes the format readable across
// builds — the corpus this parser was written against spans app versions 4.0
// through 8.6 and seven survey types, and the columns move between them, so
// every field below is located by name and never by index.
//
// A row is one BSS heard at one position at one moment, not one measurement
// point: a stop that heard eleven BSSes writes eleven rows sharing a timestamp
// and a position. Grouping them back into one point is what makes an imported
// AirMagnet survey indistinguishable from a walked one downstream.
const (
	// AirMagnetMaxBytes bounds an .svd. These are untrusted files from outside
	// the product and the whole thing is decoded in memory; the largest in the
	// reference corpus is 3.9 MB, so 128 MiB is far above any real export and
	// still a bound.
	AirMagnetMaxBytes = 128 << 20

	// airMagnetMaxRows caps observations. The largest corpus file holds about
	// 40,000; a million rows is a refusal, not a survey.
	airMagnetMaxRows = 1 << 20

	airMagnetBanner = "@AirMagnet Survey"

	// Row prefixes. A measurement row carries none.
	apRowPrefix    = "$,"
	eventRowPrefix = "%,"
)

// AirMagnetFile is one parsed .svd.
type AirMagnetFile struct {
	// Type is the survey kind the file declares: "passive", "active",
	// "iperf_active", "native_active", "native_iperf_active" or "mixed".
	Type string
	// AppVersion is the AirMagnet Survey build that wrote it, for the message
	// shown when a file cannot be read.
	AppVersion string
	// Width and Height are the survey extent in the file's own units, rounded.
	// AirMagnet writes positions in that same space, and there is no image
	// here to scale against: the floor plan is a separate file in the project
	// directory, not part of the export.
	Width, Height int
	Points        []AirMagnetPoint

	// Merged is true when this export is AirMagnet's union of several walks
	// rather than one walk. Its measurements are the other files' measurements
	// over again, so counting a merge alongside its parts counts them twice.
	Merged bool

	// APs are the access points the file lists after the measurements, under
	// its "AP Original Configuration" sections. AirMagnet writes the placement
	// an operator drew on the plan, which is the same thing Trellis stores as
	// an imported AP location.
	APs []AirMagnetAP
}

// AirMagnetAP is one access point placement from the trailing configuration
// section. Older builds omit the BSSID column, so it can be empty.
type AirMagnetAP struct {
	X, Y  int
	Name  string
	BSSID string
	// Media is the PHY the placement was recorded for — "802.11a", "802.11g".
	// AirMagnet writes it into the BSSID column with no separator, so it comes
	// out of the same field.
	Media   string
	SSID    string
	Channel int
	// PowerMW is the transmit power the project recorded for this radio, in
	// milliwatts. Zero when the build's layout omits the column.
	PowerMW int
}

// AirMagnetPoint is one position with everything heard from it.
type AirMagnetPoint struct {
	X, Y     int
	Observed time.Time
	Networks []*wifi.ScannedNetwork
}

// ParseAirMagnetSVD reads an AirMagnet Survey .svd export.
func ParseAirMagnetSVD(data []byte) (*AirMagnetFile, error) {
	if len(data) > AirMagnetMaxBytes {
		return nil, fmt.Errorf("AirMagnet file is too large: %d bytes, limit %d",
			len(data), AirMagnetMaxBytes)
	}
	text, err := decodeAirMagnetText(data)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(text, airMagnetBanner) {
		return nil, errors.New("not an AirMagnet survey export: the file does not open with " + airMagnetBanner)
	}

	file := &AirMagnetFile{Merged: strings.HasPrefix(text, airMagnetBanner+" Merged")}
	// A file is a sequence of sections: the measurements first, then whatever
	// that survey type also recorded — a voice survey adds a roaming-event log
	// and a phone list, and most files end with one or more "AP Original
	// Configuration" sections. Each section has its own column header, and
	// measurement holds the first one so a later section cannot redefine what a
	// measurement row means.
	//
	// Rows say which section they belong to with a one-character prefix, which
	// is what makes this safe to read in one pass: bare rows are measurements,
	// "$" is an AP placement, "%" is an event. A row whose prefix this parser
	// does not know is skipped rather than guessed at — reading an event row as
	// a measurement put a survey point at the event's timestamp, which the
	// store's coordinate constraint refused, taking the whole import with it.
	var measurement, section map[string]int
	rows := 0

	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64<<10), AirMagnetMaxBytes)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		switch {
		case line == "" || strings.HasPrefix(line, airMagnetBanner):
			continue

		case strings.HasPrefix(line, "&"):
			file.Width, file.Height = parseAirMagnetDimensions(line)

		case strings.HasPrefix(line, "#"):
			body := strings.TrimPrefix(line, "#")
			if cols := parseAirMagnetColumns(body); cols != nil {
				section = cols
				if measurement == nil {
					measurement = cols
				}
				continue
			}
			file.readMetadata(body)

		case strings.HasPrefix(line, apRowPrefix):
			// The AP header names its columns from the prefix onwards, so the
			// prefix itself occupies no column and is dropped before the row is
			// read. Keeping it would put "$" where Xpos should be, which is how
			// every AP placement in the reference corpus was silently lost.
			file.appendAP(splitAirMagnetRow(line)[1:], section)

		case strings.HasPrefix(line, eventRowPrefix):
			// A roaming event: when the client moved between APs, not where a
			// measurement was taken. Nothing here consumes them yet.
			continue

		case measurement == nil:
			// A merged export lists its source files as bare lines under
			// "#Merged Source Data Files:", before any column header. They are
			// metadata, not measurements, and a parser that treats an
			// unexpected line as a row would place a survey point from them.
			continue

		case !sameColumns(section, measurement):
			// An unprefixed row under a later section's header — a voice
			// survey's phone list, for instance. It is not a measurement and
			// this parser has nothing to do with it.
			continue

		default:
			if rows++; rows > airMagnetMaxRows {
				return nil, fmt.Errorf("AirMagnet file has more than %d rows", airMagnetMaxRows)
			}
			file.appendRow(splitAirMagnetRow(line), measurement)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read AirMagnet file: %w", err)
	}

	if err := file.validate(measurement); err != nil {
		return nil, err
	}
	return file, nil
}

// validate refuses a file this importer would otherwise read wrongly, naming
// what it found. A partial parse of a survey is worse than no import: it
// produces a coverage map that looks complete and is not.
func (f *AirMagnetFile) validate(columns map[string]int) error {
	switch f.Type {
	case "":
		return errors.New("AirMagnet file declares no survey type")
	case "virtual":
		return errors.New(
			"this is a virtual (planner) survey, not a measured walk: Trellis imports measured surveys only")
	}
	if columns == nil {
		return fmt.Errorf(
			"AirMagnet file (app version %s) has no column header, so its layout is unknown",
			f.versionForMessage())
	}
	for _, required := range []string{"Xpos", "Ypos", "SignalDBM", "AP"} {
		if _, ok := columns[required]; !ok {
			return fmt.Errorf(
				"AirMagnet file (app version %s) has no %s column: this layout is not one Trellis can read",
				f.versionForMessage(), required)
		}
	}
	return nil
}

func (f *AirMagnetFile) versionForMessage() string {
	if f.AppVersion == "" {
		return "unknown"
	}
	return f.AppVersion
}

// readMetadata picks out the header fields the importer uses. AirMagnet 4.0
// writes "App Versioin" — its own typo, in every 4.0-era file in the corpus —
// so the version is matched on a prefix that both spellings share.
func (f *AirMagnetFile) readMetadata(body string) {
	for _, field := range strings.Split(body, "\t") {
		field = strings.TrimSpace(field)
		switch {
		case strings.HasPrefix(field, "Type:"):
			f.Type = strings.TrimSpace(strings.TrimPrefix(field, "Type:"))
		case strings.HasPrefix(field, "App Vers"):
			_, value, _ := strings.Cut(field, ":")
			f.AppVersion = strings.TrimSpace(value)
		}
	}
}

// appendRow adds one observation, either to the point that is already open or
// as a new one. Rows arrive grouped by position, so comparing against the last
// point is enough and keeps the import a single pass.
func (f *AirMagnetFile) appendRow(fields []string, columns map[string]int) {
	value := airMagnetField(fields, columns)

	x, xOK := airMagnetInt(value("Xpos"))
	y, yOK := airMagnetInt(value("Ypos"))
	if !xOK || !yOK {
		// A row without a position cannot be placed on a map, and a placed
		// guess would be a fabricated measurement.
		return
	}
	observed := time.Time{}
	if secs, ok := airMagnetInt(value("Time")); ok {
		observed = time.Unix(int64(secs), 0).UTC()
	}

	signal, hasSignal := airMagnetInt(value("SignalDBM"))
	bssid := value("AP")
	if !hasSignal || bssid == "" {
		return
	}
	channel, _ := airMagnetInt(value("Channel"))
	noise, hasNoise := airMagnetInt(value("NoiseDBM"))

	network := &wifi.ScannedNetwork{
		SSID:      value("SSID"),
		BSSID:     bssid,
		Signal:    signal,
		Channel:   channel,
		Frequency: frequencyForChannel(channel),
		HTMode:    value("MediaType"),
		LastSeen:  observed,
	}
	if hasNoise {
		network.NoiseFloor = noise
		network.SNR = signal - noise
	}

	if n := len(f.Points); n > 0 &&
		f.Points[n-1].X == x && f.Points[n-1].Y == y &&
		f.Points[n-1].Observed.Equal(observed) {
		f.Points[n-1].Networks = append(f.Points[n-1].Networks, network)
		return
	}
	f.Points = append(f.Points, AirMagnetPoint{
		X: x, Y: y, Observed: observed, Networks: []*wifi.ScannedNetwork{network},
	})
}

// sameColumns says whether two section headers are the one header, which is
// how an unprefixed row is told from the measurements it follows.
func sameColumns(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for name, i := range a {
		if j, ok := b[name]; !ok || i != j {
			return false
		}
	}
	return true
}

// appendAP records one access-point placement from a configuration section.
func (f *AirMagnetFile) appendAP(fields []string, columns map[string]int) {
	value := airMagnetField(fields, columns)
	x, xOK := airMagnetInt(value("Xpos"))
	y, yOK := airMagnetInt(value("Ypos"))
	if !xOK || !yOK {
		return
	}
	channel, _ := airMagnetInt(value("Channel"))
	power, _ := airMagnetInt(value("Power"))
	bssid, media := splitAPColumn(value("AP"))
	f.APs = append(f.APs, AirMagnetAP{
		X: x, Y: y,
		Name:    value("Name"),
		BSSID:   bssid,
		Media:   media,
		SSID:    value("SSID"),
		Channel: channel,
		PowerMW: power,
	})
}

// splitAPColumn separates the BSSID from the media type an AP-placement row
// runs together in one column: "00:13:80:43:15:2F802.11a". Builds that write
// the address alone return it unchanged with no media.
func splitAPColumn(value string) (bssid, media string) {
	const macLen = len("00:11:22:33:44:55")
	if len(value) < macLen {
		return value, ""
	}
	for i := 2; i < macLen; i += 3 {
		if value[i] != ':' {
			return value, ""
		}
	}
	return value[:macLen], strings.TrimSpace(value[macLen:])
}

// decodeAirMagnetText returns the file as UTF-8. Every export in the corpus is
// UTF-16LE with a byte-order mark; a UTF-8 file is accepted too, since the
// banner check below is the real test of whether this is an .svd at all.
func decodeAirMagnetText(data []byte) (string, error) {
	switch {
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe:
		body := data[2:]
		if len(body)%2 != 0 {
			return "", errors.New("AirMagnet file ends mid-character: truncated UTF-16")
		}
		units := make([]uint16, len(body)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(body[i*2:])
		}
		return string(utf16.Decode(units)), nil

	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		data = data[3:]
		fallthrough
	default:
		if !utf8.Valid(data) {
			return "", errors.New("AirMagnet file is neither UTF-16 nor UTF-8 text")
		}
		return string(data), nil
	}
}

// parseAirMagnetColumns reads a `#` line as a column header, or returns nil if
// it is metadata. Names are written both bare and single-quoted depending on
// the build, and some carry a leading space.
//
// The header is told apart from metadata by shape rather than by the names it
// carries: every metadata line is a "Field: value" pair and the header has none,
// while the header is the only `#` line with several comma-separated fields
// ("#dim_X, dim_Y, GPS Map" has three, hence the bound). Recognising it by
// looking for Xpos would mean a file whose layout Trellis cannot read is
// reported as having no header at all, instead of naming the column it wanted.
func parseAirMagnetColumns(body string) map[string]int {
	const leastHeaderFields = 4
	if strings.Contains(body, ":") {
		return nil
	}
	fields := splitAirMagnetRow(body)
	if len(fields) < leastHeaderFields {
		return nil
	}
	columns := make(map[string]int, len(fields))
	for i, name := range fields {
		columns[strings.Trim(strings.TrimSpace(name), "'")] = i
	}
	return columns
}

// parseAirMagnetDimensions reads the `&,dim_X,dim_Y,GPS` line.
func parseAirMagnetDimensions(line string) (width, height int) {
	fields := splitAirMagnetRow(strings.TrimPrefix(line, "&"))
	if len(fields) < 3 {
		return 0, 0
	}
	width, _ = airMagnetInt(fields[1])
	height, _ = airMagnetInt(fields[2])
	return width, height
}

// splitAirMagnetRow splits on commas outside single quotes. An SSID may contain
// a comma — a naive split renames such a network and shifts every field after
// it, which is the kind of import that looks like it worked.
func splitAirMagnetRow(line string) []string {
	var (
		fields  []string
		current strings.Builder
		quoted  bool
	)
	for _, r := range line {
		switch {
		case r == '\'':
			quoted = !quoted
			current.WriteRune(r)
		case r == ',' && !quoted:
			fields = append(fields, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	return append(fields, current.String())
}

// airMagnetField reads a named column out of one row, empty when this build's
// layout does not carry it.
func airMagnetField(fields []string, columns map[string]int) func(string) string {
	return func(name string) string {
		i, ok := columns[name]
		if !ok || i >= len(fields) {
			return ""
		}
		return strings.Trim(strings.TrimSpace(fields[i]), "'")
	}
}

// airMagnetInt reads a field that may be written as an integer, as a float, or
// as one of the placeholders the format uses for "not measured" ("", "*", "-1"
// keeps its value: -1 is a real reading in the packet-count columns).
func airMagnetInt(s string) (int, bool) {
	s = strings.Trim(strings.TrimSpace(s), "'")
	if s == "" || s == "*" {
		return 0, false
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return int(f), true
}

// frequencyForChannel infers the band from the channel number, which is all an
// .svd carries — unlike the OS capture backends, which are told the band and
// convert with internal/capture's band-aware arithmetic. Channels above 14 are
// 5 GHz here: AirMagnet Survey predates 6 GHz and no corpus file uses it.
func frequencyForChannel(channel int) int {
	const (
		lastChannel24GHz = 14
		freq24GHzBase    = 2407
		freq24GHzCh14    = 2484
		freq5GHzBase     = 5000
		channelSpacing   = 5
	)
	switch {
	case channel <= 0:
		return 0
	case channel == lastChannel24GHz:
		return freq24GHzCh14
	case channel < lastChannel24GHz:
		return freq24GHzBase + channel*channelSpacing
	default:
		return freq5GHzBase + channel*channelSpacing
	}
}
