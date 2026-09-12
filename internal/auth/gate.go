// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/MustardSeedNetworks/foundation/pkg/csrf"
)

// HeaderCSRFToken is the header carrying the per-session CSRF token, the same
// name Seed, Stem and NIAC use.
const HeaderCSRFToken = "X-CSRF-Token"

// Login attempt budget: five tries per quarter hour per client, matching the
// fleet's auth limiter.
const (
	LoginAttemptLimit  = 5
	LoginAttemptWindow = 15 * time.Minute
)

var (
	// ErrUnauthenticated indicates the request carried no usable session.
	ErrUnauthenticated = errors.New("not authenticated")
	// ErrCSRF indicates a session without a valid CSRF token for it.
	ErrCSRF = errors.New("missing or invalid CSRF token")
)

// Gate is the daemon's authentication surface: the login routes an operator
// reaches to get a session, and the check every RPC goes through afterwards.
//
// It exists only when a credential is configured. The daemon on loopback — a
// desktop app serving its own operator's browser, which is what ADR-0007
// describes — runs without one and installs no gate, so the local experience is
// unchanged. A routable bind requires one.
type Gate struct {
	cred     Credential
	sessions *SessionManager
	csrf     *csrf.Manager
	limiter  *RateLimiter
	// secure marks the session cookies Secure. It follows the listener: a
	// Secure cookie is never sent back over the plain-HTTP loopback listener.
	secure bool
}

// NewGate returns the gate authenticating against cred.
func NewGate(cred Credential, secure bool) (*Gate, error) {
	if !cred.Configured() {
		return nil, ErrMissingCredentials
	}
	sessions, err := NewSessionManager(nil)
	if err != nil {
		return nil, err
	}
	return &Gate{
		cred:     cred,
		sessions: sessions,
		csrf:     csrf.NewManager(),
		limiter:  NewRateLimiter(LoginAttemptLimit, LoginAttemptWindow),
		secure:   secure,
	}, nil
}

// Close releases the gate's background workers.
func (g *Gate) Close() { g.csrf.Stop() }

// RegisterRoutes adds the login surface to mux. These routes are the only ones
// reachable without a session, along with the UI itself, /healthz and
// /__version — the operator has to be able to load the login page and a
// deployment check has to be able to read the version without credentials.
func (g *Gate) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", g.handleLogin)
	mux.HandleFunc("POST /auth/logout", g.handleLogout)
	mux.HandleFunc("POST /auth/refresh", g.handleRefresh)
}

// Authenticate checks the session a request carries and returns its username.
//
// It takes the header and client address rather than an *http.Request so the
// Connect interceptor, which sees only those two, can call the same code the
// HTTP routes do.
func (g *Gate) Authenticate(h http.Header, client string) (string, error) {
	token := cookieValue(h, CookieAccess)
	if token == "" {
		return "", ErrUnauthenticated
	}
	user, err := g.sessions.Validate(token, TokenAccess)
	if err != nil {
		return "", err
	}
	if err := g.csrf.Validate(csrf.SessionKey(token), h.Get(HeaderCSRFToken)); err != nil {
		return "", ErrCSRF
	}
	return user, nil
}

func (g *Gate) handleLogin(w http.ResponseWriter, r *http.Request) {
	client := clientAddr(r)
	if !g.limiter.Allow(client) {
		// Deliberately not reset on success: a client inside a rate-limited
		// window is refused even with the right password, or the limit only
		// slows an attacker until they guess.
		writeAuthError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody)).Decode(&body); err != nil {
		writeAuthError(w, http.StatusBadRequest, "malformed request")
		return
	}
	if err := g.cred.Verify(body.Username, body.Password); err != nil {
		slog.Warn("login refused", "event", "auth.forbidden", "reason", "credentials", "client", client)
		writeAuthError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	g.issueSession(w, body.Username, client)
}

func (g *Gate) handleRefresh(w http.ResponseWriter, r *http.Request) {
	client := clientAddr(r)
	token := cookieValue(r.Header, CookieRefresh)
	user, err := g.sessions.Validate(token, TokenRefresh)
	if err != nil {
		slog.Warn("refresh refused", "event", "auth.forbidden", "reason", "token", "client", client)
		writeAuthError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	// The old session's CSRF token is revoked with the tokens it was minted
	// against, so a refreshed session cannot be driven by the previous one.
	g.csrf.Revoke(csrf.SessionKey(cookieValue(r.Header, CookieAccess)))
	g.issueSession(w, user, client)
}

func (g *Gate) handleLogout(w http.ResponseWriter, r *http.Request) {
	g.csrf.Revoke(csrf.SessionKey(cookieValue(r.Header, CookieAccess)))
	http.SetCookie(w, ClearSessionCookie(CookieAccess, g.secure))
	http.SetCookie(w, ClearSessionCookie(CookieRefresh, g.secure))
	w.WriteHeader(http.StatusNoContent)
}

// issueSession mints a token pair, sets the cookies and returns the CSRF token
// the UI must echo on every RPC.
func (g *Gate) issueSession(w http.ResponseWriter, user, client string) {
	access, refresh, err := g.sessions.Issue(user)
	if err != nil {
		slog.Error("could not issue session", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "could not issue session")
		return
	}
	csrfToken, err := g.csrf.Generate(csrf.SessionKey(access))
	if err != nil {
		slog.Error("could not mint CSRF token", "error", err)
		writeAuthError(w, http.StatusInternalServerError, "could not issue session")
		return
	}
	g.limiter.Reset(client)

	http.SetCookie(w, NewSessionCookie(CookieAccess, access, AccessTokenDuration, g.secure))
	http.SetCookie(w, NewSessionCookie(CookieRefresh, refresh, RefreshTokenDuration, g.secure))
	writeJSON(w, http.StatusOK, map[string]string{"username": user, "csrfToken": csrfToken})
}

// maxLoginBody bounds the login request body. Credentials are small; anything
// larger is not a login.
const maxLoginBody = 4 << 10

func writeAuthError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encoding a map of strings cannot fail, and the status is already written.
	_ = json.NewEncoder(w).Encode(body)
}

// cookieValue reads one cookie out of a header. http.Request.Cookie is not
// usable here because the Connect interceptor holds a header, not a request.
func cookieValue(h http.Header, name string) string {
	for _, c := range (&http.Request{Header: h}).Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// clientAddr is the rate-limiter key. It is the transport peer, never a
// forwarded header: Trellis sits in front of no proxy, so a header here would
// be an attacker-chosen limiter key.
func clientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
