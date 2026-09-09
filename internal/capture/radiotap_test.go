// SPDX-License-Identifier: BUSL-1.1

package capture

import (
	"encoding/hex"
	"testing"
)

// realBeaconFrame is one beacon captured off the Edimax EW-7611ULB
// (rtl8xxxu) in monitor mode on 2437 MHz on dev-srv-ubuntu, 2026-09-09.
// A real frame rather than a hand-built one means these parsers are tested
// against the byte layout a driver actually emits, vendor elements and all.
const realBeaconFrame = "00001a002f4800007c678e510000000000028509a000f200000080000000ffffffffff" +
	"ff245a4c6bb5c8245a4c6bb5c8001398810f656a00000064003114000f4e6575726f70" +
	"6c6173746963697479010882848b961224486c0301060504000100000706555320010b" +
	"2420010023021a0046057200010000330c0c0102030405060708090a0b2a010032040c" +
	"18306030180100000fac040100000fac040200000fac02000fac0880000b0500002f12" +
	"7a2d1aad0117ffffffff000000000000000000000000000018048719003d1606000000" +
	"0000000000000000000000000000000000007f080000088000000000dd180050f20201" +
	"01000003a4000027a4000042435e0062322f00dd07000c4300000000dd21000ce70000" +
	"0000bf0cb101c0332aff92042aff9204c0050000002affc303010202dd3900156d0001" +
	"0100010220a68106245a4c6bb5c7892437343165646535612d316561392d343238382d" +
	"613434612d646462366235316630303161"

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return b
}

func TestParseRadiotapRealFrame(t *testing.T) {
	rt, err := parseRadiotap(mustHex(t, realBeaconFrame))
	if err != nil {
		t.Fatalf("parseRadiotap: %v", err)
	}

	// This adapter emits a 26-byte header: TSFT, flags, rate, channel, dBm
	// antenna signal, antenna and RX flags.
	if rt.length != 26 {
		t.Errorf("length = %d, want 26", rt.length)
	}
	if !rt.haveSignal || rt.signalDBm != -14 {
		t.Errorf("signal = %d (have %v), want -14", rt.signalDBm, rt.haveSignal)
	}
	if !rt.haveFreq || rt.freqMHz != 2437 {
		t.Errorf("freq = %d (have %v), want 2437", rt.freqMHz, rt.haveFreq)
	}
	// rtl8xxxu reports no noise measurement, which is why the backend falls
	// back to defaultNoiseFloorDBm rather than recording 0 dBm as a reading.
	if rt.haveNoise {
		t.Error("haveNoise = true, want false for this adapter")
	}
	if rt.hasFCS {
		t.Error("hasFCS = true, want false (the flags byte is 0)")
	}
}

func TestParseRadiotapRejectsShortAndTruncated(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"shorter than the fixed header", make([]byte, 7)},
		{"it_len past the buffer", []byte{0, 0, 64, 0, 0, 0, 0, 0}},
		{"it_len inside the fixed header", []byte{0, 0, 4, 0, 0, 0, 0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseRadiotap(tt.in); err == nil {
				t.Fatal("parseRadiotap succeeded, want error")
			}
		})
	}
}

// TestParseRadiotapStopsAtUnknownField guards the offset arithmetic. Once a
// present bit whose size this parser does not know is set, every later field
// offset is unknowable, so parsing must stop rather than read at a guessed
// offset. Signal (bit 5) precedes the unknown bit 20 and is still read.
func TestParseRadiotapStopsAtUnknownField(t *testing.T) {
	// present = TSFT | flags | rate | channel | dBm antenna signal | bit 20.
	b := []byte{0, 0, 24, 0, 0x2f, 0x00, 0x10, 0x00}
	b = append(b, make([]byte, 8)...)     // TSFT, offset 8
	b = append(b, 0x00, 0x02)             // flags, rate
	b = append(b, 0x85, 0x09, 0xa0, 0x00) // channel 2437, offset 18
	b = append(b, 0xf2)                   // signal -14, offset 22
	b = append(b, 0x00)                   // pad to it_len

	rt, err := parseRadiotap(b)
	if err != nil {
		t.Fatalf("parseRadiotap: %v", err)
	}
	if !rt.haveSignal || rt.signalDBm != -14 {
		t.Errorf("signal = %d (have %v), want -14 read before the unknown field",
			rt.signalDBm, rt.haveSignal)
	}
}

// TestParseRadiotapFCSFlag proves the FCS-present flag is read: a trailing
// four-byte checksum left on the frame would otherwise be parsed as an
// information element and corrupt the tail of every beacon.
func TestParseRadiotapFCSFlag(t *testing.T) {
	b := []byte{0, 0, 10, 0, 0x02, 0x00, 0x00, 0x00, radiotapFlagFCS, 0x00}
	rt, err := parseRadiotap(b)
	if err != nil {
		t.Fatalf("parseRadiotap: %v", err)
	}
	if !rt.hasFCS {
		t.Error("hasFCS = false, want true")
	}
}
