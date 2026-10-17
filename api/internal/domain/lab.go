package domain

import (
	"time"

	"github.com/google/uuid"
)

// Lab order status. Clinical status only — whether the test is PAID is derived
// from the invoice, never stored here, because two sources of truth for money
// will disagree within a week.
const (
	LabOrdered   = "ordered"
	LabCollected = "collected"
	LabResulted  = "resulted"
	LabCancelled = "cancelled"
)

// LabOrder is a request for a test.
//
// `test_name` is SNAPSHOTTED from the tariff at order time: a later rename or
// price change must not rewrite what the doctor asked for or what the patient
// was charged for.
type LabOrder struct {
	ID          uuid.UUID
	PatientID   uuid.UUID
	EncounterID uuid.UUID
	OrderNumber string

	TestCode     string
	TestName     string
	SpecimenType *string

	ClinicalIndication *string
	Priority           string
	Status             string

	OrderedAt   time.Time
	OrderedBy   uuid.UUID
	CollectedAt *time.Time
	CollectedBy *uuid.UUID
	CompletedAt *time.Time

	// InvoiceID and ChargeID link the test to its bill. Both are set in the same
	// transaction that creates the order when a tariff covers it.
	InvoiceID *uuid.UUID
	ChargeID  *uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}
