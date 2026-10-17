// Package devicekey verifies device signatures.
//
// A device signature answers a different question from a JWT: not "is this a
// valid session?" but "is this the same physical machine the session was issued
// to, and is it still trusted?". See docs/05-SECURITY.md §4.1.
//
// ═══ THE ONE TRAP IN THIS FILE ═══
//
// WebCrypto (browsers) emits ECDSA signatures as ASN.1 DER, so the verifier must
// use ecdsa.VerifyASN1. Some embedded runtimes — Node's WebCrypto, some
// WebViews — emit the raw IEEE P1363 form instead: exactly 64 bytes, r||s, no
// structure. A verifier that knows only one format produces "signature is valid
// and always fails", which is a miserable afternoon to debug.
//
// So both are accepted here, and the tests assert that both verify while a
// tampered message verifies with neither. Accepting two encodings of the same
// signature does not widen the attack surface: both are checked against the same
// key and the same message hash.
package devicekey

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const (
	// AlgorithmES256 is ECDSA over P-256 with SHA-256. The only algorithm the
	// devices table accepts today, enforced by a CHECK constraint.
	AlgorithmES256 = "ES256"

	// MaxPublicKeyChars bounds the header/body value BEFORE decoding. A P-256
	// SPKI key is 91 bytes, which is 122 base64url characters; 512 is generous
	// and refuses anything large enough to make x509 parsing worth attacking.
	MaxPublicKeyChars = 512

	// MaxSignatureChars bounds the signature. A DER P-256 signature is at most
	// 72 bytes -> 96 characters.
	MaxSignatureChars = 256

	// p1363Len is the length of a raw r||s signature: two 32-byte scalars.
	p1363Len = 64
)

var (
	// ErrInvalidKey means the supplied public key is not a usable P-256 SPKI key.
	ErrInvalidKey = errors.New("device key is not a valid ECDSA P-256 SPKI key")

	// ErrInvalidSignature means the signature did not verify.
	ErrInvalidSignature = errors.New("device signature does not verify")
)

// ParsePublicKey decodes a base64url SPKI public key and validates its curve.
//
// It returns the key AND the raw DER bytes, because the fingerprint is defined
// over the DER (not over the base64 text) — and the fingerprint must be computed
// from what we decoded, never from a value the client supplied.
func ParsePublicKey(encoded string) (*ecdsa.PublicKey, []byte, error) {
	if encoded == "" {
		return nil, nil, fmt.Errorf("%w: empty", ErrInvalidKey)
	}
	if len(encoded) > MaxPublicKeyChars {
		return nil, nil, fmt.Errorf("%w: %d characters exceeds the %d limit",
			ErrInvalidKey, len(encoded), MaxPublicKeyChars)
	}

	der, err := decodeBase64URL(encoded)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}

	// ParsePKIXPublicKey validates the point is on the curve and is not the
	// point at infinity. Hand-rolling that check is how people end up with a
	// curve-confusion bug, so we let x509 do it.
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}

	pub, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("%w: got %T, not an ECDSA key", ErrInvalidKey, parsed)
	}
	if pub.Curve == nil || pub.Curve.Params().Name != "P-256" {
		return nil, nil, fmt.Errorf("%w: curve must be P-256", ErrInvalidKey)
	}

	return pub, der, nil
}

// Fingerprint is the lowercase hex SHA-256 of the SPKI DER.
//
// This is what the admin device list shows and what a human compares when they
// suspect a key was swapped. It is NOT secret — it is a hash of a public value —
// but it is also not a signature, and it proves nothing on its own.
func Fingerprint(spkiDER []byte) string {
	sum := sha256.Sum256(spkiDER)
	return hex.EncodeToString(sum[:])
}

// Verify checks a base64url signature over message.
//
// Both DER and P1363 encodings are accepted; see the package comment for why.
func Verify(pub *ecdsa.PublicKey, message []byte, signatureB64 string) error {
	if pub == nil {
		return ErrInvalidSignature
	}
	if signatureB64 == "" || len(signatureB64) > MaxSignatureChars {
		return ErrInvalidSignature
	}

	sig, err := decodeBase64URL(signatureB64)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	// Hash exactly once and verify against the digest. Short signatures are
	// rejected by both verifiers, but checking the obvious case early keeps the
	// log honest about what happened.
	digest := sha256.Sum256(message)

	if ecdsa.VerifyASN1(pub, digest[:], sig) {
		return nil
	}

	// Fallback for runtimes that emit raw r||s. Exactly 64 bytes, no structure —
	// which is why it cannot be confused with a DER signature, whose first byte
	// is always 0x30 (a SEQUENCE tag).
	if len(sig) == p1363Len {
		r := new(big.Int).SetBytes(sig[:32])
		s := new(big.Int).SetBytes(sig[32:])
		if ecdsa.Verify(pub, digest[:], r, s) {
			return nil
		}
	}

	return ErrInvalidSignature
}

// CanonicalString builds the exact bytes a device signs.
//
// This is docs/02-SYNC-PROTOCOL.md §5.2, and it must match
// web/src/sync/deviceKey.ts byte for byte: six fields, "\n", no prefix, no
// trailing newline.
//
//	method \n path \n deviceID \n timestamp \n nonce \n b64url(sha256(body))
//
// The body is hashed RAW, before any JSON parsing, and the base64url is UNPADDED
// (RawURLEncoding). A padded variant here is a one-character mismatch that
// invalidates every signature.
func CanonicalString(method, path, deviceID, timestamp, nonce string, body []byte) string {
	sum := sha256.Sum256(body)
	return strings.Join([]string{
		strings.ToUpper(method),
		path,
		deviceID,
		timestamp,
		nonce,
		base64.RawURLEncoding.EncodeToString(sum[:]),
	}, "\n")
}

// decodeBase64URL accepts both unpadded and padded base64url.
//
// The client sends unpadded, but a hand-typed curl test will very likely be
// padded, and refusing it produces a misleading "invalid signature" instead of
// "invalid encoding".
func decodeBase64URL(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "=")); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}
