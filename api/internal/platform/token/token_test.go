package token

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

func testIssuer(t *testing.T) *Issuer {
	t.Helper()
	iss, err := NewIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	return iss
}

func TestIssueVerifyRoundTrip(t *testing.T) {
	iss := testIssuer(t)
	userID := uuid.New()
	sessionID := uuid.NewString()

	issued, err := iss.Issue(userID, domain.RoleNurse, "device-1", sessionID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if issued.Token == "" {
		t.Fatal("Issue returned an empty token")
	}

	v, err := iss.Verify(issued.Token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if v.UserID != userID {
		t.Errorf("user id: got %s, want %s", v.UserID, userID)
	}
	if v.Role != domain.RoleNurse {
		t.Errorf("role: got %s, want %s", v.Role, domain.RoleNurse)
	}
	if v.DeviceID != "device-1" {
		t.Errorf("device id: got %q", v.DeviceID)
	}
	if v.SessionID != sessionID {
		t.Errorf("session id: got %q, want %q", v.SessionID, sessionID)
	}
	if v.JTI == "" {
		t.Error("jti is empty; a denylist needs it")
	}
}

// TestAlgorithmNoneRejected is THE security test in this package.
//
// The "alg":"none" attack is the single most common JWT vulnerability: the
// attacker strips the signature and sets the header algorithm to "none", hoping
// the verifier trusts the header. If this test ever passes a token through,
// every account is compromised.
func TestAlgorithmNoneRejected(t *testing.T) {
	iss := testIssuer(t)

	// Build an unsigned token by hand, exactly as an attacker would.
	claims := jwt.MapClaims{
		"sub":       uuid.NewString(),
		"role":      "admin",
		"device_id": "attacker",
		"iss":       "phc-records-api",
		"aud":       []string{"phc-records-web"},
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	}
	forged := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	raw, err := forged.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("could not construct the forged token: %v", err)
	}

	if _, err := iss.Verify(raw); err == nil {
		t.Fatal("SECURITY: an alg=none token was ACCEPTED — algorithm confusion is possible")
	}
}

// TestHS256SignatureRequired proves a token with a tampered payload fails.
func TestTamperedPayloadRejected(t *testing.T) {
	iss := testIssuer(t)
	issued, err := iss.Issue(uuid.New(), domain.RoleCashier, "d1", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(issued.Token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	// Swap the payload for one claiming admin. The signature no longer matches.
	tampered := parts[0] + "." +
		"eyJzdWIiOiIwMDAwMDAwMC0wMDAwLTAwMDAtMDAwMC0wMDAwMDAwMDAwMDAiLCJyb2xlIjoiYWRtaW4iLCJkZXZpY2VfaWQiOiJkMSIsImlzcyI6InBoYy1yZWNvcmRzLWFwaSIsImF1ZCI6WyJwaGMtcmVjb3Jkcy13ZWIiXSwiZXhwIjo5OTk5OTk5OTk5fQ" +
		"." + parts[2]

	if _, err := iss.Verify(tampered); err == nil {
		t.Fatal("SECURITY: a token with a tampered payload was ACCEPTED")
	}
}

// TestWrongSecretRejected proves a token signed by another key fails.
func TestWrongSecretRejected(t *testing.T) {
	issuerA := testIssuer(t)

	forger, err := NewIssuer([]byte("ffffffffffffffffffffffffffffffff"), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	issued, err := forger.Issue(uuid.New(), domain.RoleAdmin, "d1", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := issuerA.Verify(issued.Token); err == nil {
		t.Fatal("SECURITY: a token signed with a different secret was ACCEPTED")
	}
}

// TestExpiredReturnsSentinel proves expiry yields ErrExpired specifically, since
// the client refreshes on that but force-logs-out on anything else.
func TestExpiredReturnsSentinel(t *testing.T) {
	iss := testIssuer(t)

	// Rewind the injectable clock so the token is already expired.
	iss.now = func() time.Time { return time.Now().Add(-48 * time.Hour) }

	issued, err := iss.Issue(uuid.New(), domain.RoleDoctor, "d1", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	iss.now = time.Now

	_, err = iss.Verify(issued.Token)
	if err == nil {
		t.Fatal("an expired token was accepted")
	}
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

// TestUnknownRoleRejected — a token carrying a role not in the enum is either a
// forgery or from an incompatible version. Neither may be trusted.
func TestUnknownRoleRejected(t *testing.T) {
	iss := testIssuer(t)

	claims := jwt.MapClaims{
		"sub":       uuid.NewString(),
		"role":      "superuser", // not one of the eight
		"device_id": "d1",
		"iss":       "phc-records-api",
		"aud":       []string{"phc-records-web"},
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(iss.secret)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := iss.Verify(raw); err == nil {
		t.Fatal("a token with an unknown role was accepted")
	}
}

// TestWrongIssuerRejected — an otherwise-valid token minted for another service
// must not be usable here.
func TestWrongIssuerRejected(t *testing.T) {
	iss := testIssuer(t)

	claims := jwt.MapClaims{
		"sub":       uuid.NewString(),
		"role":      "admin",
		"device_id": "d1",
		"iss":       "some-other-service",
		"aud":       []string{"phc-records-web"},
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(iss.secret)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := iss.Verify(raw); err == nil {
		t.Fatal("a token with the wrong issuer was accepted")
	}
}

// TestNewIssuerRejectsWeakSecret — pinning that a short key cannot be used.
func TestNewIssuerRejectsWeakSecret(t *testing.T) {
	if _, err := NewIssuer([]byte("short"), time.Minute); err == nil {
		t.Fatal("NewIssuer accepted a secret shorter than 32 bytes")
	}
	if _, err := NewIssuer([]byte(strings.Repeat("a", 32)), 0); err == nil {
		t.Fatal("NewIssuer accepted a zero TTL")
	}
}

// TestIssueRejectsInvalidInput covers the argument guards.
func TestIssueRejectsInvalidInput(t *testing.T) {
	iss := testIssuer(t)

	if _, err := iss.Issue(uuid.Nil, domain.RoleAdmin, "d1", "s1"); err == nil {
		t.Error("expected an error for a nil user id")
	}
	if _, err := iss.Issue(uuid.New(), domain.Role("wizard"), "d1", "s1"); err == nil {
		t.Error("expected an error for an invalid role")
	}
	if _, err := iss.Issue(uuid.New(), domain.RoleAdmin, "", "s1"); err == nil {
		t.Error("expected an error for a missing device id")
	}
}
