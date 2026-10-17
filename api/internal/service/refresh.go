package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
	"github.com/mikedalu/phc-records/api/internal/platform/session"
)

// RefreshInput is a token-renewal request.
type RefreshInput struct {
	RefreshToken string
	DeviceID     string
	IP           string
	UserAgent    string
}

// Refresh exchanges a refresh token for a new pair, rotating the old one.
//
// ═══ THE DECISION TABLE ═══
//
//	token not found            -> 401, no family to revoke
//	token revoked              -> 401 (already dead)
//	token EXPIRED              -> 401 (cannot revive)
//	token already USED         -> ⚠️ REUSE: revoke the whole family, 401
//	token valid                -> rotate: mark used, issue a new pair
//	device revoked             -> revoke family, 403
//
// The USED case is the important one. A consumed token being presented again
// means either theft or a lost-response retry, and we cannot distinguish them —
// so we assume theft and kill the family. Both parties must log in again. A
// forced re-login is a minor annoyance; a silently hijacked clinical session is
// not.
func (s *Auth) Refresh(ctx context.Context, in RefreshInput) (*Session, error) {
	if in.RefreshToken == "" {
		return nil, domain.NewValidation("refreshToken", "required")
	}

	hash := session.HashToken(in.RefreshToken)
	now := s.now()

	rt, err := s.tokens.FindByHash(ctx, s.pool, hash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Unknown token: nothing to revoke. Do not reveal whether it ever
			// existed.
			return nil, fmt.Errorf("%w: invalid refresh token", domain.ErrUnauthenticated)
		}
		return nil, err
	}

	// ── REUSE DETECTION ──────────────────────────────────────────
	if rt.UsedAt != nil {
		n, revErr := s.tokens.RevokeFamily(ctx, s.pool, rt.FamilyID, domain.RevokeReuseDetected)
		if revErr != nil {
			log.Error().Err(revErr).Str("family_id", rt.FamilyID.String()).
				Msg("FAILED to revoke family after reuse detection")
		}
		// Security events are logged at warn/error with correlation ids, because
		// this is the single strongest signal that a token has leaked.
		log.Warn().
			Str("user_id", rt.UserID.String()).
			Str("family_id", rt.FamilyID.String()).
			Str("device_id", rt.DeviceID.String()).
			Str("ip", in.IP).
			Int64("revoked", n).
			Msg("REFRESH TOKEN REUSE DETECTED — family revoked")
		return nil, fmt.Errorf("%w: refresh token has already been used; all sessions in this family have been revoked", domain.ErrUnauthenticated)
	}

	if rt.RevokedAt != nil {
		return nil, fmt.Errorf("%w: refresh token has been revoked", domain.ErrUnauthenticated)
	}
	if !now.Before(rt.ExpiresAt) {
		return nil, fmt.Errorf("%w: refresh token has expired", domain.ErrUnauthenticated)
	}

	// ── device state ─────────────────────────────────────────────
	// A revoked device must stop working immediately, whether or not its tokens
	// have expired. This is the reason devices are revocable at all.
	device, err := s.devices.FindByID(ctx, s.pool, rt.DeviceID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: device not registered", domain.ErrUnauthenticated)
		}
		return nil, err
	}
	if device.IsRevoked() {
		if _, err := s.tokens.RevokeFamily(ctx, s.pool, rt.FamilyID, domain.RevokeDeviceRevoked); err != nil {
			log.Error().Err(err).Msg("failed to revoke family for revoked device")
		}
		return nil, fmt.Errorf("%w: this device has been revoked", domain.ErrForbidden)
	}

	// The client must present the same device id it logged in with. A token
	// presented from a different device is either a copied token or a bug; both
	// warrant refusal.
	if in.DeviceID != "" && in.DeviceID != rt.DeviceID.String() {
		log.Warn().
			Str("token_device", rt.DeviceID.String()).
			Str("presented_device", in.DeviceID).
			Msg("refresh presented from a different device")
		return nil, fmt.Errorf("%w: refresh token belongs to a different device", domain.ErrForbidden)
	}

	// ── rotate ───────────────────────────────────────────────────
	user, err := s.users.FindByID(ctx, s.pool, rt.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, fmt.Errorf("%w: account is disabled", domain.ErrForbidden)
	}

	// MarkUsed is the ATOMIC gate. If two requests race with the same token,
	// exactly one UPDATE affects a row; the loser gets false and is treated as
	// reuse. Checking `UsedAt != nil` above is not enough on its own — both
	// requests could have read the row before either wrote.
	var rotated bool
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		ok, err := s.tokens.MarkUsed(ctx, tx, rt.ID, now)
		if err != nil {
			return err
		}
		rotated = ok
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !rotated {
		// Lost the race: another request consumed this token microseconds ago.
		// Treat exactly as reuse — the safe assumption.
		if _, err := s.tokens.RevokeFamily(ctx, s.pool, rt.FamilyID, domain.RevokeReuseDetected); err != nil {
			log.Error().Err(err).Msg("failed to revoke family after rotation race")
		}
		log.Warn().
			Str("family_id", rt.FamilyID.String()).
			Msg("REFRESH ROTATION RACE — treated as reuse, family revoked")
		return nil, fmt.Errorf("%w: refresh token has already been used", domain.ErrUnauthenticated)
	}

	// Continue the SAME family, so a future reuse anywhere in the chain can
	// still revoke everything descended from the original login.
	return s.issueSession(ctx, user, rt.DeviceID, in.IP, in.UserAgent, rt.FamilyID)
}

// Logout revokes the session containing the given refresh token.
//
// Idempotent: logging out with an unknown or already-revoked token is a success
// from the caller's point of view, because the desired end state (no live
// session) is already true. Returning an error here would make a client retry
// forever after a lost response.
func (s *Auth) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}

	rt, err := s.tokens.FindByHash(ctx, s.pool, session.HashToken(refreshToken))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}

	if _, err := s.tokens.RevokeFamily(ctx, s.pool, rt.FamilyID, domain.RevokeLogout); err != nil {
		return err
	}
	return nil
}

// RevokeAllSessions kills every live session for a user.
//
// Called after a password change: a stolen password must not keep working, and
// every device that logged in with it must re-authenticate.
func (s *Auth) RevokeAllSessions(ctx context.Context, userID uuid.UUID, reason string) (int64, error) {
	return s.tokens.RevokeAllForUser(ctx, s.pool, userID, reason)
}

// SessionInfo describes a user's live sessions, for the admin device screen.
type SessionInfo struct {
	LiveSessions int
}

// DescribeSessions reports how many usable sessions a user has.
func (s *Auth) DescribeSessions(ctx context.Context, userID uuid.UUID) (SessionInfo, error) {
	n, err := s.tokens.CountLiveForUser(ctx, s.pool, userID)
	if err != nil {
		return SessionInfo{}, err
	}
	return SessionInfo{LiveSessions: n}, nil
}

// ResetRateLimits clears the in-memory login limiters.
//
// Test seams are worth naming as such: this exists so integration tests can
// exercise login repeatedly without tripping the per-account budget. It is also
// a legitimate operational escape hatch when an admin has verified a user's
// identity and wants to clear a lockout immediately.
//
// It does NOT clear the database lockout (users.locked_until) — that requires
// RecordLoginSuccess or a direct admin action.
func (s *Auth) ResetRateLimits() {
	s.byAcct.ResetAll()
	s.byIP.ResetAll()
}

// AccessTTL exposes the configured access-token lifetime

// AccessTTL exposes the configured access-token lifetime, for the HTTP layer to
// report `expiresIn`.
func (s *Auth) AccessTTL() time.Duration { return s.accessTTL }
