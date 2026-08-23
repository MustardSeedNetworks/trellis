// SPDX-License-Identifier: BUSL-1.1

package survey

// surveyresult.go reads the `.SurveyResult` member of an AirMapper archive —
// the file that holds the measurements.
//
// It is protobuf wire format with no schema shipped alongside it, so the field
// numbers below were recovered by decoding the twelve reference captures and
// checking each candidate against something the file already asserts about
// itself. Field 21 is RSSI because its values span -84..-25 dBm across 5,297
// observations in one survey and never leave a radio's range in any of the
// twelve; field 16 is the channel because its values are all real 802.11
// channel numbers; point x/y fall inside the floor plan's own pixel
// dimensions. The strongest check is arithmetic rather than plausibility: the
// `.serial` sidecar declares surveyPointCount, and the point count recovered
// here matches it exactly in all twelve files.
//
// Wire format is self-describing enough to skip what we do not model: every
// field carries its wire type, so unknown fields are stepped over rather than
// guessed at. That is what lets this survive AirMapper adding fields.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// Field numbers in the .SurveyResult message, recovered as described above.
const (
	frTopSurvey = 10 // survey + floor-plan metadata
	frTopPoint  = 40 // one walk position and everything seen from it

	frPointX      = 10 // pixels on the floor plan
	frPointY      = 20
	frPointTime   = 30 // epoch milliseconds
	frPointObs    = 40 // one observed BSS (passive walk)
	frPointActive = 60 // the connected-AP measurement (active walk)

	// Inside an active measurement: field 1 is the local interface, field 2
	// the association it is reporting on.
	frActiveAssoc = 2
	frAssocSSID   = 1
	frAssocBSSID  = 3
	frAssocRSSI   = 6 // dBm, two's-complement varint
	frAssocNoise  = 7
	frAssocChan   = 27
	frObsBSSID    = 1
	frObsSSID     = 3
	frObsChannel  = 16
	frObsRSSI     = 21 // dBm, two's-complement in a varint
	frObsNoise    = 22 // dBm, same encoding
	frObsTime     = 24
)

// Wire types.
const (
	wireVarint = 0
	wireF64    = 1
	wireBytes  = 2
	wireF32    = 5
)

// radioFloorDBm is the weakest reading treated as a measurement rather than a
// decode error. The reference corpus bottoms out at -84 dBm; -110 is below any
// real receiver's sensitivity, so it rejects nonsense without clipping data.
const radioFloorDBm = -110

// ErrTruncated is returned when the message ends mid-field, which means the
// archive member is damaged rather than merely unfamiliar.
var ErrTruncated = errors.New("surveyresult: truncated message")

// SurveyPointRecord is one walk position and what was measured there.
//
// A passive walk records every BSS in earshot, so Networks is populated. An
// active walk records the association the radio actually held, so Active is
// populated instead. AirMapper writes them as different fields on the point,
// which is why one archive can look empty to a reader that only knows the
// other — the 245-point "Time Square 9th Floor" capture is an active survey
// and yielded zero passive observations for exactly that reason.
type SurveyPointRecord struct {
	X, Y     int
	Observed time.Time
	Networks []*wifi.ScannedNetwork
	Active   *ActiveSample
}

// decoder walks a protobuf message.
type decoder struct {
	buf []byte
	pos int
}

func (d *decoder) done() bool { return d.pos >= len(d.buf) }

func (d *decoder) varint() (uint64, error) {
	var result uint64
	var shift uint
	for {
		if d.pos >= len(d.buf) {
			return 0, ErrTruncated
		}
		b := d.buf[d.pos]
		d.pos++
		result |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
		if shift > 63 {
			return 0, errors.New("surveyresult: varint overflows 64 bits")
		}
	}
}

// next returns the next field's number and payload, stepping over wire types
// this reader does not model.
func (d *decoder) next() (field int, wire int, payload []byte, val uint64, err error) {
	key, err := d.varint()
	if err != nil {
		return 0, 0, nil, 0, err
	}
	field, wire = int(key>>3), int(key&7)
	switch wire {
	case wireVarint:
		val, err = d.varint()
		return field, wire, nil, val, err
	case wireBytes:
		n, lenErr := d.varint()
		if lenErr != nil {
			return 0, 0, nil, 0, lenErr
		}
		// A length larger than the whole buffer is a truncated or crafted
		// message. Rejecting it before any conversion keeps the arithmetic in
		// a range int can hold, rather than relying on the addition not
		// wrapping.
		if n > math.MaxInt32 || int(n) > len(d.buf)-d.pos {
			return 0, 0, nil, 0, ErrTruncated
		}
		size := int(n)
		payload = d.buf[d.pos : d.pos+size]
		d.pos += size
		return field, wire, payload, 0, nil
	case wireF32:
		if d.pos+4 > len(d.buf) {
			return 0, 0, nil, 0, ErrTruncated
		}
		val = uint64(binary.LittleEndian.Uint32(d.buf[d.pos:]))
		d.pos += 4
		return field, wire, nil, val, nil
	case wireF64:
		if d.pos+8 > len(d.buf) {
			return 0, 0, nil, 0, ErrTruncated
		}
		val = binary.LittleEndian.Uint64(d.buf[d.pos:])
		d.pos += 8
		return field, wire, nil, val, nil
	default:
		// Groups (3, 4) were removed from proto3 and AirMapper does not emit
		// them. Stopping is safer than guessing a length we cannot know.
		return 0, 0, nil, 0, fmt.Errorf("surveyresult: unsupported wire type %d for field %d", wire, field)
	}
}

