package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// tokenBytes is the entropy behind session and verification tokens. 256 bits
// puts guessing far out of reach, so these values need no rate limiting of
// their own to be unguessable.
const tokenBytes = 32

// GenerateToken returns a new random token together with the hash to persist.
//
// The raw value is returned to the caller exactly once, to be mailed or handed
// to the client; only the hash is ever stored. Callers must not log the raw
// value.
func GenerateToken() (raw string, hash string, err error) {
	buf := make([]byte, tokenBytes)
	// crypto/rand.Read is documented to always fill the buffer or return an
	// error, so a short read cannot silently produce a weak token.
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	// URL-safe and unpadded so the value can be dropped into a verification
	// link without escaping.
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken derives the stored representation of a token.
//
// Plain SHA-256 is deliberate: unlike a password, a token carries full random
// entropy, so there is nothing to brute-force and a slow KDF would only add
// latency to every authenticated request. The hash is hex encoded to fit the
// VARCHAR(255) columns used by the schema.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
