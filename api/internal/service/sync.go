package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// Sync implements POST /sync/push.
//
// ═══ THE ONE INVARIANT ═══
//
// A row write and its sync_change_log append happen in the same transaction. If
// they ever separate, the writing device looks perfectly healthy while every
// other device stays blind to the row for ever — and "sometimes records do not
// appear on the other computer" is close to undebuggable after the fact.
//
// ═══ WHY A REGISTRY ═══
//
// Applying an operation is table-specific SQL; everything else (idempotency,
// scope checks, conflict recording, cursor bookkeeping) is uniform. The registry
// is the seam: a new syncable table adds an applier and a scope row, and the
// engine itself does not change.
type Sync struct {
	pool     *pgxpool.Pool
	sync     *repo.Sync
	devices  *repo.Devices
	users    *repo.Users
	appliers map[string]syncApplier
	maxBatch int
	now      func() time.Time
}

// NewSync wires the sync service.
func NewSync(pool *pgxpool.Pool, maxBatch int) *Sync {
	s := &Sync{
		pool:     pool,
		sync:     repo.NewSync(),
		devices:  repo.NewDevices(),
		users:    repo.NewUsers(pool),
		maxBatch: maxBatch,
		now:      time.Now,
	}
	if s.maxBatch <= 0 {
		s.maxBatch = 200
	}

	s.appliers = map[string]syncApplier{
		"sync_probe": &probeApplier{sync: s.sync, now: s.now},
	}

	return s
}

// PushOperation is one operation as received.
type PushOperation struct {
	ID               string
	Table            string
	RowID            string
	Operation        string
	BaseServerRev    *int64
	DeviceRecordedAt *time.Time
	Payload          map[string]any
}

// PushInput is one batch.
type PushInput struct {
	UserID       uuid.UUID
	DeviceID     uuid.UUID
	BatchID      string
	ClientSentAt *time.Time
	// PendingCount is the client's own report, surfaced to admins.
	PendingCount int
	Operations   []PushOperation
}

