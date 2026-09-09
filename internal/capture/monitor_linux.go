// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package capture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// nl80211 commands and attributes used only by the monitor backend.
const (
	nl80211CmdGetWiphy      = 1
	nl80211CmdSetWiphy      = 2
	nl80211CmdNewInterface  = 7
	nl80211CmdDelInterface  = 8
	nl80211AttrWiphy        = 1
	nl80211AttrWiphyFreq    = 38
	nl80211AttrWiphyBands   = 22
	nl80211AttrSplitDump    = 174
	nl80211BandAttrFreqs    = 1
	nl80211FreqAttrFreq     = 1
	nl80211FreqAttrDisabled = 2
	nl80211IftypeMonitor    = 6
)

// monitorIfName is the monitor interface Trellis creates, and the marker for
// what it owns. A crashed survey cannot run its own cleanup, so the next run
// removes any interface with this name before creating its own: the name is
// the only durable record that the interface was ours rather than an
// operator's own monitor session.
const monitorIfName = "trlmon0"

// dwellPerChannel is how long the radio listens on each channel.
//
// APs beacon every 102.4 ms by default, so this is roughly two and a half
// beacon intervals — enough that missing one still leaves a second chance,
// without paying for a third. Across the fourteen 2.4 GHz channels a sweep
// costs about 3.5 s, the same order as the OS scan it replaces. It is a
// constant rather than a setting because a shorter dwell silently loses APs
// and a longer one silently ages the point a surveyor is standing on, and
// neither failure is visible in the output.
const dwellPerChannel = 250 * time.Millisecond

// band24GHzMaxFreq bounds the channel plan to 2.4 GHz. The only adapter with
// proven monitor mode is 2.4 GHz-only (#11); 5 and 6 GHz wait for hardware
// rather than shipping a channel plan nothing has ever swept.
const band24GHzMaxFreq = 2500

// monitorFrameBuffer is sized past the largest 802.11 frame plus its radiotap
// header. Frames are read one per syscall, so this is a per-scan cost, not a
// per-frame one.
const monitorFrameBuffer = 4096

// monitorScanner acquires BSSs by listening in monitor mode rather than
// asking the OS for its own scan results.
//
// The difference that matters to a survey is control of time and channel. An
// OS scan runs on the driver's schedule and returns a merged cache: the
// macOS backend measured 19 points where Linux measured 40 over the same two
// minutes (#299), and neither can say which channel a reading came from.
// Monitor mode dwells on one channel at a time for a known interval, so every
// observation is attributable to a channel and a moment.
//
// The cost is the radio: monitor mode takes the adapter away from its
// association, so this backend cannot run during an active (throughput)
// survey and is wrong for a host whose only radio is also its uplink.
type monitorScanner struct{}

// NewMonitor returns a monitor-mode scanner.
//
// It reports [ErrUnsupported] when the adapter's driver has no monitor mode —
// which is not exotic: brcmfmac, the driver behind a great many built-in
// adapters, offers none at all.
func NewMonitor() (Scanner, error) {
	conn, family, err := dial()
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	if _, _, _, err := managedInterface(conn, family); err != nil {
		return nil, err
	}
	return monitorScanner{}, nil
}

