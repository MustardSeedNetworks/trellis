// SPDX-License-Identifier: BUSL-1.1

// Package auth authenticates the single operator Trellis serves.
//
// Trellis is a one-surveyor desktop tool, so it takes Stem's shape rather than
// Seed's: the credential comes from the environment at startup and there is no
// user store, no roles and no registration. Stem dropped its own users table
// for this reason (stem#344) — a credential table with no reader implies an
// authentication path the product does not have.
package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"

	"github.com/MustardSeedNetworks/foundation/pkg/password"
)

// Environment variables carrying the operator credential. Both must be set
// before the daemon will serve anything off loopback.
const (
	EnvUsername = "TRELLIS_AUTH_USERNAME"
	EnvPassword = "TRELLIS_AUTH_PASSWORD"
)

// MinPasswordLength is the shortest operator password accepted. A surveyor's
// daemon can end up on a client's guest network, so the floor is checked when
// the credential is built rather than left to whoever writes the env file.
const MinPasswordLength = 12

var (
	// ErrInvalidCredentials is returned for any failed verification. It is
	// deliberately one error for every cause: telling a caller whether the
	// username or the password was wrong tells an attacker which half to keep.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrMissingCredentials indicates the operator credential is not configured.
	ErrMissingCredentials = errors.New(
		"missing required credentials: set " + EnvUsername + " and " + EnvPassword,
	)

	// ErrPasswordTooShort indicates the configured password is below MinPasswordLength.
	ErrPasswordTooShort = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
)

// Credential is the operator identity the daemon authenticates against. The
// zero value is the unconfigured daemon: it holds no hash and Verify refuses
// everything, which is what makes "no usable default credential" true here
// without a placeholder hash to reject.
type Credential struct {
	username string
	hash     string
}

// NewCredential hashes pass and returns the credential to verify against.
func NewCredential(username, pass string) (Credential, error) {
	if username == "" || pass == "" {
		return Credential{}, ErrMissingCredentials
	}
	if len(pass) < MinPasswordLength {
		return Credential{}, ErrPasswordTooShort
	}
	hash, err := password.Hash(pass)
	if err != nil {
		return Credential{}, fmt.Errorf("hash operator password: %w", err)
	}
	return Credential{username: username, hash: hash}, nil
}

// CredentialFromEnv builds the credential from EnvUsername and EnvPassword.
func CredentialFromEnv() (Credential, error) {
	return NewCredential(os.Getenv(EnvUsername), os.Getenv(EnvPassword))
}

// Configured reports whether a credential was supplied at all.
func (c Credential) Configured() bool { return c.hash != "" }

// Username returns the operator's username, empty if unconfigured.
func (c Credential) Username() string { return c.username }

// Hash returns the PHC-encoded Argon2id hash, empty if unconfigured.
func (c Credential) Hash() string { return c.hash }

// Verify reports whether username and pass are the configured credential.
//
// An unconfigured Credential refuses every input, including the empty one.
func (c Credential) Verify(username, pass string) error {
	if !c.Configured() {
		return ErrInvalidCredentials
	}
	// Both halves are compared in constant time and neither short-circuits, so
	// a wrong username costs the same as a wrong password.
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(c.username)) == 1
	// An error means an empty or oversized input; either is a wrong password.
	passOK, err := password.Verify(c.hash, pass)
	if err != nil || !userOK || !passOK {
		return ErrInvalidCredentials
	}
	return nil
}