// Push applies a batch and returns the per-operation results.
func (s *Sync) Push(ctx context.Context, in PushInput) (*domain.BatchResult, error) {
	batchID, err := uuid.Parse(in.BatchID)
	if err != nil {
		return nil, domain.NewValidation("batchId", "must_be_a_uuid")
	}
	if len(in.Operations) == 0 {
		return nil, domain.NewValidation("operations", "empty")
	}
	if len(in.Operations) > s.maxBatch {
		// Rejected at the cap with a clear error rather than a timeout: a
		// thousand-operation batch would either blow the request deadline or
		// fail halfway, and neither tells the client what to do.
		return nil, domain.NewValidationf("operations", "too_large",
			"%d operations, maximum %d — split the batch", len(in.Operations), s.maxBatch)
	}

	// The acting user, re-read from the database: the token's role can be
	// fifteen minutes stale, and push_roles is an authorisation decision.
	actor, err := s.users.FindByID(ctx, s.pool, in.UserID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}

	now := s.now()

	// Pre-validate ids so a malformed envelope fails the batch rather than
	// producing operations that cannot be recorded idempotently.
	parsed := make([]parsedOp, 0, len(in.Operations))
	for i, op := range in.Operations {
		opID, err := uuid.Parse(op.ID)
		if err != nil {
			return nil, domain.NewValidationf("operations", "invalid_id",
				"operation %d has an invalid operationId", i)
		}
		rowID, err := uuid.Parse(op.RowID)
		if err != nil {
			return nil, domain.NewValidationf("operations", "invalid_id",
				"operation %d has an invalid rowId", i)
		}
		parsed = append(parsed, parsedOp{op: op, id: opID, rowID: rowID})
	}

	var result *domain.BatchResult

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		inserted, err := s.sync.InsertBatch(ctx, tx, batchID, in.DeviceID, in.UserID, len(in.Operations), in.ClientSentAt)
		if err != nil {
			return err
		}

		// ── batch-level replay ─────────────────────────────────────
		//
		// The client resends a batch it never saw acknowledged. Answer from the
		// STORED results: reprocessing would double-apply anything that is not
		// operation-idempotent, and the client cannot tell the difference.
		if !inserted {
			replayed, err := s.sync.StoredResults(ctx, tx, batchID)
			if err != nil {
				return err
			}
			cursor, err := s.sync.CurrentSeq(ctx, tx)
			if err != nil {
				return err
			}

			// Report the stored outcomes as `duplicate`, not as the original
			// `applied`. Nothing was applied this time, and a client that
			// distinguishes the two (it should — `applied` means "a row moved")
			// would otherwise be told a write happened twice.
			for i := range replayed {
				if replayed[i].Status == domain.OpApplied || replayed[i].Status == domain.OpConflictLWW {
					replayed[i].Status = domain.OpDuplicate
					replayed[i].Reason = "batch_replayed"
				}
			}

			result = summarise(batchID.String(), now, replayed, cursor)
			return nil
		}

		results := make([]domain.OpResult, 0, len(parsed))
		var highestRev int64

		for _, p := range parsed {
			res, record, err := s.applyOne(ctx, tx, actor, in.UserID, in.DeviceID, p)
			if err != nil {
				return err
			}

			// A duplicate whose ORIGINAL operation row still exists must not
			// write another row with the same primary key — that is a 23505, and
			// it is what turned "replay an operation in a new batch" into a 500.
			if record {
				if err := s.sync.InsertOperation(
					ctx, tx, p.id, batchID, p.op.Table, p.rowID, p.op.Operation,
					res.Status, res.Reason, res.ServerRev,
				); err != nil {
					return err
				}
			}

			if res.ServerRev > highestRev {
				highestRev = res.ServerRev
			}
			results = append(results, res)
		}

		summary := summarise(batchID.String(), now, results, 0)

		if err := s.sync.FinishBatch(ctx, tx, batchID, summary.Applied+summary.ConflictLWW, summary.Rejected, highestRev); err != nil {
			return err
		}

		cursor, err := s.sync.CurrentSeq(ctx, tx)
		if err != nil {
			return err
		}

		if err := s.sync.TouchDeviceHealth(ctx, tx, in.DeviceID, now, in.PendingCount, cursor); err != nil {
			return err
		}
		if err := s.devices.TouchSeen(ctx, tx, in.DeviceID, now); err != nil {
			return err
		}

		summary.NextCursor = cursor
		result = summary
		return nil
	}); err != nil {
		return nil, err
	}

	log.Info().
		Str("batch_id", in.BatchID).
		Str("device_id", in.DeviceID.String()).
		Int("applied", result.Applied).
		Int("duplicate", result.Duplicate).
		Int("conflict_lww", result.ConflictLWW).
		Int("rejected", result.Rejected).
		Msg("sync push applied")

	return result, nil
}

type parsedOp struct {
	op    PushOperation
	id    uuid.UUID
	rowID uuid.UUID
}

// applyOne runs the uniform checks, then hands the write to the table's applier.
func (s *Sync) applyOne(
	ctx context.Context,
	tx pgx.Tx,
	actor *domain.User,
	userID, deviceID uuid.UUID,
	p parsedOp,
) (domain.OpResult, bool, error) {
	res := domain.OpResult{
		OperationID: p.op.ID,
		RowID:       p.op.RowID,
	}

	// ── operation-level idempotency ────────────────────────────────
	exists, err := s.sync.OperationExists(ctx, tx, p.id)
	if err != nil {
		return res, false, err
	}
	if exists {
		res.Status = domain.OpDuplicate
		res.Reason = "already_applied"
		// record=false: the original sync_operations row is still there.
		return res, false, nil
	}

	// ── scope + role ───────────────────────────────────────────────
	scope, err := s.sync.Scope(ctx, tx, p.op.Table)
	if err != nil {
		// Unknown table: a permanent client error, not a 500.
		res.Status = domain.OpRejected
		res.Reason = "unknown_table"
		return res, true, nil
	}
	if !scope.SyncEnabled {
		res.Status = domain.OpRejected
		res.Reason = "sync_disabled"
		return res, true, nil
	}
	if !scope.AllowsPush(actor.PrimaryRole()) && !hasAnyOf(actor.Roles, scope.PushRoles) {
		res.Status = domain.OpRejected
		res.Reason = "forbidden"
		return res, true, nil
	}

	applier, ok := s.appliers[p.op.Table]
	if !ok {
		res.Status = domain.OpRejected
		res.Reason = "no_applier"
		return res, true, nil
	}

	applied, err := applier.apply(ctx, tx, applyContext{
		actor:    actor,
		userID:   userID,
		deviceID: deviceID,
		op: domain.SyncOperation{
			ID:               p.op.ID,
			Table:            p.op.Table,
			RowID:            p.op.RowID,
			Operation:        p.op.Operation,
			BaseServerRev:    p.op.BaseServerRev,
			DeviceRecordedAt: p.op.DeviceRecordedAt,
			Payload:          p.op.Payload,
		},
		rowID: p.rowID,
	})
	if err != nil {
		return res, true, err
	}

	res.Status = applied.status
	res.ServerRev = applied.serverRev
	res.Reason = applied.reason
	res.KeptServerVersion = applied.keptServer
	res.ConflictID = applied.conflictID
	res.ServerPayload = applied.serverPayload
	return res, true, nil
}

