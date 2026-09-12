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
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Environment variables carrying the operator credential. Both must be set
// before the daemon will serve anything off loopback.
const (
	EnvUsername = "TRELLIS_AUTH_USERNAME"
	EnvPassword = "TRELLIS_AUTH_PASSWORD"
)

// Argon2id RFC 9106 second-recommended parameters, the same values Seed and
// Stem use. Keeping them identical across the fleet means a hash produced by
// one product is readable by the others if the credential ever moves.
const (
	argon2Memory  = 64 * 1024 // KiB
	argon2Time    = 3
	argon2Threads = 4
	argon2SaltLen = 16
	argon2KeyLen  = 32

	argon2idPrefix = "$argon2id$"
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

// NewCredential hashes password and returns the credential to verify against.
func NewCredential(username, password string) (Credential, error) {
	if username == "" || password == "" {
		return Credential{}, ErrMissingCredentials
	}
	if len(password) < MinPasswordLength {
		return Credential{}, ErrPasswordTooShort
	}
	hash, err := hashPassword(password)
	if err != nil {
		return Credential{}, err
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

// Verify reports whether username and password are the configured credential.
//
// An unconfigured Credential refuses every input, including the empty one: the
// comparison runs against an empty stored hash, which no password can produce.
func (c Credential) Verify(username, password string) error {
	if !c.Configured() {
		return ErrInvalidCredentials
	}
	// Both halves are compared in constant time and neither short-circuits, so
	// a wrong username costs the same as a wrong password.
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(c.username)) == 1
	passOK := verifyPassword(password, c.hash)
	if !userOK || !passOK {
		return ErrInvalidCredentials
	}
	return nil
}

// hashPassword returns a PHC-encoded Argon2id hash of password.
func hashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return fmt.Sprintf("%sv=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2idPrefix, argon2.Version, argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// verifyPassword reports whether password produces encoded.
func verifyPassword(password, encoded string) bool {
	salt, want, memory, time, threads, ok := parsePHC(encoded)
	if !ok {
		return false
	}
	// The stored key must be exactly the length we produce. Deriving to
	// len(want) instead would let a malformed hash choose the comparison
	// length, and a one-byte key would then match one password in 256.
	if len(want) != argon2KeyLen {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, argon2KeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// parsePHC decodes a PHC-encoded Argon2id string. The cost parameters are read
// back from the hash rather than assumed, so a credential hashed under earlier
// parameters still verifies if the constants above are ever raised.
func parsePHC(encoded string) (salt, key []byte, memory, time uint32, threads uint8, ok bool) {
	if !strings.HasPrefix(encoded, argon2idPrefix) {
		return nil, nil, 0, 0, 0, false
	}
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, key
	const wantParts = 6
	if len(parts) != wantParts {
		return nil, nil, 0, 0, 0, false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, nil, 0, 0, 0, false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return nil, nil, 0, 0, 0, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, 0, 0, 0, false
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, 0, 0, 0, false
	}
	return salt, key, m, t, p, true
}
