package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Devices provides access to registered workstations.
type Devices struct{}

// NewDevices builds the repository.
func NewDevices() *Devices { return &Devices{} }

const deviceColumns = `
    id, registered_by, name, platform,
    public_key, public_key_alg, fingerprint,
    app_version, last_seen_at, last_sync_cursor,
    revoked_at, revoked_by, revoked_reason,
    created_at, updated_at`

// FindByID loads a device, including revoked ones.
//
// Revoked devices ARE returned: the caller must be able to distinguish
// "unknown device" from "revoked device" in order to return the right error
// (DEVICE_MISMATCH vs DEVICE_REVOKED) and trigger the right client behaviour.
func (r *Devices) FindByID(ctx context.Context, q Querier, id uuid.UUID) (*domain.Device, error) {
	const sql = `SELECT` + deviceColumns + `
          FROM devices WHERE id = $1 AND deleted_at IS NULL`

	var d domain.Device
	err := q.QueryRow(ctx, sql, id).Scan(
		&d.ID, &d.RegisteredBy, &d.Name, &d.Platform,
		&d.PublicKey, &d.PublicKeyAlg, &d.Fingerprint,
		&d.AppVersion, &d.LastSeenAt, &d.LastSyncCursor,
		&d.RevokedAt, &d.RevokedBy, &d.RevokedReason,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find device: %w", err)
	}
	return &d, nil
}

// FindByFingerprint loads a device by its public-key fingerprint.
func (r *Devices) FindByFingerprint(ctx context.Context, q Querier, fingerprint string) (*domain.Device, error) {
	const sql = `SELECT` + deviceColumns + `
          FROM devices WHERE fingerprint = $1 AND deleted_at IS NULL`

	var d domain.Device
	err := q.QueryRow(ctx, sql, fingerprint).Scan(
		&d.ID, &d.RegisteredBy, &d.Name, &d.Platform,
		&d.PublicKey, &d.PublicKeyAlg, &d.Fingerprint,
		&d.AppVersion, &d.LastSeenAt, &d.LastSyncCursor,
		&d.RevokedAt, &d.RevokedBy, &d.RevokedReason,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find device by fingerprint: %w", err)
	}
	return &d, nil
}

// Insert registers a new device.
func (r *Devices) Insert(ctx context.Context, q Querier, d *domain.Device, actorID uuid.UUID) error {
	const sql = `
        INSERT INTO devices (
            id, registered_by, name, platform,
            public_key, public_key_alg, fingerprint, app_version,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`

	if _, err := q.Exec(ctx, sql,
		d.ID, d.RegisteredBy, d.Name, d.Platform,
		d.PublicKey, d.PublicKeyAlg, d.Fingerprint, d.AppVersion,
		actorID,
	); err != nil {
		// A duplicate fingerprint is a normal, expected outcome when a browser
		// re-registers after clearing storage. Surface it as a conflict the
		// caller can handle rather than a 500.
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: device fingerprint already registered", domain.ErrConflict)
		}
		return fmt.Errorf("insert device: %w", err)
	}
	return nil
}

// TouchSeen updates last_seen_at. Called on every authenticated request, so it
// is deliberately a single cheap UPDATE.
func (r *Devices) TouchSeen(ctx context.Context, q Querier, id uuid.UUID, at time.Time) error {
	const sql = `UPDATE devices SET last_seen_at = $2 WHERE id = $1`
	if _, err := q.Exec(ctx, sql, id, at); err != nil {
		return fmt.Errorf("touch device: %w", err)
	}
	return nil
}

// Revoke marks a device unusable and records why.
func (r *Devices) Revoke(ctx context.Context, q Querier, id, by uuid.UUID, reason string, at time.Time) error {
	const sql = `
        UPDATE devices
           SET revoked_at = $3, revoked_by = $2, revoked_reason = $4
         WHERE id = $1 AND revoked_at IS NULL`

	if _, err := q.Exec(ctx, sql, id, by, at, reason); err != nil {
		return fmt.Errorf("revoke device: %w", err)
	}
	return nil
}

// isUniqueViolation detects SQLSTATE 23505 without leaking pgconn into the
// service layer. Kept private: services should reason in domain errors
// (ErrConflict), never in SQLSTATE codes.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
