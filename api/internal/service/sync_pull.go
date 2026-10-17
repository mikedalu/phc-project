package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Pull: the read half of replication.
//
// ═══ TWO RULES THAT DECIDE WHETHER THIS IS CORRECT ═══
//
//  1. Run inside ONE REPEATABLE READ transaction. `seq` is not gap-free; reading
//     outside a snapshot can see seq 10240 while 10238–10239 are uncommitted,
//     advance the cursor past them, and lose those rows for ever.
//  2. `nextCursor` is the seq of the LAST ROW READ, never `max(seq)` of the
//     table. max(seq) would skip anything committed during the read.
//
// ═══ REDACTION LIVES HERE AND NOWHERE ELSE ═══
//
// The row allowlist below is the primary control: password_hash is not selected,
// so it cannot be returned even if a later filter is wrong. The delete pass is
// defence in depth. Scattering `if table == "users"` checks through the engine
// is how a hash eventually leaks.

// tableColumns is the ALLOWLIST of columns that may leave the server per table.
//
// Never interpolated from a request: the table name is validated against
// sync_table_scopes and this map before it reaches SQL.
var tableColumns = map[string]string{
	"facilities": "id, name, code, lga, state, address, phone, currency_code, " +
		"currency_minor_units, timezone, is_active, revision, server_rev, sync_state, " +
		"deleted_at, created_at, updated_at, created_by, updated_by",

	// No password_hash, no failed_login_count, no locked_until. A client needs
	// to render "prescribed by Dr X"; it does not need the credential material
	// or the lockout state.
	"users": "id, staff_number, first_name, last_name, middle_name, email, phone, " +
		"is_active, must_change_password, last_login_at, revision, server_rev, sync_state, " +
		"deleted_at, created_at, updated_at, created_by, updated_by",

	"user_roles": "id, user_id, role, granted_at, revoked_at, revoked_reason, " +
		"revision, server_rev, sync_state, deleted_at, created_at, updated_at, created_by, updated_by",

	// public_key is excluded: clients do not need another workstation's key to
	// render anything, and shipping it widening the surface for no benefit.
	"devices": "id, registered_by, name, platform, public_key_alg, fingerprint, " +
		"app_version, last_seen_at, last_sync_cursor, revoked_at, revision, server_rev, " +
		"sync_state, deleted_at, created_at, updated_at, created_by, updated_by",

	"sync_probe": "id, title, body, revision, server_rev, sync_state, deleted_at, " +
		"created_at, updated_at, created_by, updated_by",

	"patients": "id, patient_number, household_id, first_name, middle_name, surname, " +
		"date_of_birth, age_years, age_months, age_recorded_at, gender, marital_status, " +
		"phone, alt_phone, tribe, occupation, address, address_landmark, blood_group, " +
		"genotype, allergies, chronic_conditions, is_deceased, deceased_at, " +
		"registration_date, is_active, revision, server_rev, sync_state, deleted_at, " +
		"created_at, updated_at, created_by, updated_by",

	"households": "id, family_number, family_name, head_patient_id, phone, address, " +
		"notes, is_active, revision, server_rev, sync_state, deleted_at, " +
		"created_at, updated_at, created_by, updated_by",

	"encounters": "id, visit_number, patient_id, household_id, encounter_type, " +
		"workflow_stage, status, priority, chief_complaint, queued_at, started_at, " +
		"closed_at, closed_by, attending_doctor_id, attending_nurse_id, notes, " +
		"revision, server_rev, sync_state, deleted_at, created_at, updated_at, " +
		"created_by, updated_by",

	"lab_results": "id, lab_order_id, patient_id, parameter_name, value_text, " +
		"value_numeric, unit, reference_range, abnormal_flag, result_at, result_by, " +
		"verified_at, verified_by, remarks, revision, server_rev, sync_state, " +
		"deleted_at, created_at, updated_at, created_by, updated_by",

	"lab_orders": "id, patient_id, encounter_id, order_number, test_code, test_name, " +
		"specimen_type, clinical_indication, priority, status, ordered_at, ordered_by, " +
		"collected_at, collected_by, completed_at, revision, server_rev, sync_state, " +
		"deleted_at, created_at, updated_at, created_by, updated_by",

	"diagnoses": "id, patient_id, encounter_id, diagnosis_type, icd10_code, description, " +
		"clinical_notes, diagnosed_at, diagnosed_by, revision, server_rev, sync_state, " +
		"deleted_at, created_at, updated_at, created_by, updated_by",

	"vitals": "id, patient_id, encounter_id, recorded_at, bp_systolic, bp_diastolic, " +
		"temperature_c, pulse_rate, respiratory_rate, height_cm, weight_kg, spo2, bmi, " +
		"pain_score, notes, recorded_by, revision, server_rev, sync_state, deleted_at, " +
		"created_at, updated_at, created_by, updated_by",

	// Billing. Missing from this map means "not_snapshottable", which a NEW
	// device cannot recover from: its bootstrap silently skips the table, then
	// adopts a cursor past every existing row, so the invoices never arrive at
	// all. That exact gap hid the cashier's whole screen for a round.
	"tariffs": "id, code, name, category, unit_price_minor, unit, is_active, " +
		"effective_from, effective_to, revision, server_rev, sync_state, deleted_at, " +
		"created_at, updated_at, created_by, updated_by",

	"invoices": "id, invoice_number, patient_id, household_id, encounter_id, invoice_type, " +
		"status, total_minor, issued_at, issued_by, voided_at, voided_by, void_reason, notes, " +
		"revision, server_rev, sync_state, deleted_at, created_at, updated_at, " +
		"created_by, updated_by",

	"charges": "id, invoice_id, patient_id, household_id, charge_type, tariff_id, description, " +
		"reference_id, reference_type, quantity, unit_price_minor, amount_minor, status, " +
		"posted_at, voided_at, voided_by, void_reason, revision, server_rev, sync_state, " +
		"deleted_at, created_at, updated_at, created_by, updated_by",

	"payments": "id, operation_id, receipt_number, payer_patient_id, household_id, " +
		"amount_minor, currency_code, payment_method, payment_reference, status, paid_at, " +
		"received_by, reversal_of_payment_id, reversed_by_payment_id, reversal_reason, notes, " +
		"revision, server_rev, sync_state, deleted_at, created_at, updated_at, " +
		"created_by, updated_by",

	"payment_allocations": "id, payment_id, charge_id, amount_minor, allocated_at, " +
		"allocated_by, revision, server_rev, sync_state, deleted_at, created_at, " +
		"updated_at, created_by, updated_by",
}

