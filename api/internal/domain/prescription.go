package domain

import (
	"time"

	"github.com/google/uuid"
)

// Prescription status, matching the CHECK constraint.
const (
	PrescriptionPendingPayment = "pending_payment"
	PrescriptionPaid           = "paid"
	PrescriptionPartiallyDisp  = "partially_dispensed"
	PrescriptionDispensed      = "dispensed"
	PrescriptionCancelled      = "cancelled"
)

// Prescription is a drug request.
//
// Its status tracks PAYMENT as well as dispensing, because the pharmacy must not
// hand over stock that has not been paid for — the two are one workflow at a PHC
// counter, whatever the idealised separation says.
type Prescription struct {
	ID                 uuid.UUID
	PrescriptionNumber string
	PatientID          uuid.UUID
	EncounterID        uuid.UUID
	PrescriptionType   string
	Status             string
	PrescribedAt       time.Time
	PrescribedBy       uuid.UUID
	InvoiceID          *uuid.UUID
	Notes              *string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// PrescriptionItem is one drug line.
type PrescriptionItem struct {
	ID                 uuid.UUID
	PrescriptionID     uuid.UUID
	DrugName           string
	Dose               string
	Frequency          string
	DurationDays       *int16
	QuantityPrescribed float64
	Route              *string
	Instructions       *string

	ChargeID *uuid.UUID
	// Snapshotted at prescribing time: a price change must not rewrite a bill
	// that has already been paid.
	UnitPriceMinor int64
	AmountMinor    int64

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// Dispense is the fact that stock left the shelf. APPEND-ONLY.
type Dispense struct {
	ID             uuid.UUID
	PrescriptionID uuid.UUID
	PatientID      uuid.UUID
	DispensedAt    time.Time
	DispensedBy    uuid.UUID
	Notes          *string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}
