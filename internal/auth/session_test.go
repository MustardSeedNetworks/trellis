// SPDX-License-Identifier: BUSL-1.1

package auth_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

func newSessions(t *testing.T) *auth.SessionManager {
	t.Helper()
	m, err := auth.NewSessionManager(nil)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	return m
}

func TestSessionRoundTrip(t *testing.T) {
	t.Parallel()
	m := newSessions(t)
	access, refresh, err := m.Issue("surveyor")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if access == refresh {
		t.Fatal("the access and refresh tokens are the same string")
	}
	for name, tc := range map[string]struct {
		token string
		kind  auth.TokenType
	}{
		"access":  {access, auth.TokenAccess},
		"refresh": {refresh, auth.TokenRefresh},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			user, err := m.Validate(tc.token, tc.kind)
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if user != "surveyor" {
				t.Fatalf("Validate returned %q, want surveyor", user)
			}
		})
	}
}

// A refresh token presented as an access token must be refused, or the 7-day
// token silently becomes a 15-minute one's equal.
func TestSessionRefusesTheWrongTokenType(t *testing.T) {
	t.Parallel()
	m := newSessions(t)
	access, refresh, err := m.Issue("surveyor")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := m.Validate(refresh, auth.TokenAccess); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("refresh token accepted as access: %v", err)
	}
	if _, err := m.Validate(access, auth.TokenRefresh); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("access token accepted as refresh: %v", err)
	}
}

// A token signed by a different manager must not validate here: the secret is
// what separates this daemon's sessions from anyone else's.
func TestSessionRefusesAForeignSignature(t *testing.T) {
	t.Parallel()
	mine, theirs := newSessions(t), newSessions(t)
	foreign, _, err := theirs.Issue("surveyor")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := mine.Validate(foreign, auth.TokenAccess); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("a foreign-signed token validated: %v", err)
	}
}

func TestSessionRefusesGarbage(t *testing.T) {
	t.Parallel()
	m := newSessions(t)
	for _, token := range []string{"", "not-a-jwt", "a.b.c", "Bearer x"} {
		if _, err := m.Validate(token, auth.TokenAccess); !errors.Is(err, auth.ErrInvalidToken) {
			t.Fatalf("Validate(%q) = %v, want ErrInvalidToken", token, err)
		}
	}
}

func TestSessionRefusesAnExpiredToken(t *testing.T) {
	t.Parallel()
	m := newSessions(t)
	expired, err := m.IssueAt("surveyor", auth.TokenAccess, time.Now().Add(-2*auth.AccessTokenDuration))
	if err != nil {
		t.Fatalf("IssueAt: %v", err)
	}
	if _, err := m.Validate(expired, auth.TokenAccess); !errors.Is(err, auth.ErrTokenExpired) {
		t.Fatalf("Validate on an expired token = %v, want ErrTokenExpired", err)
	}
}

// A session cookie only ever travels over the TLS listener a configured
// credential turns on, so every flag is unconditional.
// Revoking either token of a pair ends both: they carry one session id, so a
// logout holding whichever cookie survived closes the whole session.
func TestRevokeEndsBothTokensOfAPair(t *testing.T) {
	t.Parallel()
	for name, revokeRefresh := range map[string]bool{"via the access token": false, "via the refresh token": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := newSessions(t)
			access, refresh, err := m.Issue("surveyor")
			if err != nil {
				t.Fatalf("Issue: %v", err)
			}
			if revokeRefresh {
				m.Revoke(refresh)
			} else {
				m.Revoke(access)
			}
			if _, err := m.Validate(access, auth.TokenAccess); !errors.Is(err, auth.ErrTokenRevoked) {
				t.Errorf("the access token validates with %v, want revoked", err)
			}
			if _, err := m.Validate(refresh, auth.TokenRefresh); !errors.Is(err, auth.ErrTokenRevoked) {
				t.Errorf("the refresh token validates with %v, want revoked", err)
			}
		})
	}
}

// A revocation has to outlive everything it could be asked about. The refresh
// token is the long half of the pair, so an entry kept only for an access
// token's lifetime stops blocking the cookie that matters after fifteen
// minutes — while the cookie itself is good for a week.
func TestRevocationOutlivesTheAccessTokenLifetime(t *testing.T) {
	t.Parallel()
	m := newSessions(t)
	stale, err := m.IssueAt("surveyor", auth.TokenRefresh, time.Now().Add(-2*auth.AccessTokenDuration))
	if err != nil {
		t.Fatalf("IssueAt: %v", err)
	}
	if _, err := m.Validate(stale, auth.TokenRefresh); err != nil {
		t.Fatalf("the refresh token is not valid to begin with: %v", err)
	}
	m.Revoke(stale)
	if _, err := m.Validate(stale, auth.TokenRefresh); !errors.Is(err, auth.ErrTokenRevoked) {
		t.Fatalf("a refresh token issued before the access lifetime validates with %v, want revoked", err)
	}
}

// Logout is called with whatever the browser still holds, which may be nothing
// or something that never was a token. Nothing to revoke is not an error.
func TestRevokeIgnoresWhatIsNotAToken(t *testing.T) {
	t.Parallel()
	m := newSessions(t)
	access, _, err := m.Issue("surveyor")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	for _, token := range []string{"", "not.a.token", access + "tampered"} {
		m.Revoke(token)
	}
	if _, err := m.Validate(access, auth.TokenAccess); err != nil {
		t.Fatalf("a live session was revoked by unusable input: %v", err)
	}
}

func TestSessionCookiesCarryTheSecurityFlags(t *testing.T) {
	t.Parallel()
	c := auth.NewSessionCookie(auth.CookieAccess, "token-value", auth.AccessTokenDuration)
	if !c.HttpOnly {
		t.Error("cookie is not HttpOnly — script can read the session")
	}
	if !c.Secure {
		t.Error("cookie is not Secure — the session can travel in clear")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie SameSite = %v, want Strict", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("cookie Path = %q, want /", c.Path)
	}
}

func TestClearedCookieExpiresImmediately(t *testing.T) {
	t.Parallel()
	c := auth.ClearSessionCookie(auth.CookieAccess)
	if c.MaxAge >= 0 {
		t.Fatalf("cleared cookie MaxAge = %d, want negative", c.MaxAge)
	}
	if c.Value != "" {
		t.Fatalf("cleared cookie still carries a value: %q", c.Value)
	}
}
