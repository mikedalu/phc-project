package domain

import (
	"time"

	"github.com/google/uuid"
)

// Referral is the clinic's record of a note it issued.
//
// The TOKEN is never stored: the database keeps what was sent and to whom, and
// the QR code carries the encrypted payload. Storing the token would make the
// encryption pointless.
type Referral struct {
	ID               uuid.UUID
	ReferralNumber   string
	PatientID        uuid.UUID
	EncounterID      uuid.UUID
	PatientName      string
	PatientAgeYears  *int16
	PatientSex       *string
	FromFacility     string
	ToFacility       string
	Reason           string
	DiagnosisSummary *string
	TreatmentGiven   *string
	Urgency          string
	Status           string
	IssuedAt         time.Time
	ExpiresAt        time.Time
	IssuedBy         uuid.UUID
	RevokedAt        *time.Time
	RevokedBy        *uuid.UUID
	RevokeReason     *string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}