// signedDBm reinterprets a varint as a signed value. AirMapper writes negative
// dBm without zigzag encoding, so -57 arrives as 2^64-57.
//
// The result is clamped to a range no receiver can leave. This parses files
// that arrive from outside, and an unclamped conversion would let a crafted
// varint land an absurd value in a column the schema then has to reject —
// better to refuse it here, where the reason is legible.
func signedDBm(v uint64) int {
	signed := int64(v) // #nosec G115 -- reinterpretation is the point; clamped below
	if signed > 0 || signed < radioFloorDBm {
		return invalidDBm
	}
	return int(signed)
}

// invalidDBm marks a reading outside any receiver's range, so callers drop the
// observation rather than store a number that means nothing.
const invalidDBm = 1

// smallNonNegative narrows a varint that must be a small count — pixel
// coordinates, channel numbers. Anything larger is a malformed file, not a
// large measurement, so it becomes zero and is rejected by the caller's own
// range checks rather than wrapping into something plausible.
func smallNonNegative(v uint64) int {
	if v > math.MaxInt32 {
		return 0
	}
	return int(v)
}

// ParseSurveyResult decodes the measurements from a .SurveyResult payload.
func ParseSurveyResult(data []byte) ([]SurveyPointRecord, error) {
	var points []SurveyPointRecord
	d := &decoder{buf: data}
	for !d.done() {
		field, _, payload, _, err := d.next()
		if err != nil {
			return nil, err
		}
		if field != frTopPoint || payload == nil {
			continue
		}
		point, err := parsePoint(payload)
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, nil
}

func parsePoint(data []byte) (SurveyPointRecord, error) {
	var p SurveyPointRecord
	d := &decoder{buf: data}
	for !d.done() {
		field, _, payload, val, err := d.next()
		if err != nil {
			return p, err
		}
		switch field {
		case frPointX:
			p.X = smallNonNegative(val)
		case frPointY:
			p.Y = smallNonNegative(val)
		case frPointTime:
			p.Observed = msToTime(val)
		case frPointObs:
			if payload == nil {
				continue
			}
			n, obsErr := parseObservation(payload)
			if obsErr != nil {
				return p, obsErr
			}
			if n != nil {
				p.Networks = append(p.Networks, n)
			}
		case frPointActive:
			if payload == nil {
				continue
			}
			a, activeErr := parseActive(payload)
			if activeErr != nil {
				return p, activeErr
			}
			if a != nil {
				p.Active = a
			}
		}
	}
	return p, nil
}

func parseObservation(data []byte) (*wifi.ScannedNetwork, error) {
	n := &wifi.ScannedNetwork{}
	d := &decoder{buf: data}
	for !d.done() {
		field, _, payload, val, err := d.next()
		if err != nil {
			return nil, err
		}
		switch field {
		case frObsBSSID:
			n.BSSID = string(payload)
		case frObsSSID:
			n.SSID = string(payload)
		case frObsChannel:
			n.Channel = smallNonNegative(val)
		case frObsRSSI:
			n.Signal = signedDBm(val)
		case frObsNoise:
			n.NoiseFloor = signedDBm(val)
		case frObsTime:
			n.LastSeen = msToTime(val)
		}
	}
	// A reading a receiver could not produce means the field map is wrong for
	// this file, not that the air was unusual. Drop it rather than persist a
	// number the schema would reject anyway.
	if n.Signal >= invalidDBm || n.Signal > 0 || n.Signal < radioFloorDBm {
		return nil, nil
	}
	if n.NoiseFloor < 0 && n.Signal >= n.NoiseFloor {
		n.SNR = n.Signal - n.NoiseFloor
	}
	n.Frequency = channelToFrequency(n.Channel)
	return n, nil
}

// parseActive reads the connected-AP measurement an active walk records.
// Field 1 of the enclosing message describes the local interface — address,
// gateway, DNS — which the survey domain does not model, so it is skipped.
func parseActive(data []byte) (*ActiveSample, error) {
	d := &decoder{buf: data}
	for !d.done() {
		field, _, payload, _, err := d.next()
		if err != nil {
			return nil, err
		}
		if field != frActiveAssoc || payload == nil {
			continue
		}
		return parseAssociation(payload)
	}
	return nil, nil
}

func parseAssociation(data []byte) (*ActiveSample, error) {
	a := &ActiveSample{}
	d := &decoder{buf: data}
	for !d.done() {
		field, _, payload, val, err := d.next()
		if err != nil {
			return nil, err
		}
		switch field {
		case frAssocSSID:
			a.SSID = string(payload)
		case frAssocBSSID:
			a.BSSID = string(payload)
		case frAssocRSSI:
			a.RSSI = signedDBm(val)
		case frAssocChan:
			_ = val // channel is carried but ActiveSample does not model it
		}
	}
	// Same rule as a passive observation: a reading no receiver could produce
	// means the field map is wrong for this file, not that the air was odd.
	if a.RSSI >= invalidDBm || a.RSSI > 0 || a.RSSI < radioFloorDBm {
		return nil, nil
	}
	return a, nil
}

func msToTime(ms uint64) time.Time {
	// Beyond this a timestamp is not a late date, it is a decode error. Year
	// 10000 in milliseconds; anything past it becomes the zero time.
	const maxMillis = 253402300799000
	if ms == 0 || ms > maxMillis {
		return time.Time{}
	}
	return time.UnixMilli(int64(ms)).UTC()
}

// channelToFrequency maps an 802.11 channel to its centre frequency in MHz.
// AirMapper records the channel; the rest of Trellis reasons in frequency, and
// deriving it here keeps that conversion in one place.
func channelToFrequency(ch int) int {
	switch {
	case ch <= 0:
		return 0
	case ch == 14:
		return 2484
	case ch < 14:
		return 2407 + ch*5
	case ch >= 32 && ch <= 177:
		return 5000 + ch*5
	default:
		return 0
	}
}
