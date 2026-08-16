// SPDX-License-Identifier: BUSL-1.1

// Command trellisd serves Trellis's measured-survey API over Connect/gRPC.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1/surveyv1connect"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

const (
	// defaultAddr binds loopback only. This server currently has no
	// authentication or TLS (both are tracked follow-ups before any
	// non-local deployment), so it must not be exposed on all interfaces by
	// default. An operator who understands the trade-off can override with
	// TRELLIS_ADDR — and owns adding auth/TLS in front of it.
	defaultAddr         = "127.0.0.1:8080"
	defaultDataDir      = "./data"
	shutdownGracePeriod = 10 * time.Second
	readHeaderTimeout   = 5 * time.Second
	readTimeout         = 30 * time.Second
	// writeTimeout is generous: report generation and heatmap rendering can
	// take a few seconds on large surveys.
	writeTimeout = 120 * time.Second
	idleTimeout  = 60 * time.Second
	// maxUploadBytes bounds a single Connect request message so an oversized
	// AirMapper upload can't exhaust memory. 64 MiB comfortably covers a
	// floor-plan-bearing .amp while capping the blast radius.
	maxUploadBytes = 64 << 20
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("trellisd exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dataDir := os.Getenv("TRELLIS_DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	addr := os.Getenv("TRELLIS_ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	manager := survey.NewManager(dataDir, nil, nil, nil, nil)
	if err := manager.LoadSurveys(); err != nil {
		return err
	}
	slog.Info("loaded surveys", "count", len(manager.ListSurveys()), "data_dir", dataDir)

	mux := http.NewServeMux()
	path, handler := surveyv1connect.NewSurveyServiceHandler(
		api.NewSurveyServiceHandler(manager),
		connect.WithReadMaxBytes(maxUploadBytes),
	)
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/__version", api.HandleBuildVersion)

	uiHandler, err := api.UIHandler()
	if err != nil {
		return err
	}
	mux.Handle("/", uiHandler)

	// Serve HTTP/2 over plain-text (h2c) so local/dev clients can speak
	// gRPC or Connect without TLS, alongside HTTP/1.1 for connect-web/JSON.
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		Protocols:         &protocols,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("trellisd listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		slog.Info("shutting down trellisd")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-serveErr
	case err := <-serveErr:
		return err
	}
}
