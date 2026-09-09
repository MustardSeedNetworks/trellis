// SPDX-License-Identifier: BUSL-1.1

package capture

import (
	"encoding/binary"
	"errors"
)

// errRadiotap means a frame's radiotap header could not be trusted. Monitor
// mode hands over whatever the driver produced, so a malformed header is an
// ordinary event on a busy channel, not a fault: the caller drops the frame
// and reads the next one.
var errRadiotap = errors.New("capture: malformed radiotap header")

// Fixed part of a radiotap header: version, pad, it_len, and the first
// present bitmask.
const radiotapFixedLen = 8

// Present bits this parser reads. Fields above radiotapMaxKnownBit are never
// reached — see [parseRadiotap].
const (
	radiotapBitTSFT      = 0
	radiotapBitFlags     = 1
	radiotapBitChannel   = 3
	radiotapBitAntSignal = 5
	radiotapBitAntNoise  = 6
	radiotapMaxKnownBit  = 14
	radiotapBitExt       = 31
)

// radiotapFlagFCS marks a frame that still carries its four-byte checksum.
const radiotapFlagFCS = 0x10

// radiotapFieldLayout is the alignment and size of each field, indexed by its
// present bit. Radiotap aligns every field to its own natural boundary
// relative to the start of the header, so both numbers are needed to find the
// next field — a size-only walk silently misreads every field after the first
// padded one.
var radiotapFieldLayout = [radiotapMaxKnownBit + 1]struct{ align, size int }{
	0:  {8, 8}, // TSFT
	1:  {1, 1}, // flags
	2:  {1, 1}, // rate
	3:  {2, 4}, // channel: frequency then channel flags
	4:  {2, 2}, // FHSS
	5:  {1, 1}, // dBm antenna signal
	6:  {1, 1}, // dBm antenna noise
	7:  {2, 2}, // lock quality
	8:  {2, 2}, // TX attenuation
	9:  {2, 2}, // dB TX attenuation
	10: {1, 1}, // dBm TX power
	11: {1, 1}, // antenna
	12: {1, 1}, // dB antenna signal
	13: {1, 1}, // dB antenna noise
	14: {2, 2}, // RX flags
}

// radiotap is the subset of a radiotap header a survey point needs.
type radiotap struct {
	// length is the full header size, and so the offset of the 802.11 frame.
	length int

	signalDBm  int
	haveSignal bool
	noiseDBm   int
	haveNoise  bool
	freqMHz    int
	haveFreq   bool

	// hasFCS reports that the frame still carries its trailing checksum.
	hasFCS bool
}

// parseRadiotap reads the header a monitor interface prepends to every frame.
//
// Fields are optional and self-describing: a present bitmask says which ones
// the driver supplied, and they follow in bit order. The offset of every
// field therefore depends on the sizes of all the fields before it, so an
// unrecognised field would invalidate every offset after it. The walk is
// bounded to bits 0..14 for that reason, which costs nothing: those bits are
// the lowest, so their fields always precede any field this parser cannot
// size, and every value a survey point needs is among them.
func parseRadiotap(b []byte) (radiotap, error) {
	if len(b) < radiotapFixedLen {
		return radiotap{}, errRadiotap
	}

	length := int(binary.LittleEndian.Uint16(b[2:4]))
	if length < radiotapFixedLen || length > len(b) {
		return radiotap{}, errRadiotap
	}

	// Walk the present words first: an extended header carries several, and
	// the field data begins only after the last of them.
	offset := 4
	var present []uint32
	for {
		if offset+4 > length {
			return radiotap{}, errRadiotap
		}
		word := binary.LittleEndian.Uint32(b[offset : offset+4])
		present = append(present, word)
		offset += 4
		if word&(1<<radiotapBitExt) == 0 {
			break
		}
	}

	rt := radiotap{length: length}

	// Only the first present word is interpreted. Later words belong to
	// vendor or extended namespaces whose field sizes are not knowable here,
	// and every field this parser wants lives in the first word.
	for bit := range radiotapMaxKnownBit + 1 {
		if present[0]&(1<<uint(bit)) == 0 {
			continue
		}
		layout := radiotapFieldLayout[bit]
		offset = alignUp(offset, layout.align)
		if offset+layout.size > length {
			return radiotap{}, errRadiotap
		}

		switch bit {
		case radiotapBitFlags:
			rt.hasFCS = b[offset]&radiotapFlagFCS != 0
		case radiotapBitChannel:
			rt.freqMHz = int(binary.LittleEndian.Uint16(b[offset : offset+2]))
			rt.haveFreq = true
		case radiotapBitAntSignal:
			rt.signalDBm = int(int8(b[offset]))
			rt.haveSignal = true
		case radiotapBitAntNoise:
			rt.noiseDBm = int(int8(b[offset]))
			rt.haveNoise = true
		case radiotapBitTSFT:
			// Present in every frame and unused: the survey timestamps a
			// point from the host clock, which is the only clock shared with
			// the walk's position marks.
		}
		offset += layout.size
	}

	return rt, nil
}

// alignUp rounds offset up to the next multiple of align.
func alignUp(offset, align int) int {
	if rem := offset % align; rem != 0 {
		offset += align - rem
	}
	return offset
}
