// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package capture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// nl80211Scanner reads the host adapter through nl80211 over generic netlink.
//
// Pure Go: unlike the macOS backend this links no C, which is why R5's cgo
// boundary costs Linux nothing (ADR-0006).
//
// Triggering a scan requires CAP_NET_ADMIN. Reading the kernel's cached results
// does not, but this backend does not quietly fall back to them — a cache is
// stale by an unknown amount and empty on a host where nothing else scans, so a
// survey built from it would record measurements that were never taken at the
// point they are attributed to.
type nl80211Scanner struct{}

// nl80211 generic-netlink family and the multicast group that announces a
// finished scan.
const (
	nl80211Family            = "nl80211"
	nl80211GroupScan         = "scan"
	nl80211CmdGetInterface   = 5
	nl80211CmdGetScan        = 32
	nl80211CmdTriggerScan    = 33
	nl80211CmdNewScanResults = 34
	nl80211CmdScanAborted    = 35
)

// Attributes of an nl80211 message (enum nl80211_attrs).
const (
	nl80211AttrIfindex   = 3
	nl80211AttrIfname    = 4
	nl80211AttrIftype    = 5
	nl80211AttrScanSSIDs = 45
	nl80211AttrBSS       = 47
)

// Attributes inside NL80211_ATTR_BSS (enum nl80211_bss).
const (
	nl80211BSSBSSID               = 1
	nl80211BSSFrequency           = 2
	nl80211BSSCapability          = 5
	nl80211BSSInformationElements = 6
	nl80211BSSSignalMBM           = 7
)

// nl80211IftypeStation is the only interface type worth surveying from: a
// monitor or AP interface reports a radio that is not scanning for us.
const nl80211IftypeStation = 2

// scanTimeout bounds the wait for the kernel's scan-complete notification. A
// full multi-band scan with DFS channels can take several seconds; past this
// the radio is not coming back and reporting that beats blocking a survey.
const scanTimeout = 20 * time.Second

// mBmPerDBm converts nl80211's signal unit. It reports milli-dBm so it can
// carry fractions in an integer.
const mBmPerDBm = 100

// New returns the host's capture backend, failing if this kernel has no
// nl80211 or the host has no wireless interface to survey from.
func New() (Scanner, error) {
	conn, family, err := dial()
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	if _, _, err := wirelessInterface(conn, family); err != nil {
		return nil, err
	}
	return nl80211Scanner{}, nil
}

// Authorize has nothing to ask for: Linux gates scanning on CAP_NET_ADMIN,
// which is granted to the process before it starts, not requested at runtime.
// A missing capability surfaces from [nl80211Scanner.Scan] as [ErrPermission].
func Authorize() error { return nil }

// Scan implements [Scanner].
//
// The interface is resolved on every scan rather than cached at construction,
// so a USB adapter plugged in after trellisd started is picked up without a
// restart — which is how survey adapters are actually used.
func (nl80211Scanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	conn, family, err := dial()
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	ifIndex, ifName, err := wirelessInterface(conn, family)
	if err != nil {
		return nil, err
	}

	if err := triggerScan(ctx, conn, family, ifIndex, ifName); err != nil {
		return nil, err
	}
	return scanResults(conn, family, ifIndex)
}

// dial opens generic netlink and resolves the nl80211 family.
func dial() (*genetlink.Conn, genetlink.Family, error) {
	conn, err := genetlink.Dial(nil)
	if err != nil {
		return nil, genetlink.Family{}, fmt.Errorf("capture: open netlink: %w", err)
	}

	family, err := conn.GetFamily(nl80211Family)
	if err != nil {
		_ = conn.Close()
		if errors.Is(err, os.ErrNotExist) {
			// No nl80211 at all: a kernel built without cfg80211, or a
			// container without the wireless stack.
			return nil, genetlink.Family{}, ErrNoInterface
		}
		return nil, genetlink.Family{}, fmt.Errorf("capture: resolve nl80211: %w", err)
	}
	return conn, family, nil
}

