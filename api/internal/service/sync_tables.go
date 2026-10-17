package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// A column-map driven applier for `lww` tables.
//
// ═══ WHY A COLUMN MAP ═══
//
// A clinical table has twenty to forty columns. Hand-writing an applier per
// table means twenty to forty opportunities to forget a column, mistype a type,
// or diverge from the pull allowlist — and the failure is silent: the field
// simply never syncs. One generic applier plus a declarative map means the
// column list is stated ONCE and every table gets the same LWW semantics.
//
// The SQL identifiers come from these maps, never from a request, so there is no
// injection surface. Values are always parameters.

type fieldKind int

const (
	kindText fieldKind = iota
	kindUUID
	kindDate
	kindInt
	// kindInt64 is for BIGINT money columns. kindInt narrows to int, which is
	// fine for quantities and wrong for money the day a total exceeds 2^31 kobo.
	kindInt64
	kindBool
	kindFloat
	// kindTimestamp is a TIMESTAMPTZ: RFC 3339 with an offset, unlike kindDate
	// which is a DATE and must stay timezone-free.
	kindTimestamp
)

type columnSpec struct {
	Field    string // camelCase name in the payload
	SQL      string // column name
	Kind     fieldKind
	Required bool
}

// lwwApplier implements syncApplier for one table.
type lwwApplier struct {
	table   string
	columns []columnSpec
	// insertOnly are columns written ONLY on insert.
	//
	// The document number lives here. Renumbering a patient on every sync
	// update would rewrite the number printed on their card and on every
	// referral that cites it, so an update must never touch it.
	insertOnly []columnSpec
	// normalize runs before conversion. It exists so a bad enum is a
	// `rejected` operation with a useful reason rather than a 500 from a CHECK
	// constraint — and so the age/DOB rules apply to the SYNC path too, not
	// only to the REST path.
	normalize func(map[string]any) error
	sync      *repo.Sync
	now       func() time.Time
}

func (a *lwwApplier) apply(ctx context.Context, tx pgx.Tx, in applyContext) (applyOutcome, error) {
	now := a.now()

	if in.op.Operation == "delete" {
		return a.applyDelete(ctx, tx, in, now)
	}
	return a.applyUpsert(ctx, tx, in, now)
}

func (a *lwwApplier) applyDelete(ctx context.Context, tx pgx.Tx, in applyContext, now time.Time) (applyOutcome, error) {
	existing, found, err := a.loadCurrent(ctx, tx, in.rowID)
	if err != nil {
		return applyOutcome{}, err
	}
	if !found || existing.deletedAt != nil {
		return applyOutcome{status: domain.OpDuplicate, reason: "already_absent"}, nil
	}

	sql := fmt.Sprintf(`
        UPDATE %s
           SET deleted_at = $2, updated_at = $2, updated_by = $3,
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1
     RETURNING server_rev`, a.table)

	var rev int64
	if err := tx.QueryRow(ctx, sql, in.rowID, now, in.userID).Scan(&rev); err != nil {
		return applyOutcome{}, fmt.Errorf("delete %s: %w", a.table, err)
	}

	if _, err := a.sync.AppendChange(ctx, tx, repo.ChangeEntry{
		Table: a.table, RowID: in.rowID, Operation: "delete",
		ServerRev: rev, ChangedBy: in.userID, DeviceID: &in.deviceID,
		DeviceRecordedAt: in.op.DeviceRecordedAt,
	}); err != nil {
		return applyOutcome{}, err
	}

	return applyOutcome{status: domain.OpApplied, serverRev: rev}, nil
}

