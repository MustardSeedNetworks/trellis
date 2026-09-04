// SPDX-License-Identifier: BUSL-1.1

//go:build !darwin && !linux && !windows

package capture

// PermissionRemedy is empty where there is no capture backend to fix.
const PermissionRemedy = ""
