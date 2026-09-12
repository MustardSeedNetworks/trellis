// SPDX-License-Identifier: BUSL-1.1

package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

const testPassword = "correct-horse-battery"

func mustCredential(t *testing.T, user, password string) auth.Credential {
	t.Helper()
	c, err := auth.NewCredential(user, password)
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	return c
}

func TestCredentialVerifiesItsOwnPassword(t *testing.T) {
	t.Parallel()
	c := mustCredential(t, "surveyor", testPassword)
	if err := c.Verify("surveyor", testPassword); err != nil {
		t.Fatalf("Verify with the configured credential: %v", err)
	}
}

func TestCredentialRefusesWrongInput(t *testing.T) {
	t.Parallel()
	c := mustCredential(t, "surveyor", testPassword)
	for name, tc := range map[string]struct{ user, pass string }{
		"wrong password":     {"surveyor", "wrong-horse-battery"},
		"wrong username":     {"someone", testPassword},
		"empty password":     {"surveyor", ""},
		"empty username":     {"", testPassword},
		"password as user":   {testPassword, "surveyor"},
		"password prefix":    {"surveyor", testPassword[:len(testPassword)-1]},
		"username case":      {"Surveyor", testPassword},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := c.Verify(tc.user, tc.pass); !errors.Is(err, auth.ErrInvalidCredentials) {
				t.Fatalf("Verify(%q, %q) = %v, want ErrInvalidCredentials", tc.user, tc.pass, err)
			}
		})
	}
}

// A hash must never be reproducible from the password alone: two credentials
// built from the same password differ, or the salt is not doing its job.
func TestCredentialHashesAreSalted(t *testing.T) {
	t.Parallel()
	a := mustCredential(t, "surveyor", testPassword)
	b := mustCredential(t, "surveyor", testPassword)
	if a.Hash() == b.Hash() {
		t.Fatal("two credentials over the same password produced an identical hash")
	}
	if !strings.HasPrefix(a.Hash(), "$argon2id$v=19$") {
		t.Fatalf("hash is not a PHC-encoded Argon2id string: %q", a.Hash())
	}
	if strings.Contains(a.Hash(), testPassword) {
		t.Fatal("the hash contains the password in clear")
	}
}

func TestCredentialRefusesAShortPassword(t *testing.T) {
	t.Parallel()
	if _, err := auth.NewCredential("surveyor", "short"); !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Fatalf("NewCredential with a short password = %v, want ErrPasswordTooShort", err)
	}
}

func TestCredentialRefusesAnEmptyUsername(t *testing.T) {
	t.Parallel()
	if _, err := auth.NewCredential("", testPassword); !errors.Is(err, auth.ErrMissingCredentials) {
		t.Fatalf("NewCredential with no username = %v, want ErrMissingCredentials", err)
	}
}

// The fleet's first-run invariant: no usable default credential. Trellis has no
// credential store to seed, so the invariant is that an unconfigured daemon
// produces no Credential at all — there is nothing to authenticate against.
func TestFirstRun_NoUsableDefaultCredential(t *testing.T) {
	t.Setenv(auth.EnvUsername, "")
	t.Setenv(auth.EnvPassword, "")
	if _, err := auth.CredentialFromEnv(); !errors.Is(err, auth.ErrMissingCredentials) {
		t.Fatalf("CredentialFromEnv with nothing set = %v, want ErrMissingCredentials", err)
	}
	var zero auth.Credential
	if err := zero.Verify("", ""); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("the zero Credential authenticated the empty password: %v", err)
	}
	if err := zero.Verify("admin", "admin"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("the zero Credential authenticated admin/admin: %v", err)
	}
}

func TestCredentialFromEnv(t *testing.T) {
	t.Setenv(auth.EnvUsername, "surveyor")
	t.Setenv(auth.EnvPassword, testPassword)
	c, err := auth.CredentialFromEnv()
	if err != nil {
		t.Fatalf("CredentialFromEnv: %v", err)
	}
	if err := c.Verify("surveyor", testPassword); err != nil {
		t.Fatalf("credential from the environment does not verify: %v", err)
	}
}

func TestCredentialFromEnvRequiresBoth(t *testing.T) {
	for name, env := range map[string][2]string{
		"no password": {"surveyor", ""},
		"no username": {"", testPassword},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(auth.EnvUsername, env[0])
			t.Setenv(auth.EnvPassword, env[1])
			if _, err := auth.CredentialFromEnv(); !errors.Is(err, auth.ErrMissingCredentials) {
				t.Fatalf("CredentialFromEnv = %v, want ErrMissingCredentials", err)
			}
		})
	}
}

func TestConfigured(t *testing.T) {
	t.Parallel()
	var zero auth.Credential
	if zero.Configured() {
		t.Fatal("the zero Credential reports itself configured")
	}
	if !mustCredential(t, "surveyor", testPassword).Configured() {
		t.Fatal("a real credential reports itself unconfigured")
	}
}