func (a *lwwApplier) applyUpsert(ctx context.Context, tx pgx.Tx, in applyContext, now time.Time) (applyOutcome, error) {
	// NORMALISE FIRST, then convert.
	//
	// The normaliser can ADD fields — it derives age from a date of birth and
	// vice versa. Converting before it runs silently wrote nothing for the
	// derived half, which showed up as a patient with no age in the pulled row.
	if a.normalize != nil {
		if err := a.normalize(in.op.Payload); err != nil {
			return applyOutcome{status: domain.OpRejected, reason: "validation:" + err.Error()}, nil
		}
	}

	// Only the columns the payload actually CARRIES.
	//
	// An absent field must leave the database default alone on insert and leave
	// the stored value alone on update. Treating it as NULL is how an update
	// that omits `is_deceased` turns a living patient into a NOT NULL violation —
	// or, worse, clears a nullable field the client never meant to touch.
	// An EXPLICIT null is different, and is still written as NULL.
	used := make([]columnSpec, 0, len(a.columns))
	values := make([]any, 0, len(a.columns))
	for _, col := range a.columns {
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

	clientUpdated := saneClientTime(payloadTime(in.op.Payload, "updatedAt"), now)
	if in.op.DeviceRecordedAt != nil {
		clientUpdated = saneClientTime(in.op.DeviceRecordedAt, clientUpdated)
	}
	clientCreated := saneClientTime(payloadTime(in.op.Payload, "createdAt"), clientUpdated)

	existing, found, err := a.loadCurrent(ctx, tx, in.rowID)
	if err != nil {
		return applyOutcome{}, err
	}

	if !found {
		return a.insert(ctx, tx, in, used, values, clientCreated, clientUpdated)
	}

	// ── the LWW decision ───────────────────────────────────────────
	concurrent := in.op.BaseServerRev == nil || *in.op.BaseServerRev != existing.serverRev

	if concurrent && !clientUpdated.After(existing.updatedAt) {
		conflictID, err := a.recordConflict(ctx, tx, in, existing, "server", nil)
		if err != nil {
			return applyOutcome{}, err
		}
		payload, err := a.fetchPayload(ctx, tx, in.rowID)
		if err != nil {
			return applyOutcome{}, err
		}
		return applyOutcome{
			status:        domain.OpConflictLWW,
			serverRev:     existing.serverRev,
			reason:        "server_copy_newer",
			keptServer:    true,
			conflictID:    conflictID,
			serverPayload: payload,
		}, nil
	}

	rev, err := a.update(ctx, tx, in, used, values, clientUpdated)
	if err != nil {
		return applyOutcome{}, err
	}

	var conflictID string
	if concurrent {
		conflictID, err = a.recordConflict(ctx, tx, in, existing, "client", in.op.Payload)
		if err != nil {
			return applyOutcome{}, err
		}
	}

	if _, err := a.sync.AppendChange(ctx, tx, repo.ChangeEntry{
		Table: a.table, RowID: in.rowID, Operation: "upsert",
		ServerRev: rev, ChangedBy: in.userID, DeviceID: &in.deviceID,
		DeviceRecordedAt: in.op.DeviceRecordedAt,
	}); err != nil {
		return applyOutcome{}, err
	}

	if concurrent {
		return applyOutcome{
			status: domain.OpConflictLWW, serverRev: rev,
			reason: "client_copy_newer", keptServer: false, conflictID: conflictID,
		}, nil
	}
	return applyOutcome{status: domain.OpApplied, serverRev: rev}, nil
}

func (a *lwwApplier) insert(
	ctx context.Context,
	tx pgx.Tx,
	in applyContext,
	used []columnSpec,
	values []any,
	created, updated time.Time,
) (applyOutcome, error) {
	all := append(append([]columnSpec{}, used...), a.insertOnly...)

	names := make([]string, 0, len(all))
	placeholders := make([]string, 0, len(all))
	for i, col := range all {
		names = append(names, col.SQL)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2)) // $1 is id
	}

	next := len(all) + 2
	sql := fmt.Sprintf(`
        INSERT INTO %s (
            id, %s, revision, server_rev, sync_state,
            created_at, updated_at, created_by, updated_by
        ) VALUES ($1, %s, 1, 1, 'synced', $%d, $%d, $%d, $%d)
        RETURNING server_rev`,
		a.table, strings.Join(names, ", "), strings.Join(placeholders, ", "),
		next, next+1, next+2, next+2)

	// Insert-only values are converted here, not with the rest, so a malformed
	// document number is a rejected operation with a clear reason.
	for _, col := range a.insertOnly {
		value, err := convertField(col, in.op.Payload)
		if err != nil {
			return applyOutcome{status: domain.OpRejected, reason: "validation:" + col.Field}, nil
		}
		values = append(values, value)
	}

	args := make([]any, 0, len(values)+5)
	args = append(args, in.rowID)
	args = append(args, values...)
	args = append(args, created, updated, in.userID)

	// A SAVEPOINT, because a failed INSERT poisons the WHOLE transaction in
	// Postgres until it is rolled back. Without this, catching the duplicate
	// number and recording a conflict fails with 25P02 ("current transaction is
	// aborted") — the recovery path itself becomes the error.
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		return applyOutcome{}, fmt.Errorf("savepoint %s: %w", a.table, err)
	}

	var rev int64
	if err := savepoint.QueryRow(ctx, sql, args...).Scan(&rev); err != nil {
		_ = savepoint.Rollback(ctx)

		// A duplicate document number is EXPECTED once devices generate their
		// own: two offline clerks can collide. It must be a rejected operation
		// the client can reassign and retry, never a 500 that wedges the device.
		if db.IsUniqueViolation(err) {
			id, _ := a.recordConflict(ctx, tx, in, currentRow{}, "server", in.op.Payload)
			return applyOutcome{
				status:     domain.OpRejected,
				reason:     "unique_violation",
				conflictID: id,
			}, nil
		}
		return applyOutcome{}, fmt.Errorf("insert %s: %w", a.table, err)
	}

	if err := savepoint.Commit(ctx); err != nil {
		return applyOutcome{}, fmt.Errorf("release savepoint %s: %w", a.table, err)
	}

	if _, err := a.sync.AppendChange(ctx, tx, repo.ChangeEntry{
		Table: a.table, RowID: in.rowID, Operation: "upsert",
		ServerRev: rev, ChangedBy: in.userID, DeviceID: &in.deviceID,
		DeviceRecordedAt: in.op.DeviceRecordedAt,
	}); err != nil {
		return applyOutcome{}, err
	}

	return applyOutcome{status: domain.OpApplied, serverRev: rev}, nil
}

