// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"context"
	"errors"
	"maps"
	"net/http"

	"connectrpc.com/connect/v2"
)

// Interceptor authenticates every RPC against the gate.
//
// Trellis serves its API over Connect rather than the REST handlers Seed and
// Stem wrap, so the fleet's auth middleware becomes an interceptor. Everything
// behind it is survey data; the UI bundle, /healthz and /__version stay outside
// it, because the operator has to be able to load the login page and a
// deployment check has to read the version without credentials.
//
// CSRF is checked on every RPC, not only on the mutating ones. Connect carries
// reads and writes over the same POST, so there is no method to discriminate on
// and no reason to leave the read path as the exception.
// It is a connect.ServerInterceptor, which wraps every RPC the server
// dispatches whatever its stream type, so a streaming RPC cannot reach its
// handler unauthenticated (#521). A stream is authenticated once, from the
// headers of the request that opened it, before any message is read: its
// credentials cannot change mid-stream, and a session that expires while it is
// open is a lifetime question this gate does not answer for unary calls either.
func (g *Gate) Interceptor() connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			// A transport that dispatches without call info has no
			// credentials to check, so the call is refused rather than
			// served.
			info, ok := connect.CallInfoForServerContext(ctx)
			if !ok {
				return refusal(ErrUnauthenticated)
			}
			header := http.Header(maps.Collect(info.RequestHeader().All()))
			if _, err := g.Authenticate(header, info.PeerAddr); err != nil {
				return refusal(err)
			}
			return next(ctx, spec, stream)
		}
	}
}

// refusal maps an authentication failure to a Connect code, without saying
// which half of the check failed.
//
// A session that is present but not proven to be the caller's is
// permission_denied, not unauthenticated: the difference is what stops the UI
// answering a missing CSRF token by sending the operator back to the login
// page, which would never fix it.
func refusal(err error) error {
	if errors.Is(err, ErrCSRF) {
		return connect.NewError(connect.CodePermissionDenied, "request not permitted")
	}
	return connect.NewError(connect.CodeUnauthenticated, "authentication required")
}
