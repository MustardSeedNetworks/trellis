// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"strings"
	"testing"
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
			err := requireLoopback(tc.addr)
			if tc.ok && err != nil {
				t.Fatalf("requireLoopback(%q) = %v, want nil", tc.addr, err)
			}
			if !tc.ok {
				if !errors.Is(err, errNotLoopback) {
					t.Fatalf("requireLoopback(%q) = %v, want errNotLoopback", tc.addr, err)
				}
				// The remedy travels with the refusal; a bare "invalid address"
				// would send the operator to the docs to learn why.
				if got := err.Error(); !strings.Contains(got, "#160") || !strings.Contains(got, tc.addr) {
					t.Errorf("error %q should name the address and #160", got)
				}
			}
		})
	}
}

func TestRequireLoopbackRejectsUnparseableAddress(t *testing.T) {
	t.Parallel()
	if err := requireLoopback("8446"); err == nil || errors.Is(err, errNotLoopback) {
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
