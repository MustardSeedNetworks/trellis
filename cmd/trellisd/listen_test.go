// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/foundation/pkg/httpserver"
)

// freeRange returns the first of n consecutive free TCP ports on loopback,
// having closed them again. Nothing else in this binary competes for them, so
// the window between probing and binding is advisory but sufficient.
func freeRange(t *testing.T, n int) int {
	t.Helper()

	var lc net.ListenConfig
	for attempt := 0; attempt < 20; attempt++ {
		probe, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("probe for a free port: %v", err)
		}
		base := probe.Addr().(*net.TCPAddr).Port
		_ = probe.Close()

		held := make([]net.Listener, 0, n)
		for offset := range n {
			ln, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(base+offset)))
			if err != nil {
				break
			}
			held = append(held, ln)
		}
		for _, ln := range held {
			_ = ln.Close()
		}
		if len(held) == n {
			return base
		}
	}
	t.Fatalf("no run of %d consecutive free ports after 20 attempts", n)
	return 0
}

// hold binds addr and closes it when the test ends, standing in for whatever
// else on the machine is squatting the port.
func hold(t *testing.T, port int) {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("hold port %d: %v", port, err)
	}
	t.Cleanup(func() { _ = ln.Close() })
}

// serve runs a minimal daemon on ln for the duration of the test, so a request
// against it exercises the listener rather than a bare Accept.
func serve(t *testing.T, ln net.Listener) {
	t.Helper()

	srv := &http.Server{
		ReadHeaderTimeout: readHeaderTimeout,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "trellisd")
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
}

// An operator who names a port wants that port. Silently serving a different
// one would break whatever they pointed at it.
//
// Both listeners are asserted because they reach the rule by different routes:
// the plaintext one picks BindExact itself, the TLS one passes Explicit down to
// foundation, and either can be got wrong without the other noticing.
func TestListenDoesNotWalkForAnExplicitAddress(t *testing.T) {
	for _, protected := range []bool{false, true} {
		t.Run(map[bool]string{false: "plaintext", true: "tls"}[protected], func(t *testing.T) {
			port := freeRange(t, 2)
			hold(t, port)

			ln, err := listen(t.Context(), listenConfig{
				addr:      net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
				explicit:  true,
				protected: protected,
				dataDir:   t.TempDir(),
			})
			if err == nil {
				_ = ln.Close()
				t.Fatal("listen walked past an explicitly requested port that was in use")
			}
		})
	}
}

// The built-in default walks instead: a Trellis.app launched from the Finder
// has no terminal, so a port it cannot have reads as an app that does nothing
// at all (#151). Both listeners again, for the same reason as above.
func TestListenWalksForTheDefaultAddress(t *testing.T) {
	for _, protected := range []bool{false, true} {
		t.Run(map[bool]string{false: "plaintext", true: "tls"}[protected], func(t *testing.T) {
			port := freeRange(t, 2)
			hold(t, port)

			ln, err := listen(t.Context(), listenConfig{
				addr:      net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
				protected: protected,
				dataDir:   t.TempDir(),
			})
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			defer func() { _ = ln.Close() }()

			want := net.JoinHostPort("127.0.0.1", strconv.Itoa(port+1))
			if got := ln.Addr().String(); got != want {
				t.Errorf("bound %s, want %s", got, want)
			}
		})
	}
}

// Unprotected, the daemon is the loopback desktop app of ADR-0007: plain HTTP,
// no certificate generated and nothing to redirect to.
func TestUnprotectedListenerServesPlaintextAndWritesNoCertificate(t *testing.T) {
	dataDir := t.TempDir()

	ln, err := listen(t.Context(), listenConfig{addr: "127.0.0.1:0", dataDir: dataDir})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	serve(t, ln)

	resp, err := get(t, http.DefaultClient, "http://"+ln.Addr().String()+"/")
	if err != nil {
		t.Fatalf("plaintext GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if _, err := os.Stat(filepath.Join(dataDir, certDirName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat certs dir: %v, want it never created", err)
	}
}

