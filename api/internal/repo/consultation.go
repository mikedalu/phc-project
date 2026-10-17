package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

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
