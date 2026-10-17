package domain

import (
	"time"

	"github.com/google/uuid"
)

// LabResult is one measured parameter.
//
// APPEND-ONLY like vitals: a corrected value is a NEW result, because the
// original measurement is what the doctor acted on. Overwriting it would hide
// that a decision was made on a number that later changed.
type LabResult struct {
	ID         uuid.UUID
	LabOrderID uuid.UUID
	// Denormalised: a result without its patient is never a useful query, and
	// the patient timeline needs it without a join across two tables.
	PatientID uuid.UUID

	ParameterName  string
	ValueText      *string
	ValueNumeric   *float64
	Unit           *string
	ReferenceRange *string
	AbnormalFlag   *string

	ResultAt   time.Time
	ResultBy   uuid.UUID
	VerifiedAt *time.Time
	VerifiedBy *uuid.UUID
	Remarks    *string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}
