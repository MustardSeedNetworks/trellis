// SPDX-License-Identifier: BUSL-1.1

package auth_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

func newGate(t *testing.T) *auth.Gate {
	t.Helper()
	g, err := auth.NewGate(mustCredential(t, "surveyor", testPassword))
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	t.Cleanup(g.Close)
	return g
}

func newGateMux(t *testing.T) (*auth.Gate, *http.ServeMux) {
	t.Helper()
	g := newGate(t)
	mux := http.NewServeMux()
	g.RegisterRoutes(mux)
	return g, mux
}

// session is what a browser holds after a successful login.
type session struct {
	cookies []*http.Cookie
	csrf    string
	// raw overrides cookies when a test needs to send a value under a name it
	// was not issued under, which no real http.Cookie would carry.
	raw []string
}

func login(t *testing.T, mux *http.ServeMux, user, password string) (*httptest.ResponseRecorder, session) {
	t.Helper()
	body := strings.NewReader(`{"username":"` + user + `","password":"` + password + `"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/login", body)
	req.RemoteAddr = "10.0.0.1:54321"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var out struct {
		CSRFToken string `json:"csrfToken"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, session{cookies: rec.Result().Cookies(), csrf: out.CSRFToken}
}

// refresh sends the refresh request a browser holding s would send.
func refresh(t *testing.T, mux *http.ServeMux, s session) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/refresh", nil)
	req.Header = s.header()
	req.RemoteAddr = "10.0.0.1:54321"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// probeSession returns the session /auth/session hands back to a caller
// holding s — the reload path, and the recovery path a logout has to close.
func probeSession(t *testing.T, mux *http.ServeMux, s session) session {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/session", nil)
	req.Header = s.header()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out struct {
		Authenticated bool   `json:"authenticated"`
		CSRFToken     string `json:"csrfToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("session probe response: %v", err)
	}
	return session{cookies: s.cookies, csrf: out.CSRFToken}
}

// logout sends the logout request a browser holding s would send.
func logout(t *testing.T, mux *http.ServeMux, s session) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
	req.Header = s.header()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout returned %d, want 204: %s", rec.Code, rec.Body)
	}
}

// renewedSession reads the session a successful refresh handed back.
func renewedSession(t *testing.T, rec *httptest.ResponseRecorder) session {
	t.Helper()
	var out struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("refresh response: %v", err)
	}
	return session{cookies: rec.Result().Cookies(), csrf: out.CSRFToken}
}

// header builds the request headers a browser would send carrying s.
func (s session) header() http.Header {
	h := http.Header{}
	for _, c := range s.cookies {
		h.Add("Cookie", c.Name+"="+c.Value)
	}
	for _, c := range s.raw {
		h.Add("Cookie", c)
	}
	if s.csrf != "" {
		h.Set(auth.HeaderCSRFToken, s.csrf)
	}
	return h
}

// cookieByName returns the session a browser would still hold if it kept only
// one of the two cookies. The retained-cookie cases below each start from one.
func cookieByName(s session, name string) session {
	var kept []*http.Cookie
	for _, c := range s.cookies {
		if c.Name == name {
			kept = append(kept, c)
		}
	}
	return session{cookies: kept}
}

func TestLoginIssuesASession(t *testing.T) {
	t.Parallel()
	_, mux := newGateMux(t)
	rec, s := login(t, mux, "surveyor", testPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("login returned %d, want 200: %s", rec.Code, rec.Body)
	}
	if s.csrf == "" {
		t.Fatal("login returned no CSRF token")
	}
	var access, refresh bool
	for _, c := range s.cookies {
		switch c.Name {
		case auth.CookieAccess:
			access = true
		case auth.CookieRefresh:
			refresh = true
		}
		if !c.HttpOnly || !c.Secure {
			t.Errorf("cookie %s is not HttpOnly+Secure", c.Name)
		}
	}
	if !access || !refresh {
		t.Fatalf("login set access=%v refresh=%v, want both", access, refresh)
	}
	if strings.Contains(rec.Body.String(), testPassword) {
		t.Fatal("the login response echoes the password")
	}
}

func TestLoginRefusesWrongCredentials(t *testing.T) {
	t.Parallel()
	_, mux := newGateMux(t)
	rec, s := login(t, mux, "surveyor", "wrong-horse-battery")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("login with a wrong password returned %d, want 401", rec.Code)
	}
	if len(s.cookies) != 0 {
		t.Fatalf("a failed login set %d cookies", len(s.cookies))
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	t.Parallel()
	_, mux := newGateMux(t)
	var got int
	for range auth.LoginAttemptLimit + 1 {
		rec, _ := login(t, mux, "surveyor", "wrong-horse-battery")
		got = rec.Code
	}
	if got != http.StatusTooManyRequests {
		t.Fatalf("the attempt past the limit returned %d, want 429", got)
	}
	// The limit must hold even once the right password arrives, or it is only
	// slowing an attacker down until they guess.
	rec, _ := login(t, mux, "surveyor", testPassword)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a correct password inside a rate-limited window returned %d, want 429", rec.Code)
	}
}

func TestAuthenticateAcceptsALiveSession(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	user, err := g.Authenticate(s.header(), "10.0.0.1")
	if err != nil {
		t.Fatalf("Authenticate on a fresh session: %v", err)
	}
	if user != "surveyor" {
		t.Fatalf("Authenticate returned %q, want surveyor", user)
	}
}

func TestAuthenticateRefusesWhatItMust(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)

	t.Run("no cookie at all", func(t *testing.T) {
		t.Parallel()
		h := http.Header{}
		h.Set(auth.HeaderCSRFToken, s.csrf)
		if _, err := g.Authenticate(h, "10.0.0.1"); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Fatalf("an anonymous request authenticated: %v", err)
		}
	})

	t.Run("no csrf token", func(t *testing.T) {
		t.Parallel()
		bare := session{cookies: s.cookies}
		if _, err := g.Authenticate(bare.header(), "10.0.0.1"); !errors.Is(err, auth.ErrCSRF) {
			t.Fatalf("a session cookie with no CSRF token authenticated: %v", err)
		}
	})

	t.Run("wrong csrf token", func(t *testing.T) {
		t.Parallel()
		wrong := session{cookies: s.cookies, csrf: "not-the-token"}
		if _, err := g.Authenticate(wrong.header(), "10.0.0.1"); !errors.Is(err, auth.ErrCSRF) {
			t.Fatalf("a wrong CSRF token authenticated: %v", err)
		}
	})

	t.Run("refresh cookie as access", func(t *testing.T) {
		t.Parallel()
		var refresh *http.Cookie
		for _, c := range s.cookies {
			if c.Name == auth.CookieRefresh {
				refresh = c
			}
		}
		swapped := session{raw: []string{auth.CookieAccess + "=" + refresh.Value}, csrf: s.csrf}
		if _, err := g.Authenticate(swapped.header(), "10.0.0.1"); err == nil {
			t.Fatal("a refresh token presented as an access token authenticated")
		}
	})
}

func TestLogoutEndsTheSession(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
	req.Header = s.header()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout returned %d, want 204", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge >= 0 {
			t.Errorf("logout left cookie %s alive (MaxAge=%d)", c.Name, c.MaxAge)
		}
	}
	// The CSRF token was minted against this session and must not outlive it.
	if _, err := g.Authenticate(s.header(), "10.0.0.1"); err == nil {
		t.Fatal("the session still authenticates after logout")
	}
}

func TestRefreshRenewsTheAccessToken(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/refresh", nil)
	req.Header = s.header()
	req.RemoteAddr = "10.0.0.1:54321"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh returned %d, want 200: %s", rec.Code, rec.Body)
	}
	var renewed *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieAccess {
			renewed = c
		}
	}
	if renewed == nil || renewed.Value == "" {
		t.Fatal("refresh did not set a new access cookie")
	}
	var out struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.CSRFToken == "" {
		t.Fatalf("refresh returned no CSRF token for the renewed session: %v", err)
	}
	next := session{cookies: []*http.Cookie{renewed}, csrf: out.CSRFToken}
	if _, err := g.Authenticate(next.header(), "10.0.0.1"); err != nil {
		t.Fatalf("the renewed session does not authenticate: %v", err)
	}
}

// Refresh shares the login budget: it is the other unauthenticated route that
// validates a caller-supplied token, so without a limiter a peer on a routable
// bind can hammer it with a bad cookie for free.
func TestRefreshIsRateLimited(t *testing.T) {
	t.Parallel()
	_, mux := newGateMux(t)
	bad := session{raw: []string{auth.CookieRefresh + "=not-a-token"}}

	var got int
	for range auth.LoginAttemptLimit + 1 {
		rec := refresh(t, mux, bad)
		got = rec.Code
	}
	if got != http.StatusTooManyRequests {
		t.Fatalf("the refresh past the limit returned %d, want 429", got)
	}
}

// A browser refreshes on a timer, so the budget a bad refresh spends has to
// come back when a good one succeeds — otherwise an ordinary session locks
// itself out every quarter hour.
func TestSuccessfulRefreshesAreNotRateLimited(t *testing.T) {
	t.Parallel()
	_, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)

	for i := range auth.LoginAttemptLimit + 1 {
		rec := refresh(t, mux, s)
		if rec.Code != http.StatusOK {
			t.Fatalf("refresh %d returned %d, want 200: %s", i+1, rec.Code, rec.Body)
		}
		s = renewedSession(t, rec)
	}
}

func TestRefreshRefusesAnAccessToken(t *testing.T) {
	t.Parallel()
	_, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	var access *http.Cookie
	for _, c := range s.cookies {
		if c.Name == auth.CookieAccess {
			access = c
		}
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/refresh", nil)
	req.Header = session{raw: []string{auth.CookieRefresh + "=" + access.Value}, csrf: s.csrf}.header()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh with an access token returned %d, want 401", rec.Code)
	}
}

// A gate cannot exist without a credential: that is what makes "no usable
// default credential" structural rather than a check someone can forget.
func TestNewGateRequiresACredential(t *testing.T) {
	t.Parallel()
	if _, err := auth.NewGate(auth.Credential{}); !errors.Is(err, auth.ErrMissingCredentials) {
		t.Fatalf("NewGate with no credential = %v, want ErrMissingCredentials", err)
	}
}

// The UI has to know whether this daemon wants a login before it renders
// anything. A daemon with no gate never registers the route, so a 404 is the
// "no credential configured" answer and needs no flag of its own.
func TestSessionProbe(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)

	probe := func(h http.Header) (int, map[string]any) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/session", nil)
		if h != nil {
			req.Header = h
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	code, out := probe(nil)
	if code != http.StatusOK {
		t.Fatalf("anonymous probe returned %d, want 200", code)
	}
	if out["authenticated"] != false {
		t.Fatalf("anonymous probe says authenticated=%v", out["authenticated"])
	}
	if _, leaked := out["csrfToken"]; leaked {
		t.Fatal("the anonymous probe handed out a CSRF token")
	}

	_, s := login(t, mux, "surveyor", testPassword)
	code, out = probe(s.header())
	if code != http.StatusOK {
		t.Fatalf("authenticated probe returned %d, want 200", code)
	}
	if out["authenticated"] != true || out["username"] != "surveyor" {
		t.Fatalf("authenticated probe returned %v", out)
	}
	// A reload must recover a usable CSRF token, or the session survives in the
	// cookie while every RPC fails.
	token, _ := out["csrfToken"].(string)
	if token == "" {
		t.Fatal("the authenticated probe returned no CSRF token")
	}
	recovered := session{cookies: s.cookies, csrf: token}
	if _, err := g.Authenticate(recovered.header(), "10.0.0.1"); err != nil {
		t.Fatalf("the token the probe returned does not authenticate: %v", err)
	}
}

// Logout has to end the credentials, not only the cookies. A caller who kept
// the access cookie reaches /auth/session, which mints a CSRF token for any
// access token that still parses — handing back exactly the pair Authenticate
// wants (#501).
func TestLogoutRevokesTheAccessToken(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	logout(t, mux, s)

	retained := cookieByName(s, auth.CookieAccess)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/session", nil)
	req.Header = retained.header()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var out struct {
		Authenticated bool   `json:"authenticated"`
		CSRFToken     string `json:"csrfToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("session probe response: %v", err)
	}
	if out.Authenticated {
		t.Error("the probe called a logged-out access cookie authenticated")
	}
	if out.CSRFToken != "" {
		t.Error("the probe minted a CSRF token for a logged-out access cookie")
	}
	recovered := session{cookies: retained.cookies, csrf: out.CSRFToken}
	if _, err := g.Authenticate(recovered.header(), "10.0.0.1"); err == nil {
		t.Fatal("a logged-out access cookie authenticated through the session probe")
	}
}

// The refresh cookie outlives the access cookie by a week, so a logout that
// leaves it usable leaves the session usable for that week (#501).
func TestLogoutRevokesTheRefreshToken(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	logout(t, mux, s)

	rec := refresh(t, mux, cookieByName(s, auth.CookieRefresh))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh with a logged-out cookie returned %d, want 401: %s", rec.Code, rec.Body)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("the refused refresh still issued cookies")
	}
	restored := renewedSession(t, rec)
	if _, err := g.Authenticate(restored.header(), "10.0.0.1"); err == nil {
		t.Fatal("a logged-out refresh cookie minted a working session")
	}
}

