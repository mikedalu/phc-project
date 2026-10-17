package password

import (
	"strings"
	"testing"
)

// TestHashVerifyRoundTrip is the baseline: a correct password verifies, an
// incorrect one does not, and neither is confused with an error.
func TestHashVerifyRoundTrip(t *testing.T) {
	// Use a deliberately cheap cost so the test suite stays fast. A 64 MiB
	// hash takes ~50-100 ms; running that dozens of times would make `go test`
	// unpleasant, and slow tests get skipped.
	cheap := Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

	hash, err := Hash("ChangeMe!2026", cheap)
	if err != nil {
		t.Fatalf("Hash returned an error: %v", err)
	}

	t.Run("correct password verifies", func(t *testing.T) {
		ok, err := Verify("ChangeMe!2026", hash)
		if err != nil {
			t.Fatalf("Verify error: %v", err)
		}
		if !ok {
			t.Fatal("expected the correct password to verify, got false")
		}
	})

	t.Run("wrong password does not verify", func(t *testing.T) {
		ok, err := Verify("wrong-password", hash)
		if err != nil {
			t.Fatalf("Verify error: %v", err)
		}
		if ok {
			t.Fatal("expected a wrong password to fail, got true")
		}
	})

	t.Run("wrong password is not reported as an error", func(t *testing.T) {
		// This distinction matters: callers must NOT treat (false, nil) as a
		// system failure. Conflating them hides database corruption as a
		// login failure.
		if _, err := Verify("nope", hash); err != nil {
			t.Fatalf("a wrong password must not produce an error, got: %v", err)
		}
	})
}

// TestHashIsNondeterministic proves the salt is random. Two hashes of the same
// password must differ — otherwise a stolen database reveals which users share
// a password, and precomputation becomes viable.
func TestHashIsNondeterministic(t *testing.T) {
	cheap := Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

	a, err := Hash("same-password", cheap)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Hash("same-password", cheap)
	if err != nil {
		t.Fatal(err)
	}

	if a == b {
		t.Fatal("two hashes of the same password are identical — the salt is not random")
	}

	// Both must still verify.
	for _, h := range []string{a, b} {
		ok, err := Verify("same-password", h)
		if err != nil || !ok {
			t.Fatalf("hash failed to verify: ok=%v err=%v", ok, err)
		}
	}
}

// TestEncodedFormat pins the PHC string shape, because the stored value is read
// back by decode() and must survive a round trip through the database.
func TestEncodedFormat(t *testing.T) {
	cheap := Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

	hash, err := Hash("x", cheap)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("expected 6 $-delimited parts, got %d in %q", len(parts), hash)
	}
	if parts[1] != "argon2id" {
		t.Errorf("expected algorithm argon2id, got %q", parts[1])
	}
	if !strings.HasPrefix(parts[2], "v=") {
		t.Errorf("expected version part, got %q", parts[2])
	}
	// Check padding ONLY on the base64 parts (index 4 and 5).
	//
	// Trap: parts[2] and parts[3] contain '=' legitimately ("v=19",
	// "m=8192,t=1,p=1"), so scanning the whole string for '=' produces a false
	// failure. RawStdEncoding must emit NO padding — if this ever trips, the
	// encoding changed and previously stored hashes stop verifying.
	for _, idx := range []int{4, 5} {
		if strings.Contains(parts[idx], "=") {
			t.Errorf("base64 part %d contains padding: %q", idx, parts[idx])
		}
	}
}

