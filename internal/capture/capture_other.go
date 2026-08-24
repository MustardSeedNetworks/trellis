// SPDX-License-Identifier: BUSL-1.1

//go:build !darwin

package capture

// New reports that this platform has no capture backend yet. Linux and Windows
// host-NIC backends are planned (docs/06-ROADMAP); until then a survey on those
// hosts must come from an external-hardware backend or an import.
func New() (Scanner, error) {
	return nil, ErrUnsupported
}
