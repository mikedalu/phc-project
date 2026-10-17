package domain

import (
	"time"

	"github.com/google/uuid"
)

// Invoice status, matching the CHECK constraint.
const (
	InvoiceUnpaid        = "unpaid"
	InvoicePartiallyPaid = "partially_paid"
	InvoicePaid          = "paid"
	InvoiceVoid          = "void"
)

// Payment methods.
const (
	PaymentCash     = "cash"
	PaymentTransfer = "transfer"
	PaymentPOSCard  = "pos_card"
	PaymentWaiver   = "waiver"
)

// Invoice is the bill.
type Invoice struct {
	ID            uuid.UUID
	InvoiceNumber string
	PatientID     uuid.UUID
	HouseholdID   *uuid.UUID
	EncounterID   *uuid.UUID
	InvoiceType   string
	Status        string
	// TotalMinor is the sum of its charges, written in the SAME transaction as
	// them. It is NOT a balance: payments do not change it.
	TotalMinor int64
	IssuedAt   time.Time
	IssuedBy   uuid.UUID
	VoidedAt   *time.Time
	VoidedBy   *uuid.UUID
	VoidReason *string
	Notes      *string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// Charge is one line of a bill.
type Charge struct {
	ID          uuid.UUID
	InvoiceID   uuid.UUID
	PatientID   uuid.UUID
	HouseholdID *uuid.UUID

	ChargeType    string
	TariffID      *uuid.UUID
	Description   string
	ReferenceID   *uuid.UUID
	ReferenceType *string

	Quantity float64
	// UnitPriceMinor is SNAPSHOTTED at creation: a later price change must not
	// rewrite an invoice that has already been handed to a patient.
	UnitPriceMinor int64
	AmountMinor    int64
	Status         string
	PostedAt       time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// Payment is money received. APPEND-ONLY: a correction is a reversal row.
type Payment struct {
	ID             uuid.UUID
	OperationID    uuid.UUID
	ReceiptNumber  string
	PayerPatientID *uuid.UUID
	HouseholdID    *uuid.UUID
	AmountMinor    int64
	CurrencyCode   string
	Method         string
	Reference      *string
	Status         string
	PaidAt         time.Time
	ReceivedBy     uuid.UUID
	ReversalOf     *uuid.UUID
	ReversedBy     *uuid.UUID
	ReversalReason *string
	Notes          *string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// Allocation is the part of a payment applied to one charge.
type Allocation struct {
	ID          uuid.UUID
	PaymentID   uuid.UUID
	ChargeID    uuid.UUID
	AmountMinor int64
	AllocatedAt time.Time
	AllocatedBy uuid.UUID
}

// InvoiceBalance is DERIVED, never stored.
//
// A stored balance is correct until the first concurrent write from a second
// device, and silently wrong for ever after. Deriving it costs one aggregate and
// cannot drift.
type InvoiceBalance struct {
	InvoiceID    uuid.UUID
	TotalMinor   int64
	PaidMinor    int64
	BalanceMinor int64
	Status       string
}
