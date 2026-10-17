package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Tokens stores access-token revocations.
type Tokens struct{}

// NewTokens builds the repository.
func NewTokens() *Tokens { return &Tokens{} }

// Revoke records that an access token must stop working now.
//
// Idempotent: logging out twice is harmless, and a retried request must not
// fail because the revocation already happened.
func (r *Tokens) Revoke(ctx context.Context, q Querier, jti uuid.UUID, expiresAt time.Time, reason string) error {
	const sql = `
        INSERT INTO revoked_access_tokens (jti, expires_at, reason)
        VALUES ($1, $2, $3)
        ON CONFLICT (jti) DO NOTHING`

	if _, err := q.Exec(ctx, sql, jti, expiresAt, nullIfEmpty(reason)); err != nil {
		return fmt.Errorf("revoke access token: %w", err)
	}
	return nil
}

// IsRevoked answers the only question the middleware asks.
func (r *Tokens) IsRevoked(ctx context.Context, q Querier, jti uuid.UUID) (bool, error) {
	const sql = `SELECT EXISTS (SELECT 1 FROM revoked_access_tokens WHERE jti = $1)`

	var revoked bool
	if err := q.QueryRow(ctx, sql, jti).Scan(&revoked); err != nil {
		return false, fmt.Errorf("check revocation: %w", err)
	}
	return revoked, nil
}

// Prune drops revocations whose tokens have already expired: an expired token is
// rejected by signature verification anyway, so its denylist entry earns nothing.
func (r *Tokens) Prune(ctx context.Context, q Querier) (int64, error) {
	const sql = `DELETE FROM revoked_access_tokens WHERE expires_at < now()`

	tag, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, fmt.Errorf("prune revocations: %w", err)
	}
	return tag.RowsAffected(), nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