// Scan implements [Scanner] with one full sweep of the 2.4 GHz channel plan.
func (monitorScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	conn, family, err := dial()
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	_, managedName, wiphy, err := managedInterface(conn, family)
	if err != nil {
		return nil, err
	}

	// Recover from a run that was killed before it could clean up.
	//
	// A monitor interface under our name is the evidence that the previous
	// run, not the operator, is why the managed interface is down — nothing
	// else creates it. So it is not enough to delete the leftover: the link
	// has to be raised too, or a crashed survey leaves the operator's Wi-Fi
	// switched off and the next run faithfully "restores" it to off.
	if idx, lookupErr := interfaceIndex(monitorIfName); lookupErr == nil {
		_ = delInterface(conn, family, idx)
		if err := writeInterfaceFlag(managedName, true); err != nil {
			return nil, err
		}
	}

	// Monitor mode needs the radio's channel, and the managed interface holds
	// it while it is up: on the Edimax RTL8723BU, setting a frequency with the
	// managed interface up fails with EBUSY and every frame keeps arriving on
	// the channel the association is using. Bringing it down is therefore not
	// tidiness, it is what makes channel hopping work at all.
	restoreManaged, err := setInterfaceUp(managedName, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = restoreManaged() }()

	freqs, err := channelPlan(conn, family, wiphy)
	if err != nil {
		return nil, err
	}
	if len(freqs) == 0 {
		return nil, ErrNoInterface
	}

	monIndex, err := addMonitorInterface(conn, family, wiphy)
	if err != nil {
		return nil, err
	}
	defer func() { _ = delInterface(conn, family, monIndex) }()

	if _, err := setInterfaceUp(monitorIfName, true); err != nil {
		return nil, err
	}

	fd, err := openPacketSocket(monIndex)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unix.Close(fd) }()

	return sweep(ctx, conn, family, fd, monIndex, freqs)
}

// sweep dwells on each channel in turn and merges what it hears.
func sweep(
	ctx context.Context,
	conn *genetlink.Conn,
	family genetlink.Family,
	fd, monIndex int,
	freqs []int,
) ([]wifi.ScannedNetwork, error) {
	strongest := make(map[string]wifi.ScannedNetwork)

	for _, freq := range freqs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := setFrequency(conn, family, monIndex, freq); err != nil {
			// One refused channel does not end a sweep: a regulatory domain
			// can forbid a channel the band still advertises, and the rest of
			// the plan is still worth walking.
			continue
		}
		collect(fd, strongest)
	}

	networks := make([]wifi.ScannedNetwork, 0, len(strongest))
	for _, n := range strongest {
		networks = append(networks, n)
	}
	sort.Slice(networks, func(i, j int) bool {
		return networks[i].BSSID < networks[j].BSSID
	})
	return networks, nil
}

// collect reads frames for one dwell and records the BSSs it decodes.
//
// A BSS heard more than once keeps its strongest reading. Weakening is what
// an adjacent channel and a missed beacon both look like, and a survey point
// asks how well an AP reaches this spot — the best of several observations
// from one position is the better answer to that than the last.
func collect(fd int, strongest map[string]wifi.ScannedNetwork) {
	deadline := time.Now().Add(dwellPerChannel)
	buf := make([]byte, monitorFrameBuffer)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		if err := setReadTimeout(fd, remaining); err != nil {
			return
		}

		n, err := unix.Read(fd, buf)
		if err != nil {
			// EAGAIN is the dwell expiring, which is the normal exit.
			return
		}

		network, ok := networkFromFrame(buf[:n], time.Now())
		if !ok {
			continue
		}
		if prev, seen := strongest[network.BSSID]; seen && prev.Signal >= network.Signal {
			continue
		}
		strongest[network.BSSID] = network
	}
}

// networkFromFrame turns one captured frame into a survey observation.
func networkFromFrame(frame []byte, seen time.Time) (wifi.ScannedNetwork, bool) {
	rt, err := parseRadiotap(frame)
	if err != nil {
		return wifi.ScannedNetwork{}, false
	}
	// Without a signal reading the frame cannot contribute to a heatmap, and
	// recording it at 0 dBm would place a fictional strong AP at the point.
	if !rt.haveSignal {
		return wifi.ScannedNetwork{}, false
	}

	mgmt, ok := parseManagementFrame(frame[rt.length:], rt.hasFCS)
	if !ok {
		return wifi.ScannedNetwork{}, false
	}

	channel := channelFromElements(mgmt.elements)
	freq := rt.freqMHz
	if channel != 0 {
		freq = channelToFrequency(channel, band24GHz)
	}

	noise := defaultNoiseFloorDBm
	if rt.haveNoise {
		noise = rt.noiseDBm
	}

	width := widthFromElements(mgmt.elements)

	var utilization *int
	if percent, ok := channelUtilizationFromElements(mgmt.elements); ok {
		utilization = &percent
	}

	return wifi.ScannedNetwork{
		SSID:         ssidFromElements(mgmt.elements),
		BSSID:        mgmt.bssid.String(),
		Signal:       rt.signalDBm,
		Channel:      channel,
		Frequency:    freq,
		Security:     securityFromElements(mgmt.elements, mgmt.capability),
		ChannelWidth: width,
		NoiseFloor:   noise,
		SNR:          rt.signalDBm - noise,
		HTMode:       htModeForWidth(width),
		// 2.4 GHz has no DFS channels, so this is false by construction
		// rather than by omission. It becomes a real question when a 5 GHz
		// adapter arrives (#11).
		IsDFS:              false,
		LastSeen:           seen,
		ChannelUtilization: utilization,
	}, true
}

