package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// probeApplier syncs `sync_probe`, the development table the Sprint 2 demo uses.
//
// It is a deliberate template: a real clinical applier differs only in its
// column list, its validation, and whether its policy is `lww` or `append_only`.
// Everything expensive (idempotency, scope checks, cursor bookkeeping) already
// lives in Sync.Push.
type probeApplier struct {
	sync *repo.Sync
	now  func() time.Time
}

type probeRow struct {
	ServerRev int64
	Revision  int64
	Title     string
	Body      *string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	DeletedAt *time.Time
}

func (a *probeApplier) load(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*probeRow, bool, error) {
	const sql = `
        SELECT server_rev, revision, title, body, created_at, updated_at, created_by, deleted_at
          FROM sync_probe
         WHERE id = $1`

	var row probeRow
	err := tx.QueryRow(ctx, sql, id).Scan(
		&row.ServerRev, &row.Revision, &row.Title, &row.Body,
		&row.CreatedAt, &row.UpdatedAt, &row.CreatedBy, &row.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &row, true, nil
}

// payload renders a row as the map returned to a client that lost the LWW race.
func (r *probeRow) payload(id uuid.UUID) map[string]any {
	out := map[string]any{
		"id":        id.String(),
		"title":     r.Title,
		"serverRev": r.ServerRev,
		"revision":  r.Revision,
		"createdAt": r.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updatedAt": r.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if r.Body != nil {
		out["body"] = *r.Body
	}
	if r.DeletedAt != nil {
		out["deletedAt"] = r.DeletedAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}

func (a *probeApplier) apply(ctx context.Context, tx pgx.Tx, in applyContext) (applyOutcome, error) {
	now := a.now()

	// ── deletes ────────────────────────────────────────────────────
	if in.op.Operation == "delete" {
		existing, found, err := a.load(ctx, tx, in.rowID)
		if err != nil {
			return applyOutcome{}, err
		}
		if !found || existing.DeletedAt != nil {
			// Deleting something already gone is the desired end state.
			return applyOutcome{status: domain.OpDuplicate, reason: "already_absent"}, nil
		}

		const del = `
            UPDATE sync_probe
               SET deleted_at = $2, updated_at = $2, updated_by = $3,
                   revision = revision + 1, server_rev = server_rev + 1
             WHERE id = $1
         RETURNING server_rev`

		var rev int64
		if err := tx.QueryRow(ctx, del, in.rowID, now, in.userID).Scan(&rev); err != nil {
			return applyOutcome{}, err
		}

		if _, err := a.sync.AppendChange(ctx, tx, repo.ChangeEntry{
			Table: "sync_probe", RowID: in.rowID, Operation: "delete",
			ServerRev: rev, ChangedBy: in.userID, DeviceID: &in.deviceID,
			DeviceRecordedAt: in.op.DeviceRecordedAt,
		}); err != nil {
			return applyOutcome{}, err
		}

		return applyOutcome{status: domain.OpApplied, serverRev: rev}, nil
	}

	// ── inserts and updates ────────────────────────────────────────
	title, err := payloadString(in.op.Payload, "title")
	if err != nil {
		return applyOutcome{
			status: domain.OpRejected,
			reason: "validation:title",
		}, nil
	}
	body := payloadOptionalString(in.op.Payload, "body")

	clientUpdated := saneClientTime(payloadTime(in.op.Payload, "updatedAt"), now)
	if in.op.DeviceRecordedAt != nil {
		clientUpdated = saneClientTime(in.op.DeviceRecordedAt, clientUpdated)
	}
	clientCreated := saneClientTime(payloadTime(in.op.Payload, "createdAt"), clientUpdated)

	existing, found, err := a.load(ctx, tx, in.rowID)
	if err != nil {
		return applyOutcome{}, err
	}

	// ── new row ────────────────────────────────────────────────────
	if !found {
		const ins = `
            INSERT INTO sync_probe (
                id, title, body, revision, server_rev, sync_state,
                created_at, updated_at, created_by, updated_by
            ) VALUES ($1, $2, $3, 1, 1, 'synced', $4, $5, $6, $6)
            RETURNING server_rev`

		var rev int64
		if err := tx.QueryRow(ctx, ins, in.rowID, title, body, clientCreated, clientUpdated, in.userID).Scan(&rev); err != nil {
			return applyOutcome{}, err
		}

		if _, err := a.sync.AppendChange(ctx, tx, repo.ChangeEntry{
			Table: "sync_probe", RowID: in.rowID, Operation: "upsert",
			ServerRev: rev, ChangedBy: in.userID, DeviceID: &in.deviceID,
			DeviceRecordedAt: in.op.DeviceRecordedAt,
		}); err != nil {
			return applyOutcome{}, err
		}

		return applyOutcome{status: domain.OpApplied, serverRev: rev}, nil
	}

	// ── existing row: the LWW decision ─────────────────────────────
	//
	// Fast path first: the client edited the version it had. That is the common
	// case and it must not pay for a conflict record.
	concurrent := in.op.BaseServerRev == nil || *in.op.BaseServerRev != existing.ServerRev

	if concurrent {
		// Tiebreaker order is docs/02-SYNC-PROTOCOL.md §6.2: the payload's
		// updated_at, then deviceRecordedAt, then server_rev. We already folded
		// deviceRecordedAt into clientUpdated above; the server_rev tiebreak is
		// "the server wins", which is the conservative default.
		if !clientUpdated.After(existing.UpdatedAt) {
			conflictID, err := a.recordConflict(ctx, tx, in, existing, "server", nil)
			if err != nil {
				return applyOutcome{}, err
			}
			return applyOutcome{
				status:        domain.OpConflictLWW,
				serverRev:     existing.ServerRev,
				reason:        "server_copy_newer",
				keptServer:    true,
				conflictID:    conflictID,
				serverPayload: existing.payload(in.rowID),
			}, nil
		}
	}

	const upd = `
        UPDATE sync_probe
           SET title = $2, body = $3, updated_at = $4, updated_by = $5,
               revision = revision + 1, server_rev = server_rev + 1,
               deleted_at = NULL
         WHERE id = $1
     RETURNING server_rev`

	var rev int64
	if err := tx.QueryRow(ctx, upd, in.rowID, title, body, clientUpdated, in.userID).Scan(&rev); err != nil {
		return applyOutcome{}, err
	}

	var conflictID string
	if concurrent {
		// The client won, but it still overwrote another device's edit. Record
		// it: "what happened to my edit" is a question that gets asked.
		conflictID, err = a.recordConflict(ctx, tx, in, existing, "client", map[string]any{
			"title": title,
			"body":  body,
		})
		if err != nil {
			return applyOutcome{}, err
		}
	}

	if _, err := a.sync.AppendChange(ctx, tx, repo.ChangeEntry{
		Table: "sync_probe", RowID: in.rowID, Operation: "upsert",
		ServerRev: rev, ChangedBy: in.userID, DeviceID: &in.deviceID,
		DeviceRecordedAt: in.op.DeviceRecordedAt,
	}); err != nil {
		return applyOutcome{}, err
	}

	if concurrent {
		return applyOutcome{
			status:     domain.OpConflictLWW,
			serverRev:  rev,
			reason:     "client_copy_newer",
			keptServer: false,
			conflictID: conflictID,
		}, nil
	}

	return applyOutcome{status: domain.OpApplied, serverRev: rev}, nil
}

// recordConflict stores both payloads and returns the conflict id.
func (a *probeApplier) recordConflict(
	ctx context.Context,
	tx pgx.Tx,
	in applyContext,
	existing *probeRow,
	winner string,
	clientPayload map[string]any,
) (string, error) {
	serverPayload := existing.payload(in.rowID)

	winning, losing := clientPayload, serverPayload
	if winner == "server" {
		winning, losing = serverPayload, in.op.Payload
	}
	if winning == nil {
		winning = serverPayload
	}

	clientRev := in.op.BaseServerRev
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	if err := a.sync.InsertConflict(ctx, tx, repo.ConflictRecord{
		ID:             id,
		Table:          "sync_probe",
		RowID:          in.rowID,
		DeviceID:       in.deviceID,
		UserID:         in.userID,
		ConflictType:   "lww_overwrite",
		ClientRevision: clientRev,
		ServerRevision: &existing.ServerRev,
		WinningPayload: winning,
		LosingPayload:  losing,
	}); err != nil {
		return "", err
	}

	return id.String(), nil
}
