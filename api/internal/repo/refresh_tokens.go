package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// RefreshTokens provides access to issued refresh tokens.
//
// This table is SERVER-ONLY: it has no sync columns and is never replicated to a
// client. See docs/01-DATA-MODEL.md §5.5.
type RefreshTokens struct{}

// NewRefreshTokens builds the repository.
func NewRefreshTokens() *RefreshTokens { return &RefreshTokens{} }

const refreshTokenColumns = `
    id, user_id, device_id, token_hash, family_id,
    issued_at, expires_at, used_at, revoked_at, revoked_reason,
    host(ip), user_agent`

// Insert records a newly issued refresh token.
func (r *RefreshTokens) Insert(ctx context.Context, q Querier, t *domain.RefreshToken) error {
	const sql = `
        INSERT INTO refresh_tokens (
            id, user_id, device_id, token_hash, family_id,
            issued_at, expires_at, ip, user_agent
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`

	if _, err := q.Exec(ctx, sql,
		t.ID, t.UserID, t.DeviceID, t.TokenHash, t.FamilyID,
		t.IssuedAt, t.ExpiresAt, t.IP, t.UserAgent,
	); err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// FindByHash loads a token by its hash, whatever its state.
//
// Crucially this returns REVOKED and USED tokens too. The reuse-detection
// policy needs to see a consumed token in order to revoke its whole family —
// filtering those out here would silently disable the protection.
func (r *RefreshTokens) FindByHash(ctx context.Context, q Querier, hash string) (*domain.RefreshToken, error) {
	const sql = `SELECT` + refreshTokenColumns + `
          FROM refresh_tokens
         WHERE token_hash = $1`

	var t domain.RefreshToken
	err := q.QueryRow(ctx, sql, hash).Scan(
		&t.ID, &t.UserID, &t.DeviceID, &t.TokenHash, &t.FamilyID,
		&t.IssuedAt, &t.ExpiresAt, &t.UsedAt, &t.RevokedAt, &t.RevokedReason,
		&t.IP, &t.UserAgent,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	return &t, nil
}

// MarkUsed stamps a token as consumed. This is the rotation step.
//
// The `AND used_at IS NULL` guard makes consumption ATOMIC: if two requests race
// with the same token, exactly one UPDATE matches and the other affects zero
// rows, which the service interprets as reuse. Without this guard both requests
// would "succeed" and the same token would be exchanged twice.
func (r *RefreshTokens) MarkUsed(ctx context.Context, q Querier, id uuid.UUID, at time.Time) (bool, error) {
	const sql = `
        UPDATE refresh_tokens
           SET used_at = $2
         WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL`

	tag, err := q.Exec(ctx, sql, id, at)
	if err != nil {
		return false, fmt.Errorf("mark refresh token used: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// RevokeFamily revokes every live token in a family.
//
// Called on logout, on reuse detection, and on device revocation. Revoking the
// family rather than a single token is what makes reuse detection meaningful:
// once a token is known stolen, every sibling token descended from the same
// login must die too.
func (r *RefreshTokens) RevokeFamily(ctx context.Context, q Querier, familyID uuid.UUID, reason string) (int64, error) {
	const sql = `
        UPDATE refresh_tokens
           SET revoked_at = now(), revoked_reason = $2
         WHERE family_id = $1 AND revoked_at IS NULL`

	tag, err := q.Exec(ctx, sql, familyID, reason)
	if err != nil {
		return 0, fmt.Errorf("revoke token family: %w", err)
	}
	return tag.RowsAffected(), nil
}

// RevokeAllForUser revokes every live session a user has.
//
// Called on password change (a stolen password must not keep working) and by an
// admin forcing re-authentication.
func (r *RefreshTokens) RevokeAllForUser(ctx context.Context, q Querier, userID uuid.UUID, reason string) (int64, error) {
	const sql = `
        UPDATE refresh_tokens
           SET revoked_at = now(), revoked_reason = $2
         WHERE user_id = $1 AND revoked_at IS NULL`

	tag, err := q.Exec(ctx, sql, userID, reason)
	if err != nil {
		return 0, fmt.Errorf("revoke user sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CountLiveForUser reports how many usable sessions a user has.
func (r *RefreshTokens) CountLiveForUser(ctx context.Context, q Querier, userID uuid.UUID) (int, error) {
	const sql = `
        SELECT count(*) FROM refresh_tokens
         WHERE user_id = $1
           AND revoked_at IS NULL
           AND used_at IS NULL
           AND expires_at > now()`

	var n int
	if err := q.QueryRow(ctx, sql, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count live sessions: %w", err)
	}
	return n, nil
}
