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
	g, err := auth.NewGate(mustCredential(t, "surveyor", testPassword), true)
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
}

func login(t *testing.T, mux *http.ServeMux, user, password string) (*httptest.ResponseRecorder, session) {
	t.Helper()
	body := strings.NewReader(`{"username":"` + user + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", body)
	req.RemoteAddr = "10.0.0.1:54321"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var out struct {
		CSRFToken string `json:"csrfToken"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, session{cookies: rec.Result().Cookies(), csrf: out.CSRFToken}
}

// header builds the request headers a browser would send carrying s.
func (s session) header() http.Header {
	h := http.Header{}
	for _, c := range s.cookies {
		h.Add("Cookie", c.Name+"="+c.Value)
	}
	if s.csrf != "" {
		h.Set(auth.HeaderCSRFToken, s.csrf)
	}
	return h
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
		swapped := session{
			cookies: []*http.Cookie{{Name: auth.CookieAccess, Value: refresh.Value}},
			csrf:    s.csrf,
		}
		if _, err := g.Authenticate(swapped.header(), "10.0.0.1"); err == nil {
			t.Fatal("a refresh token presented as an access token authenticated")
		}
	})
}

func TestLogoutEndsTheSession(t *testing.T) {
	t.Parallel()
	g, mux := newGateMux(t)
	_, s := login(t, mux, "surveyor", testPassword)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
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

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
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
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header = session{
		cookies: []*http.Cookie{{Name: auth.CookieRefresh, Value: access.Value}},
		csrf:    s.csrf,
	}.header()
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
	if _, err := auth.NewGate(auth.Credential{}, true); !errors.Is(err, auth.ErrMissingCredentials) {
		t.Fatalf("NewGate with no credential = %v, want ErrMissingCredentials", err)
	}
}
