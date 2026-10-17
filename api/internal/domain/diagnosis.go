package domain

import (
	"time"

	"github.com/google/uuid"
)

// Diagnosis types, matching the CHECK constraint.
const (
	DiagnosisPrimary      = "primary"
	DiagnosisSecondary    = "secondary"
	DiagnosisDifferential = "differential"
	DiagnosisProvisional  = "provisional"
	DiagnosisFinal        = "final"
)

// Diagnosis is one clinical opinion about one encounter.
//
// APPEND-ONLY: a changed mind is a NEW diagnosis, not an edit. The record of
// what the doctor thought first is clinically valuable — and a diagnosis is
// the row a referral and a bill are justified by.
type Diagnosis struct {
	ID          uuid.UUID
	PatientID   uuid.UUID
	EncounterID uuid.UUID

	Type          string
	ICD10Code     *string
	Description   string
	ClinicalNotes *string

	DiagnosedAt time.Time
	DiagnosedBy uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}
