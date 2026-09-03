// SPDX-License-Identifier: BUSL-1.1

//go:build !e2e

package main

import (
	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/internal/capture"
)

// newScanner returns the host's radio as the survey engine's capture backend.
//
// capture.Scanner and survey.Scanner are deliberately the same shape (ADR-0006),
// so the backend is consumed directly with no adapter in between. The e2e build
// tag replaces this with a scripted scanner; see scanner_e2e.go.
func newScanner() (survey.Scanner, error) {
	backend, err := capture.New()
	if err != nil {
		return nil, err
	}
	return backend, nil
}
