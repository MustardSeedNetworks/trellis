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
// It implements the whole of connect.Interceptor rather than returning a
// connect.UnaryInterceptorFunc, whose streaming halves are no-ops: survey.proto
// is unary throughout today, so such a gate looks correct and would serve the
// first streaming RPC with no authentication at all (#521). The check belongs
// to the transport, not to the shape of the call.
func (g *Gate) Interceptor() connect.Interceptor {
	return interceptor{gate: g}
}

// interceptor is the gate as a connect.Interceptor.
type interceptor struct {
	gate *Gate
}

// WrapUnary authenticates a unary RPC before its handler runs.
func (i interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if _, err := i.gate.Authenticate(req.Header(), req.Peer().Addr); err != nil {
			return nil, refusal(err)
		}
		return next(ctx, req)
	}
}

// WrapStreamingHandler authenticates a streaming RPC from the headers of the
// request that opened the stream, before any message is read or the handler
// runs. A stream is authenticated once, at that point: its credentials cannot
// change mid-stream, and a session that expires while it is open is a lifetime
// question this gate does not answer for unary calls either.
func (i interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if _, err := i.gate.Authenticate(conn.RequestHeader(), conn.Peer().Addr); err != nil {
			return refusal(err)
		}
		return next(ctx, conn)
	}
}

// WrapStreamingClient passes through. The gate authenticates callers of this
// daemon; nothing here is a Connect client, and a server-side credential check
// has no meaning on an outbound call.
func (i interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
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