func (a *lwwApplier) update(
	ctx context.Context,
	tx pgx.Tx,
	in applyContext,
	used []columnSpec,
	values []any,
	updated time.Time,
) (int64, error) {
	sets := make([]string, 0, len(used))
	for i, col := range used {
		sets = append(sets, fmt.Sprintf("%s = $%d", col.SQL, i+2))
	}
	if len(sets) == 0 {
		// Nothing to change but the metadata; still bump the revision so the
		// change replicates and the client's outbox drains.
		sets = append(sets, "revision = revision")
	}
	next := len(used) + 2

	sql := fmt.Sprintf(`
        UPDATE %s
           SET %s,
               updated_at = $%d, updated_by = $%d,
               revision = revision + 1, server_rev = server_rev + 1, deleted_at = NULL
         WHERE id = $1
     RETURNING server_rev`,
		a.table, strings.Join(sets, ", "), next, next+1)

	args := make([]any, 0, len(values)+3)
	args = append(args, in.rowID)
	args = append(args, values...)
	args = append(args, updated, in.userID)

	var rev int64
	if err := tx.QueryRow(ctx, sql, args...).Scan(&rev); err != nil {
		return 0, fmt.Errorf("update %s: %w", a.table, err)
	}
	return rev, nil
}

type currentRow struct {
	serverRev int64
	updatedAt time.Time
	deletedAt *time.Time
}

func (a *lwwApplier) loadCurrent(ctx context.Context, tx pgx.Tx, id uuid.UUID) (currentRow, bool, error) {
	sql := fmt.Sprintf(`SELECT server_rev, updated_at, deleted_at FROM %s WHERE id = $1`, a.table)
	var row currentRow
	err := tx.QueryRow(ctx, sql, id).Scan(&row.serverRev, &row.updatedAt, &row.deletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return currentRow{}, false, nil
		}
		return currentRow{}, false, err
	}
	return row, true, nil
}

// fetchPayload returns the server's stored row in the camelCase wire shape, so
// a client that lost the LWW race can replace its local copy immediately.
func (a *lwwApplier) fetchPayload(ctx context.Context, tx pgx.Tx, id uuid.UUID) (map[string]any, error) {
	columns, ok := tableColumns[a.table]
	if !ok {
		return nil, nil
	}

	rows, err := a.sync.FetchJSONRows(ctx, tx, a.table, columns, []uuid.UUID{id})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	clean := redactRows(a.table, rows)
	return clean[0], nil
}

func (a *lwwApplier) recordConflict(
	ctx context.Context,
	tx pgx.Tx,
	in applyContext,
	existing currentRow,
	winner string,
	clientPayload map[string]any,
) (string, error) {
	serverPayload := map[string]any{"id": in.rowID.String(), "serverRev": existing.serverRev}

	winning, losing := clientPayload, serverPayload
	if winner == "server" {
		winning, losing = serverPayload, clientPayload
	}
	if winning == nil {
		winning = serverPayload
	}

	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	if err := a.sync.InsertConflict(ctx, tx, repo.ConflictRecord{
		ID:             id,
		Table:          a.table,
		RowID:          in.rowID,
		DeviceID:       in.deviceID,
		UserID:         in.userID,
		ConflictType:   "lww_overwrite",
		ClientRevision: in.op.BaseServerRev,
		ServerRevision: &existing.serverRev,
		WinningPayload: winning,
		LosingPayload:  losing,
	}); err != nil {
		return "", err
	}
	return id.String(), nil
}

