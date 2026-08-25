// SPDX-License-Identifier: BUSL-1.1

//go:build linux || windows

package capture

import "encoding/binary"

// 802.11 information-element parsing.
//
// Linux (nl80211) and Windows (Native Wifi) both hand back the raw IE blob from
// a beacon or probe response and leave the interpretation to the caller, where
// CoreWLAN decodes it and reports SSID, security and width directly. This is
// that decoding, written once for both.

// Element IDs used here, from IEEE 802.11.
const (
	elemSSID         = 0
	elemHTOperation  = 61
	elemRSN          = 48
	elemVendor       = 221
	elemVHTOperation = 192
)

// Selector suites are OUI (3 bytes) + type (1 byte). 00-0F-AC is the IEEE
// suite used by RSN; 00-50-F2 is Microsoft's, used by the original WPA.
var (
	ouiIEEE      = [3]byte{0x00, 0x0f, 0xac}
	ouiMicrosoft = [3]byte{0x00, 0x50, 0xf2}
)

// RSN AKM suite types that decide which generation a network reports as.
const (
	akmDot1X          = 1 // WPA2-Enterprise
	akmPSK            = 2
	akmDot1XSHA256    = 5
	akmSAE            = 8  // WPA3-Personal
	akmDot1XSuiteB    = 11 // WPA3-Enterprise
	akmDot1XSuiteB192 = 12
	akmOWE            = 18 // Enhanced Open
)

// capabilityPrivacy is the Privacy bit in a BSS's capability field. Set without
// an RSN or WPA element, it means WEP.
const capabilityPrivacy = 0x0010

// element is one parsed information element.
type element struct {
	id   byte
	data []byte
}

// parseElements walks an IE blob. A truncated trailing element ends the walk
// rather than failing the scan: a beacon that was cut short still carries usable
// elements before the cut, and dropping the whole BSS would lose a real AP.
func parseElements(b []byte) []element {
	var elements []element
	for len(b) >= 2 {
		id, length := b[0], int(b[1])
		if len(b) < 2+length {
			break
		}
		elements = append(elements, element{id: id, data: b[2 : 2+length]})
		b = b[2+length:]
	}
	return elements
}

// ssidFromElements returns the network name, or "" for a hidden network.
//
// A hidden SSID is advertised either as a zero-length element or as one padded
// with NUL bytes; both mean "not telling", so both become "".
func ssidFromElements(elements []element) string {
	for _, e := range elements {
		if e.id != elemSSID {
			continue
		}
		for _, c := range e.data {
			if c != 0 {
				return string(e.data)
			}
		}
		return ""
	}
	return ""
}

// securityFromElements names the strongest scheme a BSS advertises, in the
// vocabulary core/wifi records.
//
// "Strongest" matters for transition modes: an AP offering both SAE and PSK is
// a WPA3 network that still accepts WPA2 clients, and reporting it as WPA2
// would understate what the site actually supports.
func securityFromElements(elements []element, capability uint16) string {
	var hasRSN, hasWPA, sae, owe, enterprise, psk bool

	for _, e := range elements {
		switch {
		case e.id == elemRSN:
			hasRSN = true
			for _, akm := range rsnAKMSuites(e.data) {
				switch akm {
				case akmSAE:
					sae = true
				case akmOWE:
					owe = true
				case akmPSK:
					psk = true
				case akmDot1X, akmDot1XSHA256, akmDot1XSuiteB, akmDot1XSuiteB192:
					enterprise = true
				}
			}
		case e.id == elemVendor && isWPAVendorElement(e.data):
			hasWPA = true
		}
	}

	switch {
	case sae:
		return "WPA3"
	case owe:
		// Enhanced Open: unauthenticated but encrypted. Calling it "Open"
		// would put it beside a genuinely unencrypted network on a report.
		return "OWE"
	case hasRSN && enterprise && !psk:
		return "Enterprise"
	case hasRSN:
		return "WPA2"
	case hasWPA:
		return "WPA"
	case capability&capabilityPrivacy != 0:
		return "WEP"
	default:
		return "Open"
	}
}

// rsnAKMSuites extracts the AKM suite types from an RSN element, skipping the
// version, group cipher and pairwise cipher list that precede them.
func rsnAKMSuites(data []byte) []byte {
	const (
		versionLen = 2
		suiteLen   = 4
		countLen   = 2
	)

	offset := versionLen + suiteLen // version + group cipher suite
	if len(data) < offset+countLen {
		return nil
	}

	pairwiseCount := int(binary.LittleEndian.Uint16(data[offset:]))
	offset += countLen + pairwiseCount*suiteLen
	if len(data) < offset+countLen {
		return nil
	}

	akmCount := int(binary.LittleEndian.Uint16(data[offset:]))
	offset += countLen

	var types []byte
	for i := 0; i < akmCount; i++ {
		start := offset + i*suiteLen
		if len(data) < start+suiteLen {
			break
		}
		if [3]byte(data[start:start+3]) == ouiIEEE {
			types = append(types, data[start+3])
		}
	}
	return types
}

// isWPAVendorElement reports whether a vendor element is the original WPA one:
// Microsoft's OUI with vendor type 1.
func isWPAVendorElement(data []byte) bool {
	const wpaVendorType = 1
	return len(data) >= 4 && [3]byte(data[0:3]) == ouiMicrosoft && data[3] == wpaVendorType
}

// widthFromElements returns the channel width in MHz a BSS advertises.
//
// HE and EHT operation elements encode width differently again and are not
// decoded: an 802.11ax or 802.11be AP reports the width its VHT element claims,
// which is correct for 5 GHz and understates 6 GHz-only APs at 20 MHz. That is
// a known gap rather than a silent one — 6 GHz hardware to verify against is
// what it needs.
func widthFromElements(elements []element) int {
	width := width20MHz

	for _, e := range elements {
		switch e.id {
		case elemHTOperation:
			// Byte 1, bits 0-1: secondary channel offset. Above (1) or below
			// (3) means the AP is bonding two 20 MHz channels.
			if len(e.data) >= 2 && e.data[1]&0x03 != 0 {
				width = max(width, width40MHz)
			}
		case elemVHTOperation:
			if len(e.data) >= 1 {
				width = max(width, vhtWidth(e.data[0]))
			}
		}
	}
	return width
}

// vhtWidth maps a VHT Operation element's channel-width field onto MHz.
func vhtWidth(field byte) int {
	const (
		vht20or40 = 0
		vht80     = 1
		vht160    = 2
		vht80p80  = 3
	)
	switch field {
	case vht80, vht80p80:
		return width80MHz
	case vht160:
		return width160MHz
	case vht20or40:
		// The HT element already decided between 20 and 40.
		return width20MHz
	default:
		return width20MHz
	}
}
