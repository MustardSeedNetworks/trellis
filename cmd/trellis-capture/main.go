// SPDX-License-Identifier: BUSL-1.1

//go:build darwin

// Command trellis-capture reads the host Wi-Fi adapter and emits scan results
// as newline-delimited JSON.
//
// It is a separate process from trellisd so that cgo and OS radio APIs stay out
// of the Go core (docs/02-ARCHITECTURE, docs/07-RISKS R5), and so the survey
// engine can consume host-NIC capture and an external-hardware backend through
// the same contract.
//
// It ships inside a signed application bundle. macOS hides Wi-Fi network names
// and BSSIDs from any process without Location Services authorization, and that
// permission is granted per user, in a login session, to a signed bundle
// carrying com.apple.security.personal-information.location. An unbundled or
// unentitled binary scans successfully and sees nothing identifiable.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MustardSeedNetworks/foundation/pkg/corewlan"

	"github.com/MustardSeedNetworks/trellis/internal/capture"
)

// authorizationWait bounds the startup permission request. Requesting is also
// what registers the bundle with locationd, which is what makes it appear in
// System Settings at all.
const authorizationWait = 5 * time.Second

func main() {
	interval := flag.Duration("interval", 0,
		"scan repeatedly at this interval; zero performs a single scan and exits")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *interval); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, interval time.Duration) error {
	if status := corewlan.RequestAuthorization(authorizationWait); status != corewlan.AuthAuthorized {
		fmt.Fprintf(os.Stderr,
			"location services %s: scans will omit network names and BSSIDs.\n"+
				"Enable \"Trellis Capture\" in System Settings > Privacy & Security > Location Services.\n",
			status)
	}

	scanner, err := capture.New()
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	for {
		if err := scanOnce(scanner, enc); err != nil {
			return err
		}
		if interval <= 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

// scanOnce writes one scan as a single JSON line, so a consumer can read a walk
// incrementally rather than waiting for the survey to finish.
func scanOnce(scanner capture.Scanner, enc *json.Encoder) error {
	networks, err := scanner.Scan()
	if err != nil {
		// A permission failure is a condition an operator fixes, not a
		// transient error worth retrying in a loop.
		if errors.Is(err, capture.ErrPermission) {
			return fmt.Errorf("%w\nenable \"Trellis Capture\" in System Settings > "+
				"Privacy & Security > Location Services", err)
		}
		return err
	}

	if err := enc.Encode(networks); err != nil {
		return fmt.Errorf("write scan: %w", err)
	}
	return nil
}