// redactedColumns is defence in depth for columns that must never leave.
var redactedColumns = map[string][]string{
	"users":   {"password_hash", "password_changed_at", "failed_login_count", "locked_until"},
	"devices": {"public_key"},
}

// PullInput is one pull request.
type PullInput struct {
	ActorID uuid.UUID
	Since   int64
	Limit   int
}

const (
	defaultPullLimit = 500
	maxPullLimit     = 2000
	defaultSnapLimit = 500
	maxSnapLimit     = 1000
)

// Pull returns one page of changes since a cursor.
func (s *Sync) Pull(ctx context.Context, in PullInput) (*domain.PullResult, error) {
	actor, err := s.users.FindByID(ctx, s.pool, in.ActorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultPullLimit
	}
	if limit > maxPullLimit {
		limit = maxPullLimit
	}

	now := s.now()

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// A cursor older than what we still retain cannot be caught up. Saying so is
	// the whole point: silently returning "nothing since 88410" would leave the
	// device permanently missing everything before it.
	oldest, err := s.sync.OldestSeq(ctx, tx)
	if err != nil {
		return nil, err
	}
	if in.Since > 0 && oldest > 0 && in.Since < oldest-1 {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &domain.PullResult{
			FullResyncRequired: true,
			Reason:             "cursor_expired",
			OldestAvailableSeq: oldest,
			NextCursor:         in.Since,
			ServerTime:         now.UTC(),
			Rows:               map[string][]map[string]any{},
		}, nil
	}

	changes, err := s.sync.PullChanges(ctx, tx, in.Since, limit)
	if err != nil {
		return nil, err
	}

	result := &domain.PullResult{
		Rows:       map[string][]map[string]any{},
		NextCursor: in.Since,
		HasMore:    len(changes) == limit,
		ServerTime: now.UTC(),
	}
	if len(changes) > 0 {
		result.NextCursor = changes[len(changes)-1].Seq
	}

	// Filter the MANIFEST by role, but advance the cursor past everything read.
	//
	// The cursor is a single sequence shared by all tables. If a change the
	// caller may not read failed to advance it, that client would re-request the
	// same window for ever and never reach anything it IS allowed to see.
	idsByTable := make(map[string][]uuid.UUID)
	for _, c := range changes {
		scope, err := s.sync.Scope(ctx, tx, c.Table)
		if err != nil {
			continue // unscoped table: skip, but the cursor still moves
		}
		if !scope.SyncEnabled || !hasAnyOf(actor.Roles, scope.PullRoles) {
			continue
		}
		if _, known := tableColumns[c.Table]; !known {
			continue
		}

		result.Changes = append(result.Changes, domain.PullChange{
			Seq:       c.Seq,
			Table:     c.Table,
			RowID:     c.RowID.String(),
			Operation: c.Operation,
			ServerRev: c.ServerRev,
		})
		idsByTable[c.Table] = appendUniqueID(idsByTable[c.Table], c.RowID)
	}

	for table, ids := range idsByTable {
		rows, err := s.sync.FetchJSONRows(ctx, tx, table, tableColumns[table], ids)
		if err != nil {
			return nil, err
		}
		result.Rows[table] = redactRows(table, rows)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// SnapshotInput is one bootstrap page request.
type SnapshotInput struct {
	ActorID uuid.UUID
	Table   string
	After   uuid.UUID
	Limit   int
}

// Snapshot returns one keyset page of a table's current state.
//
// `cursorAtStart` is captured BEFORE the page is read. A client stores it only
// after the whole table has been paged through, so changes made during a long
// bootstrap are not skipped.
func (s *Sync) Snapshot(ctx context.Context, in SnapshotInput) (*domain.SnapshotResult, error) {
	actor, err := s.users.FindByID(ctx, s.pool, in.ActorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}

	scope, err := s.sync.Scope(ctx, s.pool, in.Table)
	if err != nil {
		return nil, domain.NewValidation("table", "unknown_table")
	}
	if !scope.SyncEnabled || !hasAnyOf(actor.Roles, scope.PullRoles) {
		return nil, domain.Public(domain.ErrForbidden, "You may not read that table.")
	}

	columns, known := tableColumns[in.Table]
	if !known {
		return nil, domain.NewValidation("table", "not_snapshottable")
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultSnapLimit
	}
	if limit > maxSnapLimit {
		limit = maxSnapLimit
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cursorAtStart, err := s.sync.CurrentSeq(ctx, tx)
	if err != nil {
		return nil, err
	}

	// One extra row to learn whether another page exists, without a COUNT.
	rows, err := s.sync.FetchJSONPage(ctx, tx, in.Table, columns, in.After, limit+1)
	if err != nil {
		return nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &domain.SnapshotResult{
		Table:         in.Table,
		Rows:          redactRows(in.Table, rows),
		HasMore:       hasMore,
		CursorAtStart: cursorAtStart,
	}, nil
}

// redactRows converts snake_case column names to the camelCase wire shape and
// removes anything on the deny list.
func redactRows(table string, rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	banned := redactedColumns[table]

	for _, row := range rows {
		clean := make(map[string]any, len(row))
		for key, value := range row {
			clean[camelKey(key)] = value
		}
		for _, key := range banned {
			delete(clean, camelKey(key))
		}
		out = append(out, clean)
	}
	return out
}

// camelKey turns a snake_case column into the camelCase key clients receive.
func camelKey(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

func appendUniqueID(ids []uuid.UUID, id uuid.UUID) []uuid.UUID {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}
