// SPDX-License-Identifier: BUSL-1.1

package survey_test

// The .SurveyResult decoder had no default test at all: the package reported
// 86.7% coverage while every function in surveyresult.go sat at 0.0%, because
// its only test required a private corpus and skipped without it. A
// reverse-engineered binary decoder for customer archives could be changed
// arbitrarily without a single default test failing.
//
// The fixtures here are built from the wire format rather than committed as
// opaque blobs. Bytes a reviewer cannot read are bytes a reviewer cannot check,
// and a builder additionally carries no customer data. The corpus test still
// runs against real captures where they are available; this is what runs
// everywhere else.

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

// Field numbers, restated here deliberately. A test that imports the
// production constants moves with them, so a renumbering would rewrite both
// sides of the comparison and prove nothing.
const (
	tagTopPoint = 40

	tagPointX      = 10
	tagPointY      = 20
	tagPointTime   = 30
	tagPointObs    = 40
	tagPointActive = 60

	tagActiveAssoc = 2
	tagAssocSSID   = 1
	tagAssocBSSID  = 3
	tagAssocRSSI   = 6
	tagAssocChan   = 27

	tagObsBSSID   = 1
	tagObsSSID    = 3
	tagObsChannel = 16
	tagObsRSSI    = 21
	tagObsNoise   = 22
	tagObsTime    = 24
)

const (
	wireVarint = 0
	wireF64    = 1
	wireBytes  = 2
	wireGroup  = 3
	wireF32    = 5
)

// putVarint appends a base-128 varint.
func putVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

func tag(field, wire uint64) []byte {
	return putVarint(nil, field<<3|wire)
}

func varintField(field, v uint64) []byte {
	return putVarint(tag(field, wireVarint), v)
}

// negDBm is the wire encoding AirMapper uses for a negative dBm reading: the
// magnitude's 64-bit two's complement written as a plain varint, not zigzag.
// That is the whole reason signedDBm exists, and computing it with ^x+1 states
// the encoding directly rather than leaning on a signed conversion.
func negDBm(magnitude uint64) uint64 { return ^magnitude + 1 }

func bytesField(field uint64, payload []byte) []byte {
	out := putVarint(tag(field, wireBytes), uint64(len(payload)))
	return append(out, payload...)
}

func strField(field uint64, s string) []byte { return bytesField(field, []byte(s)) }

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// observation builds one passive BSS observation.
func observation(bssid, ssid string, channel, rssi, noise, atMillis uint64) []byte {
	return bytesField(tagPointObs, concat(
		strField(tagObsBSSID, bssid),
		strField(tagObsSSID, ssid),
		varintField(tagObsChannel, channel),
		varintField(tagObsRSSI, rssi),
		varintField(tagObsNoise, noise),
		varintField(tagObsTime, atMillis),
	))
}

// activeSample builds the connected-AP measurement of an active walk, wrapped
// in the interface message that encloses it.
func activeSample(ssid, bssid string, rssi, channel uint64) []byte {
	assoc := concat(
		strField(tagAssocSSID, ssid),
		strField(tagAssocBSSID, bssid),
		varintField(tagAssocRSSI, rssi),
		varintField(tagAssocChan, channel),
	)
	return bytesField(tagPointActive, bytesField(tagActiveAssoc, assoc))
}

func point(parts ...[]byte) []byte { return bytesField(tagTopPoint, concat(parts...)) }

// The reference instant used throughout. Declared as an untyped constant so it
// converts to whatever each side needs -- uint64 for the wire, int64 for the
// clock -- without a conversion anyone has to reason about.
const refMillis = 1788004800000

var refTime = time.UnixMilli(refMillis).UTC()