// convertField turns one JSON value into the Go value pgx should send.
func convertField(col columnSpec, payload map[string]any) (any, error) {
	raw, present := payload[col.Field]
	if !present || raw == nil {
		if col.Required {
			return nil, fmt.Errorf("%s is required", col.Field)
		}
		return nil, nil
	}

	switch col.Kind {
	case kindText:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be a string", col.Field)
		}
		s = strings.TrimSpace(s)
		if s == "" {
			if col.Required {
				return nil, fmt.Errorf("%s must not be empty", col.Field)
			}
			return nil, nil
		}
		return s, nil

	case kindUUID:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be a uuid", col.Field)
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("%s must be a uuid", col.Field)
		}
		return id, nil

	case kindDate:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be YYYY-MM-DD", col.Field)
		}
		parsed, err := time.Parse("2006-01-02", s)
		if err != nil {
			// A DATE that arrives as an ISO instant (…T00:00:00Z) is a client
			// bug, but taking the date part is kinder than rejecting a record.
			if len(s) >= 10 {
				if p2, err2 := time.Parse("2006-01-02", s[:10]); err2 == nil {
					return p2, nil
				}
			}
			return nil, fmt.Errorf("%s must be YYYY-MM-DD", col.Field)
		}
		return parsed, nil

	case kindInt:
		switch v := raw.(type) {
		case float64:
			return int16(v), nil
		case int:
			return int16(v), nil
		default:
			return nil, fmt.Errorf("%s must be a number", col.Field)
		}

	case kindInt64:
		switch v := raw.(type) {
		case float64:
			return int64(v), nil
		case int:
			return int64(v), nil
		case int64:
			return v, nil
		default:
			return nil, fmt.Errorf("%s must be an integer", col.Field)
		}

	case kindBool:
		b, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be a boolean", col.Field)
		}
		return b, nil

	case kindFloat:
		switch v := raw.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		default:
			return nil, fmt.Errorf("%s must be a number", col.Field)
		}

	case kindTimestamp:
		s, ok := raw.(string)
		if !ok || s == "" {
			return nil, fmt.Errorf("%s must be a timestamp", col.Field)
		}
		parsed, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, fmt.Errorf("%s must be an RFC 3339 timestamp", col.Field)
		}
		return parsed, nil
	}

	return nil, fmt.Errorf("unsupported field %s", col.Field)
}

// patientColumnsForSync is the declarative map for `patients`.
//
// NOTE: `patient_number` is deliberately NOT here. It is assigned at
// registration and must not be changed by a sync update — renumbering a patient
// would rewrite every printed card and referral.
var patientInsertOnly = []columnSpec{
	{Field: "patientNumber", SQL: "patient_number", Kind: kindText, Required: true},
}

var patientColumnsForSync = []columnSpec{
	{Field: "householdId", SQL: "household_id", Kind: kindUUID},
	{Field: "firstName", SQL: "first_name", Kind: kindText, Required: true},
	{Field: "middleName", SQL: "middle_name", Kind: kindText},
	{Field: "surname", SQL: "surname", Kind: kindText, Required: true},
	{Field: "dateOfBirth", SQL: "date_of_birth", Kind: kindDate},
	{Field: "ageYears", SQL: "age_years", Kind: kindInt},
	{Field: "ageMonths", SQL: "age_months", Kind: kindInt},
	{Field: "ageRecordedAt", SQL: "age_recorded_at", Kind: kindDate},
	{Field: "gender", SQL: "gender", Kind: kindText, Required: true},
	{Field: "maritalStatus", SQL: "marital_status", Kind: kindText},
	{Field: "phone", SQL: "phone", Kind: kindText},
	{Field: "altPhone", SQL: "alt_phone", Kind: kindText},
	{Field: "tribe", SQL: "tribe", Kind: kindText},
	{Field: "occupation", SQL: "occupation", Kind: kindText},
	{Field: "address", SQL: "address", Kind: kindText},
	{Field: "addressLandmark", SQL: "address_landmark", Kind: kindText},
	{Field: "bloodGroup", SQL: "blood_group", Kind: kindText},
	{Field: "genotype", SQL: "genotype", Kind: kindText},
	{Field: "allergies", SQL: "allergies", Kind: kindText},
	{Field: "chronicConditions", SQL: "chronic_conditions", Kind: kindText},
	{Field: "isDeceased", SQL: "is_deceased", Kind: kindBool},
	{Field: "deceasedAt", SQL: "deceased_at", Kind: kindDate},
	{Field: "registrationDate", SQL: "registration_date", Kind: kindDate, Required: true},
	{Field: "isActive", SQL: "is_active", Kind: kindBool},
}