// wirelessInterface returns the first station-mode interface the kernel
// reports, which is the one a survey walks with.
func wirelessInterface(conn *genetlink.Conn, family genetlink.Family) (uint32, string, error) {
	msgs, err := conn.Execute(
		genetlink.Message{Header: genetlink.Header{Command: nl80211CmdGetInterface, Version: family.Version}},
		family.ID,
		netlink.Request|netlink.Dump,
	)
	if err != nil {
		return 0, "", fmt.Errorf("capture: list wireless interfaces: %w", err)
	}

	for _, msg := range msgs {
		ad, err := netlink.NewAttributeDecoder(msg.Data)
		if err != nil {
			return 0, "", fmt.Errorf("capture: decode interface: %w", err)
		}

		var (
			index  uint32
			name   string
			ifType uint32
			// Absent NL80211_ATTR_IFTYPE is treated as a station rather than
			// skipped: some drivers omit it, and refusing those would report
			// "no Wi-Fi interface" on a host that plainly has one.
			haveType bool
		)
		for ad.Next() {
			switch ad.Type() {
			case nl80211AttrIfindex:
				index = ad.Uint32()
			case nl80211AttrIfname:
				name = ad.String()
			case nl80211AttrIftype:
				ifType, haveType = ad.Uint32(), true
			}
		}
		if err := ad.Err(); err != nil {
			return 0, "", fmt.Errorf("capture: decode interface: %w", err)
		}

		if index != 0 && (!haveType || ifType == nl80211IftypeStation) {
			return index, name, nil
		}
	}
	return 0, "", ErrNoInterface
}

// triggerScan asks the kernel to sweep every supported channel and waits for it
// to say the sweep finished.
//
// The subscription is a *second* netlink socket, not this one. Multicast
// notifications carry sequence number 0, so a group joined on the command
// socket interleaves them with the request's own reply and netlink rejects the
// exchange with "mismatched sequence in netlink reply" — which is what happened
// the first time this ran against real hardware.
func triggerScan(
	ctx context.Context,
	conn *genetlink.Conn,
	family genetlink.Family,
	ifIndex uint32,
	ifName string,
) error {
	// Subscribed before requesting: a scan over cached channels can finish
	// quickly, and a subscription opened afterwards would miss the
	// notification and wait out the whole timeout.
	listener, err := subscribeScan(family)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	ae := netlink.NewAttributeEncoder()
	ae.Uint32(nl80211AttrIfindex, ifIndex)
	// One zero-length SSID is a wildcard probe request: an active scan, which
	// is what CoreWLAN does on macOS. Omitting this attribute entirely would
	// scan passively and see fewer APs, making the same site look different
	// depending on which host surveyed it.
	ae.Nested(nl80211AttrScanSSIDs, func(nae *netlink.AttributeEncoder) error {
		nae.Bytes(1, nil)
		return nil
	})
	attrs, err := ae.Encode()
	if err != nil {
		return fmt.Errorf("capture: encode scan request: %w", err)
	}

	_, err = conn.Execute(
		genetlink.Message{
			Header: genetlink.Header{Command: nl80211CmdTriggerScan, Version: family.Version},
			Data:   attrs,
		},
		family.ID,
		netlink.Request|netlink.Acknowledge,
	)
	if err != nil {
		if errors.Is(err, unix.EPERM) {
			return fmt.Errorf("%w: triggering a scan on %s needs CAP_NET_ADMIN", ErrPermission, ifName)
		}
		if errors.Is(err, unix.EBUSY) {
			// Another process is mid-scan. Its results arrive on the
			// subscription already open, so this is not an error.
			return waitForScan(ctx, listener)
		}
		return fmt.Errorf("capture: trigger scan on %s: %w", ifName, err)
	}
	return waitForScan(ctx, listener)
}

// subscribeScan opens a netlink socket joined to the group the kernel announces
// scan completion on.
func subscribeScan(family genetlink.Family) (*genetlink.Conn, error) {
	var group uint32
	for _, g := range family.Groups {
		if g.Name == nl80211GroupScan {
			group = g.ID
		}
	}
	if group == 0 {
		return nil, errors.New("capture: nl80211 has no scan multicast group")
	}

	listener, err := genetlink.Dial(nil)
	if err != nil {
		return nil, fmt.Errorf("capture: open scan subscription: %w", err)
	}
	if err := listener.JoinGroup(group); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("capture: subscribe to scan results: %w", err)
	}
	return listener, nil
}

