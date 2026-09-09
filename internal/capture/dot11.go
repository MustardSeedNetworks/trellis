// SPDX-License-Identifier: BUSL-1.1

//go:build linux || windows

package capture

import "net"

// 802.11 frame-control fields. A monitor interface delivers every frame in
// the air, so the type and subtype are the filter that keeps a survey to the
// two frames that actually describe a BSS.
const (
	dot11TypeManagement   = 0
	dot11SubtypeProbeResp = 5
	dot11SubtypeBeacon    = 8
	dot11FrameControlLen  = 2
	dot11ManagementHdrLen = 24
	dot11FixedParamsLen   = 12 // timestamp (8), beacon interval (2), capability (2)
	dot11CapabilityOffset = 10 // within the fixed parameters
	dot11FCSLen           = 4
)

// managementFrame is the part of a beacon or probe response that describes
// the BSS that sent it.
type managementFrame struct {
	bssid      net.HardwareAddr
	capability uint16
	elements   []element
}

// parseManagementFrame decodes a beacon or probe response.
//
// It reports false rather than an error for anything else, because "anything
// else" is the overwhelming majority of what a monitor interface delivers —
// data frames, acknowledgements, control frames and the tail ends of frames
// that began before the radio landed on the channel. Those are not faults and
// must not be logged as such.
//
// hasFCS comes from the radiotap header. When the driver leaves the trailing
// checksum on the frame it has to be removed before the elements are walked,
// or those four bytes are read as an element header and the last real element
// of every beacon is lost.
func parseManagementFrame(b []byte, hasFCS bool) (managementFrame, bool) {
	if hasFCS {
		if len(b) < dot11FCSLen {
			return managementFrame{}, false
		}
		b = b[:len(b)-dot11FCSLen]
	}
	if len(b) < dot11ManagementHdrLen+dot11FixedParamsLen {
		return managementFrame{}, false
	}

	// Frame control: bits 2-3 are the type, bits 4-7 the subtype.
	frameType := (b[0] >> 2) & 0x03
	subtype := (b[0] >> 4) & 0x0f
	if frameType != dot11TypeManagement {
		return managementFrame{}, false
	}
	if subtype != dot11SubtypeBeacon && subtype != dot11SubtypeProbeResp {
		return managementFrame{}, false
	}

	// Address 3 of a management frame is the BSSID. Address 2 is the
	// transmitter, which is the same radio for a beacon but not in general.
	bssid := make(net.HardwareAddr, 6)
	copy(bssid, b[16:22])

	fixed := b[dot11ManagementHdrLen:]
	capability := uint16(fixed[dot11CapabilityOffset]) |
		uint16(fixed[dot11CapabilityOffset+1])<<8

	return managementFrame{
		bssid:      bssid,
		capability: capability,
		elements:   parseElements(b[dot11ManagementHdrLen+dot11FixedParamsLen:]),
	}, true
}

// channelFromElements returns the channel the AP says it operates on.
//
// This is deliberately not the channel the radio was dwelling on when the
// frame arrived: an AP on channel 6 is heard on channels 4 through 8, and
// recording the dwell channel would file those observations against channels
// that have no AP on them — inventing co-channel interference in the survey's
// own output. The DS Parameter Set is the authority; the HT Operation
// element's primary channel is the fallback for APs that omit it.
func channelFromElements(elements []element) int {
	for _, e := range elements {
		if e.id == elemDSParameterSet && len(e.data) == 1 {
			return int(e.data[0])
		}
	}
	for _, e := range elements {
		if e.id == elemHTOperation && len(e.data) > 0 {
			return int(e.data[0])
		}
	}
	return 0
}