var householdInsertOnly = []columnSpec{
	{Field: "familyNumber", SQL: "family_number", Kind: kindText, Required: true},
}

// encounterColumnsForSync is the declarative map for `encounters`.
//
// `visitNumber` is insert-only: renumbering a visit would rewrite the number on
// any printed ticket and break the unique index for the next device that reuses
// the sequence.
var encounterInsertOnly = []columnSpec{
	{Field: "visitNumber", SQL: "visit_number", Kind: kindText, Required: true},
}

var encounterColumnsForSync = []columnSpec{
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "householdId", SQL: "household_id", Kind: kindUUID},
	{Field: "encounterType", SQL: "encounter_type", Kind: kindText},
	{Field: "workflowStage", SQL: "workflow_stage", Kind: kindText},
	{Field: "status", SQL: "status", Kind: kindText},
	{Field: "priority", SQL: "priority", Kind: kindInt},
	{Field: "chiefComplaint", SQL: "chief_complaint", Kind: kindText},
	{Field: "queuedAt", SQL: "queued_at", Kind: kindTimestamp},
	{Field: "startedAt", SQL: "started_at", Kind: kindTimestamp},
	{Field: "closedAt", SQL: "closed_at", Kind: kindTimestamp},
	{Field: "closedBy", SQL: "closed_by", Kind: kindUUID},
	{Field: "attendingDoctorId", SQL: "attending_doctor_id", Kind: kindUUID},
	{Field: "attendingNurseId", SQL: "attending_nurse_id", Kind: kindUUID},
	{Field: "notes", SQL: "notes", Kind: kindText},
}

// vitalsColumnsForSync: every clinical field, no update path.
var vitalsColumnsForSync = []columnSpec{
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "encounterId", SQL: "encounter_id", Kind: kindUUID},
	{Field: "recordedAt", SQL: "recorded_at", Kind: kindTimestamp},
	{Field: "bpSystolic", SQL: "bp_systolic", Kind: kindInt},
	{Field: "bpDiastolic", SQL: "bp_diastolic", Kind: kindInt},
	{Field: "temperatureC", SQL: "temperature_c", Kind: kindFloat},
	{Field: "pulseRate", SQL: "pulse_rate", Kind: kindInt},
	{Field: "respiratoryRate", SQL: "respiratory_rate", Kind: kindInt},
	{Field: "heightCm", SQL: "height_cm", Kind: kindFloat},
	{Field: "weightKg", SQL: "weight_kg", Kind: kindFloat},
	{Field: "spo2", SQL: "spo2", Kind: kindInt},
	{Field: "bmi", SQL: "bmi", Kind: kindFloat},
	{Field: "painScore", SQL: "pain_score", Kind: kindInt},
	{Field: "notes", SQL: "notes", Kind: kindText},
	{Field: "recordedBy", SQL: "recorded_by", Kind: kindUUID},
}

// labResultColumnsForSync: append-only measurements.
var labResultColumnsForSync = []columnSpec{
	{Field: "labOrderId", SQL: "lab_order_id", Kind: kindUUID, Required: true},
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "parameterName", SQL: "parameter_name", Kind: kindText, Required: true},
	{Field: "valueText", SQL: "value_text", Kind: kindText},
	{Field: "valueNumeric", SQL: "value_numeric", Kind: kindFloat},
	{Field: "unit", SQL: "unit", Kind: kindText},
	{Field: "referenceRange", SQL: "reference_range", Kind: kindText},
	{Field: "abnormalFlag", SQL: "abnormal_flag", Kind: kindText},
	{Field: "resultAt", SQL: "result_at", Kind: kindTimestamp},
	{Field: "resultBy", SQL: "result_by", Kind: kindUUID},
	{Field: "remarks", SQL: "remarks", Kind: kindText},
}

