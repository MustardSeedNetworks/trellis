// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package capture

// PermissionRemedy is the Windows answer: WLAN scanning is gated on the
// system-wide location setting and on the per-app consent beneath it, and the
// consent prompt only appears for a user sitting at the machine (#152).
const PermissionRemedy = "enable Settings > Privacy & security > Location, " +
	"and allow desktop apps to access your location"
