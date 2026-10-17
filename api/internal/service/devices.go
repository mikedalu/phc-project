package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
	"github.com/mikedalu/phc-records/api/internal/platform/devicekey"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// Devices implements workstation registration and revocation.
//
// A device row is created during login (so last_seen_at and the audit trail are
// meaningful from the first request) but it has NO public key at that point — an
// empty key is honest, because the device has not yet proved possession of a
// private key. This service is how that proof is recorded.
type Devices struct {
	pool    *pgxpool.Pool
	devices *repo.Devices
	tokens  *repo.RefreshTokens
	users   *repo.Users
	now     func() time.Time
}

// NewDevices wires the device service.
func NewDevices(pool *pgxpool.Pool) *Devices {
	return &Devices{
		pool:    pool,
		devices: repo.NewDevices(),
		tokens:  repo.NewRefreshTokens(),
		users:   repo.NewUsers(pool),
		now:     time.Now,
	}
}

const (
	// MaxDeviceNameLen keeps a label readable in the admin list. "Triage laptop 2"
	// fits; a paragraph does not, and the admin list is the point.
	MaxDeviceNameLen = 80

	// MinRevokeReasonLen mirrors the user_roles rule: a revocation without a
	// usable reason is a revocation nobody can explain in six months.
	MinRevokeReasonLen = 5

	// MaxRevokeReasonLen bounds a free-text field that lands in an audit trail.
	MaxRevokeReasonLen = 500
)

// RegisterDeviceInput is a device key-registration request.
type RegisterDeviceInput struct {
	// TokenDeviceID comes from the verified JWT (`device_id` claim). It is the
	// ONLY device the caller may act as.
	TokenDeviceID uuid.UUID

	// ClaimedDeviceID is what the body said. Optional; if present it must match
	// the token. Mismatch is a client bug or a probe, and both deserve a 400.
	ClaimedDeviceID uuid.UUID

	PublicKey  string
	Algorithm  string
	Name       string
	Platform   string
	AppVersion string
}

// Register attaches an ECDSA P-256 public key to the calling device.
//
// The rules, in order, and why each exists:
//
//  1. Parse and validate the key. Never store a key that cannot be parsed.
//  2. Refuse a revoked device. Re-registration after revocation needs a human,
//     otherwise the control is defeated by clicking "register" again.
//  3. First write wins. A device that already has a key cannot swap it using
//     nothing but a bearer token — that would let a stolen token take over the
//     device identity it was issued to.
//  4. Compute the fingerprint HERE, from the decoded DER. A client-supplied
//     fingerprint proves nothing and could be used to make two distinct keys
//     look like one.
func (s *Devices) Register(ctx context.Context, in RegisterDeviceInput) (*domain.Device, error) {
	if in.TokenDeviceID == uuid.Nil {
		return nil, domain.NewValidation("deviceId", "missing_from_session")
	}
	if in.ClaimedDeviceID != uuid.Nil && in.ClaimedDeviceID != in.TokenDeviceID {
		return nil, domain.NewValidation("deviceId", "does_not_match_session")
	}

	algorithm := strings.ToUpper(strings.TrimSpace(in.Algorithm))
	if algorithm == "" {
		algorithm = devicekey.AlgorithmES256
	}
	if algorithm != devicekey.AlgorithmES256 {
		return nil, domain.NewValidation("algorithm", "only_es256_supported")
	}

	_, spkiDER, err := devicekey.ParsePublicKey(strings.TrimSpace(in.PublicKey))
	if err != nil {
		// Public message: the client needs to know its key is malformed. The
		// detail stays in the error chain for the 400 body, which is a client
		// validation failure, not a server secret.
		return nil, domain.NewValidationf("publicKey", "invalid", "%v", err)
	}

	name, err := optionalLabel(in.Name, "label")
	if err != nil {
		return nil, err
	}
	platform := optionalText(in.Platform, 120, "platform")
	appVersion := optionalText(in.AppVersion, 40, "appVersion")

	device, err := s.devices.FindByID(ctx, s.pool, in.TokenDeviceID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// The token names a device that does not exist. That is a stale
			// session, not a client typo, so the client must sign in again.
			return nil, domain.Public(domain.ErrNotFound,
				"This device is not registered. Sign in again.")
		}
		return nil, err
	}

	if device.IsRevoked() {
		return nil, domain.Public(domain.ErrDeviceRevoked,
			"This device has been revoked. An administrator must re-enable it before it can be used again.")
	}

	// Idempotent re-registration of the SAME key is fine — a client may retry
	// after a network failure. A DIFFERENT key is a key change, and that needs a
	// human.
	if device.PublicKey != "" {
		if device.PublicKey == in.PublicKey {
			return device, nil
		}
		return nil, domain.Public(domain.ErrConflict,
			"This device already has a different signing key. An administrator must approve a key change.")
	}

	fingerprint := devicekey.Fingerprint(spkiDER)

	bound, err := s.devices.BindPublicKey(
		ctx, s.pool, device.ID, in.PublicKey, algorithm, fingerprint, name, platform, appVersion,
	)
	if err != nil {
		return nil, err
	}

	if !bound {
		// Another request won the race between our read and the UPDATE. The
		// outcome must still be decided by the same rule, so re-read rather than
		// reporting a spurious failure.
		fresh, ferr := s.devices.FindByID(ctx, s.pool, device.ID)
		if ferr == nil && fresh.PublicKey == in.PublicKey {
			return fresh, nil
		}
		return nil, domain.Public(domain.ErrConflict,
			"This device was registered concurrently. Sign in again and retry.")
	}

	if err := s.devices.TouchSeen(ctx, s.pool, device.ID, s.now()); err != nil {
		// Non-fatal: last_seen_at is operational telemetry, not correctness.
		log.Warn().Err(err).Str("device_id", device.ID.String()).Msg("failed to touch device")
	}

	device.PublicKey = in.PublicKey
	device.PublicKeyAlg = algorithm
	device.Fingerprint = fingerprint
	if name != nil {
		device.Name = *name
	}
	if platform != nil {
		device.Platform = platform
	}
	if appVersion != nil {
		device.AppVersion = appVersion
	}

	log.Info().
		Str("device_id", device.ID.String()).
		Str("fingerprint", shortFingerprint(fingerprint)).
		Msg("device public key registered")

	return device, nil
}