// waitForScan blocks until the kernel reports the sweep finished or aborted.
func waitForScan(ctx context.Context, listener *genetlink.Conn) error {
	deadline := time.Now().Add(scanTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := listener.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("capture: set scan deadline: %w", err)
	}

	for {
		msgs, _, err := listener.Receive()
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return fmt.Errorf("capture: scan did not finish within %s", scanTimeout)
			}
			return fmt.Errorf("capture: await scan results: %w", err)
		}

		for _, msg := range msgs {
			switch msg.Header.Command {
			case nl80211CmdNewScanResults:
				return nil
			case nl80211CmdScanAborted:
				// Aborted usually means the radio was needed for something
				// else. Whatever it managed to see is still in the cache.
				return nil
			}
		}
	}
}

// scanResults dumps the kernel's BSS list for an interface.
func scanResults(conn *genetlink.Conn, family genetlink.Family, ifIndex uint32) ([]wifi.ScannedNetwork, error) {
	ae := netlink.NewAttributeEncoder()
	ae.Uint32(nl80211AttrIfindex, ifIndex)
	attrs, err := ae.Encode()
	if err != nil {
		return nil, fmt.Errorf("capture: encode scan dump: %w", err)
	}

	msgs, err := conn.Execute(
		genetlink.Message{
			Header: genetlink.Header{Command: nl80211CmdGetScan, Version: family.Version},
			Data:   attrs,
		},
		family.ID,
		netlink.Request|netlink.Dump,
	)
	if err != nil {
		return nil, fmt.Errorf("capture: read scan results: %w", err)
	}

	seen := time.Now().UTC()
	networks := make([]wifi.ScannedNetwork, 0, len(msgs))
	for _, msg := range msgs {
		network, ok, err := networkFromBSS(msg.Data, seen)
		if err != nil {
			return nil, err
		}
		if ok {
			networks = append(networks, network)
		}
	}
	return networks, nil
}

// networkFromBSS maps one dumped BSS onto Trellis's scan model. A message
// carrying no BSS attribute is not an error — the dump interleaves other
// message types — so it reports ok=false instead.
func networkFromBSS(data []byte, seen time.Time) (wifi.ScannedNetwork, bool, error) {
	ad, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return wifi.ScannedNetwork{}, false, fmt.Errorf("capture: decode BSS: %w", err)
	}

	var (
		bssid      net.HardwareAddr
		freqMHz    int
		signalDBm  int
		capability uint16
		elements   []element
		found      bool
	)

	for ad.Next() {
		if ad.Type() != nl80211AttrBSS {
			continue
		}
		found = true
		ad.Nested(func(nad *netlink.AttributeDecoder) error {
			for nad.Next() {
				switch nad.Type() {
				case nl80211BSSBSSID:
					bssid = net.HardwareAddr(nad.Bytes())
				case nl80211BSSFrequency:
					freqMHz = int(nad.Uint32())
				case nl80211BSSSignalMBM:
					// Int32, not Uint32: NL80211_BSS_SIGNAL_MBM is signed, and
					// reinterpreting the unsigned form would turn every real
					// (negative) reading into a huge positive one.
					signalDBm = int(nad.Int32()) / mBmPerDBm
				case nl80211BSSCapability:
					capability = nad.Uint16()
				case nl80211BSSInformationElements:
					elements = parseElements(nad.Bytes())
				}
			}
			return nil
		})
	}
	if err := ad.Err(); err != nil {
		return wifi.ScannedNetwork{}, false, fmt.Errorf("capture: decode BSS: %w", err)
	}
	if !found || len(bssid) == 0 {
		return wifi.ScannedNetwork{}, false, nil
	}

	channel, band := channelForFrequency(freqMHz)
	width := widthFromElements(elements)

	return wifi.ScannedNetwork{
		SSID:         ssidFromElements(elements),
		BSSID:        bssid.String(),
		Signal:       signalDBm,
		Channel:      channel,
		Frequency:    freqMHz,
		Security:     securityFromElements(elements, capability),
		ChannelWidth: width,
		// nl80211 reports no noise measurement with scan results, so the SNR
		// below is derived from the same assumed floor the macOS backend uses
		// when CoreWLAN omits one. Keeping them consistent matters more than
		// either being exact: a survey compares points, not absolutes.
		NoiseFloor: defaultNoiseFloorDBm,
		SNR:        signalDBm - defaultNoiseFloorDBm,
		HTMode:     htModeForWidth(width),
		IsDFS:      band == band5GHz && isDFSChannel(channel),
		LastSeen:   seen,
	}, true, nil
}
