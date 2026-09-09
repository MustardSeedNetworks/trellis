// SPDX-License-Identifier: BUSL-1.1

package capture

import "testing"

// TestParseManagementFrameRealBeacon decodes the captured beacon past its
// radiotap header. The expected values were read off the same frame with
// tcpdump, so this asserts agreement with an independent decoder rather than
// with the parser's own output.
func TestParseManagementFrameRealBeacon(t *testing.T) {
	raw := mustHex(t, realBeaconFrame)
	rt, err := parseRadiotap(raw)
	if err != nil {
		t.Fatalf("parseRadiotap: %v", err)
	}

	frame, ok := parseManagementFrame(raw[rt.length:], rt.hasFCS)
	if !ok {
		t.Fatal("parseManagementFrame returned false for a real beacon")
	}

	if got, want := frame.bssid.String(), "24:5a:4c:6b:b5:c8"; got != want {
		t.Errorf("bssid = %q, want %q", got, want)
	}
	if got, want := ssidFromElements(frame.elements), "Neuroplasticity"; got != want {
		t.Errorf("ssid = %q, want %q", got, want)
	}
	// The DS Parameter Set, not the channel the radio was dwelling on: an AP
	// on channel 6 is heard on adjacent channels too, and recording the dwell
	// channel would file those observations against the wrong channel.
	if got, want := channelFromElements(frame.elements), 6; got != want {
		t.Errorf("channel = %d, want %d", got, want)
	}
	if got, want := securityFromElements(frame.elements, frame.capability), "WPA3"; got != want {
		t.Errorf("security = %q, want %q", got, want)
	}
}

func TestParseManagementFrameRejectsNonBeacons(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"shorter than a management header", make([]byte, 20)},
		// Frame control 0x0008: type 2 (data), subtype 0. A survey must not
		// mine data frames for BSSs — they carry no SSID or capability.
		{"data frame", append([]byte{0x08, 0x00}, make([]byte, 40)...)},
		// Type 0 subtype 4 is a probe REQUEST, sent by clients. Treating one
		// as a BSS would invent an AP at the surveyor's own position.
		{"probe request", append([]byte{0x40, 0x00}, make([]byte, 40)...)},
		// A beacon truncated before its fixed parameters.
		{"beacon without fixed params", append([]byte{0x80, 0x00}, make([]byte, 28)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := parseManagementFrame(tt.in, false); ok {
				t.Fatal("parseManagementFrame returned true, want false")
			}
		})
	}
}

// TestParseManagementFrameProbeResponse proves probe responses are accepted:
// a hidden-SSID AP answers a probe with the name it withholds from beacons,
// and dropping those would leave hidden BSSs unnamed in every survey.
func TestParseManagementFrameProbeResponse(t *testing.T) {
	raw := mustHex(t, realBeaconFrame)
	rt, err := parseRadiotap(raw)
	if err != nil {
		t.Fatalf("parseRadiotap: %v", err)
	}
	body := append([]byte(nil), raw[rt.length:]...)
	body[0] = 0x50 // type 0, subtype 5: probe response

	if _, ok := parseManagementFrame(body, false); !ok {
		t.Fatal("parseManagementFrame rejected a probe response")
	}
}

// TestParseManagementFrameStripsFCS proves the trailing checksum is removed
// before the elements are walked. With the FCS left on, the four extra bytes
// are parsed as an element header and the final real element is lost.
func TestParseManagementFrameStripsFCS(t *testing.T) {
	raw := mustHex(t, realBeaconFrame)
	rt, err := parseRadiotap(raw)
	if err != nil {
		t.Fatalf("parseRadiotap: %v", err)
	}
	body := raw[rt.length:]

	withFCS := append(append([]byte(nil), body...), 0xde, 0xad, 0xbe, 0xef)
	stripped, ok := parseManagementFrame(withFCS, true)
	if !ok {
		t.Fatal("parseManagementFrame returned false")
	}
	plain, ok := parseManagementFrame(body, false)
	if !ok {
		t.Fatal("parseManagementFrame returned false for the plain frame")
	}
	if len(stripped.elements) != len(plain.elements) {
		t.Errorf("elements with FCS stripped = %d, want %d",
			len(stripped.elements), len(plain.elements))
	}
}