func TestParseSurveyResultDecodesAPassiveWalk(t *testing.T) {
	msg := concat(
		point(
			varintField(tagPointX, 120),
			varintField(tagPointY, 340),
			varintField(tagPointTime, refMillis),
			observation("00:11:22:33:44:55", "corp-wifi", 6, negDBm(57), negDBm(95), refMillis),
			observation("66:77:88:99:aa:bb", "guest", 149, negDBm(72), negDBm(92), refMillis),
		),
	)

	points, err := survey.ParseSurveyResult(msg)
	if err != nil {
		t.Fatalf("ParseSurveyResult: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("got %d points, want 1", len(points))
	}

	p := points[0]
	if p.X != 120 || p.Y != 340 {
		t.Errorf("position = (%d,%d), want (120,340)", p.X, p.Y)
	}
	if !p.Observed.Equal(refTime) {
		t.Errorf("Observed = %s, want %s", p.Observed, refTime)
	}
	if p.Active != nil {
		t.Errorf("Active = %+v, want nil on a passive point", p.Active)
	}
	if len(p.Networks) != 2 {
		t.Fatalf("got %d networks, want 2", len(p.Networks))
	}

	// Every decoded field is asserted by value. Asserting the count alone is
	// how this package's heatmap defects survived their own tests.
	first := p.Networks[0]
	switch {
	case first.BSSID != "00:11:22:33:44:55":
		t.Errorf("BSSID = %q", first.BSSID)
	case first.SSID != "corp-wifi":
		t.Errorf("SSID = %q", first.SSID)
	case first.Channel != 6:
		t.Errorf("Channel = %d, want 6", first.Channel)
	case first.Signal != -57:
		t.Errorf("Signal = %d, want -57", first.Signal)
	case first.NoiseFloor != -95:
		t.Errorf("NoiseFloor = %d, want -95", first.NoiseFloor)
	case first.SNR != 38:
		t.Errorf("SNR = %d, want 38 (-57 - -95)", first.SNR)
	case first.Frequency != 2437:
		t.Errorf("Frequency = %d, want 2437 (2407 + 6*5)", first.Frequency)
	case !first.LastSeen.Equal(refTime):
		t.Errorf("LastSeen = %s, want %s", first.LastSeen, refTime)
	}

	second := p.Networks[1]
	if second.Channel != 149 || second.Frequency != 5745 {
		t.Errorf("5 GHz network: channel %d freq %d, want 149 / 5745", second.Channel, second.Frequency)
	}
	if second.Signal != -72 || second.SNR != 20 {
		t.Errorf("5 GHz network: signal %d snr %d, want -72 / 20", second.Signal, second.SNR)
	}
}

func TestParseSurveyResultDecodesAnActiveWalk(t *testing.T) {
	msg := concat(
		point(
			varintField(tagPointX, 10),
			varintField(tagPointY, 20),
			varintField(tagPointTime, refMillis),
			activeSample("corp-wifi", "00:11:22:33:44:55", negDBm(63), 36),
		),
	)

	points, err := survey.ParseSurveyResult(msg)
	if err != nil {
		t.Fatalf("ParseSurveyResult: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("got %d points, want 1", len(points))
	}
	if len(points[0].Networks) != 0 {
		t.Errorf("got %d passive networks on an active point, want 0", len(points[0].Networks))
	}

	a := points[0].Active
	if a == nil {
		t.Fatal("Active is nil; an active walk must populate it")
	}
	if a.SSID != "corp-wifi" || a.BSSID != "00:11:22:33:44:55" || a.RSSI != -63 {
		t.Errorf("ActiveSample = %+v, want ssid corp-wifi bssid 00:11:22:33:44:55 rssi -63", a)
	}
}

// An unknown field must be stepped over, not guessed at -- that is what lets
// the decoder survive AirMapper adding fields.
func TestParseSurveyResultSkipsUnknownFieldsOfEveryWireType(t *testing.T) {
	f32 := make([]byte, 4)
	binary.LittleEndian.PutUint32(f32, 0xDEADBEEF)
	f64 := make([]byte, 8)
	binary.LittleEndian.PutUint64(f64, 0xFEEDFACECAFEBEEF)

	msg := concat(
		varintField(900, 12345),                  // unknown varint at top level
		bytesField(901, []byte("something new")), // unknown bytes at top level
		append(tag(902, wireF32), f32...),        // unknown fixed32
		append(tag(903, wireF64), f64...),        // unknown fixed64
		point(
			varintField(904, 7),
			strField(905, "unmodelled"),
			varintField(tagPointX, 5),
			varintField(tagPointY, 6),
			observation("aa:bb:cc:dd:ee:ff", "net", 1, negDBm(40), negDBm(90), refMillis),
		),
	)

	points, err := survey.ParseSurveyResult(msg)
	if err != nil {
		t.Fatalf("ParseSurveyResult: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("got %d points, want 1", len(points))
	}
	if points[0].X != 5 || points[0].Y != 6 {
		t.Errorf("position = (%d,%d), want (5,6)", points[0].X, points[0].Y)
	}
	if len(points[0].Networks) != 1 || points[0].Networks[0].Signal != -40 {
		t.Errorf("networks = %+v, want one at -40 dBm", points[0].Networks)
	}
}

// A reading no receiver could produce means the field map is wrong for this
// file. The observation is dropped rather than persisted.
func TestParseSurveyResultDropsUnreceivableReadings(t *testing.T) {
	tests := []struct {
		name string
		rssi uint64
	}{
		{"positive dBm", 12},
		{"zero dBm", 0},
		{"below any receiver floor", negDBm(111)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := point(
				varintField(tagPointX, 1),
				observation("aa:bb:cc:dd:ee:ff", "net", 6, tt.rssi, negDBm(95), refMillis),
			)
			points, err := survey.ParseSurveyResult(msg)
			if err != nil {
				t.Fatalf("ParseSurveyResult: %v", err)
			}
			if len(points) != 1 {
				t.Fatalf("got %d points, want 1", len(points))
			}
			if len(points[0].Networks) != 0 {
				t.Errorf("kept %+v; %s must be dropped", points[0].Networks, tt.name)
			}
		})
	}
}

func TestParseSurveyResultDropsUnreceivableActiveReading(t *testing.T) {
	msg := point(activeSample("corp", "00:11:22:33:44:55", 5, 36))

	points, err := survey.ParseSurveyResult(msg)
	if err != nil {
		t.Fatalf("ParseSurveyResult: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("got %d points, want 1", len(points))
	}
	if points[0].Active != nil {
		t.Errorf("Active = %+v, want nil for a +5 dBm reading", points[0].Active)
	}
}

// Damage must be reported as damage. A decoder that returns partial results
// for a truncated archive silently loses measurements.
func TestParseSurveyResultRejectsMalformedMessages(t *testing.T) {
	overflow := make([]byte, 0, 12)
	overflow = append(overflow, tag(tagTopPoint, wireVarint)...)
	for range 10 {
		overflow = append(overflow, 0xFF)
	}
	overflow = append(overflow, 0x01)

	tests := []struct {
		name string
		msg  []byte
	}{
		{"varint ends mid-field", []byte{0x80}},
		{"length-delimited header ends after the tag", tag(tagTopPoint, wireBytes)},
		{
			"declared length exceeds the buffer",
			append(putVarint(tag(tagTopPoint, wireBytes), 64), []byte("short")...),
		},
		{
			"length overflows int32",
			putVarint(tag(tagTopPoint, wireBytes), math.MaxInt32+1),
		},
		{"fixed32 is short", append(tag(902, wireF32), 0x01, 0x02)},
		{"fixed64 is short", append(tag(903, wireF64), 0x01, 0x02, 0x03, 0x04)},
		{"varint overflows 64 bits", overflow},
		{"group wire type is not supported", tag(tagTopPoint, wireGroup)},
		{
			"damage inside a point payload propagates",
			bytesField(tagTopPoint, tag(tagPointObs, wireBytes)),
		},
		{
			"damage inside an observation propagates",
			point(bytesField(tagPointObs, tag(tagObsSSID, wireBytes))),
		},
		{
			"damage inside an active sample propagates",
			point(bytesField(tagPointActive, tag(tagActiveAssoc, wireBytes))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			points, err := survey.ParseSurveyResult(tt.msg)
			if err == nil {
				t.Fatalf("got %d points and no error; malformed input must be rejected", len(points))
			}
			if points != nil {
				t.Errorf("got %d points alongside the error, want none", len(points))
			}
		})
	}
}

// An empty message is a valid archive with nothing in it, not damage.
func TestParseSurveyResultAcceptsAnEmptyMessage(t *testing.T) {
	points, err := survey.ParseSurveyResult(nil)
	if err != nil {
		t.Fatalf("ParseSurveyResult(nil): %v", err)
	}
	if len(points) != 0 {
		t.Errorf("got %d points, want 0", len(points))
	}
}

// Channel-to-frequency is exercised through the decoder because that is the
// only way it is reachable; the mapping is what the rest of Trellis reasons on.
func TestParseSurveyResultDerivesFrequencyFromChannel(t *testing.T) {
	tests := []struct {
		channel uint64
		freq    int
	}{
		{1, 2412},
		{6, 2437},
		{13, 2472},
		{14, 2484}, // the 2.4 GHz exception
		{36, 5180},
		{149, 5745},
		{177, 5885},
		{15, 0},  // between the bands: no centre frequency
		{178, 0}, // above the mapped range
		{0, 0},   // channel absent from the file
	}

	for _, tt := range tests {
		msg := point(observation("aa:bb:cc:dd:ee:ff", "net", tt.channel, negDBm(50), negDBm(95), refMillis))
		points, err := survey.ParseSurveyResult(msg)
		if err != nil {
			t.Fatalf("channel %d: %v", tt.channel, err)
		}
		if len(points) != 1 || len(points[0].Networks) != 1 {
			t.Fatalf("channel %d: got %d points", tt.channel, len(points))
		}
		if got := points[0].Networks[0].Frequency; got != tt.freq {
			t.Errorf("channel %d: frequency = %d, want %d", tt.channel, got, tt.freq)
		}
	}
}

// A timestamp outside any plausible range is a decode error, not a late date.
func TestParseSurveyResultRejectsImplausibleTimestamps(t *testing.T) {
	tests := []struct {
		name   string
		millis uint64
		want   time.Time
	}{
		{"absent", 0, time.Time{}},
		{"beyond year 10000", 253402300799001, time.Time{}},
		{"at the boundary", 253402300799000, time.UnixMilli(253402300799000).UTC()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := point(varintField(tagPointTime, tt.millis))
			points, err := survey.ParseSurveyResult(msg)
			if err != nil {
				t.Fatalf("ParseSurveyResult: %v", err)
			}
			if !points[0].Observed.Equal(tt.want) {
				t.Errorf("Observed = %v, want %v", points[0].Observed, tt.want)
			}
		})
	}
}

// A coordinate larger than int32 is a malformed file, not a large measurement.
func TestParseSurveyResultZeroesOversizedCoordinates(t *testing.T) {
	msg := point(
		varintField(tagPointX, math.MaxInt32+1),
		varintField(tagPointY, 42),
	)
	points, err := survey.ParseSurveyResult(msg)
	if err != nil {
		t.Fatalf("ParseSurveyResult: %v", err)
	}
	if points[0].X != 0 {
		t.Errorf("X = %d, want 0 for an oversized coordinate", points[0].X)
	}
	if points[0].Y != 42 {
		t.Errorf("Y = %d, want 42", points[0].Y)
	}
}

// SNR is only meaningful when both readings are present and ordered.
func TestParseSurveyResultOmitsSNRWithoutAUsableNoiseFloor(t *testing.T) {
	tests := []struct {
		name  string
		noise uint64
		want  int
	}{
		{"noise floor absent", 0, 0},
		{"noise floor above signal", negDBm(30), 0},
		{"noise floor below signal", negDBm(95), 45},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := concat(
				strField(tagObsBSSID, "aa:bb:cc:dd:ee:ff"),
				varintField(tagObsChannel, 6),
				varintField(tagObsRSSI, negDBm(50)),
			)
			if tt.noise != 0 {
				obs = append(obs, varintField(tagObsNoise, tt.noise)...)
			}
			msg := point(bytesField(tagPointObs, obs))

			points, err := survey.ParseSurveyResult(msg)
			if err != nil {
				t.Fatalf("ParseSurveyResult: %v", err)
			}
			if len(points[0].Networks) != 1 {
				t.Fatalf("got %d networks, want 1", len(points[0].Networks))
			}
			if got := points[0].Networks[0].SNR; got != tt.want {
				t.Errorf("SNR = %d, want %d", got, tt.want)
			}
		})
	}
}