// managedInterface returns the station interface a survey walks with, along
// with the PHY it lives on — the monitor interface has to be created on the
// same PHY, since that is the radio.
func managedInterface(
	conn *genetlink.Conn, family genetlink.Family,
) (uint32, string, uint32, error) {
	index, name, err := wirelessInterface(conn, family)
	if err != nil {
		return 0, "", 0, err
	}

	msgs, err := conn.Execute(
		genetlink.Message{Header: genetlink.Header{Command: nl80211CmdGetInterface, Version: family.Version}},
		family.ID,
		netlink.Request|netlink.Dump,
	)
	if err != nil {
		return 0, "", 0, fmt.Errorf("capture: list wireless interfaces: %w", err)
	}

	for _, msg := range msgs {
		ad, decErr := netlink.NewAttributeDecoder(msg.Data)
		if decErr != nil {
			return 0, "", 0, fmt.Errorf("capture: decode interface: %w", decErr)
		}
		var thisIndex, wiphy uint32
		for ad.Next() {
			switch ad.Type() {
			case nl80211AttrIfindex:
				thisIndex = ad.Uint32()
			case nl80211AttrWiphy:
				wiphy = ad.Uint32()
			}
		}
		if thisIndex == index {
			return index, name, wiphy, nil
		}
	}
	return 0, "", 0, ErrNoInterface
}

// addMonitorInterface creates the monitor interface on the adapter's PHY.
//
// A separate interface, rather than switching the operator's interface to
// monitor type, is what makes the failure mode survivable: the managed
// interface keeps its type throughout, so a process killed mid-sweep leaves
// an extra interface and a downed link rather than an adapter in a mode
// nothing records as temporary.
func addMonitorInterface(
	conn *genetlink.Conn, family genetlink.Family, wiphy uint32,
) (int, error) {
	ae := netlink.NewAttributeEncoder()
	ae.Uint32(nl80211AttrWiphy, wiphy)
	ae.String(nl80211AttrIfname, monitorIfName)
	ae.Uint32(nl80211AttrIftype, nl80211IftypeMonitor)
	data, err := ae.Encode()
	if err != nil {
		return 0, fmt.Errorf("capture: encode monitor interface request: %w", err)
	}

	// Request only, deliberately: NL80211_CMD_NEW_INTERFACE answers with the
	// interface it created. Asking for an acknowledgement as well leaves that
	// reply unread on the socket, and the next command on this connection
	// fails validation with a mismatched sequence number — which is how this
	// was found, as a wiphy dump that failed only when it ran after a
	// successful interface creation.
	if _, err := conn.Execute(
		genetlink.Message{
			Header: genetlink.Header{Command: nl80211CmdNewInterface, Version: family.Version},
			Data:   data,
		},
		family.ID,
		netlink.Request,
	); err != nil {
		if errors.Is(err, unix.EPERM) {
			return 0, ErrPermission
		}
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) {
			return 0, ErrUnsupported
		}
		return 0, fmt.Errorf("capture: create monitor interface: %w", err)
	}

	// The reply carries the new ifindex, but reading it back by name works
	// whether or not the driver echoed the attribute.
	index, err := interfaceIndex(monitorIfName)
	if err != nil {
		return 0, err
	}
	return index, nil
}