// The access token lasts fifteen minutes and the refresh cookie a week, so an
// operator who logs out after an idle spell presents only the refresh cookie.
// Revoking what that request actually carries is the difference between ending
// the session and ending nothing (#501).
func TestLogoutWithOnlyTheRefreshCookie(t *testing.T) {
	t.Parallel()
	_, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	logout(t, mux, cookieByName(s, auth.CookieRefresh))

	rec := refresh(t, mux, cookieByName(s, auth.CookieRefresh))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after a refresh-cookie-only logout returned %d, want 401: %s", rec.Code, rec.Body)
	}
}

// Logout is a state change like any RPC, so a request carrying the session
// has to prove it came from the page holding that session's CSRF token. A
// refused logout must change nothing: not the credentials, and not the
// browser's cookies either, since clearing them is the forged request's whole
// effect (#576).
func TestLogoutRefusesARequestWithoutTheCSRFToken(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		// strip builds the forged request's session from the operator's.
		strip func(session) session
		want  int
	}{
		{"both cookies, no token", func(s session) session { return session{cookies: s.cookies} }, http.StatusForbidden},
		{"both cookies, wrong token", func(s session) session { return session{cookies: s.cookies, csrf: "not-the-token"} }, http.StatusForbidden},
		{"access cookie only, no token", func(s session) session { return cookieByName(s, auth.CookieAccess) }, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, mux := newGateMux(t)
			_, s := login(t, mux, "surveyor", testPassword)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
			req.Header = tc.strip(s).header()
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("forged logout returned %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
			if cookies := rec.Result().Cookies(); len(cookies) != 0 {
				t.Errorf("the refused logout set %d cookies", len(cookies))
			}
			if _, err := g.Authenticate(s.header(), "10.0.0.1"); err != nil {
				t.Fatalf("the operator's session stopped authenticating after a refused logout: %v", err)
			}
			if rec := refresh(t, mux, s); rec.Code != http.StatusOK {
				t.Fatalf("the operator's refresh cookie returned %d after a refused logout, want 200", rec.Code)
			}
		})
	}
}

