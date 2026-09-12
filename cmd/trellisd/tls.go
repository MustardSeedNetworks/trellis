// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// An operator with a real certificate names it; otherwise the daemon generates
// a self-signed one, as Seed, Stem and NIAC do.
const (
	envTLSCert = "TRELLIS_TLS_CERT"
	envTLSKey  = "TRELLIS_TLS_KEY"
)

const (
	certLifetime = 365 * 24 * time.Hour
	certDirName  = "certs"
	certFileName = "trellisd.crt"
	keyFileName  = "trellisd.key"
)

// tlsConfigFor returns the server TLS configuration for a protected daemon,
// generating a self-signed certificate under dataDir if the operator named none.
//
// TLS 1.2 is the floor, matching the rest of the fleet.
func tlsConfigFor(dataDir string) (*tls.Config, error) {
	// Cleaned because these come from the environment: an operator naming a
	// path with traversal in it gets the path they meant, not a walk out of it.
	certPath, keyPath := cleanPath(os.Getenv(envTLSCert)), cleanPath(os.Getenv(envTLSKey))
	if (certPath == "") != (keyPath == "") {
		return nil, fmt.Errorf("%s and %s must be set together", envTLSCert, envTLSKey)
	}
	if certPath == "" {
		dir := filepath.Join(cleanPath(dataDir), certDirName)
		certPath, keyPath = filepath.Join(dir, certFileName), filepath.Join(dir, keyFileName)
		if err := ensureSelfSignedCert(dir, certPath, keyPath); err != nil {
			return nil, err
		}
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load TLS keypair: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, nil
}

// ensureSelfSignedCert writes a self-signed certificate and key if none exists.
func ensureSelfSignedCert(dir, certPath, keyPath string) error {
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", certPath, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}
	host, _ := os.Hostname()
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "trellisd", Organization: []string{"Mustard Seed Networks"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(certLifetime),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		// The surveyor reaches the daemon by whatever address their tablet can
		// route to, which is not knowable here, so the certificate names the
		// host and loopback and is expected to be trusted out of band.
		DNSNames:    []string{"localhost", host},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}
	if err := writePEM(certPath, 0o600, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	return writePEM(keyPath, 0o600, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func writePEM(path string, mode os.FileMode, block *pem.Block) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := pem.Encode(f, block); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}

// cleanPath normalises an operator-supplied path, leaving "" as "".
func cleanPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}