// delInterface removes an interface by index.
func delInterface(conn *genetlink.Conn, family genetlink.Family, index int) error {
	ae := netlink.NewAttributeEncoder()
	ae.Uint32(nl80211AttrIfindex, uint32(index))
	data, err := ae.Encode()
	if err != nil {
		return fmt.Errorf("capture: encode delete-interface request: %w", err)
	}

	if _, err := conn.Execute(
		genetlink.Message{
			Header: genetlink.Header{Command: nl80211CmdDelInterface, Version: family.Version},
			Data:   data,
		},
		family.ID,
		netlink.Request|netlink.Acknowledge,
	); err != nil {
		return fmt.Errorf("capture: delete interface: %w", err)
	}
	return nil
}

// setFrequency parks the monitor interface on one channel.
func setFrequency(conn *genetlink.Conn, family genetlink.Family, index, freq int) error {
	ae := netlink.NewAttributeEncoder()
	ae.Uint32(nl80211AttrIfindex, uint32(index))
	ae.Uint32(nl80211AttrWiphyFreq, uint32(freq))
	data, err := ae.Encode()
	if err != nil {
		return fmt.Errorf("capture: encode set-frequency request: %w", err)
	}

	if _, err := conn.Execute(
		genetlink.Message{
			Header: genetlink.Header{Command: nl80211CmdSetWiphy, Version: family.Version},
			Data:   data,
		},
		family.ID,
		netlink.Request|netlink.Acknowledge,
	); err != nil {
		return fmt.Errorf("capture: set frequency %d: %w", freq, err)
	}
	return nil
}

// channelPlan is the 2.4 GHz channels the kernel says this adapter may use.
//
// The kernel has already applied the regulatory domain to this list, so it is
// the channel plan — there is nothing further to "learn", and deriving one
// from a country code would only risk disagreeing with the radio's own rules.
// Channels marked no-IR stay in: that flag forbids transmitting, and a
// monitor sweep only listens.
func channelPlan(conn *genetlink.Conn, family genetlink.Family, wiphy uint32) ([]int, error) {
	ae := netlink.NewAttributeEncoder()
	ae.Uint32(nl80211AttrWiphy, wiphy)
	ae.Flag(nl80211AttrSplitDump, true)
	data, err := ae.Encode()
	if err != nil {
		return nil, fmt.Errorf("capture: encode wiphy request: %w", err)
	}

	msgs, err := conn.Execute(
		genetlink.Message{
			Header: genetlink.Header{Command: nl80211CmdGetWiphy, Version: family.Version},
			Data:   data,
		},
		family.ID,
		netlink.Request|netlink.Dump,
	)
	if err != nil {
		return nil, fmt.Errorf("capture: read wiphy bands: %w", err)
	}

	seen := make(map[int]struct{})
	var freqs []int
	// A split dump spreads the bands across several messages, so every
	// message is inspected and the frequencies unioned rather than taken from
	// the first one that carries a band.
	for _, msg := range msgs {
		ad, decErr := netlink.NewAttributeDecoder(msg.Data)
		if decErr != nil {
			continue
		}
		for ad.Next() {
			if ad.Type() != nl80211AttrWiphyBands {
				continue
			}
			ad.Nested(func(bands *netlink.AttributeDecoder) error {
				collectBandFrequencies(bands, seen, &freqs)
				return nil
			})
		}
	}

	sort.Ints(freqs)
	return freqs, nil
}

// collectBandFrequencies appends the usable 2.4 GHz frequencies of every band
// in the decoder.
func collectBandFrequencies(bands *netlink.AttributeDecoder, seen map[int]struct{}, freqs *[]int) {
	for bands.Next() {
		bands.Nested(func(band *netlink.AttributeDecoder) error {
			for band.Next() {
				if band.Type() != nl80211BandAttrFreqs {
					continue
				}
				band.Nested(func(list *netlink.AttributeDecoder) error {
					for list.Next() {
						list.Nested(func(freq *netlink.AttributeDecoder) error {
							appendFrequency(freq, seen, freqs)
							return nil
						})
					}
					return nil
				})
			}
			return nil
		})
	}
}