// An access cookie the daemon no longer honours — a restart changed the
// signing key, or it outlived its token by a second — is not a session to prove
// anything about, so it must not stop the refresh cookie beside it being ended.
func TestLogoutWithADeadAccessCookie(t *testing.T) {
	t.Parallel()
	_, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)
	dead := cookieByName(s, auth.CookieRefresh)
	dead.raw = []string{auth.CookieAccess + "=not-a-token"}
	logout(t, mux, dead)

	if rec := refresh(t, mux, cookieByName(s, auth.CookieRefresh)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after a logout beside a dead access cookie returned %d, want 401: %s", rec.Code, rec.Body)
	}
}

// Exchanging a refresh token spends it. Without rotation a copied refresh
// cookie stays usable for its whole lifetime alongside the operator's own
// session, and each exchange extends it again (#501).
func TestRefreshRotatesTheRefreshToken(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)

	first := refresh(t, mux, s)
	if first.Code != http.StatusOK {
		t.Fatalf("first refresh returned %d, want 200: %s", first.Code, first.Body)
	}
	renewed := renewedSession(t, first)
	if _, err := g.Authenticate(renewed.header(), "10.0.0.1"); err != nil {
		t.Fatalf("the renewed session does not authenticate: %v", err)
	}

	replay := refresh(t, mux, cookieByName(s, auth.CookieRefresh))
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("the spent refresh token returned %d, want 401: %s", replay.Code, replay.Body)
	}
}
