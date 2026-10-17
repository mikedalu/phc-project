package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Consultation holds the diagnosis and lab SQL.
type Consultation struct{}

// NewConsultation builds the repository.
func NewConsultation() *Consultation { return &Consultation{} }

const diagnosisColumns = `
    id, patient_id, encounter_id, diagnosis_type, icd10_code,
    description, clinical_notes, diagnosed_at, diagnosed_by,
    created_at, updated_at, created_by, updated_by`

// InsertDiagnosis writes a diagnosis. There is no update method, by design.
func (r *Consultation) InsertDiagnosis(ctx context.Context, q Querier, d *domain.Diagnosis) error {
	const sql = `
        INSERT INTO diagnoses (
            id, patient_id, encounter_id, diagnosis_type, icd10_code,
            description, clinical_notes, diagnosed_at, diagnosed_by,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)`

	if _, err := q.Exec(ctx, sql,
		d.ID, d.PatientID, d.EncounterID, d.Type, d.ICD10Code,
		d.Description, d.ClinicalNotes, d.DiagnosedAt, d.DiagnosedBy,
		d.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert diagnosis: %w", err)
	}
	return nil
}

// DiagnosesForPatient returns a patient's diagnoses, newest first.
func (r *Consultation) DiagnosesForPatient(ctx context.Context, q Querier, patientID uuid.UUID) ([]*domain.Diagnosis, error) {
	const sql = `SELECT` + diagnosisColumns + `
          FROM diagnoses
         WHERE patient_id = $1 AND deleted_at IS NULL
         ORDER BY diagnosed_at DESC`

	rows, err := q.Query(ctx, sql, patientID)
	if err != nil {
		return nil, fmt.Errorf("list diagnoses: %w", err)
	}
	defer rows.Close()

	var out []*domain.Diagnosis
	for rows.Next() {
		var d domain.Diagnosis
		if err := rows.Scan(
			&d.ID, &d.PatientID, &d.EncounterID, &d.Type, &d.ICD10Code,
			&d.Description, &d.ClinicalNotes, &d.DiagnosedAt, &d.DiagnosedBy,
			&d.CreatedAt, &d.UpdatedAt, &d.CreatedBy, &d.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan diagnosis: %w", err)
		}
		out = append(out, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate diagnoses: %w", err)
	}
	return out, nil
}

const labOrderColumns = `
    id, patient_id, encounter_id, order_number, test_code, test_name,
    specimen_type, clinical_indication, priority, status,
    ordered_at, ordered_by, collected_at, collected_by, completed_at,
    created_at, updated_at, created_by, updated_by`

func scanLabOrder(row pgx.Row) (*domain.LabOrder, error) {
	var o domain.LabOrder
	err := row.Scan(
		&o.ID, &o.PatientID, &o.EncounterID, &o.OrderNumber, &o.TestCode, &o.TestName,
		&o.SpecimenType, &o.ClinicalIndication, &o.Priority, &o.Status,
		&o.OrderedAt, &o.OrderedBy, &o.CollectedAt, &o.CollectedBy, &o.CompletedAt,
		&o.CreatedAt, &o.UpdatedAt, &o.CreatedBy, &o.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &o, nil
}

// InsertLabOrder writes a test request.
func (r *Consultation) InsertLabOrder(ctx context.Context, q Querier, o *domain.LabOrder) error {
	const sql = `
        INSERT INTO lab_orders (
            id, patient_id, encounter_id, order_number, test_code, test_name,
            specimen_type, clinical_indication, priority, status,
            ordered_at, ordered_by, created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`

	if _, err := q.Exec(ctx, sql,
		o.ID, o.PatientID, o.EncounterID, o.OrderNumber, o.TestCode, o.TestName,
		o.SpecimenType, o.ClinicalIndication, o.Priority, o.Status,
		o.OrderedAt, o.OrderedBy, o.CreatedBy,
	); err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: order number already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert lab order: %w", err)
	}
	return nil
}

// FindLabOrder loads one order.
func (r *Consultation) FindLabOrder(ctx context.Context, q Querier, id uuid.UUID) (*domain.LabOrder, error) {
	const sql = `SELECT` + labOrderColumns + ` FROM lab_orders WHERE id = $1 AND deleted_at IS NULL`

	o, err := scanLabOrder(q.QueryRow(ctx, sql, id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("find lab order: %w", err)
	}
	return o, nil
}

// LabOrdersForPatient lists a patient's orders, newest first.
func (r *Consultation) LabOrdersForPatient(ctx context.Context, q Querier, patientID uuid.UUID) ([]*domain.LabOrder, error) {
	const sql = `SELECT` + labOrderColumns + `
          FROM lab_orders
         WHERE patient_id = $1 AND deleted_at IS NULL
         ORDER BY ordered_at DESC`

	rows, err := q.Query(ctx, sql, patientID)
	if err != nil {
		return nil, fmt.Errorf("list lab orders: %w", err)
	}
	defer rows.Close()

	var out []*domain.LabOrder
	for rows.Next() {
		o, err := scanLabOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("scan lab order: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lab orders: %w", err)
	}
	return out, nil
}

// MarkLabOrderCollected records that the specimen was taken.
//
// Pinned to status 'ordered': two stations collecting the same specimen is not
// a thing, and the second one gets 0 rows rather than silently rewriting time.
func (r *Consultation) MarkLabOrderCollected(ctx context.Context, q Querier, id, actorID uuid.UUID) (bool, error) {
	const sql = `
        UPDATE lab_orders
           SET status = 'collected', collected_at = now(), collected_by = $2,
               updated_by = $2, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND status = 'ordered' AND deleted_at IS NULL`

	tag, err := q.Exec(ctx, sql, id, actorID)
	if err != nil {
		return false, fmt.Errorf("collect lab order: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
