// SPDX-License-Identifier: BUSL-1.1

//go:build !darwin || !cgo

package capture

// New reports that this build has no capture backend.
//
// Off macOS that is a missing feature: the Linux and Windows host-NIC backends
// are planned (docs/06-ROADMAP), and until they land a survey on those hosts
// comes from an external-hardware backend or an import.
//
// On macOS it means the binary was built with CGO_ENABLED=0. The backend links
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
