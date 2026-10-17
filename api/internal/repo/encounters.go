package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Encounters holds the workflow and observation SQL.
type Encounters struct{}

// NewEncounters builds the repository.
func NewEncounters() *Encounters { return &Encounters{} }

const encounterColumns = `
    id, visit_number, patient_id, household_id,
    encounter_type, workflow_stage, status, priority,
    chief_complaint, queued_at, started_at, closed_at, closed_by,
    attending_doctor_id, attending_nurse_id, notes,
    created_at, updated_at, created_by, updated_by`

func scanEncounter(row pgx.Row) (*domain.Encounter, error) {
	var e domain.Encounter
	err := row.Scan(
		&e.ID, &e.VisitNumber, &e.PatientID, &e.HouseholdID,
		&e.EncounterType, &e.WorkflowStage, &e.Status, &e.Priority,
		&e.ChiefComplaint, &e.QueuedAt, &e.StartedAt, &e.ClosedAt, &e.ClosedBy,
		&e.AttendingDoctorID, &e.AttendingNurseID, &e.Notes,
		&e.CreatedAt, &e.UpdatedAt, &e.CreatedBy, &e.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

// InsertEncounter writes a new visit.
func (r *Encounters) InsertEncounter(ctx context.Context, q Querier, e *domain.Encounter) error {
	const sql = `
        INSERT INTO encounters (
            id, visit_number, patient_id, household_id,
            encounter_type, workflow_stage, status, priority,
            chief_complaint, queued_at,
            attending_doctor_id, attending_nurse_id, notes,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14)`

	if _, err := q.Exec(ctx, sql,
		e.ID, e.VisitNumber, e.PatientID, e.HouseholdID,
		e.EncounterType, e.WorkflowStage, e.Status, e.Priority,
		e.ChiefComplaint, e.QueuedAt,
		e.AttendingDoctorID, e.AttendingNurseID, e.Notes,
		e.CreatedBy,
	); err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: visit number already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert encounter: %w", err)
	}
	return nil
}

// FindEncounterByID loads one encounter.
func (r *Encounters) FindEncounterByID(ctx context.Context, q Querier, id uuid.UUID) (*domain.Encounter, error) {
	const sql = `SELECT` + encounterColumns + ` FROM encounters WHERE id = $1 AND deleted_at IS NULL`

	e, err := scanEncounter(q.QueryRow(ctx, sql, id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("find encounter: %w", err)
	}
	return e, nil
}

// FindActiveEncounter returns the patient's open visit, if any.
func (r *Encounters) FindActiveEncounter(ctx context.Context, q Querier, patientID uuid.UUID) (*domain.Encounter, error) {
	const sql = `SELECT` + encounterColumns + `
          FROM encounters
         WHERE patient_id = $1 AND status = 'active' AND deleted_at IS NULL
         ORDER BY queued_at DESC
         LIMIT 1`

	e, err := scanEncounter(q.QueryRow(ctx, sql, patientID))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("find active encounter: %w", err)
	}
	return e, nil
}

// Queue lists active encounters, emergencies first then oldest.
func (r *Encounters) Queue(ctx context.Context, q Querier, limit int) ([]*domain.Encounter, error) {
	const sql = `SELECT` + encounterColumns + `
          FROM encounters
         WHERE status = 'active' AND deleted_at IS NULL
         ORDER BY priority DESC, queued_at
         LIMIT $1`

	rows, err := q.Query(ctx, sql, limit)
	if err != nil {
		return nil, fmt.Errorf("list queue: %w", err)
	}
	defer rows.Close()

	var out []*domain.Encounter
	for rows.Next() {
		e, err := scanEncounter(rows)
		if err != nil {
			return nil, fmt.Errorf("scan encounter: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue: %w", err)
	}
	return out, nil
}

// Transition moves an encounter to a new stage.
//
// The WHERE clause pins the CURRENT stage: two devices that both saw
// `awaiting_doctor` cannot both move it. The loser updates 0 rows and is told.
func (r *Encounters) Transition(
	ctx context.Context,
	q Querier,
	id uuid.UUID,
	from, to string,
	actorID uuid.UUID,
	closed bool,
) (bool, error) {
	// NOTE: five parameters, each used. A placeholder that appears in the SQL but
	// not in the args (or vice versa) fails in Exec mode with "could not determine
	// data type of parameter $n", because there is no prepared statement to infer
	// the type from.
	const sql = `
        UPDATE encounters
           SET workflow_stage = $4,
               status = CASE WHEN $5 THEN 'closed' ELSE status END,
               closed_at = CASE WHEN $5 THEN now() ELSE closed_at END,
               closed_by = CASE WHEN $5 THEN $3 ELSE closed_by END,
               started_at = CASE WHEN $4 = 'in_consultation' THEN now() ELSE started_at END,
               updated_by = $3, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND workflow_stage = $2 AND deleted_at IS NULL`

	tag, err := q.Exec(ctx, sql, id, from, actorID, to, closed)
	if err != nil {
		return false, fmt.Errorf("transition encounter: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

const vitalsColumns = `
    id, patient_id, encounter_id, recorded_at,
    bp_systolic, bp_diastolic, temperature_c, pulse_rate, respiratory_rate,
    height_cm, weight_kg, spo2, bmi, pain_score, notes, recorded_by,
    created_at, updated_at, created_by, updated_by`

// InsertVitals records an observation. There is deliberately NO update method:
// a second reading is a new row.
func (r *Encounters) InsertVitals(ctx context.Context, q Querier, v *domain.Vitals) error {
	const sql = `
        INSERT INTO vitals (
            id, patient_id, encounter_id, recorded_at,
            bp_systolic, bp_diastolic, temperature_c, pulse_rate, respiratory_rate,
            height_cm, weight_kg, spo2, bmi, pain_score, notes, recorded_by,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$16,$16)`

	if _, err := q.Exec(ctx, sql,
		v.ID, v.PatientID, v.EncounterID, v.RecordedAt,
		v.BPSystolic, v.BPDiastolic, v.TemperatureC, v.PulseRate, v.RespiratoryRate,
		v.HeightCm, v.WeightKg, v.SpO2, v.BMI, v.PainScore, v.Notes, v.RecordedBy,
	); err != nil {
		return fmt.Errorf("insert vitals: %w", err)
	}
	return nil
}

// VitalsForPatient returns readings newest first.
func (r *Encounters) VitalsForPatient(ctx context.Context, q Querier, patientID uuid.UUID, limit int) ([]*domain.Vitals, error) {
	const sql = `SELECT` + vitalsColumns + `
          FROM vitals
         WHERE patient_id = $1 AND deleted_at IS NULL
         ORDER BY recorded_at DESC
         LIMIT $2`

	rows, err := q.Query(ctx, sql, patientID, limit)
	if err != nil {
		return nil, fmt.Errorf("list vitals: %w", err)
	}
	defer rows.Close()

	var out []*domain.Vitals
	for rows.Next() {
		var v domain.Vitals
		if err := rows.Scan(
			&v.ID, &v.PatientID, &v.EncounterID, &v.RecordedAt,
			&v.BPSystolic, &v.BPDiastolic, &v.TemperatureC, &v.PulseRate, &v.RespiratoryRate,
			&v.HeightCm, &v.WeightKg, &v.SpO2, &v.BMI, &v.PainScore, &v.Notes, &v.RecordedBy,
			&v.CreatedAt, &v.UpdatedAt, &v.CreatedBy, &v.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan vitals: %w", err)
		}
		out = append(out, &v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate vitals: %w", err)
	}
	return out, nil
}
