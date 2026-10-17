package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Sync holds the server-side sync machinery: scopes, batches, operations,
// change log, conflicts and device health.
//
// Every method takes a Querier so the whole push runs in ONE transaction. That
// is not a preference: a batch that inserts a row without appending to the
// change log leaves every other device permanently blind to that row, and the
// symptom — "sometimes records do not appear on the other computer" — is close
// to undebuggable after the fact.
type Sync struct{}

// NewSync builds the repository.
func NewSync() *Sync { return &Sync{} }

// Scope loads the policy for one table.
func (r *Sync) Scope(ctx context.Context, q Querier, table string) (*domain.SyncScope, error) {
	const sql = `
        SELECT table_name, sync_enabled, pull_roles, push_roles,
               conflict_policy, retention_days, notes
          FROM sync_table_scopes
         WHERE table_name = $1`

	var s domain.SyncScope
	err := q.QueryRow(ctx, sql, table).Scan(
		&s.TableName, &s.SyncEnabled, &s.PullRoles, &s.PushRoles,
		&s.ConflictPolicy, &s.RetentionDays, &s.Notes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// NOT domain.ErrNotFound: "this table does not sync" is a client
			// error (rejected operation), not a missing resource.
			return nil, domain.Public(domain.ErrValidation, "That table does not sync.")
		}
		return nil, fmt.Errorf("load sync scope: %w", err)
	}
	return &s, nil
}

