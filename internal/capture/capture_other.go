// SPDX-License-Identifier: BUSL-1.1

//go:build (!darwin && !linux && !windows) || (darwin && !cgo)

package capture

// New reports that this build has no capture backend.
//
// macOS, Linux and Windows all have one. Reaching this file means either a
// platform with no host-NIC backend — where a survey comes from external
// hardware or an import — or a macOS build made with CGO_ENABLED=0.
//
// In the macOS case it means the binary was built with CGO_ENABLED=0. The backend links
// CoreWLAN through cgo, so a cgo-less macOS build cannot scan at all. Release
// builds set CGO_ENABLED=1 for darwin (.goreleaser.yml); this constraint exists
// so `CGO_ENABLED=0 go build ./...` still compiles the tree, which is how CI
// proves cgo stays confined to this package (docs/07-RISKS R5).
func New() (Scanner, error) {
	return nil, ErrUnsupported
}

// Authorize has nothing to ask for: without a backend there is no radio to
// read, and [New] already reports that.
func Authorize() error { return nil }
