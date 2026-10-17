// Package token issues and verifies JWT access tokens.
//
// ═══ WHAT AN ACCESS TOKEN IS AND IS NOT ═══
//
// It is a SHORT-LIVED (15 min) assertion of identity, signed HS256. It is NOT a
// session: it cannot be revoked before expiry, which is exactly why it is short.
// Revocation happens on the refresh token (see the refresh_tokens table).
//
// ═══ WHY `role` IS IN THE TOKEN ═══
//
// The server re-checks roles against the database on every request, so the
// embedded role is NOT the security boundary. It exists for a specific offline
// reason: with no network, the SPA must decide whether to render the "Dispense"
// button. Without the role in the token an offline device cannot make even that
// cosmetic decision. The service layer remains the authority.
package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

var (
	// ErrInvalidToken — signature, structure, or claim validation failed.
	// Deliberately ONE error for all of them: telling a caller precisely which
	// check failed hands an attacker an oracle.
	ErrInvalidToken = errors.New("token: invalid")

	// ErrExpired — well-formed but expired. Separated from ErrInvalidToken
	// because the client handles it differently: refresh and retry, rather than
	// force a full re-login.
	ErrExpired = errors.New("token: expired")

	// ErrSigningMethod — the token's `alg` header was not HS256.
	ErrSigningMethod = errors.New("token: unexpected signing method")
)

// Claims is the access-token payload.
type Claims struct {
	Role     string `json:"role"`
	DeviceID string `json:"device_id"`
	SID      string `json:"sid"` // session id = refresh-token family id

	jwt.RegisteredClaims
}

// Issuer mints and verifies access tokens.
type Issuer struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time // injectable clock, so expiry is testable
}

// NewIssuer builds an Issuer.
//
// secret must be at least 32 bytes: HS256's security is bounded by the key
// length, and a short key makes the signature brute-forceable.
func NewIssuer(secret []byte, ttl time.Duration) (*Issuer, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("token: secret must be at least 32 bytes, got %d", len(secret))
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("token: ttl must be positive, got %s", ttl)
	}
	return &Issuer{
		secret:   secret,
		issuer:   "phc-records-api",
		audience: "phc-records-web",
		ttl:      ttl,
		now:      time.Now,
	}, nil
}

// Issued is the result of minting an access token.
type Issued struct {
	Token     string
	ExpiresAt time.Time
	JTI       string
}

// Issue mints an access token for a user on a device within a session.
//
// sessionID should be the refresh-token family id, so revoking the session
// revokes the whole chain.
func (i *Issuer) Issue(userID uuid.UUID, role domain.Role, deviceID, sessionID string) (Issued, error) {
	if userID == uuid.Nil {
		return Issued{}, errors.New("token: user id is required")
	}
	if !role.Valid() {
		return Issued{}, fmt.Errorf("token: invalid role %q", role)
	}
	if deviceID == "" {
		return Issued{}, errors.New("token: device id is required")
	}

	now := i.now()
	expiresAt := now.Add(i.ttl)
	jti := uuid.NewString()

	claims := Claims{
		Role:     role.String(),
		DeviceID: deviceID,
		SID:      sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			// `sub` is the user id — the only identity that matters. Every
			// created_by is derived from this, never from a request body.
			Subject:   userID.String(),
			Issuer:    i.issuer,
			Audience:  jwt.ClaimStrings{i.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			// Enables a denylist for one token without waiting for expiry.
			ID: jti,
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(i.secret)
	if err != nil {
		return Issued{}, fmt.Errorf("token: sign: %w", err)
	}

	return Issued{Token: signed, ExpiresAt: expiresAt, JTI: jti}, nil
}

// Verified is a validated access token.
type Verified struct {
	UserID    uuid.UUID
	Role      domain.Role
	DeviceID  string
	SessionID string
	JTI       string
	// ExpiresAt is carried so a caller can record HOW LONG a revocation needs to
	// live. Without it the denylist would have to guess, and a guess that is too
	// short silently un-revokes a token.
	ExpiresAt time.Time
}

// Verify validates a token and returns its claims.
func (i *Issuer) Verify(raw string) (*Verified, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(i.issuer),
		jwt.WithAudience(i.audience),
		jwt.WithLeeway(30*time.Second), // tolerate small clock drift
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)

	claims := &Claims{}
	_, err := parser.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		// ═══ ALGORITHM CONFUSION GUARD ═══
		//
		// Not redundant with WithValidMethods — this is the belt to that brace,
		// and it is the difference between a secure system and a compromise.
		//
		// The classic attack: a client sends a token with "alg":"none" and no
		// signature, and a verifier that trusts the header accepts it. Worse,
		// some libraries let an attacker set alg:HS256 on a token signed RS256
		// and then use the PUBLIC key as the HMAC secret — public keys are
		// public, so they can forge anything.
		//
		// Asserting the concrete Go type means only HMAC ever reaches the
		// signature check; any other alg fails before it is verified.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %q", ErrSigningMethod, t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpired
		}
		// One opaque error outward: do not tell an attacker which check failed.
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	return verifiedFrom(claims)
}

// verifiedFrom converts validated claims into typed values.
func verifiedFrom(c *Claims) (*Verified, error) {
	userID, err := uuid.Parse(c.Subject)
	if err != nil {
		return nil, fmt.Errorf("%w: subject is not a uuid", ErrInvalidToken)
	}

	role := domain.Role(c.Role)
	if !role.Valid() {
		// An unknown role means a future version or a forgery. Untrusted either
		// way.
		return nil, fmt.Errorf("%w: unknown role %q", ErrInvalidToken, c.Role)
	}

	if c.DeviceID == "" {
		return nil, fmt.Errorf("%w: missing device_id", ErrInvalidToken)
	}

	return &Verified{
		UserID:    userID,
		Role:      role,
		DeviceID:  c.DeviceID,
		SessionID: c.SID,
		JTI:       c.ID,
		ExpiresAt: c.ExpiresAt.Time,
	}, nil
}