// hasAnyOf reports whether the actor holds any of the scope's roles.
func hasAnyOf(held []domain.Role, allowed []string) bool {
	for _, want := range allowed {
		if want == "*" {
			return true
		}
		for _, have := range held {
			if have.String() == want {
				return true
			}
		}
	}
	return false
}

func summarise(batchID string, now time.Time, results []domain.OpResult, cursor int64) *domain.BatchResult {
	out := &domain.BatchResult{
		BatchID:    batchID,
		ServerTime: now.UTC(),
		Results:    results,
		NextCursor: cursor,
	}
	for _, r := range results {
		switch r.Status {
		case domain.OpApplied:
			out.Applied++
		case domain.OpDuplicate:
			out.Duplicate++
		case domain.OpConflictLWW:
			out.ConflictLWW++
		case domain.OpRejected:
			out.Rejected++
		}
	}
	return out
}

// applyContext carries everything an applier needs.
type applyContext struct {
	actor    *domain.User
	userID   uuid.UUID
	deviceID uuid.UUID
	op       domain.SyncOperation
	rowID    uuid.UUID
}

// applyOutcome is what an applier reports back.
type applyOutcome struct {
	status        domain.OpStatus
	serverRev     int64
	reason        string
	keptServer    bool
	conflictID    string
	serverPayload map[string]any
}

// syncApplier applies one table's operations.
type syncApplier interface {
	apply(ctx context.Context, tx pgx.Tx, in applyContext) (applyOutcome, error)
}

// saneClientTime bounds a device-supplied timestamp.
//
// A device clock can be manipulated (T7). A timestamp far in the future or older
// than the retention window is replaced by server time rather than rejected:
// the clinical content is still real, and refusing it would lose the nurse's
// work over a misconfigured clock. The discrepancy is recorded in the change log
// through device_recorded_at.
func saneClientTime(t *time.Time, fallback time.Time) time.Time {
	if t == nil {
		return fallback
	}
	if t.After(fallback.Add(5*time.Minute)) || t.Before(fallback.Add(-14*24*time.Hour)) {
		return fallback
	}
	return *t
}

// payloadString reads a required string field.
func payloadString(payload map[string]any, field string) (string, error) {
	raw, ok := payload[field]
	if !ok {
		return "", fmt.Errorf("%s is required", field)
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", field)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s must not be empty", field)
	}
	return value, nil
}

// payloadOptionalString reads an optional string field.
func payloadOptionalString(payload map[string]any, field string) *string {
	raw, ok := payload[field]
	if !ok || raw == nil {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return nil
	}
	return &value
}

// payloadTime parses an RFC 3339 field.
func payloadTime(payload map[string]any, field string) *time.Time {
	raw := payloadOptionalString(payload, field)
	if raw == nil {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *raw)
	if err != nil {
		return nil
	}
	return &parsed
}

// errUnsupported marks a table with no applier.
var errUnsupported = errors.New("no applier for table")