// InsertBatch records a batch, reporting false when it already existed.
//
// The primary key IS the client's batch id, so a retried batch collides here
// and is answered from the stored results instead of being applied twice.
func (r *Sync) InsertBatch(
	ctx context.Context,
	q Querier,
	id, deviceID, userID uuid.UUID,
	operationCount int,
	clientSentAt *time.Time,
) (bool, error) {
	const sql = `
        INSERT INTO sync_batches (
            id, device_id, user_id, operation_count,
            accepted_count, rejected_count, client_sent_at, highest_server_rev
        ) VALUES ($1, $2, $3, $4, 0, 0, $5, 0)
        ON CONFLICT (id) DO NOTHING`

	tag, err := q.Exec(ctx, sql, id, deviceID, userID, operationCount, clientSentAt)
	if err != nil {
		return false, fmt.Errorf("insert sync batch: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// StoredResults replays the results of a batch that was already applied.
//
// Ordering by applied_at keeps the response stable between the original push and
// a replay, which matters because clients compare the two when debugging.
func (r *Sync) StoredResults(ctx context.Context, q Querier, batchID uuid.UUID) ([]domain.OpResult, error) {
	const sql = `
        SELECT id, table_name, row_id, operation, result, COALESCE(reason, ''), COALESCE(server_rev, 0)
          FROM sync_operations
         WHERE batch_id = $1
         ORDER BY applied_at, id`

	rows, err := q.Query(ctx, sql, batchID)
	if err != nil {
		return nil, fmt.Errorf("load stored results: %w", err)
	}
	defer rows.Close()

	var out []domain.OpResult
	for rows.Next() {
		var (
			res       domain.OpResult
			table     string
			rowID     uuid.UUID
			opName    string
			status    string
			reason    string
			serverRev int64
		)
		if err := rows.Scan(&res.OperationID, &table, &rowID, &opName, &status, &reason, &serverRev); err != nil {
			return nil, fmt.Errorf("scan stored result: %w", err)
		}
		res.Status = domain.OpStatus(status)
		res.RowID = rowID.String()
		res.ServerRev = serverRev
		res.Reason = reason
		out = append(out, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stored results: %w", err)
	}
	return out, nil
}

// OperationExists reports whether this operation id was already processed.
//
// This is the real idempotency guarantee: a single operation can be replayed
// inside a NEW batch and is still recognised, which batch-level idempotency
// alone would miss.
func (r *Sync) OperationExists(ctx context.Context, q Querier, id uuid.UUID) (bool, error) {
	const sql = `SELECT EXISTS (SELECT 1 FROM sync_operations WHERE id = $1)`

	var exists bool
	if err := q.QueryRow(ctx, sql, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("check operation: %w", err)
	}
	return exists, nil
}

// InsertOperation records one operation's outcome.
func (r *Sync) InsertOperation(
	ctx context.Context,
	q Querier,
	id, batchID uuid.UUID,
	table string,
	rowID uuid.UUID,
	operation string,
	result domain.OpStatus,
	reason string,
	serverRev int64,
) error {
	const sql = `
        INSERT INTO sync_operations (
            id, batch_id, table_name, row_id, operation, result, reason, server_rev
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	var rev *int64
	if serverRev > 0 {
		rev = &serverRev
	}
	var why *string
	if reason != "" {
		why = &reason
	}

	if _, err := q.Exec(ctx, sql, id, batchID, table, rowID, operation, string(result), why, rev); err != nil {
		return fmt.Errorf("insert sync operation: %w", err)
	}
	return nil
}

// ChangeEntry is one row appended to the pull cursor.
type ChangeEntry struct {
	Table            string
	RowID            uuid.UUID
	Operation        string // upsert | delete
	ServerRev        int64
	ChangedBy        uuid.UUID
	DeviceID         *uuid.UUID
	DeviceRecordedAt *time.Time
}

// AppendChange writes one change-log row and returns its sequence number.
//
// EVERY accepted write must call this. Forgetting it makes the row invisible to
// every other device for ever, while the writing device looks perfectly healthy.
func (r *Sync) AppendChange(ctx context.Context, q Querier, e ChangeEntry) (int64, error) {
	const sql = `
        INSERT INTO sync_change_log (
            table_name, row_id, operation, server_rev,
            changed_by, device_id, device_recorded_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING seq`

	var seq int64
	if err := q.QueryRow(ctx, sql,
		e.Table, e.RowID, e.Operation, e.ServerRev,
		e.ChangedBy, e.DeviceID, e.DeviceRecordedAt,
	).Scan(&seq); err != nil {
		return 0, fmt.Errorf("append change log: %w", err)
	}
	return seq, nil
}

// ConflictRecord is one recorded overwrite or rejection.
type ConflictRecord struct {
	ID             uuid.UUID
	Table          string
	RowID          uuid.UUID
	DeviceID       uuid.UUID
	UserID         uuid.UUID
	ConflictType   string
	ClientRevision *int64
	ServerRevision *int64
	WinningPayload map[string]any
	LosingPayload  map[string]any
}

// InsertConflict records a conflict, keeping BOTH payloads.
//
// The losing payload is never discarded. Storage is cheap; a lost clinical edit
// is not, and "what happened to my edit" is a question that will be asked.
func (r *Sync) InsertConflict(ctx context.Context, q Querier, c ConflictRecord) error {
	const sql = `
        INSERT INTO sync_conflicts (
            id, table_name, row_id, device_id, user_id, conflict_type,
            client_revision, server_revision, winning_payload, losing_payload
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb)`

	// Marshalled HERE rather than handed to pgx as a map.
	//
	// A map[string]any containing a *string (a nil body, say) has no pgx encode
	// plan for JSONB in QueryExecModeExec — the pooler does not support prepared
	// statements, so the type must be resolved from the literal. The failure is
	// "unable to encode ... into text format for unknown type (OID 0)", which is
	// an unhelpful way to learn that.
	winning, err := marshalJSON(c.WinningPayload)
	if err != nil {
		return fmt.Errorf("encode winning payload: %w", err)
	}
	losing, err := marshalJSON(c.LosingPayload)
	if err != nil {
		return fmt.Errorf("encode losing payload: %w", err)
	}

	if _, err := q.Exec(ctx, sql,
		c.ID, c.Table, c.RowID, c.DeviceID, c.UserID, c.ConflictType,
		c.ClientRevision, c.ServerRevision, winning, losing,
	); err != nil {
		return fmt.Errorf("insert sync conflict: %w", err)
	}
	return nil
}

// marshalJSON encodes an optional payload as TEXT, returning nil for an absent
// one so the column stays NULL rather than the JSON literal `null`.
//
// A string, not []byte: pgx sends []byte as `bytea`, and Postgres rejects
// bytea→jsonb with "invalid input syntax for type json" (22P02). Text casts
// cleanly, which is what the ::jsonb in the statement expects.
func marshalJSON(payload map[string]any) (*string, error) {
	if payload == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	text := string(encoded)
	return &text, nil
}

// FinishBatch records the outcome counts and the highest rev assigned.
func (r *Sync) FinishBatch(
	ctx context.Context,
	q Querier,
	id uuid.UUID,
	accepted, rejected int,
	highestServerRev int64,
) error {
	const sql = `
        UPDATE sync_batches
           SET accepted_count = $2, rejected_count = $3, highest_server_rev = $4
         WHERE id = $1`

	if _, err := q.Exec(ctx, sql, id, accepted, rejected, highestServerRev); err != nil {
		return fmt.Errorf("finish sync batch: %w", err)
	}
	return nil
}

// TouchDeviceHealth updates the admin-visible sync state for a device.
//
// `pending_operation_count` is the client's own report of what is still queued.
// It is the field that lets an admin see "this workstation has 40 operations
// stuck" before the user complains.
func (r *Sync) TouchDeviceHealth(
	ctx context.Context,
	q Querier,
	deviceID uuid.UUID,
	pushedAt time.Time,
	pending int,
	cursor int64,
) error {
	const sql = `
        INSERT INTO device_sync_state (
            device_id, last_push_at, pending_operation_count, last_pull_cursor, updated_at
        ) VALUES ($1, $2, $3, $4, now())
        ON CONFLICT (device_id) DO UPDATE
           SET last_push_at = EXCLUDED.last_push_at,
               pending_operation_count = EXCLUDED.pending_operation_count,
               last_pull_cursor = GREATEST(device_sync_state.last_pull_cursor, EXCLUDED.last_pull_cursor),
               updated_at = now()`

	if _, err := q.Exec(ctx, sql, deviceID, pushedAt, pending, cursor); err != nil {
		return fmt.Errorf("touch device sync state: %w", err)
	}
	return nil
}

// CurrentSeq returns the newest change-log sequence.
//
// Used as the client's next pull cursor. Reading it inside the push transaction
// is what makes the returned cursor consistent with what was just written.
func (r *Sync) CurrentSeq(ctx context.Context, q Querier) (int64, error) {
	const sql = `SELECT COALESCE(max(seq), 0) FROM sync_change_log`

	var seq int64
	if err := q.QueryRow(ctx, sql).Scan(&seq); err != nil {
		return 0, fmt.Errorf("read change-log head: %w", err)
	}
	return seq, nil
}

// ChangeRow is one entry of the ordered pull manifest.
type ChangeRow struct {
	Seq       int64
	Table     string
	RowID     uuid.UUID
	Operation string
	ServerRev int64
}

// PullChanges reads the change log after a cursor.
//
// MUST be called inside a REPEATABLE READ transaction. `seq` comes from a
// sequence and is NOT gap-free: a row can be committed with seq 10240 while
// 10238 and 10239 are still uncommitted. Outside a snapshot, the reader sees
// 10240, advances the cursor past the gap, and those two rows are lost for ever.
func (r *Sync) PullChanges(ctx context.Context, q Querier, since int64, limit int) ([]ChangeRow, error) {
	const sql = `
        SELECT seq, table_name, row_id, operation, server_rev
          FROM sync_change_log
         WHERE seq > $1
         ORDER BY seq
         LIMIT $2`

	rows, err := q.Query(ctx, sql, since, limit)
	if err != nil {
		return nil, fmt.Errorf("read change log: %w", err)
	}
	defer rows.Close()

	var out []ChangeRow
	for rows.Next() {
		var c ChangeRow
		if err := rows.Scan(&c.Seq, &c.Table, &c.RowID, &c.Operation, &c.ServerRev); err != nil {
			return nil, fmt.Errorf("scan change: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate changes: %w", err)
	}
	return out, nil
}

// OldestSeq is the lowest sequence still retained.
//
// Used to detect a cursor that predates retention: such a device cannot catch up
// incrementally, and pretending otherwise silently drops everything it missed.
func (r *Sync) OldestSeq(ctx context.Context, q Querier) (int64, error) {
	const sql = `SELECT COALESCE(min(seq), 0) FROM sync_change_log`

	var seq int64
	if err := q.QueryRow(ctx, sql).Scan(&seq); err != nil {
		return 0, fmt.Errorf("read oldest seq: %w", err)
	}
	return seq, nil
}

// FetchJSONRows returns whole rows as JSON objects, keyed by column name.
//
// `columns` is an ALLOWLIST supplied by the service, never interpolated from a
// request. Selecting an explicit column list is the primary redaction control:
// password_hash is not selected, so it cannot leak even if a later filter has a
// bug. Deleting keys afterwards (see the service) is defence in depth.
func (r *Sync) FetchJSONRows(
	ctx context.Context,
	q Querier,
	table, columns string,
	ids []uuid.UUID,
) ([]map[string]any, error) {
	// []uuid.UUID has no encode plan in QueryExecModeExec (the pooler supports no
	// prepared statements, so there is no parameter type to infer from). A
	// []string plus an explicit ::uuid[] cast makes the target type explicit and
	// the conversion happens in Postgres.
	idStrings := make([]string, 0, len(ids))
	for _, id := range ids {
		idStrings = append(idStrings, id.String())
	}

	sql := fmt.Sprintf(`
        SELECT to_jsonb(t)
          FROM (SELECT %s FROM %s WHERE id = ANY($1::uuid[])) t`, columns, table)

	rows, err := q.Query(ctx, sql, idStrings)
	if err != nil {
		return nil, fmt.Errorf("fetch %s rows: %w", table, err)
	}
	defer rows.Close()

	return scanJSONObjects(rows)
}

// FetchJSONPage returns one keyset page of a table, ordered by id.
//
// Keyset, not OFFSET: an offset scan re-reads everything it skipped, which
// degrades linearly and is exactly wrong for a bootstrap over a slow link.
func (r *Sync) FetchJSONPage(
	ctx context.Context,
	q Querier,
	table, columns string,
	after uuid.UUID,
	limit int,
) ([]map[string]any, error) {
	sql := fmt.Sprintf(`
        SELECT to_jsonb(t)
          FROM (SELECT %s FROM %s WHERE id > $1 ORDER BY id LIMIT $2) t`, columns, table)

	rows, err := q.Query(ctx, sql, after, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch %s page: %w", table, err)
	}
	defer rows.Close()

	return scanJSONObjects(rows)
}

func scanJSONObjects(rows pgx.Rows) ([]map[string]any, error) {
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan json row: %w", err)
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, fmt.Errorf("decode json row: %w", err)
		}
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate json rows: %w", err)
	}
	return out, nil
}
