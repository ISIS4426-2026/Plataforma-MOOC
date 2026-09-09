package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
)

func TestHashPasswordProducesVerifiableHash(t *testing.T) {
	const plain = "correct horse battery staple"

	hash, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword returned an error: %v", err)
	}

	if hash == plain {
		t.Fatal("hash must never equal the plaintext password")
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("expected a bcrypt hash, got %q", hash)
	}

	if err := auth.VerifyPassword(hash, plain); err != nil {
		t.Errorf("VerifyPassword rejected the correct password: %v", err)
	}
}

// bcrypt seeds a fresh salt per call, so the same password must never produce a
// repeated hash. Equal hashes would let an attacker spot shared passwords from
// the database alone.
func TestHashPasswordIsSaltedPerCall(t *testing.T) {
	const plain = "same-password"

	first, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("first HashPassword failed: %v", err)
	}
	second, err := auth.HashPassword(plain)
	if err != nil {
		t.Fatalf("second HashPassword failed: %v", err)
	}

	if first == second {
		t.Error("expected different hashes for the same password, salting is not applied")
	}

	for _, hash := range []string{first, second} {
		if err := auth.VerifyPassword(hash, plain); err != nil {
			t.Errorf("VerifyPassword rejected a valid hash: %v", err)
		}
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := auth.HashPassword("the-right-one")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	err = auth.VerifyPassword(hash, "the-wrong-one")
	if !errors.Is(err, auth.ErrPasswordMismatch) {
		t.Errorf("expected ErrPasswordMismatch, got %v", err)
	}
}

// A malformed hash must fail exactly like a wrong password: distinguishing the
// two would tell an attacker which accounts have unusable credentials.
func TestVerifyPasswordRejectsMalformedHashIndistinguishably(t *testing.T) {
	err := auth.VerifyPassword("not-a-bcrypt-hash", "any-password")
	if !errors.Is(err, auth.ErrPasswordMismatch) {
		t.Errorf("expected ErrPasswordMismatch for a malformed hash, got %v", err)
	}
}

// bcrypt silently truncates past 72 bytes, which would make two distinct long
// passwords interchangeable. The limit must be rejected, not absorbed.
func TestHashPasswordRejectsOverlongPassword(t *testing.T) {
	tooLong := strings.Repeat("a", auth.MaxPasswordBytes+1)

	_, err := auth.HashPassword(tooLong)
	if !errors.Is(err, auth.ErrPasswordTooLong) {
		t.Errorf("expected ErrPasswordTooLong, got %v", err)
	}
}

func TestHashPasswordAcceptsPasswordAtTheLimit(t *testing.T) {
	atLimit := strings.Repeat("a", auth.MaxPasswordBytes)

	hash, err := auth.HashPassword(atLimit)
	if err != nil {
		t.Fatalf("a password of exactly %d bytes must be accepted: %v", auth.MaxPasswordBytes, err)
	}
	if err := auth.VerifyPassword(hash, atLimit); err != nil {
		t.Errorf("VerifyPassword rejected the boundary password: %v", err)
	}
}