// RevokeDeviceInput is a device revocation request.
type RevokeDeviceInput struct {
	ActorID  uuid.UUID
	DeviceID uuid.UUID
	Reason   string
}

// Revoke marks a device unusable and kills every session bound to it.
//
// Authorisation is checked HERE against the DATABASE, not in the route
// middleware. The middleware's role comes from a token that may be 15 minutes
// stale, and "an admin was demoted" must take effect without waiting for a
// refresh. The middleware still runs first so the common case fails cheaply.
func (s *Devices) Revoke(ctx context.Context, in RevokeDeviceInput) (*domain.Device, int64, error) {
	actor, err := s.users.FindByID(ctx, s.pool, in.ActorID)
	if err != nil {
		return nil, 0, err
	}
	if !domain.HasRole(actor.Roles, domain.RoleAdmin) {
		return nil, 0, domain.Public(domain.ErrForbidden,
			"Only an administrator can revoke a device.")
	}

	reason := strings.TrimSpace(in.Reason)
	if len(reason) < MinRevokeReasonLen {
		return nil, 0, domain.NewValidationf("reason", "too_short",
			"%d characters minimum", MinRevokeReasonLen)
	}
	if len(reason) > MaxRevokeReasonLen {
		return nil, 0, domain.NewValidationf("reason", "too_long",
			"%d characters maximum", MaxRevokeReasonLen)
	}

	device, err := s.devices.FindByID(ctx, s.pool, in.DeviceID)
	if err != nil {
		return nil, 0, err
	}

	// Idempotent. The caller wanted the device unusable; it already is. Refusing
	// would push an operator into retrying a destructive action to make a screen
	// go away, which is exactly the wrong reflex to train.
	if device.IsRevoked() {
		return device, 0, nil
	}

	var revokedSessions int64
	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.devices.Revoke(ctx, tx, device.ID, in.ActorID, reason, s.now()); err != nil {
			return err
		}
		n, err := s.tokens.RevokeAllForDevice(ctx, tx, device.ID, domain.RevokeDeviceRevoked)
		if err != nil {
			return err
		}
		revokedSessions = n
		return nil
	}); err != nil {
		return nil, 0, err
	}

	// Access tokens already issued are NOT denylisted yet — there is no jti
	// denylist (recorded as a Sprint 9 hardening item). They carry `device_id`
	// on every signed route, and the signature middleware re-reads this row, so
	// a revoked device cannot push within one request. It can still read
	// JWT-only endpoints until its access token expires, at most 15 minutes.
	log.Warn().
		Str("device_id", device.ID.String()).
		Str("actor_id", in.ActorID.String()).
		Int64("revoked_sessions", revokedSessions).
		Msg("device revoked")

	updated, err := s.devices.FindByID(ctx, s.pool, device.ID)
	if err != nil {
		// The revocation committed; returning the stale pre-revocation row would
		// be worse than returning the row we just loaded.
		return device, revokedSessions, nil
	}
	return updated, revokedSessions, nil
}

// optionalLabel trims and validates a human-facing label.
func optionalLabel(raw, field string) (*string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if len(value) > MaxDeviceNameLen {
		return nil, domain.NewValidationf(field, "too_long", "%d characters maximum", MaxDeviceNameLen)
	}
	return &value, nil
}

// optionalText trims an optional free-text field and truncation-checks it.
func optionalText(raw string, max int, field string) *string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if len(value) > max {
		value = value[:max]
	}
	return &value
}

// shortFingerprint is the first 16 hex characters, which is what goes into a log.
//
// The full fingerprint is not secret, but a log line is not the place to dump a
// 64-character identifier that has to be read by a human at 2 a.m.
func shortFingerprint(f string) string {
	if len(f) <= 16 {
		return f
	}
	return f[:16]
}
