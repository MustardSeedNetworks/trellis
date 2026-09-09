// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package capture

import (
	"testing"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/genetlink/genltest"
	"github.com/mdlayher/netlink"
)

// These tests drive the nl80211 commands against a fake kernel. They assert
// what actually goes on the wire, which no amount of hardware testing makes
// visible: a command that names the wrong attribute number still "works" on a
// real radio right up until it silently configures the wrong thing.

// decodeAttrs collects the flat attributes of a request for assertion.
//
// The harness calls the serving function a second time with an empty message
// once the exchange is done, so every test here records only the first
// request — otherwise the assertions run against that empty trailer.
func decodeAttrs(t *testing.T, data []byte) map[uint16][]byte {
	t.Helper()
	ad, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		t.Fatalf("decode request: %v", err)
	}
	attrs := make(map[uint16][]byte)
	for ad.Next() {
		attrs[ad.Type()] = ad.Bytes()
	}
	if err := ad.Err(); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	return attrs
}

func TestSetFrequencyRequest(t *testing.T) {
	var got map[uint16][]byte
	var command uint8

	conn := genltest.Dial(func(greq genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
		if got == nil && len(greq.Data) > 0 {
			command = greq.Header.Command
			got = decodeAttrs(t, greq.Data)
		}
		return nil, nil
	})
	defer func() { _ = conn.Close() }()

	if err := setFrequency(conn, testFamily(), 7, 2437); err != nil {
		t.Fatalf("setFrequency: %v", err)
	}

	if command != nl80211CmdSetWiphy {
		t.Errorf("command = %d, want NL80211_CMD_SET_WIPHY (%d)", command, nl80211CmdSetWiphy)
	}
	// The frequency must be carried on the interface, not the wiphy: sending
	// it without an ifindex changes the PHY's idea of the channel and leaves
	// the monitor interface where it was.
	if _, ok := got[nl80211AttrIfindex]; !ok {
		t.Error("request carries no NL80211_ATTR_IFINDEX")
	}
	if _, ok := got[nl80211AttrWiphyFreq]; !ok {
		t.Error("request carries no NL80211_ATTR_WIPHY_FREQ")
	}
	if v := nativeUint32(t, got[nl80211AttrWiphyFreq]); v != 2437 {
		t.Errorf("frequency = %d, want 2437", v)
	}
	if v := nativeUint32(t, got[nl80211AttrIfindex]); v != 7 {
		t.Errorf("ifindex = %d, want 7", v)
	}
}

func TestAddMonitorInterfaceRequest(t *testing.T) {
	var got map[uint16][]byte
	var command uint8

	conn := genltest.Dial(func(greq genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
		if got == nil && len(greq.Data) > 0 {
			command = greq.Header.Command
			got = decodeAttrs(t, greq.Data)
		}
		return nil, nil
	})
	defer func() { _ = conn.Close() }()

	// The interface is never really created here, so the lookup by name after
	// the command fails; the request itself is what this test is about.
	_, _ = addMonitorInterface(conn, testFamily(), 3)

	if command != nl80211CmdNewInterface {
		t.Errorf("command = %d, want NL80211_CMD_NEW_INTERFACE (%d)", command, nl80211CmdNewInterface)
	}
	if v := nativeUint32(t, got[nl80211AttrIftype]); v != nl80211IftypeMonitor {
		t.Errorf("iftype = %d, want monitor (%d)", v, nl80211IftypeMonitor)
	}
	if v := nativeUint32(t, got[nl80211AttrWiphy]); v != 3 {
		t.Errorf("wiphy = %d, want 3", v)
	}
	// The name is the ownership marker the crash-recovery path keys on, so a
	// drifting name would silently disable recovery.
	if name := string(trimNUL(got[nl80211AttrIfname])); name != monitorIfName {
		t.Errorf("ifname = %q, want %q", name, monitorIfName)
	}
}

func TestDelInterfaceRequest(t *testing.T) {
	var got map[uint16][]byte
	var command uint8

	conn := genltest.Dial(func(greq genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
		if got == nil && len(greq.Data) > 0 {
			command = greq.Header.Command
			got = decodeAttrs(t, greq.Data)
		}
		return nil, nil
	})
	defer func() { _ = conn.Close() }()

	if err := delInterface(conn, testFamily(), 9); err != nil {
		t.Fatalf("delInterface: %v", err)
	}

	if command != nl80211CmdDelInterface {
		t.Errorf("command = %d, want NL80211_CMD_DEL_INTERFACE (%d)", command, nl80211CmdDelInterface)
	}
	if v := nativeUint32(t, got[nl80211AttrIfindex]); v != 9 {
		t.Errorf("ifindex = %d, want 9", v)
	}
}

// TestChannelPlanRequestsSplitDump guards the flag that makes a wiphy dump
// complete. Without it the kernel truncates the reply for a complex device
// and the channel plan silently loses channels — a sweep that skips channels
// looks exactly like an airspace with no APs on them.
func TestChannelPlanRequestsSplitDump(t *testing.T) {
	var got map[uint16][]byte

	conn := genltest.Dial(func(greq genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
		if got == nil && len(greq.Data) > 0 {
			got = decodeAttrs(t, greq.Data)
		}
		return nil, nil
	})
	defer func() { _ = conn.Close() }()

	if _, err := channelPlan(conn, testFamily(), 0); err != nil {
		t.Fatalf("channelPlan: %v", err)
	}
	if _, ok := got[nl80211AttrSplitDump]; !ok {
		t.Error("wiphy dump does not set NL80211_ATTR_SPLIT_WIPHY_DUMP")
	}
}

func nativeUint32(t *testing.T, b []byte) uint32 {
	t.Helper()
	if len(b) != 4 {
		t.Fatalf("attribute is %d bytes, want 4", len(b))
	}
	ad, err := netlink.NewAttributeDecoder([]byte{4 + 4, 0, 1, 0, b[0], b[1], b[2], b[3]})
	if err != nil {
		t.Fatalf("decode uint32: %v", err)
	}
	for ad.Next() {
		return ad.Uint32()
	}
	t.Fatal("no attribute decoded")
	return 0
}

func trimNUL(b []byte) []byte {
	for i, c := range b {
		if c == 0 {
			return b[:i]
		}
	}
	return b
}

// testFamily stands in for the resolved nl80211 family. The ID must be
// non-zero: zero is the generic-netlink control family, which the test
// harness answers itself.
func testFamily() genetlink.Family {
	return genetlink.Family{ID: 26, Version: 1, Name: nl80211Family}
}
