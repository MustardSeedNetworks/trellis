// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"net/http"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1/surveyv1connect"
	"github.com/MustardSeedNetworks/trellis/internal/api"
	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

// muxDeps is what the daemon's routing needs from the rest of run().
//
// gate is nil on the loopback desktop daemon, which serves its own operator's
// browser without a credential (ADR-0007); ui is injected rather than read from
// the embedded bundle so that a test can compose the same mux without a built
// frontend.
type muxDeps struct {
	survey surveyv1connect.SurveyServiceHandler
	gate   *auth.Gate
	ui     http.Handler
}

// newMux builds the daemon's routing, including the auth interceptor the
// survey service is served behind.
//
// It is a function rather than eight lines inside run() because the wiring is
// the security boundary: which handler is wrapped in the gate's interceptor,
// and which routes are deliberately outside it. Inline, nothing could assert
// any of that without starting the whole daemon, so nothing did (#520).
func newMux(deps muxDeps) *http.ServeMux {
	options := []connect.HandlerOption{connect.WithReadMaxBytes(maxUploadBytes)}
	if deps.gate != nil {
		options = append(options, connect.WithInterceptors(deps.gate.Interceptor()))
	}

	mux := http.NewServeMux()
	path, handler := surveyv1connect.NewSurveyServiceHandler(deps.survey, options...)
	mux.Handle(path, handler)
	if deps.gate != nil {
		deps.gate.RegisterRoutes(mux)
	}

	// Outside the gate on purpose: a deployment check reads the version and a
	// health probe reads /healthz without credentials, and the operator has to
	// be able to load the login page.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/__version", api.HandleBuildVersion)
	mux.Handle("/", deps.ui)

	return mux
}
