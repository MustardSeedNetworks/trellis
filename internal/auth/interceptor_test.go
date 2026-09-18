// SPDX-License-Identifier: BUSL-1.1

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/internal/auth"
	"google.golang.org/protobuf/types/known/emptypb"
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
	_, err = g.Interceptor().WrapUnary(next)(context.Background(), req)
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

// The interceptor is where a logged-out credential actually costs something:
// every survey RPC goes through it. The session probe is the recovery path, so
// the RPC under test carries what the probe hands a retained access cookie —
// not the CSRF token logout already revoked, which proves nothing (#501).
func TestInterceptorRefusesALoggedOutSession(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	logout(t, mux, s)

	recovered := probeSession(t, mux, cookieByName(s, auth.CookieAccess))
	reached, err := callWith(t, g, recovered.header())
	if reached {
		t.Fatal("an RPC from a logged-out session reached the handler")
	}
	if got := connect.CodeOf(err); got != connect.CodeUnauthenticated {
		t.Fatalf("the logged-out RPC was refused with %v, want unauthenticated", got)
	}
}

// streamProcedure is a procedure no proto declares; google.protobuf.Empty needs
// no generation. The gate is transport-level, so the messages are irrelevant to
// what these tests assert.
const streamProcedure = "/trellis.test.v1.StreamService/Stream"

// callStreamWith opens a streaming RPC carrying h through a real server behind
// the gate's interceptor, and reports whether the call reached the handler. A
// hand-built connect.StreamingHandlerConn is not worth the risk here: it has a
// dozen methods and a fake that stubs the wrong one fails as a panic rather
// than as an assertion.
func callStreamWith(t *testing.T, g *auth.Gate, h http.Header) (reached bool, err error) {
	t.Helper()

	handler := connect.NewServerStreamHandler(
		streamProcedure,
		func(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[emptypb.Empty]) error {
			reached = true
			return nil
		},
		connect.WithInterceptors(g.Interceptor()),
	)
	mux := http.NewServeMux()
	mux.Handle(streamProcedure, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := connect.NewClient[emptypb.Empty, emptypb.Empty](server.Client(), server.URL+streamProcedure)
	req := connect.NewRequest(&emptypb.Empty{})
	for k, vs := range h {
		for _, v := range vs {
			req.Header().Add(k, v)
		}
	}
	stream, err := client.CallServerStream(t.Context(), req)
	if err != nil {
		return reached, err
	}
	defer func() { _ = stream.Close() }()
	// Drain it; the stream error is what is asserted.
	for stream.Receive() {
	}
	return reached, stream.Err()
}

func TestInterceptorAdmitsALiveSessionOnAStream(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	reached, err := callStreamWith(t, g, s.header())
	if err != nil {
		t.Fatalf("an authenticated streaming RPC was refused: %v", err)
	}
	if !reached {
		t.Fatal("the interceptor swallowed an authenticated streaming RPC")
	}
}

func TestInterceptorRefusesAnAnonymousStream(t *testing.T) {
	t.Parallel()
	g := newGate(t)
	reached, err := callStreamWith(t, g, http.Header{})
	if reached {
		t.Fatal("an anonymous streaming RPC reached the handler")
	}
	if got := connect.CodeOf(err); got != connect.CodeUnauthenticated {
		t.Fatalf("anonymous streaming RPC refused with %v, want unauthenticated", got)
	}
}

// A stream is refused for the same reasons and with the same codes as a unary
// call: the CSRF half must not answer as unauthenticated here either, or the UI
// sends the operator back to a login page that cannot fix it.
func TestInterceptorRefusesAStreamWithoutCSRF(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	bare := session{cookies: s.cookies}
	reached, err := callStreamWith(t, g, bare.header())
	if reached {
		t.Fatal("a streaming RPC with no CSRF token reached the handler")
	}
	if got := connect.CodeOf(err); got != connect.CodePermissionDenied {
		t.Fatalf("streaming RPC without CSRF refused with %v, want permission_denied", got)
	}
}
