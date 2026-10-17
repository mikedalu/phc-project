package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// appendOnlyApplier is for tables whose rows are FACTS: vitals today, payments
// and dispenses later.
//
// ═══ WHY UPDATES ARE REJECTED RATHER THAN MERGED ═══
//
// Two nurses taking a blood pressure on two devices produce two readings, and
// both are true. Last-write-wins would silently discard one and present a trend
// line no clinician ever observed — the worst outcome in a clinical record,
// because it is invisible.
//
// A delete is allowed only within the same working day: enough to fix "I typed
// the weight into the height field", not enough to rewrite last week.
type appendOnlyApplier struct {
	// insert reuses the LWW applier's insert path, which already builds the
	// column list, writes the change log and returns the assigned revision.
	insert *lwwApplier
	sync   *repo.Sync
	now    func() time.Time
}

func (a *appendOnlyApplier) apply(ctx context.Context, tx pgx.Tx, in applyContext) (applyOutcome, error) {
	switch in.op.Operation {
	case "insert":
		return a.applyInsert(ctx, tx, in)
	case "update":
		return applyOutcome{status: domain.OpRejected, reason: "append_only"}, nil
	case "delete":
		return a.applyDelete(ctx, tx, in)
	default:
		return applyOutcome{status: domain.OpRejected, reason: "unknown_operation"}, nil
	}
}

func (a *appendOnlyApplier) applyInsert(ctx context.Context, tx pgx.Tx, in applyContext) (applyOutcome, error) {
	if in.op.BaseServerRev != nil {
		// An "update" payload arriving as an insert means the device thinks this
		// row already exists. For an append-only table that is a client bug, and
		// merging it would defeat the policy.
		return applyOutcome{status: domain.OpRejected, reason: "append_only"}, nil
	}

	// Attribution comes from the SESSION, never from the payload.
	//
	// `recorded_by` is NOT NULL, and a client has no business asserting who
	// recorded an observation — that is precisely the field a tampered client
	// would forge. When the payload omits it, the acting user fills it.
	if _, present := in.op.Payload["recordedBy"]; !present {
		in.op.Payload["recordedBy"] = in.userID.String()
	}

	// Collect only the columns the payload carries — see D51.
	used := make([]columnSpec, 0, len(a.insert.columns))
	values := make([]any, 0, len(a.insert.columns))
	for _, col := range a.insert.columns {
		if _, present := in.op.Payload[col.Field]; !present {
			if col.Required {
				return applyOutcome{status: domain.OpRejected, reason: "validation:" + col.Field}, nil
			}
			continue
		}
		value, err := convertField(col, in.op.Payload)
		if err != nil {
			return applyOutcome{status: domain.OpRejected, reason: "validation:" + col.Field}, nil
		}
		used = append(used, col)
		values = append(values, value)
	}

	_, found, err := a.insert.loadCurrent(ctx, tx, in.rowID)
	if err != nil {
		return applyOutcome{}, err
	}
	if found {
		// The same reading arriving twice. The operation id usually catches this;
		// this is the belt-and-braces for a device that regenerated its ids.
		return applyOutcome{status: domain.OpDuplicate, reason: "already_exists"}, nil
	}

	now := a.now()
	created := saneClientTime(payloadTime(in.op.Payload, "createdAt"), now)
	recorded := saneClientTime(payloadTime(in.op.Payload, "recordedAt"), created)

	return a.insert.insert(ctx, tx, in, used, values, created, recorded)
}

func (a *appendOnlyApplier) applyDelete(ctx context.Context, tx pgx.Tx, in applyContext) (applyOutcome, error) {
	const ageSQL = `SELECT created_at FROM %s WHERE id = $1`

	var createdAt time.Time
	if err := tx.QueryRow(ctx, fmt.Sprintf(ageSQL, a.insert.table), in.rowID).Scan(&createdAt); err != nil {
		return applyOutcome{status: domain.OpDuplicate, reason: "already_absent"}, nil
	}

	if a.now().Sub(createdAt) > 24*time.Hour {
		return applyOutcome{status: domain.OpRejected, reason: "append_only_too_old"}, nil
	}

	sql := fmt.Sprintf(`
        UPDATE %s
           SET deleted_at = now(), updated_at = now(), updated_by = $2,
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND deleted_at IS NULL
     RETURNING server_rev`, a.insert.table)

	var rev int64
	if err := tx.QueryRow(ctx, sql, in.rowID, in.userID).Scan(&rev); err != nil {
		return applyOutcome{status: domain.OpDuplicate, reason: "already_absent"}, nil
	}

	if _, err := a.sync.AppendChange(ctx, tx, repo.ChangeEntry{
		Table: a.insert.table, RowID: in.rowID, Operation: "delete",
		ServerRev: rev, ChangedBy: in.userID, DeviceID: &in.deviceID,
		DeviceRecordedAt: in.op.DeviceRecordedAt,
	}); err != nil {
		return applyOutcome{}, err
	}

	return applyOutcome{status: domain.OpApplied, serverRev: rev}, nil
}