// labOrderColumnsForSync is LWW, because STATUS advances (ordered -> collected
// -> resulted) and that is an edit to the row, not a new fact.
var labOrderInsertOnly = []columnSpec{
	{Field: "orderNumber", SQL: "order_number", Kind: kindText, Required: true},
}

var labOrderColumnsForSync = []columnSpec{
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "encounterId", SQL: "encounter_id", Kind: kindUUID, Required: true},
	{Field: "testCode", SQL: "test_code", Kind: kindText, Required: true},
	{Field: "testName", SQL: "test_name", Kind: kindText, Required: true},
	{Field: "specimenType", SQL: "specimen_type", Kind: kindText},
	{Field: "clinicalIndication", SQL: "clinical_indication", Kind: kindText},
	{Field: "priority", SQL: "priority", Kind: kindText},
	{Field: "status", SQL: "status", Kind: kindText},
	{Field: "orderedAt", SQL: "ordered_at", Kind: kindTimestamp},
	{Field: "orderedBy", SQL: "ordered_by", Kind: kindUUID},
	{Field: "collectedAt", SQL: "collected_at", Kind: kindTimestamp},
	{Field: "collectedBy", SQL: "collected_by", Kind: kindUUID},
	{Field: "completedAt", SQL: "completed_at", Kind: kindTimestamp},
}

// diagnosisColumnsForSync: no update path, like every other clinical fact.
var diagnosisColumnsForSync = []columnSpec{
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "encounterId", SQL: "encounter_id", Kind: kindUUID, Required: true},
	{Field: "diagnosisType", SQL: "diagnosis_type", Kind: kindText},
	{Field: "icd10Code", SQL: "icd10_code", Kind: kindText},
	{Field: "description", SQL: "description", Kind: kindText, Required: true},
	{Field: "clinicalNotes", SQL: "clinical_notes", Kind: kindText},
	{Field: "diagnosedAt", SQL: "diagnosed_at", Kind: kindTimestamp},
	{Field: "diagnosedBy", SQL: "diagnosed_by", Kind: kindUUID},
}

// ─── billing ───────────────────────────────────────────────────────

var tariffColumnsForSync = []columnSpec{
	{Field: "code", SQL: "code", Kind: kindText, Required: true},
	{Field: "name", SQL: "name", Kind: kindText, Required: true},
	{Field: "category", SQL: "category", Kind: kindText},
	{Field: "unitPriceMinor", SQL: "unit_price_minor", Kind: kindInt64},
	{Field: "unit", SQL: "unit", Kind: kindText},
	{Field: "isActive", SQL: "is_active", Kind: kindBool},
	{Field: "effectiveFrom", SQL: "effective_from", Kind: kindDate},
	{Field: "effectiveTo", SQL: "effective_to", Kind: kindDate},
}

var invoiceInsertOnly = []columnSpec{
	// An invoice number is printed on a receipt; renumbering rewrites history.
	{Field: "invoiceNumber", SQL: "invoice_number", Kind: kindText, Required: true},
}

var invoiceColumnsForSync = []columnSpec{
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "householdId", SQL: "household_id", Kind: kindUUID},
	{Field: "encounterId", SQL: "encounter_id", Kind: kindUUID},
	{Field: "invoiceType", SQL: "invoice_type", Kind: kindText},
	{Field: "status", SQL: "status", Kind: kindText},
	{Field: "totalMinor", SQL: "total_minor", Kind: kindInt64},
	{Field: "issuedAt", SQL: "issued_at", Kind: kindTimestamp},
	{Field: "issuedBy", SQL: "issued_by", Kind: kindUUID},
	{Field: "voidedAt", SQL: "voided_at", Kind: kindTimestamp},
	{Field: "voidedBy", SQL: "voided_by", Kind: kindUUID},
	{Field: "voidReason", SQL: "void_reason", Kind: kindText},
	{Field: "notes", SQL: "notes", Kind: kindText},
}

