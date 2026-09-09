// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package capture

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
)

// TestCollectBandFrequenciesKeepsOnly24GHz walks the nested band structure
// nl80211 actually returns. The 5 GHz entry must be dropped: no adapter here
// can monitor it, and sweeping it would spend dwell time on channels the
// radio cannot tune, then report the band as empty rather than unsupported.
func TestCollectBandFrequenciesKeepsOnly24GHz(t *testing.T) {
	ae := netlink.NewAttributeEncoder()
	ae.Nested(1, func(band *netlink.AttributeEncoder) error {
		band.Nested(nl80211BandAttrFreqs, func(list *netlink.AttributeEncoder) error {
			list.Nested(1, func(f *netlink.AttributeEncoder) error {
				f.Uint32(nl80211FreqAttrFreq, 2412)
				return nil
			})
			list.Nested(2, func(f *netlink.AttributeEncoder) error {
				f.Uint32(nl80211FreqAttrFreq, 2437)
				return nil
			})
			list.Nested(3, func(f *netlink.AttributeEncoder) error {
				f.Uint32(nl80211FreqAttrFreq, 5180)
				return nil
			})
			return nil
		})
		return nil
	})
	encoded, err := ae.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	ad, err := netlink.NewAttributeDecoder(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	var freqs []int
	collectBandFrequencies(ad, make(map[int]struct{}), &freqs)

	want := []int{2412, 2437}
	if len(freqs) != len(want) {
		t.Fatalf("freqs = %v, want %v", freqs, want)
	}
	for i := range want {
		if freqs[i] != want[i] {
			t.Fatalf("freqs = %v, want %v", freqs, want)
		}
	}
}

// TestSweepHonoursCancellation proves a cancelled context stops a sweep
// between channels. A survey that is cancelled mid-walk must not keep the
// operator's radio for another three seconds.
func TestSweepHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The channel list is non-empty so the loop would run if cancellation
	// were not checked; a nil connection would panic if it reached the radio.
	if _, err := sweep(ctx, nil, genetlinkFamilyZero(), 0, 0, []int{2412}); !errors.Is(err, context.Canceled) {
		t.Fatalf("sweep error = %v, want context.Canceled", err)
	}
}

// TestSweepWithNoChannels covers the empty plan: no channel to visit means no
// BSSs, not a nil slice a caller has to special-case.
func TestSweepWithNoChannels(t *testing.T) {
	networks, err := sweep(context.Background(), nil, genetlinkFamilyZero(), 0, 0, nil)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if networks == nil {
		t.Error("networks = nil, want an empty slice")
	}
	if len(networks) != 0 {
		t.Errorf("networks = %v, want empty", networks)
	}
}

// TestOpenPacketSocketWithoutPrivilege pins the permission mapping. An
// unprivileged sweep has to report that the OS refused it, because an empty
// BSS list would be recorded as a genuine dead spot in the survey.
func TestOpenPacketSocketWithoutPrivilege(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; this asserts the unprivileged path")
	}
	if _, err := openPacketSocket(1); !errors.Is(err, ErrPermission) {
		t.Fatalf("openPacketSocket error = %v, want ErrPermission", err)
	}
}

func TestWriteInterfaceFlagUnknownInterface(t *testing.T) {
	if err := writeInterfaceFlag("trellis-no-such-interface", true); err == nil {
		t.Fatal("writeInterfaceFlag succeeded for a missing interface")
	}
}

// genetlinkFamilyZero is the zero family, used by the sweep tests that return
// before any netlink command is sent.
func genetlinkFamilyZero() genetlink.Family { return genetlink.Family{} }
