// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

func TestRequireLoopback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		addr string
		ok   bool
	}{
		{"127.0.0.1:8446", true},
		{"127.0.0.2:8446", true},
		{"[::1]:8446", true},
		{"localhost:8446", true},
		{"0.0.0.0:8446", false},
		{"[::]:8446", false},
		{":8446", false},
		{"10.44.10.5:8446", false},
		{"192.168.1.20:0", false},
		{"trellis.msn.lab:8446", false},
	}
	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			t.Parallel()
			err := requireBindAllowed(tc.addr, false)
			if tc.ok && err != nil {
				t.Fatalf("requireLoopback(%q) = %v, want nil", tc.addr, err)
			}
			if !tc.ok {
				if !errors.Is(err, errNotLoopback) {
					t.Fatalf("requireLoopback(%q) = %v, want errNotLoopback", tc.addr, err)
				}
				// The remedy travels with the refusal; a bare "invalid address"
				// would send the operator to the docs to learn why.
				// The remedy travels with the refusal: the operator is
				// told what to set, not just that they may not do this.
				got := err.Error()
				if !strings.Contains(got, tc.addr) {
					t.Errorf("error %q should name the address", got)
				}
				for _, want := range []string{auth.EnvUsername, auth.EnvPassword} {
					if !strings.Contains(got, want) {
						t.Errorf("error %q should name %s", got, want)
					}
				}
			}
		})
	}
}

func TestRequireLoopbackRejectsUnparseableAddress(t *testing.T) {
	t.Parallel()
	if err := requireBindAllowed("8446", false); err == nil || errors.Is(err, errNotLoopback) {
		t.Fatalf("requireLoopback(\"8446\") = %v, want a parse error", err)
	}
}

// The gate runs before anything is opened: no store, no radio, no listener.
// run returning the error is what makes main exit non-zero.
func TestRunRefusesNonLoopbackAddress(t *testing.T) {
	t.Setenv("TRELLIS_ADDR", "0.0.0.0:0")
	t.Setenv("TRELLIS_DATA_DIR", t.TempDir())

	err := run()
	if !errors.Is(err, errNotLoopback) {
		t.Fatalf("run() = %v, want errNotLoopback", err)
	}
}

// The whole point of the feature: a protected daemon may bind a routable
// address. The gate stays for an unprotected one.
func TestRequireBindAllowedAdmitsAProtectedDaemon(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"0.0.0.0:8446", "10.44.10.5:8446", "[::]:8446"} {
		t.Run(addr, func(t *testing.T) {
			t.Parallel()
			if err := requireBindAllowed(addr, true); err != nil {
				t.Fatalf("requireBindAllowed(%q, protected) = %v, want nil", addr, err)
			}
		})
	}
}