var chargeColumnsForSync = []columnSpec{
	{Field: "invoiceId", SQL: "invoice_id", Kind: kindUUID, Required: true},
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "householdId", SQL: "household_id", Kind: kindUUID},
	{Field: "chargeType", SQL: "charge_type", Kind: kindText},
	{Field: "tariffId", SQL: "tariff_id", Kind: kindUUID},
	{Field: "description", SQL: "description", Kind: kindText, Required: true},
	{Field: "referenceId", SQL: "reference_id", Kind: kindUUID},
	{Field: "referenceType", SQL: "reference_type", Kind: kindText},
	{Field: "quantity", SQL: "quantity", Kind: kindFloat},
	{Field: "unitPriceMinor", SQL: "unit_price_minor", Kind: kindInt64},
	{Field: "amountMinor", SQL: "amount_minor", Kind: kindInt64},
	{Field: "status", SQL: "status", Kind: kindText},
	{Field: "postedAt", SQL: "posted_at", Kind: kindTimestamp},
}

var paymentInsertOnly = []columnSpec{
	// The idempotency key and the receipt number must never change on an update.
	{Field: "operationId", SQL: "operation_id", Kind: kindUUID, Required: true},
	{Field: "receiptNumber", SQL: "receipt_number", Kind: kindText, Required: true},
}

var paymentColumnsForSync = []columnSpec{
	{Field: "payerPatientId", SQL: "payer_patient_id", Kind: kindUUID},
	{Field: "householdId", SQL: "household_id", Kind: kindUUID},
	{Field: "amountMinor", SQL: "amount_minor", Kind: kindInt64, Required: true},
	{Field: "currencyCode", SQL: "currency_code", Kind: kindText},
	{Field: "paymentMethod", SQL: "payment_method", Kind: kindText},
	{Field: "paymentReference", SQL: "payment_reference", Kind: kindText},
	{Field: "status", SQL: "status", Kind: kindText},
	{Field: "paidAt", SQL: "paid_at", Kind: kindTimestamp},
	{Field: "receivedBy", SQL: "received_by", Kind: kindUUID},
	{Field: "reversalOfPaymentId", SQL: "reversal_of_payment_id", Kind: kindUUID},
	{Field: "notes", SQL: "notes", Kind: kindText},
}

var allocationColumnsForSync = []columnSpec{
	{Field: "paymentId", SQL: "payment_id", Kind: kindUUID, Required: true},
	{Field: "chargeId", SQL: "charge_id", Kind: kindUUID, Required: true},
	{Field: "amountMinor", SQL: "amount_minor", Kind: kindInt64, Required: true},
	{Field: "allocatedAt", SQL: "allocated_at", Kind: kindTimestamp},
	{Field: "allocatedBy", SQL: "allocated_by", Kind: kindUUID},
}

// ─── prescriptions ─────────────────────────────────────────────────

var prescriptionInsertOnly = []columnSpec{
	{Field: "prescriptionNumber", SQL: "prescription_number", Kind: kindText, Required: true},
}

var prescriptionColumnsForSync = []columnSpec{
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "encounterId", SQL: "encounter_id", Kind: kindUUID, Required: true},
	{Field: "prescriptionType", SQL: "prescription_type", Kind: kindText},
	{Field: "status", SQL: "status", Kind: kindText},
	{Field: "prescribedAt", SQL: "prescribed_at", Kind: kindTimestamp},
	{Field: "prescribedBy", SQL: "prescribed_by", Kind: kindUUID},
	{Field: "invoiceId", SQL: "invoice_id", Kind: kindUUID},
	{Field: "notes", SQL: "notes", Kind: kindText},
}

var prescriptionItemColumnsForSync = []columnSpec{
	{Field: "prescriptionId", SQL: "prescription_id", Kind: kindUUID, Required: true},
	{Field: "drugName", SQL: "drug_name", Kind: kindText, Required: true},
	{Field: "dose", SQL: "dose", Kind: kindText},
	{Field: "frequency", SQL: "frequency", Kind: kindText},
	{Field: "durationDays", SQL: "duration_days", Kind: kindInt},
	{Field: "quantityPrescribed", SQL: "quantity_prescribed", Kind: kindFloat},
	{Field: "route", SQL: "route", Kind: kindText},
	{Field: "instructions", SQL: "instructions", Kind: kindText},
	{Field: "chargeId", SQL: "charge_id", Kind: kindUUID},
	{Field: "unitPriceMinor", SQL: "unit_price_minor", Kind: kindInt64},
	{Field: "amountMinor", SQL: "amount_minor", Kind: kindInt64},
}

var dispenseColumnsForSync = []columnSpec{
	{Field: "prescriptionId", SQL: "prescription_id", Kind: kindUUID, Required: true},
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "dispensedAt", SQL: "dispensed_at", Kind: kindTimestamp},
	{Field: "dispensedBy", SQL: "dispensed_by", Kind: kindUUID},
	{Field: "notes", SQL: "notes", Kind: kindText},
}

