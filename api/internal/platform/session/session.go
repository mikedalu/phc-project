// Package session handles refresh tokens: generation, hashing, and the
// rotation-with-reuse-detection policy.
//
// ═══ WHY REFRESH TOKENS EXIST ═══
//
// An access token cannot be revoked before it expires. Keeping it short (15 min)
// bounds that exposure, but then the client must obtain a new one constantly —
// which is only acceptable offline if it does not need the network every time.
// The refresh token is the long-lived, revocable half of the pair.
//
// ═══ ROTATION AND REUSE DETECTION ═══
//
// Every refresh consumes the token and issues a new one. If a token that has
// ALREADY been used is presented again, one of two things happened:
//
//  1. An attacker stole the token and used it.
//  2. The legitimate client retried after a lost response.
//
// We cannot tell these apart, so we assume the worse case: revoke the ENTIRE
// token family. That logs out both the attacker and the legitimate user, which
// is the correct trade — a forced re-login is annoying, a silently hijacked
// session is not.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// tokenBytes is the entropy of a refresh token.
//
// 32 bytes = 256 bits. At 10^9 guesses per second that is ~10^60 years, so the
// token is unguessable by brute force and the only real threat is theft.
const tokenBytes = 32

// NewToken returns a fresh opaque refresh token, base64url-encoded.
//
// The raw value is returned to the caller ONCE, to be sent to the client. Only
// its hash is stored: a database dump must not yield usable sessions.
func NewToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("session: generate token: %w", err)
	}
	// RawURLEncoding: no padding, URL-safe. The token travels in JSON, so
	// either encoding works, but URL-safe keeps it usable in a query string if
	// that is ever needed for a debugging tool.
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns the hex-encoded SHA-256 of a refresh token.
//
// ═══ WHY SHA-256 AND NOT ARGON2 ═══
//
// Argon2 is for LOW-ENTROPY secrets — passwords, PINs — where the attacker can
// mount a dictionary attack. A 256-bit random token has no dictionary: brute
// force is hopeless regardless of how fast the hash is. Using Argon2 here would
// add ~75 ms to every refresh (and refresh happens constantly) while buying no
// security, and it would let an attacker DoS the API by spamming refreshes.
//
// SHA-256 is the right tool for a high-entropy secret. The comparison at lookup
// time is by database index on the hash, so the timing-attack concern that
// motivated constant-time comparison for passwords does not apply.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