// A credential is what puts TLS in front of the daemon, and the certificate it
// generates lands under the data directory rather than the working directory.
func TestProtectedListenerServesTLSFromTheDataDir(t *testing.T) {
	dataDir := t.TempDir()

	ln, err := listen(t.Context(), listenConfig{addr: "127.0.0.1:0", protected: true, dataDir: dataDir})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	serve(t, ln)

	// The client trusts exactly the certificate the listener wrote under the
	// data directory, so a handshake that succeeds proves both that TLS is on
	// and that the generated pair is the one being served -- which skipping
	// verification would not.
	for _, name := range []string{httpserver.DefaultCertFileName, httpserver.DefaultKeyFileName} {
		if _, err := os.Stat(filepath.Join(dataDir, certDirName, name)); err != nil {
			t.Fatalf("stat %s under the data dir: %v", name, err)
		}
	}
	pemBytes, err := os.ReadFile(filepath.Join(dataDir, certDirName, httpserver.DefaultCertFileName))
	if err != nil {
		t.Fatalf("read the generated certificate: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pemBytes) {
		t.Fatal("the generated certificate is not a usable PEM")
	}

	// The surveyor reaches the daemon by whatever address their tablet can
	// route to, so the certificate has to cover this host's own name and not
	// only loopback.
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("the generated certificate has no PEM block")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse the generated certificate: %v", err)
	}
	// Named from os.Hostname rather than from certDNSNames, which is the code
	// under test: asking the production helper what to expect would agree with
	// itself however wrong it got.
	want := []string{"localhost", "127.0.0.1"}
	if host, err := os.Hostname(); err == nil && host != "" && host != "localhost" {
		want = append(want, host)
	}
	for _, name := range want {
		if err := leaf.VerifyHostname(name); err != nil {
			t.Errorf("the generated certificate does not cover %q: %v", name, err)
		}
	}
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13}},
	}

	resp, err := get(t, client, "https://"+ln.Addr().String()+"/")
	if err != nil {
		t.Fatalf("https GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// UI-TRL-11: an operator who types the address without a scheme reaches the
// page instead of a connection refused, and nothing else is served in plain
// text. The Location keeps the host, port and path it was asked for.
func TestProtectedListenerRedirectsPlaintextToHTTPS(t *testing.T) {
	ln, err := listen(t.Context(), listenConfig{addr: "127.0.0.1:0", protected: true, dataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	serve(t, ln)

	addr := ln.Addr().String()
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatalf("dial plaintext: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	if _, err := io.WriteString(conn, "GET /surveys HTTP/1.1\r\nHost: "+addr+"\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatalf("write plaintext request: %v", err)
	}
	raw, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read plaintext response: %v", err)
	}

	response := string(raw)
	if !strings.HasPrefix(response, "HTTP/1.1 308 ") {
		t.Fatalf("plaintext response %q, want a 308", firstLine(response))
	}
	location := headerValue(response, "Location")
	want := (&url.URL{Scheme: "https", Host: addr, Path: "/surveys"}).String()
	if location != want {
		t.Errorf("Location %q, want %q", location, want)
	}
	if strings.Contains(response, "trellisd") {
		t.Error("the redirect served the handler's body in plain text")
	}
}

// A certificate without its key is a misconfiguration, not a reason to quietly
// generate a self-signed pair the operator did not ask for.
func TestProtectedListenRejectsACertificateWithoutItsKey(t *testing.T) {
	t.Setenv(envTLSCert, filepath.Join(t.TempDir(), "operator.crt"))

	ln, err := listen(t.Context(), listenConfig{addr: "127.0.0.1:0", protected: true, dataDir: t.TempDir()})
	if err == nil {
		_ = ln.Close()
		t.Fatal("listen accepted a certificate with no key")
	}
	if !strings.Contains(err.Error(), envTLSKey) {
		t.Errorf("error %q does not name %s", err, envTLSKey)
	}
}

// get issues a GET carrying the test's context, which is what the daemon sees
// from a real client and what noctx asks for.
func get(t *testing.T, client *http.Client, target string) (*http.Response, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatalf("build request for %s: %v", target, err)
	}
	return client.Do(req)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\r\n")
	return line
}

// headerValue returns the value of the named header in a raw HTTP/1.x
// response, or "" when it is absent.
func headerValue(response, name string) string {
	head, _, _ := strings.Cut(response, "\r\n\r\n")
	for _, line := range strings.Split(head, "\r\n")[1:] {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
