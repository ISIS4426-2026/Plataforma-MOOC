// Package auth holds the credential primitives shared by registration, login,
// password recovery and administrative user management.
package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// MaxPasswordBytes is the hard limit imposed by bcrypt: it silently truncates
// anything past 72 bytes, which would make two different long passwords
// interchangeable. Longer input is rejected instead of quietly accepted.
const MaxPasswordBytes = 72

// ErrPasswordTooLong is returned when a password exceeds MaxPasswordBytes.
var ErrPasswordTooLong = errors.New("password exceeds the maximum supported length")

// ErrPasswordMismatch is returned when a password does not match the hash. It
// is deliberately indistinguishable from a malformed-hash failure so callers
// cannot leak whether an account exists.
var ErrPasswordMismatch = errors.New("password does not match")

// HashPassword derives a bcrypt hash suitable for the users.password_hash
// column. The cost is bcrypt.DefaultCost, which the library raises over time.
func HashPassword(plain string) (string, error) {
	// len() counts bytes, which is what bcrypt truncates on; a multi-byte
	// password can pass a rune-based check and still be cut short.
	if len(plain) > MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword reports whether plain matches hash.
//
// It returns ErrPasswordMismatch for both a wrong password and an unusable
// stored hash, so a caller cannot tell the two apart and neither the password
// nor the hash is ever included in the error.
func VerifyPassword(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return ErrPasswordMismatch
	}
	return nil
}
