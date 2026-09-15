// SPDX-License-Identifier: BUSL-1.1

// Command trellisd serves Trellis's measured-survey API over Connect/gRPC.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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
	"github.com/MustardSeedNetworks/trellis/internal/apppaths"
	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

const (
	// defaultAddr binds loopback: the desktop app of ADR-0007, one operator's
	// browser on the same machine. A TRELLIS_ADDR that would put the API on a
	// network is refused by requireBindAllowed unless an operator credential
	// is configured, which is what installs auth, CSRF and TLS in front of it
	// (#439, the feature #160 gated).
	defaultAddr         = "127.0.0.1:8446"
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
	closeLog, err := installLogger()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = closeLog() }()

	if err := run(); err != nil {
		slog.Error("trellisd exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dataDir := os.Getenv("TRELLIS_DATA_DIR")
	if dataDir == "" {
		var err error
		if dataDir, err = apppaths.DataDir(); err != nil {
			return err
		}
	}

	addr := os.Getenv("TRELLIS_ADDR")
	explicitAddr := addr != ""
	if !explicitAddr {
		addr = defaultAddr
	}
	// The operator credential is what lifts the loopback gate. Read it before
	// anything is opened, so an unprotected routable bind fails with no store,
	// no radio and no listener touched.
	credential, credErr := auth.CredentialFromEnv()
	if credErr != nil && !errors.Is(credErr, auth.ErrMissingCredentials) {
		return credErr
	}
	if err := requireBindAllowed(addr, credential.Configured()); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Capture is linked in rather than run as a helper process (ADR-0006), and
	// the survey engine consumes it directly rather than through an adapter:
	// capture.Scanner and survey.Scanner are deliberately the same shape. A
	// host with no backend still serves imported surveys, so a missing backend
	// is reported and not fatal.
	scanner, scannerErr := newScanner()
	if scannerErr != nil {
		slog.Warn("no Wi-Fi capture backend on this host; imported surveys only", "error", scannerErr)
	}

	manager, err := survey.NewManager(dataDir, scanner, nil, newThroughputMeter(), nil)
	if err != nil {
		return fmt.Errorf("open survey store %s: %w", dataDir, err)
	}
	defer func() { _ = manager.Close() }()
	if err := manager.LoadSurveys(); err != nil {
		return err
	}
	slog.Info("loaded surveys", "count", len(manager.ListSurveys()), "data_dir", dataDir)

	// What this host can measure, said once here from what building the
	// scanner reported, and again by the readiness scan below when it learns
	// something the constructor could not — a permission the OS has not
	// granted, for instance.
	surveyHandler := api.NewSurveyServiceHandler(manager)
	if scannerErr != nil {
		surveyHandler.SetCaptureCapability(api.CaptureCapability{Reason: scannerErr.Error()})
	} else {
		go reportCaptureReadiness(ctx, scanner, surveyHandler)
	}

	// A credential turns the daemon from a desktop app into one serving other
	// devices: every RPC is authenticated and CSRF-checked, and the listener
	// gets TLS below. Without one the handler is registered bare and the bind
	// gate has already confined it to loopback.
	options := []connect.HandlerOption{connect.WithReadMaxBytes(maxUploadBytes)}
	var gate *auth.Gate
	if credential.Configured() {
		if gate, err = auth.NewGate(credential); err != nil {
			return err
		}
		defer gate.Close()
		options = append(options, connect.WithInterceptors(gate.Interceptor()))
	}

	mux := http.NewServeMux()
	path, handler := surveyv1connect.NewSurveyServiceHandler(surveyHandler, options...)
	mux.Handle(path, handler)
	if gate != nil {
		gate.RegisterRoutes(mux)
	}
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

	ln, boundAddr, err := listen(ctx, addr, explicitAddr)
	if err != nil {
		return err
	}
	scheme := "http"
	if gate != nil {
		tlsConfig, err := tlsConfigFor(dataDir)
		if err != nil {
			return err
		}
		ln, scheme = tls.NewListener(ln, tlsConfig), "https"
	}

	srv := &http.Server{
		Addr:              boundAddr,
		Handler:           mux,
		Protocols:         &protocols,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("trellisd listening", "addr", boundAddr, "scheme", scheme, "authenticated", gate != nil)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

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
