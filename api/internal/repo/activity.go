package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Activity is one recorded change, with who made it.
//
// It reads `sync_change_log` rather than a separate audit table: that log is
// already the authoritative record of every write to every synced table, it is
// appended by trigger (so it cannot be forgotten), and it carries the actor. A
// second audit table would be a second truth to keep in step.
type Activity struct {
	Seq        int64
	Table      string
	RowID      uuid.UUID
	Operation  string
	ServerRev  int64
	ChangedBy  uuid.UUID
	ActorName  string
	ActorStaff string
	ChangedAt  time.Time
	DeviceID   *uuid.UUID
}

// Activity lists recent changes, newest first.
func (r *Sync) Activity(ctx context.Context, q Querier, limit int) ([]Activity, error) {
	const sql = `
        SELECT c.seq, c.table_name, c.row_id, c.operation, c.server_rev,
               c.changed_by, coalesce(u.first_name || ' ' || u.last_name, 'Unknown'),
               coalesce(u.staff_number, '—'), c.changed_at, c.device_id
          FROM sync_change_log c
          LEFT JOIN users u ON u.id = c.changed_by
         ORDER BY c.seq DESC
         LIMIT $1`

	rows, err := q.Query(ctx, sql, limit)
	if err != nil {
		return nil, fmt.Errorf("list activity: %w", err)
	}
	defer rows.Close()

	out := make([]Activity, 0, limit)
	for rows.Next() {
		var a Activity
		if err := rows.Scan(
			&a.Seq, &a.Table, &a.RowID, &a.Operation, &a.ServerRev,
			&a.ChangedBy, &a.ActorName, &a.ActorStaff, &a.ChangedAt, &a.DeviceID,
		); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity: %w", err)
	}
	return out, nil
}

// Conflicts lists recent LWW decisions, newest first.
func (r *Sync) RecentConflicts(ctx context.Context, q Querier, limit int) ([]map[string]any, error) {
	const sql = `
        SELECT id, table_name, row_id,
               coalesce(conflict_type, 'lww'), detected_at
          FROM sync_conflicts
         ORDER BY detected_at DESC
         LIMIT $1`

	rows, err := q.Query(ctx, sql, limit)
	if err != nil {
		return nil, fmt.Errorf("list conflicts: %w", err)
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, rowID uuid.UUID
		var table, resolution string
		var detectedAt time.Time
		if err := rows.Scan(&id, &table, &rowID, &resolution, &detectedAt); err != nil {
			return nil, fmt.Errorf("scan conflict: %w", err)
		}
		out = append(out, map[string]any{
			"id": id.String(), "table": table, "rowId": rowID.String(),
			"resolution": resolution, "createdAt": detectedAt.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conflicts: %w", err)
	}
	return out, nil
}