// ─── ward ──────────────────────────────────────────────────────────

var admissionColumnsForSync = []columnSpec{
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "encounterId", SQL: "encounter_id", Kind: kindUUID, Required: true},
	{Field: "ward", SQL: "ward", Kind: kindText, Required: true},
	{Field: "bedNumber", SQL: "bed_number", Kind: kindText},
	{Field: "status", SQL: "status", Kind: kindText},
	{Field: "admittedAt", SQL: "admitted_at", Kind: kindTimestamp},
	{Field: "admittedBy", SQL: "admitted_by", Kind: kindUUID},
	{Field: "dischargedAt", SQL: "discharged_at", Kind: kindTimestamp},
	{Field: "dischargedBy", SQL: "discharged_by", Kind: kindUUID},
	{Field: "dischargeNotes", SQL: "discharge_notes", Kind: kindText},
	{Field: "invoiceId", SQL: "invoice_id", Kind: kindUUID},
}

var wardChargeColumnsForSync = []columnSpec{
	{Field: "admissionId", SQL: "admission_id", Kind: kindUUID, Required: true},
	{Field: "patientId", SQL: "patient_id", Kind: kindUUID, Required: true},
	{Field: "chargeDate", SQL: "charge_date", Kind: kindDate, Required: true},
	{Field: "ward", SQL: "ward", Kind: kindText},
	{Field: "unitPriceMinor", SQL: "unit_price_minor", Kind: kindInt64},
	{Field: "amountMinor", SQL: "amount_minor", Kind: kindInt64},
	{Field: "invoiceId", SQL: "invoice_id", Kind: kindUUID},
	{Field: "chargeId", SQL: "charge_id", Kind: kindUUID},
}

var householdColumnsForSync = []columnSpec{
	{Field: "familyName", SQL: "family_name", Kind: kindText, Required: true},
	{Field: "headPatientId", SQL: "head_patient_id", Kind: kindUUID},
	{Field: "phone", SQL: "phone", Kind: kindText},
	{Field: "address", SQL: "address", Kind: kindText},
	{Field: "notes", SQL: "notes", Kind: kindText},
	{Field: "isActive", SQL: "is_active", Kind: kindBool},
}

// normalizePatientPayload enforces the registry rules on the SYNC path.
//
// The REST path derives age/DOB in the service; an offline device sends a row
// built by its own copy of the same rules. Re-deriving here means a client bug
// cannot store a patient with no age at all — which would make dosing on a later
// visit guess.
func normalizePatientPayload(payload map[string]any) error {
	gender, _ := payload["gender"].(string)
	switch gender {
	case "male", "female", "other", "unknown":
	default:
		return errors.New("gender")
	}

	if marital, ok := payload["maritalStatus"].(string); ok && marital != "" {
		switch marital {
		case "single", "married", "divorced", "widowed", "separated", "unknown":
		default:
			return errors.New("maritalStatus")
		}
	}

	dobRaw, _ := payload["dateOfBirth"].(string)
	years, hasYears := payload["ageYears"].(float64)
	months, hasMonths := payload["ageMonths"].(float64)
	hasDOB := dobRaw != ""

	if !hasDOB && !hasYears && !hasMonths {
		return errors.New("dateOfBirth")
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if hasDOB && !hasYears {
		parsed, err := time.Parse("2006-01-02", dobRaw)
		if err != nil {
			return errors.New("dateOfBirth")
		}
		if parsed.After(today) {
			return errors.New("dateOfBirth")
		}
		total := int(today.Sub(parsed).Hours() / (24 * 30.4375))
		if total < 0 {
			total = 0
		}
		payload["ageYears"] = float64(total / 12)
		payload["ageMonths"] = float64(total)
		if _, ok := payload["ageRecordedAt"]; !ok {
			payload["ageRecordedAt"] = today.Format("2006-01-02")
		}
	}

	if !hasDOB {
		total := int(years)*12 + int(months)
		if total < 0 || total > 130*12 {
			return errors.New("ageYears")
		}
		payload["dateOfBirth"] = today.AddDate(0, -total, 0).Format("2006-01-02")
		if _, ok := payload["ageRecordedAt"]; !ok {
			payload["ageRecordedAt"] = today.Format("2006-01-02")
		}
	}

	return nil
}
