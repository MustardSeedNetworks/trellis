// SPDX-License-Identifier: BUSL-1.1

//go:build darwin && cgo

package capture

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MustardSeedNetworks/foundation/pkg/corewlan"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// coreWLANScanner reads the host adapter through Apple's CoreWLAN framework.
//
// macOS redacts SSID and BSSID from any process without Location Services
// authorization, which is granted per user, in a login session, to a signed
// application bundle carrying the location entitlement. A survey binary that is
// not bundled and entitled will scan successfully and see nothing identifiable,
// so that condition is reported as [ErrPermission] rather than as an empty
// airspace.
type coreWLANScanner struct{}

// New returns the host's capture backend.
func New() (Scanner, error) {
	return coreWLANScanner{}, nil
}

// Scan implements [Scanner].
func (coreWLANScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	// Checked here and not again: CoreWLAN's scan is a blocking cgo call with
	// no cancellation of its own, so this is the last point at which a
	// cancelled context can still mean anything.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	observed, err := corewlan.Scan()
	if err != nil {
		switch {
		case errors.Is(err, corewlan.ErrLocationDenied):
			return nil, ErrPermission
		case errors.Is(err, corewlan.ErrNoInterface):
			return nil, ErrNoInterface
		default:
			return nil, fmt.Errorf("capture: scan: %w", err)
		}
	}

	seen := time.Now().UTC()
	networks := make([]wifi.ScannedNetwork, 0, len(observed))
	for _, n := range observed {
		networks = append(networks, networkFrom(n, seen))
	}
	// Deduped before anything counts them: CoreWLAN answers with observations
	// rather than with BSSs, and this office's radio reports some of them more
	// than once. nl80211 and Native Wifi both hand back a BSS list keyed by
	// BSSID, so neither needs this.
	return withAssociation(dedupeBSSes(networks), currentNetwork(seen)), nil
}

// currentNetwork reports the BSS this host is joined to, or nil.
//
// Not being associated is the ordinary state of a survey laptop walking a
// floor, and a radio that cannot answer is no reason to fail a scan that
// already succeeded, so every failure here means "no association" rather than
// an error. CoreWLAN is the only one of the three platforms that has to be
// asked separately: nl80211 flags the joined BSS on the scan dump itself, and
// Native Wifi's equivalent query is not wired up yet (see docs/10-WIFI-CAPTURE).
func currentNetwork(seen time.Time) *wifi.ScannedNetwork {
	current, err := corewlan.Current()
	if err != nil || current == nil {
		return nil
	}
	network := networkFrom(*current, seen)
	return &network
}

// networkFrom maps a CoreWLAN observation onto Trellis's scan model.
func networkFrom(n corewlan.Network, seen time.Time) wifi.ScannedNetwork {
	noise := n.Noise
	if noise == 0 {
		noise = defaultNoiseFloorDBm
	}

	width := n.ChannelWidth
	if width == 0 {
		width = width20MHz
	}

	return wifi.ScannedNetwork{
		SSID:         n.SSID,
		BSSID:        n.BSSID,
		Signal:       n.RSSI,
		Channel:      n.Channel,
		Frequency:    channelToFrequency(n.Channel, int(n.Band)),
		Security:     securityName(n.Security),
		ChannelWidth: width,
		NoiseFloor:   noise,
		SNR:          n.RSSI - noise,
		HTMode:       htModeForWidth(width),
		IsDFS:        n.Band == corewlan.Band5GHz && isDFSChannel(n.Channel),
		LastSeen:     seen,
	}
}

// securityName maps CoreWLAN's scheme names onto the vocabulary core/wifi
// records ("WPA3", "Open", ...).
func securityName(security string) string {
	switch security {
	case "none":
		return "Open"
	case "wep":
		return "WEP"
	case "wpaPersonal", "wpaPersonalMixed", "wpaEnterprise":
		return "WPA"
	case "wpa2Personal", "wpa2Enterprise":
		return "WPA2"
	case "wpa3Personal", "wpa3Transition", "wpa3Enterprise":
		return "WPA3"
	case "enterprise":
		return "Enterprise"
	default:
		return ""
	}
}

// authorizationWait bounds the startup permission request. Requesting is also
// what registers the bundle with locationd, which is what makes it appear in
// System Settings at all — a process that never asks is never listed, and an
// operator then has nothing to switch on.
const authorizationWait = 5 * time.Second

// Authorize asks macOS for Location Services authorization, which is what
// decides whether a scan can see network names at all, and waits briefly for an
// answer.
//
// It returns [ErrPermission] wrapping the status macOS reported. Scanning still
// works in that state; it just names nothing, which is why this is worth
// reporting at startup rather than at the first survey point.
func Authorize() error {
	if status := corewlan.RequestAuthorization(authorizationWait); status != corewlan.AuthAuthorized {
		return fmt.Errorf("%w (location services %s)", ErrPermission, status)
	}
	return nil
}
