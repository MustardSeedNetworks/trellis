// SPDX-License-Identifier: BUSL-1.1

//go:build darwin

package capture

// PermissionRemedy is the macOS answer: Location Services authorization is
// granted per user, in a login session, to a signed application bundle carrying
// the location entitlement — so running the bare binary can never satisfy it,
// however the setting is set.
const PermissionRemedy = "enable Trellis in System Settings > Privacy & Security > " +
	"Location Services, and launch it as Trellis.app rather than running the binary directly"
