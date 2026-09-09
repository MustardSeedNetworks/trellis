// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package capture

import (
	"net"
	"testing"
	"time"

	"github.com/mdlayher/netlink"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// signalOffset is where the dBm antenna signal sits in the captured fixture's
// radiotap header, so a test can vary the one field it cares about.
const signalOffset = 22

// frameWithSignal returns the captured beacon with its signal reading
// replaced, which is how a test stands in for the same AP heard twice at
// different strengths.
func frameWithSignal(t *testing.T, dBm int8) []byte {
	t.Helper()
	frame := append([]byte(nil), mustHex(t, realBeaconFrame)...)
	frame[signalOffset] = byte(dBm)
	return frame
}

func TestNetworkFromFrame(t *testing.T) {
	network, ok := networkFromFrame(mustHex(t, realBeaconFrame), time.Now())
	if !ok {
		t.Fatal("networkFromFrame returned false for a real beacon")
	}

	if got, want := network.BSSID, "24:5a:4c:6b:b5:c8"; got != want {
		t.Errorf("BSSID = %q, want %q", got, want)
	}
	if got, want := network.SSID, "Neuroplasticity"; got != want {
		t.Errorf("SSID = %q, want %q", got, want)
	}
	if got, want := network.Channel, 6; got != want {
		t.Errorf("Channel = %d, want %d", got, want)
	}
	// The frequency is derived from the channel the AP claimed, not from the
	// radiotap header, so the two cannot disagree in stored output.
	if got, want := network.Frequency, 2437; got != want {
		t.Errorf("Frequency = %d, want %d", got, want)
	}
	if got, want := network.Signal, -14; got != want {
		t.Errorf("Signal = %d, want %d", got, want)
	}
	// No driver noise reading, so the shared assumed floor applies and SNR is
	// derived from it rather than from 0 dBm.
	if got, want := network.NoiseFloor, defaultNoiseFloorDBm; got != want {
		t.Errorf("NoiseFloor = %d, want %d", got, want)
	}
	if got, want := network.SNR, -14-defaultNoiseFloorDBm; got != want {
		t.Errorf("SNR = %d, want %d", got, want)
	}
	// 2.4 GHz has no DFS channels at all.
	if network.IsDFS {
		t.Error("IsDFS = true, want false on 2.4 GHz")
	}
}

func TestNetworkFromFrameRejects(t *testing.T) {
	// A radiotap header with no signal field: present = flags only. Recording
	// this at 0 dBm would put a fictional very strong AP at the survey point.
	noSignal := []byte{0, 0, 10, 0, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00}
	noSignal = append(noSignal, mustHex(t, realBeaconFrame)[26:]...)

	tests := []struct {
		name string
		in   []byte
	}{
		{"not a frame", []byte{1, 2, 3}},
		{"no signal reading", noSignal},
		{"radiotap header only", mustHex(t, realBeaconFrame)[:26]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := networkFromFrame(tt.in, time.Now()); ok {
				t.Fatal("networkFromFrame returned true, want false")
			}
		})
	}
}

// TestCollectKeepsStrongestReading drives the dwell loop over a socket pair
// rather than a radio, which is enough to exercise the real read path: the
// same BSS heard twice must keep its better reading, because a survey point
// asks how well an AP reaches this spot and the weaker sample is a missed
// beacon or an adjacent-channel echo.
func TestCollectKeepsStrongestReading(t *testing.T) {
	reader, writer := framePipe(t)

	// Weaker first, so a last-one-wins implementation fails this.
	writeFrame(t, writer, frameWithSignal(t, -70))
	writeFrame(t, writer, frameWithSignal(t, -30))
	writeFrame(t, writer, frameWithSignal(t, -55))

	strongest := make(map[string]wifi.ScannedNetwork)
	collect(reader, strongest)

	if len(strongest) != 1 {
		t.Fatalf("collected %d BSSs, want 1", len(strongest))
	}
	got := strongest["24:5a:4c:6b:b5:c8"]
	if got.Signal != -30 {
		t.Errorf("Signal = %d, want -30 (the strongest of the three)", got.Signal)
	}
}

// TestCollectIgnoresUndecodableFrames proves the loop survives the traffic a
// monitor interface actually delivers. Most frames on a busy channel are data
// and control frames, and a sweep that treated them as faults would abandon
// the dwell on the first one.
func TestCollectIgnoresUndecodableFrames(t *testing.T) {
	reader, writer := framePipe(t)

	writeFrame(t, writer, []byte{0xff, 0xff, 0xff})
	writeFrame(t, writer, frameWithSignal(t, -40))
	writeFrame(t, writer, make([]byte, 64))

	strongest := make(map[string]wifi.ScannedNetwork)
	collect(reader, strongest)

	if len(strongest) != 1 {
		t.Fatalf("collected %d BSSs, want 1", len(strongest))
	}
}

