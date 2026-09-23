// SPDX-License-Identifier: BUSL-1.1

package auth_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

// The gate's own tests drive a mux with hand-built headers. This one drives it
// the way the operator's browser does: over TLS, through a cookie jar that
// applies Set-Cookie itself. Trellis has no Playwright coverage of logout — the
// E2E daemon runs on loopback with no credential and so registers no /auth
// routes at all — and a jar is what makes the difference between a cookie the
// browser dropped and a credential the daemon stopped honouring. Secure is
// unconditional on these cookies, so the server has to be a TLS one: a jar
// withholds a Secure cookie from an http:// request and the test would pass
// having sent nothing.
func newBrowser(t *testing.T) (*auth.Gate, *httptest.Server, *http.Client) {
	t.Helper()
	g, mux := newGateMux(t)
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	client := srv.Client()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client.Jar = jar
	return g, srv, client
}

// do sends one request the way the browser would: with the jar's cookies, and
// with the body drained so the connection is reusable.
func do(t *testing.T, client *http.Client, method, target, body string, header http.Header) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, target, reader)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close %s %s: %v", method, target, err)
		}
	}()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s: %v", method, target, err)
	}
	return resp.StatusCode, payload
}

// browserLogin returns the CSRF token the login response handed back.
func browserLogin(t *testing.T, srv *httptest.Server, client *http.Client) string {
	t.Helper()
	body := `{"username":"surveyor","password":"` + testPassword + `"}`
	code, payload := do(t, client, http.MethodPost, srv.URL+"/auth/login", body,
		http.Header{"Content-Type": []string{"application/json"}})
	if code != http.StatusOK {
		t.Fatalf("login returned %d, want 200", code)
	}
	var out struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("login response: %v", err)
	}
	if out.CSRFToken == "" {
		t.Fatal("login returned no CSRF token")
	}
	return out.CSRFToken
}

// browserProbe reports what /auth/session tells client.
func browserProbe(t *testing.T, srv *httptest.Server, client *http.Client) (authenticated bool, csrfToken string) {
	t.Helper()
	_, payload := do(t, client, http.MethodGet, srv.URL+"/auth/session", "", nil)
	var out struct {
		Authenticated bool   `json:"authenticated"`
		CSRFToken     string `json:"csrfToken"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("session probe response: %v", err)
	}
	return out.Authenticated, out.CSRFToken
}

func TestBrowserLogoutLeavesNothingUsable(t *testing.T) {
	t.Parallel()
	_, srv, client := newBrowser(t)
	site, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("server URL: %v", err)
	}

	csrfToken := browserLogin(t, srv, client)
	if authenticated, _ := browserProbe(t, srv, client); !authenticated {
		t.Fatal("a fresh login is not authenticated")
	}

	// What an operator's machine still holds at the moment they click Log out:
	// a copy taken here is the retained-cookie case, and it is the only way to
	// tell a cleared jar apart from a revoked credential.
	retained := client.Jar.Cookies(site)
	if len(retained) != 2 {
		t.Fatalf("the jar holds %d session cookies, want 2", len(retained))
	}

	code, _ := do(t, client, http.MethodPost, srv.URL+"/auth/logout", "",
		http.Header{auth.HeaderCSRFToken: []string{csrfToken}})
	if code != http.StatusNoContent {
		t.Fatalf("logout returned %d, want 204", code)
	}
	if left := client.Jar.Cookies(site); len(left) != 0 {
		t.Errorf("logout left %d cookies in the jar", len(left))
	}

	// A second browser handed the cookies the first one was told to forget.
	stale := srv.Client()
	staleJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	staleJar.SetCookies(site, retained)
	stale.Jar = staleJar

	if authenticated, token := browserProbe(t, srv, stale); authenticated || token != "" {
		t.Errorf("retained cookies probe as authenticated=%v, minting token %q", authenticated, token)
	}
	if code, _ := do(t, stale, http.MethodPost, srv.URL+"/auth/refresh", "", nil); code != http.StatusUnauthorized {
		t.Errorf("retained refresh cookie returned %d, want 401", code)
	}

	// Revocation must end this session, not the operator's ability to start one.
	if token := browserLogin(t, srv, client); token == "" {
		t.Fatal("logging in again after logout did not return a session")
	}
	if authenticated, _ := browserProbe(t, srv, client); !authenticated {
		t.Fatal("the session after a fresh login is not authenticated")
	}
}

// The browser's own sign-out carries the token; a page on another origin that
// gets the browser to post the same request cannot. That request must leave
// the jar and the session exactly as they were (#576).
func TestBrowserLogoutWithoutTheCSRFTokenChangesNothing(t *testing.T) {
	t.Parallel()
	_, srv, client := newBrowser(t)
	site, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("server URL: %v", err)
	}
	browserLogin(t, srv, client)

	code, _ := do(t, client, http.MethodPost, srv.URL+"/auth/logout", "", nil)
	if code != http.StatusForbidden {
		t.Fatalf("logout without the CSRF token returned %d, want 403", code)
	}
	if held := client.Jar.Cookies(site); len(held) != 2 {
		t.Errorf("the refused logout left %d session cookies in the jar, want 2", len(held))
	}
	if authenticated, token := browserProbe(t, srv, client); !authenticated || token == "" {
		t.Fatalf("after a refused logout the session probes as authenticated=%v with token %q", authenticated, token)
	}
}

// The UI renews a session when an RPC is refused as unauthenticated
// (UI-TRL-14). That path rests on three answers, taken here the way the
// browser sees them once the fifteen-minute access cookie has lapsed: an RPC
// is unauthenticated, not permission_denied (which the UI deliberately leaves
// alone); the refresh cookie on its own renews the session; and only the CSRF
// token the refresh returned admits the retry, because the refresh revoked the
// one the RPC was first sent with.
func TestBrowserRefreshAfterTheAccessCookieLapses(t *testing.T) {
	t.Parallel()
	g, srv, client := newBrowser(t)
	site, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("server URL: %v", err)
	}
	first := browserLogin(t, srv, client)

	var kept []*http.Cookie
	for _, c := range client.Jar.Cookies(site) {
		if c.Name == auth.CookieRefresh {
			kept = append(kept, c)
		}
	}
	if len(kept) != 1 {
		t.Fatalf("the jar holds %d refresh cookies, want 1", len(kept))
	}
	lapsed, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	lapsed.SetCookies(site, kept)
	client.Jar = lapsed

	rpc := func(csrfToken string) (bool, error) {
		h := http.Header{auth.HeaderCSRFToken: []string{csrfToken}}
		for _, c := range client.Jar.Cookies(site) {
			h.Add("Cookie", c.String())
		}
		return callWith(t, g, h)
	}

	if _, err := rpc(first); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("an RPC after the access cookie lapsed was refused with %v, want unauthenticated", connect.CodeOf(err))
	}

	code, payload := do(t, client, http.MethodPost, srv.URL+"/auth/refresh", "", nil)
	if code != http.StatusOK {
		t.Fatalf("refresh with only the refresh cookie returned %d, want 200", code)
	}
	var out struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(payload, &out); err != nil || out.CSRFToken == "" {
		t.Fatalf("refresh returned no CSRF token: %v", err)
	}

	if _, err := rpc(first); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a retry with the spent CSRF token was refused with %v, want permission_denied", connect.CodeOf(err))
	}
	if reached, err := rpc(out.CSRFToken); err != nil || !reached {
		t.Fatalf("a retry with the renewed CSRF token was refused: %v", err)
	}
}