// appendFrequency records one frequency if it is enabled and in band.
func appendFrequency(freq *netlink.AttributeDecoder, seen map[int]struct{}, freqs *[]int) {
	var (
		mhz      int
		disabled bool
	)
	for freq.Next() {
		switch freq.Type() {
		case nl80211FreqAttrFreq:
			mhz = int(freq.Uint32())
		case nl80211FreqAttrDisabled:
			disabled = true
		}
	}
	if disabled || mhz == 0 || mhz > band24GHzMaxFreq {
		return
	}
	if _, dup := seen[mhz]; dup {
		return
	}
	seen[mhz] = struct{}{}
	*freqs = append(*freqs, mhz)
}

// interfaceIndex resolves an interface name to its kernel index.
func interfaceIndex(name string) (int, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return 0, fmt.Errorf("capture: look up %s: %w", name, err)
	}
	return iface.Index, nil
}

// setInterfaceUp brings a link up or down and returns a function restoring
// the flag to what it was.
func setInterfaceUp(name string, up bool) (func() error, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return nil, fmt.Errorf("capture: open control socket: %w", err)
	}
	defer func() { _ = unix.Close(fd) }()

	ifreq, err := unix.NewIfreq(name)
	if err != nil {
		return nil, fmt.Errorf("capture: reference %s: %w", name, err)
	}
	if err := unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, ifreq); err != nil {
		return nil, fmt.Errorf("capture: read %s flags: %w", name, err)
	}

	was := ifreq.Uint16()
	wasUp := was&unix.IFF_UP != 0
	if wasUp == up {
		return func() error { return nil }, nil
	}

	if err := writeInterfaceFlag(name, up); err != nil {
		return nil, err
	}
	return func() error { return writeInterfaceFlag(name, wasUp) }, nil
}

// writeInterfaceFlag sets or clears IFF_UP on an interface.
func writeInterfaceFlag(name string, up bool) error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return fmt.Errorf("capture: open control socket: %w", err)
	}
	defer func() { _ = unix.Close(fd) }()

	ifreq, err := unix.NewIfreq(name)
	if err != nil {
		return fmt.Errorf("capture: reference %s: %w", name, err)
	}
	if err := unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, ifreq); err != nil {
		return fmt.Errorf("capture: read %s flags: %w", name, err)
	}

	flags := ifreq.Uint16()
	if up {
		flags |= unix.IFF_UP
	} else {
		flags &^= unix.IFF_UP
	}
	ifreq.SetUint16(flags)

	if err := unix.IoctlIfreq(fd, unix.SIOCSIFFLAGS, ifreq); err != nil {
		if errors.Is(err, unix.EPERM) {
			return ErrPermission
		}
		return fmt.Errorf("capture: set %s flags: %w", name, err)
	}
	return nil
}

// openPacketSocket binds a raw socket to the monitor interface.
func openPacketSocket(index int) (int, error) {
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		if errors.Is(err, unix.EPERM) {
			return 0, ErrPermission
		}
		return 0, fmt.Errorf("capture: open packet socket: %w", err)
	}

	if err := unix.Bind(fd, &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  index,
	}); err != nil {
		_ = unix.Close(fd)
		return 0, fmt.Errorf("capture: bind packet socket: %w", err)
	}
	return fd, nil
}

// setReadTimeout bounds one read so a silent channel cannot outlast its dwell.
func setReadTimeout(fd int, d time.Duration) error {
	tv := unix.NsecToTimeval(int64(d))
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		return fmt.Errorf("capture: set read timeout: %w", err)
	}
	return nil
}

// htons converts to the network byte order AF_PACKET expects.
func htons(v uint16) uint16 {
	return v<<8 | v>>8
}
