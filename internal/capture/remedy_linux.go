// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package capture

// PermissionRemedy is the Linux answer: triggering a scan is a privileged
// nl80211 operation, so the process needs the capability rather than a setting
// somebody has to click.
const PermissionRemedy = "grant trellisd CAP_NET_ADMIN " +
	"(setcap cap_net_admin+ep /path/to/trellisd), or run it as root"
