// SPDX-License-Identifier: BUSL-1.1

package auth_test

import (
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

func TestRateLimiterAllowsUpToTheLimit(t *testing.T) {
	t.Parallel()
	l := auth.NewRateLimiter(3, time.Minute)
	for i := range 3 {
		if !l.Allow("10.0.0.1") {
			t.Fatalf("attempt %d refused while under the limit", i+1)
		}
	}
	if l.Allow("10.0.0.1") {
		t.Fatal("the attempt past the limit was allowed")
	}
}

func TestRateLimiterIsPerClient(t *testing.T) {
	t.Parallel()
	l := auth.NewRateLimiter(1, time.Minute)
	if !l.Allow("10.0.0.1") || !l.Allow("10.0.0.2") {
		t.Fatal("a first attempt from each of two clients was refused")
	}
	if l.Allow("10.0.0.1") {
		t.Fatal("one client's exhausted budget did not stay its own")
	}
}

func TestRateLimiterWindowExpires(t *testing.T) {
	t.Parallel()
	l := auth.NewRateLimiter(1, 20*time.Millisecond)
	if !l.Allow("10.0.0.1") {
		t.Fatal("first attempt refused")
	}
	if l.Allow("10.0.0.1") {
		t.Fatal("second attempt inside the window allowed")
	}
	time.Sleep(40 * time.Millisecond)
	if !l.Allow("10.0.0.1") {
		t.Fatal("the budget did not come back after the window passed")
	}
}

// A successful login must return the budget, or an operator who fat-fingers
// their password four times locks themselves out of their own survey.
func TestRateLimiterResetOnSuccess(t *testing.T) {
	t.Parallel()
	l := auth.NewRateLimiter(2, time.Minute)
	l.Allow("10.0.0.1")
	l.Allow("10.0.0.1")
	l.Reset("10.0.0.1")
	if !l.Allow("10.0.0.1") {
		t.Fatal("Reset did not return the client's budget")
	}
}

// The limiter is fed by whatever the transport calls a client address, so it
// must not grow without bound when that is attacker-controlled.
func TestRateLimiterBoundsItsMemory(t *testing.T) {
	t.Parallel()
	l := auth.NewRateLimiter(1, time.Minute)
	for i := range auth.MaxTrackedClients * 2 {
		l.Allow(time.Duration(i).String())
	}
	if got := l.Tracked(); got > auth.MaxTrackedClients {
		t.Fatalf("limiter tracks %d clients, above the %d cap", got, auth.MaxTrackedClients)
	}
}