// framePipe returns a read fd and a write fd behaving like the packet socket
// collect reads from: datagram boundaries preserved, and a receive timeout.
func framePipe(t *testing.T) (int, int) {
	t.Helper()
	fds, err := unixSocketpair()
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	t.Cleanup(func() {
		_ = closeFD(fds[0])
		_ = closeFD(fds[1])
	})
	return fds[0], fds[1]
}

func writeFrame(t *testing.T, fd int, frame []byte) {
	t.Helper()
	if err := writeFD(fd, frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func TestAppendFrequencySkipsDisabledAndOutOfBand(t *testing.T) {
	tests := []struct {
		name     string
		freq     uint32
		disabled bool
		want     []int
	}{
		{"usable 2.4 GHz channel", 2437, false, []int{2437}},
		// A disabled channel is one the regulatory domain forbids here.
		{"disabled", 2437, true, nil},
		// 5 GHz is out of scope until an adapter that can monitor it exists;
		// sweeping it would spend dwell time on channels this radio cannot
		// tune and report the band as empty rather than unsupported.
		{"5 GHz", 5180, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ae := netlink.NewAttributeEncoder()
			ae.Uint32(nl80211FreqAttrFreq, tt.freq)
			if tt.disabled {
				ae.Flag(nl80211FreqAttrDisabled, true)
			}
			encoded, err := ae.Encode()
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			ad, err := netlink.NewAttributeDecoder(encoded)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			var freqs []int
			appendFrequency(ad, make(map[int]struct{}), &freqs)

			if len(freqs) != len(tt.want) {
				t.Fatalf("freqs = %v, want %v", freqs, tt.want)
			}
			for i := range freqs {
				if freqs[i] != tt.want[i] {
					t.Errorf("freqs = %v, want %v", freqs, tt.want)
				}
			}
		})
	}
}

// TestAppendFrequencyDeduplicates matters because a split wiphy dump can
// repeat a band across messages; counting a channel twice would make every
// sweep dwell on it twice.
func TestAppendFrequencyDeduplicates(t *testing.T) {
	ae := netlink.NewAttributeEncoder()
	ae.Uint32(nl80211FreqAttrFreq, 2412)
	encoded, err := ae.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	seen := make(map[int]struct{})
	var freqs []int
	for range 2 {
		ad, decErr := netlink.NewAttributeDecoder(encoded)
		if decErr != nil {
			t.Fatalf("decode: %v", decErr)
		}
		appendFrequency(ad, seen, &freqs)
	}

	if len(freqs) != 1 {
		t.Errorf("freqs = %v, want one entry", freqs)
	}
}

func TestAlignUp(t *testing.T) {
	tests := []struct{ offset, align, want int }{
		{0, 8, 0},
		{1, 8, 8},
		{8, 8, 8},
		{9, 2, 10},
		{17, 1, 17},
	}
	for _, tt := range tests {
		if got := alignUp(tt.offset, tt.align); got != tt.want {
			t.Errorf("alignUp(%d, %d) = %d, want %d", tt.offset, tt.align, got, tt.want)
		}
	}
}

// TestHtons pins the byte swap AF_PACKET needs. Getting it wrong binds the
// socket to a protocol nothing sends, so the sweep reads no frames at all
// and reports an empty airspace rather than an error.
func TestHtons(t *testing.T) {
	if got, want := htons(0x0003), uint16(0x0300); got != want {
		t.Errorf("htons(0x0003) = %#04x, want %#04x", got, want)
	}
}

// TestInterfaceIndexUnknown proves a missing interface is an error rather
// than index 0, which is what the stale-interface cleanup relies on to tell
// "no leftover" from "a leftover at index 0".
func TestInterfaceIndexUnknown(t *testing.T) {
	if _, err := interfaceIndex("trellis-no-such-interface"); err == nil {
		t.Fatal("interfaceIndex succeeded for a missing interface")
	}
}

// TestSetInterfaceUpNoOp covers the path where the link is already in the
// wanted state: the restore function must be a no-op, so a sweep cannot
// change an interface it never had to touch.
func TestSetInterfaceUpNoOp(t *testing.T) {
	lo, err := net.InterfaceByName("lo")
	if err != nil {
		t.Skipf("no loopback interface: %v", err)
	}
	if lo.Flags&net.FlagUp == 0 {
		t.Skip("loopback is down on this host")
	}

	restore, err := setInterfaceUp("lo", true)
	if err != nil {
		t.Fatalf("setInterfaceUp: %v", err)
	}
	if err := restore(); err != nil {
		t.Errorf("restore: %v", err)
	}

	after, err := net.InterfaceByName("lo")
	if err != nil {
		t.Fatalf("re-read lo: %v", err)
	}
	if after.Flags&net.FlagUp == 0 {
		t.Error("loopback was brought down by a call that should have done nothing")
	}
}
