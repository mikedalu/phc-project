// Package password implements Argon2id password hashing and verification.
//
// ═══ WHY ARGON2ID AND NOT BCRYPT OR SHA-256 ═══
//
// Argon2id is memory-hard: it costs an attacker RAM as well as CPU, which is
// what makes GPU and ASIC cracking uneconomical. bcrypt is CPU-hard only; plain
// SHA-256 is a fast hash and must never be used for a password.
//
// The "id" variant is the hybrid: Argon2i resists side-channel timing attacks
// but is weaker against time-memory trade-off attacks; Argon2d resists the
// latter but leaks timing. Argon2id runs Argon2i for the first half of the first
// pass and Argon2d afterwards, getting both properties. It is the recommended
// default (RFC 9106).
//
// ═══ THE ENCODED FORMAT ═══
//
//	$argon2id$v=19$m=65536,t=3,p=4$<salt-b64>$<key-b64>
//
// Storing the parameters WITH the hash means a future change to the cost does
// not invalidate existing passwords: Verify reads the cost from the stored
// string, and NeedsRehash reports when an old hash is below the current target
// so it can be upgraded at the next successful login.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are the Argon2id cost parameters.
//
// Chosen for a PHC workstation, which may be an old laptop: 64 MiB / 3 passes
// targets roughly 50-100 ms on modest hardware and is the OWASP-recommended
// baseline. Raising Memory is the cheapest way to increase cracking cost.
type Params struct {
	Memory      uint32 // KiB
	Iterations  uint32 // passes
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultParams is the current target cost.
var DefaultParams = Params{
	Memory:      64 * 1024, // 64 MiB
	Iterations:  3,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

var (
	// ErrInvalidHash means the stored value is not a well-formed PHC string.
	// This is a data-integrity problem, not a wrong password.
	ErrInvalidHash = errors.New("password: hash is not in the expected PHC format")

	// ErrIncompatibleVersion means the hash was produced by a different Argon2
	// version. Nothing to do but reject it and force a password reset.
	ErrIncompatibleVersion = errors.New("password: incompatible argon2 version")
)

// Hash derives an Argon2id hash and returns it in PHC string format.
//
// The salt is generated with crypto/rand. NEVER use math/rand here: a
// predictable salt lets an attacker precompute a rainbow table and defeats the
// entire purpose.
func Hash(plaintext string, p Params) (string, error) {
	if plaintext == "" {
		return "", errors.New("password: plaintext must not be empty")
	}

	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password: generate salt: %w", err)
	}

	key := argon2.IDKey(
		[]byte(plaintext), salt,
		p.Iterations, p.Memory, p.Parallelism, p.KeyLength,
	)

	// RawStdEncoding: no padding. Padding bytes ('=') are harmless in a hash
	// string but need URL-escaping in some contexts; omitting them keeps the
	// stored value simple and copy-paste safe.
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify checks a plaintext password against a PHC-format hash.
//
// It returns (true, nil) on a match, (false, nil) on a wrong password, and
// (false, err) when the stored hash itself is malformed. Callers must treat the
// error case as a SYSTEM failure, not as "wrong password" — conflating the two
// masks database corruption as a login failure.
func Verify(plaintext, encodedHash string) (bool, error) {
	p, salt, expectedKey, err := decode(encodedHash)
	if err != nil {
		return false, err
	}

	actualKey := argon2.IDKey(
		[]byte(plaintext), salt,
		p.Iterations, p.Memory, p.Parallelism, uint32(len(expectedKey)),
	)

	// Constant-time comparison. A byte-by-byte comparison returns early on the
	// first mismatch, and that timing difference lets an attacker learn the
	// hash one byte at a time.
	//
	// subtle.ConstantTimeCompare returns 0 or 1; comparing to 1 keeps the
	// result a bool without branching on the secret.
	return subtle.ConstantTimeCompare(actualKey, expectedKey) == 1, nil
}

// NeedsRehash reports whether a stored hash was produced with weaker parameters
// than the current target, so it can be upgraded transparently after a
// successful login.
//
// This is why the parameters live in the stored string: you can raise the cost
// later without locking anyone out.
func NeedsRehash(encodedHash string, target Params) bool {
	p, _, _, err := decode(encodedHash)
	if err != nil {
		// A malformed hash certainly needs replacing.
		return true
	}
	return p.Memory != target.Memory ||
		p.Iterations != target.Iterations ||
		p.Parallelism != target.Parallelism ||
		p.KeyLength != target.KeyLength
}

// decode parses a PHC-format Argon2id string into its components.
func decode(encodedHash string) (p Params, salt, key []byte, err error) {
	// Expected: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<key>"]
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return p, nil, nil, ErrInvalidHash
	}
	if parts[1] != "argon2id" {
		return p, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return p, nil, nil, ErrIncompatibleVersion
	}

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return p, nil, nil, ErrInvalidHash
	}

	if salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	if key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil {
		return p, nil, nil, ErrInvalidHash
	}

	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(key))

	// Guard against a hostile or corrupted hash demanding absurd resources: a
	// stored "m=8388608" would allocate 8 GiB per login attempt and take the
	// server down. This is a denial-of-service vector if left unchecked.
	if p.Memory > 1024*1024 { // 1 GiB ceiling
		return p, nil, nil, fmt.Errorf("%w: memory cost too large (%d KiB)", ErrInvalidHash, p.Memory)
	}

	return p, salt, key, nil
}
