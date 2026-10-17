package domain

import (
	"time"

	"github.com/google/uuid"
)

// Admission status.
const (
	AdmissionAdmitted    = "admitted"
	AdmissionDischarged  = "discharged"
	AdmissionTransferred = "transferred"
)

// Admission is one stay in the ward.
type Admission struct {
	ID             uuid.UUID
	PatientID      uuid.UUID
	EncounterID    uuid.UUID
	Ward           string
	BedNumber      *string
	Status         string
	AdmittedAt     time.Time
	AdmittedBy     uuid.UUID
	DischargedAt   *time.Time
	DischargedBy   *uuid.UUID
	DischargeNotes *string
	InvoiceID      *uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// WardCharge is one billable NIGHT.
//
// Identified by (admission_id, charge_date), which is what the unique index
// enforces: a bed-day is the night, not the moment a clerk typed it. That is the
// only thing preventing two devices from billing the same sleep twice.
type WardCharge struct {
	ID             uuid.UUID
	AdmissionID    uuid.UUID
	PatientID      uuid.UUID
	ChargeDate     time.Time // DATE
	Ward           string
	UnitPriceMinor int64
	AmountMinor    int64
	InvoiceID      *uuid.UUID
	ChargeID       *uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}
