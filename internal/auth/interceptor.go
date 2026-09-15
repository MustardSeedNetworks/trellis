// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"context"
	"errors"

	"connectrpc.com/connect"
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
func (g *Gate) Interceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if _, err := g.Authenticate(req.Header(), req.Peer().Addr); err != nil {
				return nil, refusal(err)
			}
			return next(ctx, req)
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
		return connect.NewError(connect.CodePermissionDenied, errors.New("request not permitted"))
	}
	return connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
}