// TestVerifyRejectsMalformedHash covers corruption and hostile input. Each case
// must return an ERROR (not merely false), so the caller knows the stored data
// is broken rather than that the user typed the wrong password.
func TestVerifyRejectsMalformedHash(t *testing.T) {
	cases := []struct{ name, hash string }{
		{"empty", ""},
		{"not a phc string", "hello"},
		{"wrong algorithm", "$argon2i$v=19$m=8192,t=1,p=1$c2FsdHNhbHQ$a2V5"},
		{"too few parts", "$argon2id$v=19$m=8192,t=1,p=1"},
		{"bad version", "$argon2id$v=99$m=8192,t=1,p=1$c2FsdHNhbHQ$a2V5"},
		{"non-numeric params", "$argon2id$v=19$m=x,t=1,p=1$c2FsdHNhbHQ$a2V5"},
		{"bad base64 salt", "$argon2id$v=19$m=8192,t=1,p=1$!!!!$a2V5"},
		{"bad base64 key", "$argon2id$v=19$m=8192,t=1,p=1$c2FsdHNhbHQ$!!!!"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := Verify("x", tc.hash)
			if err == nil {
				t.Fatalf("expected an error for malformed hash, got ok=%v err=nil", ok)
			}
			if ok {
				t.Fatal("a malformed hash must never verify")
			}
		})
	}
}

// TestVerifyRejectsAbsurdMemoryCost guards a denial-of-service vector: a
// tampered hash demanding 8 GiB per attempt would take the server down. If this
// test fails, remove the ceiling in decode() and watch a login flood kill the
// container.
func TestVerifyRejectsAbsurdMemoryCost(t *testing.T) {
	// m=8388608 KiB = 8 GiB
	hostile := "$argon2id$v=19$m=8388608,t=1,p=1$c2FsdHNhbHQxMjM0NTY3OA$a2V5"

	_, err := Verify("x", hostile)
	if err == nil {
		t.Fatal("expected an error for an absurd memory cost, got nil")
	}
	if !strings.Contains(err.Error(), "memory cost too large") {
		t.Fatalf("expected a memory-cost error, got: %v", err)
	}
}

// TestNeedsRehash covers the transparent-upgrade path.
func TestNeedsRehash(t *testing.T) {
	weak := Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}
	strong := Params{Memory: 64 * 1024, Iterations: 3, Parallelism: 4, SaltLength: 16, KeyLength: 32}

	hash, err := Hash("x", weak)
	if err != nil {
		t.Fatal(err)
	}

	if !NeedsRehash(hash, strong) {
		t.Error("a weak hash should need rehashing against stronger params")
	}
	if NeedsRehash(hash, weak) {
		t.Error("a hash at the target cost should not need rehashing")
	}
	if !NeedsRehash("garbage", strong) {
		t.Error("a malformed hash should always need rehashing")
	}
}

// TestEmptyPasswordRejected — hashing an empty string would mean an account
// with no password, and Argon2 would happily produce a hash for it.
func TestEmptyPasswordRejected(t *testing.T) {
	cheap := Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}
	if _, err := Hash("", cheap); err == nil {
		t.Fatal("expected an error when hashing an empty password")
	}
}

// TestDefaultParamsAreSane pins the production cost so a careless edit cannot
// silently weaken every password in the system.
func TestDefaultParamsAreSane(t *testing.T) {
	if DefaultParams.Memory < 46*1024 {
		t.Errorf("memory cost %d KiB is below the OWASP minimum of 46 MiB", DefaultParams.Memory)
	}
	if DefaultParams.Iterations < 2 {
		t.Errorf("iteration count %d is below the OWASP minimum of 2", DefaultParams.Iterations)
	}
	if DefaultParams.SaltLength < 16 {
		t.Errorf("salt length %d is below the recommended 16 bytes", DefaultParams.SaltLength)
	}
	if DefaultParams.KeyLength < 32 {
		t.Errorf("key length %d is below the recommended 32 bytes", DefaultParams.KeyLength)
	}
}

// TestSeededAdminHashIsValid verifies the exact hash committed in migration
// 00004 actually accepts its documented password. If this fails, the bootstrap
// admin cannot log in and a fresh install is unusable.
func TestSeededAdminHashIsValid(t *testing.T) {
	const seeded = "$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo"

	ok, err := Verify("ChangeMe!2026", seeded)
	if err != nil {
		t.Fatalf("the seeded hash is malformed: %v", err)
	}
	if !ok {
		t.Fatal("the seeded hash does not accept the documented password ChangeMe!2026")
	}

	if ok, _ := Verify("wrong", seeded); ok {
		t.Fatal("the seeded hash accepted an incorrect password")
	}
}
