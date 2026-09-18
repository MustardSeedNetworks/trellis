// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/MustardSeedNetworks/foundation/pkg/httpserver"
)

// An operator with a real certificate names it; otherwise the daemon generates
// a self-signed one, as Seed, Stem and NIAC do.
const (
	envTLSCert = "TRELLIS_TLS_CERT"
	envTLSKey  = "TRELLIS_TLS_KEY"
)

// certDirName is where a generated self-signed pair lives under the data
// directory. The file names inside it come from foundation.
const certDirName = "certs"

// listenConfig is what the daemon knows about its listener before it opens it.
type listenConfig struct {
	// addr is the address to bind, and explicit marks it as operator-supplied
	// via TRELLIS_ADDR rather than the built-in default.
	addr     string
	explicit bool

	// protected marks an operator credential as configured. It is what turns
	// the daemon from the loopback desktop app of ADR-0007 into one another
	// device can reach, and so it is also what puts TLS in front (#160).
	protected bool

	dataDir string
}

// listen opens the daemon's listener.
//
// Unprotected, Trellis is plain HTTP confined to loopback by requireBindAllowed,
// so there is no TLS to configure and no https scheme to redirect anything to:
// this takes foundation's canonical-port fallback alone. Protected, it takes
// the whole shared listener — TLS, the self-signed default, and the same-port
// plaintext 308 for an operator who types the address without a scheme.
func listen(ctx context.Context, cfg listenConfig) (net.Listener, error) {
	logger := slog.Default()

	if !cfg.protected {
		if cfg.explicit {
			return httpserver.BindExact(ctx, logger, cfg.addr)
		}
		return httpserver.Bind(ctx, logger, cfg.addr)
	}

	certFile, keyFile := cleanPath(os.Getenv(envTLSCert)), cleanPath(os.Getenv(envTLSKey))
	if (certFile == "") != (keyFile == "") {
		return nil, fmt.Errorf("%s and %s must be set together", envTLSCert, envTLSKey)
	}

	return httpserver.Listen(ctx, httpserver.Config{
		Addr:     cfg.addr,
		Explicit: cfg.explicit,
		// MinVersion is left at foundation's default of TLS 1.3, which is what
		// seed, stem and niac already serve. trellis asked for 1.2 alone, and
		// keeping the fleet's floor in one place is the point of the shared
		// listener; foundation refuses anything below 1.2 at start.
		CertFile: certFile,
		KeyFile:  keyFile,
		CertDir:  filepath.Join(cleanPath(cfg.dataDir), certDirName),
		Cert: httpserver.CertOptions{
			CommonName: "trellisd",
			// The surveyor reaches the daemon by whatever address their tablet
			// can route to, which is not knowable here, so the certificate
			// names the host alongside the loopback SANs foundation always
			// adds, and is expected to be trusted out of band.
			DNSNames: certDNSNames(),
		},
		Logger: logger,
	})
}

// certDNSNames returns the names the generated certificate must cover. A host
// with no resolvable name yields just "localhost" rather than an empty SAN,
// which no certificate can be verified against.
func certDNSNames() []string {
	names := []string{"localhost"}
	if host, err := os.Hostname(); err == nil && host != "" && host != "localhost" {
		names = append(names, host)
	}
	return names
}

// cleanPath normalises an operator-supplied path, leaving "" as "".
func cleanPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}
