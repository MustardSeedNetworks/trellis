// SPDX-License-Identifier: BUSL-1.1

//go:build linux || windows

package capture

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// ie builds one information element.
//
// An element's length field is a single byte, so a fixture longer than 255
// bytes could not exist on the air either — hence the check rather than a
// silent truncation.
func ie(id byte, data ...byte) []byte {
	return append([]byte{id, uint8Len(data)}, data...)
}

// uint8Len is the length field of an information element. It saturates rather
// than wrapping, because a fixture that long is a mistake in the test and a
// silently wrapped length would make the parser look broken instead.
func uint8Len(data []byte) uint8 {
	switch n := len(data); {
	case n > math.MaxUint8:
		return math.MaxUint8
	case n < 0:
		return 0
	default:
		return uint8(n)
	}
}

// uint16Count is the same for a two-byte count field.
func uint16Count(n int) uint16 {
	switch {
	case n > math.MaxUint16:
		return math.MaxUint16
	case n < 0:
		return 0
	default:
		return uint16(n)
	}
}

// rsnIE builds an RSN element advertising the given IEEE AKM suite types.
func rsnIE(akms ...byte) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x01, 0x00})             // version 1
	b.Write([]byte{0x00, 0x0f, 0xac, 0x04}) // group cipher CCMP
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	b.Write([]byte{0x00, 0x0f, 0xac, 0x04}) // one pairwise cipher, CCMP
	_ = binary.Write(&b, binary.LittleEndian, uint16Count(len(akms)))
	for _, akm := range akms {
		b.Write([]byte{0x00, 0x0f, 0xac, akm})
	}
	b.Write([]byte{0x00, 0x00}) // RSN capabilities
	return ie(elemRSN, b.Bytes()...)
}

func TestSSIDFromElements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		blob []byte
		want string
	}{
		{"named network", ie(elemSSID, 'T', 'r', 'e', 'l', 'l', 'i', 's'), "Trellis"},
		{"hidden, zero length", ie(elemSSID), ""},
		{"hidden, NUL padded", ie(elemSSID, 0, 0, 0, 0), ""},
		{"no SSID element at all", ie(elemHTOperation, 0x01, 0x00), ""},
		{"SSID after another element", append(ie(elemHTOperation, 0x01, 0x00), ie(elemSSID, 'x')...), "x"},
		// A NUL inside a real name is not a hidden network.
		{"embedded NUL with real bytes", ie(elemSSID, 'a', 0, 'b'), "a\x00b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ssidFromElements(parseElements(tt.blob)); got != tt.want {
				t.Errorf("ssidFromElements = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSecurityFromElements(t *testing.T) {
	t.Parallel()

	wpaVendor := ie(elemVendor, 0x00, 0x50, 0xf2, 0x01, 0x01, 0x00)

	tests := []struct {
		name       string
		blob       []byte
		capability uint16
		want       string
	}{
		{"open", ie(elemSSID, 'a'), 0, "Open"},
		{"WEP is privacy without RSN or WPA", ie(elemSSID, 'a'), capabilityPrivacy, "WEP"},
		{"WPA2 personal", rsnIE(akmPSK), capabilityPrivacy, "WPA2"},
		{"WPA3 personal", rsnIE(akmSAE), capabilityPrivacy, "WPA3"},
		// The case that decides whether a site reads as WPA3-capable: an AP
		// offering both is WPA3 that still accepts WPA2 clients.
		{"WPA3 transition reports as WPA3", rsnIE(akmSAE, akmPSK), capabilityPrivacy, "WPA3"},
		{"enterprise", rsnIE(akmDot1X), capabilityPrivacy, "Enterprise"},
		{"enterprise SHA256", rsnIE(akmDot1XSHA256), capabilityPrivacy, "Enterprise"},
		{"WPA3 enterprise Suite B", rsnIE(akmDot1XSuiteB192), capabilityPrivacy, "Enterprise"},
		{"OWE is not Open", rsnIE(akmOWE), capabilityPrivacy, "OWE"},
		{"original WPA", wpaVendor, capabilityPrivacy, "WPA"},
		// A non-WPA vendor element must not be mistaken for WPA.
		{"unrelated vendor element", ie(elemVendor, 0x00, 0x10, 0x18, 0x02), 0, "Open"},
		{"RSN wins over the legacy WPA element", append(rsnIE(akmPSK), wpaVendor...), capabilityPrivacy, "WPA2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := securityFromElements(parseElements(tt.blob), tt.capability)
			if got != tt.want {
				t.Errorf("securityFromElements = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWidthFromElements(t *testing.T) {
	t.Parallel()

	htNoSecondary := ie(elemHTOperation, 0x24, 0x00)
	htAbove := ie(elemHTOperation, 0x24, 0x01)
	htBelow := ie(elemHTOperation, 0x24, 0x03)

	tests := []struct {
		name string
		blob []byte
		want int
	}{
		{"no operation elements", ie(elemSSID, 'a'), width20MHz},
		{"HT with no secondary channel", htNoSecondary, width20MHz},
		{"HT bonded above", htAbove, width40MHz},
		{"HT bonded below", htBelow, width40MHz},
		{"VHT80", append(htAbove, ie(elemVHTOperation, 0x01, 0x2a, 0x00)...), width80MHz},
		{"VHT160", append(htAbove, ie(elemVHTOperation, 0x02, 0x32, 0x00)...), width160MHz},
		{"VHT80+80 counts as 80", append(htAbove, ie(elemVHTOperation, 0x03, 0x2a, 0x00)...), width80MHz},
		// VHT field 0 defers to HT, which is what says 40 here.
		{"VHT deferring to HT", append(htAbove, ie(elemVHTOperation, 0x00, 0x00, 0x00)...), width40MHz},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := widthFromElements(parseElements(tt.blob)); got != tt.want {
				t.Errorf("widthFromElements = %d, want %d", got, tt.want)
			}
		})
	}
}

// A beacon cut short mid-element still carries usable elements before the cut,
// and dropping the BSS entirely would lose a real AP from the survey.
func TestParseElementsTruncated(t *testing.T) {
	t.Parallel()

	blob := append(ie(elemSSID, 'o', 'k'), 61, 8, 0x01) // claims 8 bytes, carries 1
	elements := parseElements(blob)

	if len(elements) != 1 {
		t.Fatalf("elements = %d, want 1 (the complete one before the cut)", len(elements))
	}
	if got := ssidFromElements(elements); got != "ok" {
		t.Errorf("ssid = %q, want %q", got, "ok")
	}
}

func TestParseElementsEmpty(t *testing.T) {
	t.Parallel()

	for _, blob := range [][]byte{nil, {}, {elemSSID}} {
		if got := parseElements(blob); len(got) != 0 {
			t.Errorf("parseElements(%v) = %v, want none", blob, got)
		}
	}
}
