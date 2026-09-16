// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token lifetimes, matching Seed and Stem so an operator moving between the
// products meets one session behaviour rather than three.
const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour

	jwtSecretLength = 32
	sessionIDLength = 16
	issuer          = "Trellis"
)

// Cookie names. The prefix keeps them distinct from a sibling product's cookies
// when both are served from localhost during development.
const (
	CookieAccess  = "trellis_access"
	CookieRefresh = "trellis_refresh"
)

// TokenType distinguishes the short access token from the long refresh token.
// It is carried inside the signed claims, so a refresh token cannot be replayed
// as an access token even though both are signed by the same key.
type TokenType string

const (
	TokenAccess  TokenType = "access"
	TokenRefresh TokenType = "refresh"
)

var (
	// ErrInvalidToken signals a token that could not be parsed, was signed by
	// another key, or is not of the type the caller asked for.
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenExpired indicates a well-formed token that is past its expiry.
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenRevoked indicates a token whose session was ended — by a logout,
	// or by the refresh that spent it — before it expired on its own.
	ErrTokenRevoked = errors.New("token revoked")
)

// claims is Trellis's JWT payload. There is one principal, so there is no role,
// scope or tenant here — only who, which kind of token, and which session.
//
// SessionID names the login, not the token: the access and refresh tokens of a
// pair carry the same one. That is what lets a logout holding either cookie end
// both, and a refresh end the pair it replaces. It is deliberately not jti,
// which RFC 7519 requires to be unique per token.
type claims struct {
	jwt.RegisteredClaims

	Username  string    `json:"username"`
	TokenType TokenType `json:"token_type"`
	SessionID string    `json:"sid"`
}

// SessionManager issues and validates the operator's session tokens.
//
// Signed tokens are self-contained, so ending a session early takes state the
// signature cannot carry: revoked holds the sessions that must stop being
// honoured, against the moment nothing carrying them can still parse.
type SessionManager struct {
	secret []byte

	mu      sync.Mutex
	revoked map[string]time.Time
}

// NewSessionManager returns a manager signing with secret. A nil or empty
// secret gets a fresh random one, which means sessions do not survive a restart
// — correct for a daemon an operator starts and stops with their survey.
func NewSessionManager(secret []byte) (*SessionManager, error) {
	if len(secret) == 0 {
		secret = make([]byte, jwtSecretLength)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate session secret: %w", err)
		}
	}
	return &SessionManager{secret: secret, revoked: map[string]time.Time{}}, nil
}

// Issue returns a new access and refresh token pair for username. Both carry
// one session id, so revoking the session revokes the pair.
func (m *SessionManager) Issue(username string) (access, refresh string, err error) {
	now := time.Now()
	session, err := newSessionID()
	if err != nil {
		return "", "", err
	}
	if access, err = m.issue(username, TokenAccess, now, session); err != nil {
		return "", "", err
	}
	if refresh, err = m.issue(username, TokenRefresh, now, session); err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// IssueAt returns one token of kind, issued as if at issuedAt, in a session of
// its own. Tests use it to produce an already-expired token without waiting.
func (m *SessionManager) IssueAt(username string, kind TokenType, issuedAt time.Time) (string, error) {
	session, err := newSessionID()
	if err != nil {
		return "", err
	}
	return m.issue(username, kind, issuedAt, session)
}

func newSessionID() (string, error) {
	id := make([]byte, sessionIDLength)
	if _, err := rand.Read(id); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(id), nil
}

func (m *SessionManager) issue(username string, kind TokenType, issuedAt time.Time, session string) (string, error) {
	lifetime := AccessTokenDuration
	if kind == TokenRefresh {
		lifetime = RefreshTokenDuration
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(lifetime)),
		},
		Username:  username,
		TokenType: kind,
		SessionID: session,
	})
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign %s token: %w", kind, err)
	}
	return signed, nil
}

// Validate parses token, requires it to be of type want, and returns the
// username it carries.
func (m *SessionManager) Validate(token string, want TokenType) (string, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		// Pinning the method is what stops an "alg: none" or an RS256 token
		// whose "public key" is our HMAC secret from being accepted.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(issuer))
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return "", ErrTokenExpired
	case err != nil:
		return "", fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if c.TokenType != want {
		return "", fmt.Errorf("%w: token is a %s token, wanted %s", ErrInvalidToken, c.TokenType, want)
	}
	if c.Username == "" {
		return "", fmt.Errorf("%w: token carries no username", ErrInvalidToken)
	}
	if c.SessionID == "" {
		return "", fmt.Errorf("%w: token carries no session id", ErrInvalidToken)
	}
	if m.isRevoked(c.SessionID) {
		return "", ErrTokenRevoked
	}
	return c.Username, nil
}

// Revoke ends the session token belongs to, so neither it nor the other token
// of its pair is honoured again.
//
// It is called with whatever cookies a caller still has, which on a logout may
// be one, none, or something that never was a token, so anything unusable is
// simply nothing to revoke: a token that does not parse is already refused.
func (m *SessionManager) Revoke(token string) {
	var c claims
	if _, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(issuer)); err != nil {
		return
	}
	if c.SessionID == "" || c.IssuedAt == nil {
		return
	}
	// An entry has to outlive everything it could be asked about. The refresh
	// token is the longer-lived half of the pair and shares the issue time, so
	// its expiry is when the session stops being able to present itself at all.
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for session, until := range m.revoked {
		if now.After(until) {
			delete(m.revoked, session)
		}
	}
	m.revoked[c.SessionID] = c.IssuedAt.Add(RefreshTokenDuration)
}

func (m *SessionManager) isRevoked(session string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	until, found := m.revoked[session]
	return found && time.Now().Before(until)
}

// NewSessionCookie returns the cookie carrying a session token.
//
// Secure is unconditional. There is no plain-HTTP session to serve: the gate
// exists only when a credential is configured, and a configured credential is
// exactly what turns the listener into a TLS one. A daemon on loopback issues
// no session cookies at all.
func NewSessionCookie(name, value string, lifetime time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(lifetime.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

// ClearSessionCookie returns the cookie that deletes a session cookie. The
// attributes are repeated rather than borrowed from NewSessionCookie: a
// browser only replaces a cookie when the new one matches on them.
func ClearSessionCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}
