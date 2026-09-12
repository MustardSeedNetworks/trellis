// SPDX-License-Identifier: BUSL-1.1

package auth_test

import (
	"context"
	"strings"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

// callWith runs the gate's interceptor over a request carrying h and reports
// whether the call reached the handler.
func callWith(t *testing.T, g *auth.Gate, h http.Header) (reached bool, err error) {
	t.Helper()
	req := connect.NewRequest(&struct{}{})
	for k, vs := range h {
		for _, v := range vs {
			req.Header().Add(k, v)
		}
	}
	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		reached = true
		return connect.NewResponse(&struct{}{}), nil
	}
	_, err = g.Interceptor()(next)(context.Background(), req)
	return reached, err
}

func TestInterceptorAdmitsALiveSession(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	reached, err := callWith(t, g, s.header())
	if err != nil {
		t.Fatalf("an authenticated RPC was refused: %v", err)
	}
	if !reached {
		t.Fatal("the interceptor swallowed an authenticated RPC")
	}
}

func TestInterceptorRefusesAnAnonymousRPC(t *testing.T) {
	t.Parallel()
	g := newGate(t)
	reached, err := callWith(t, g, http.Header{})
	if reached {
		t.Fatal("an anonymous RPC reached the handler")
	}
	if got := connect.CodeOf(err); got != connect.CodeUnauthenticated {
		t.Fatalf("anonymous RPC refused with %v, want unauthenticated", got)
	}
}

// A session with no CSRF token must be refused as permission-denied rather than
// unauthenticated: the caller is known, the request is not proven to be theirs,
// and the UI must not answer it by trying to log in again.
func TestInterceptorRefusesASessionWithoutCSRF(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	bare := session{cookies: s.cookies}
	reached, err := callWith(t, g, bare.header())
	if reached {
		t.Fatal("an RPC with no CSRF token reached the handler")
	}
	if got := connect.CodeOf(err); got != connect.CodePermissionDenied {
		t.Fatalf("RPC without CSRF refused with %v, want permission_denied", got)
	}
}

// The refusal must not describe which half was wrong.
func TestInterceptorRefusalIsOpaque(t *testing.T) {
	t.Parallel()
	g := newGate(t)
	_, err := callWith(t, g, http.Header{})
	if err == nil {
		t.Fatal("anonymous RPC was not refused")
	}
	for _, leak := range []string{testPassword, "surveyor", "cookie", "trellis_access"} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("the refusal message leaks %q: %v", leak, err)
		}
	}
}

