package domain

import (
	"time"

	"github.com/google/uuid"
)

// Workflow stages. The strings match the CHECK constraint and the client enum.
const (
	StageRegistered      = "registered"
	StageVitalsDone      = "vitals_done"
	StageAwaitingDoctor  = "awaiting_doctor"
	StageInConsultation  = "in_consultation"
	StageAwaitingLab     = "awaiting_lab"
	StageLabDone         = "lab_done"
	StageAwaitingLabPay  = "awaiting_lab_payment"
	StageAwaitingPharm   = "awaiting_pharmacy"
	StageAwaitingDrugPay = "awaiting_drug_payment"
	StagePharmacyDone    = "pharmacy_done"
	StageAdmitted        = "admitted"
	StageClosed          = "closed"
)

// Encounter is one visit. It is ALSO the queue entry — there is no separate
// queue table, so a patient cannot be in two places in the queue at once.
type Encounter struct {
	ID                uuid.UUID
	VisitNumber       string
	PatientID         uuid.UUID
	HouseholdID       *uuid.UUID
	EncounterType     string
	WorkflowStage     string
	Status            string
	Priority          int16
	ChiefComplaint    *string
	QueuedAt          time.Time
	StartedAt         *time.Time
	ClosedAt          *time.Time
	ClosedBy          *uuid.UUID
	AttendingDoctorID *uuid.UUID
	AttendingNurseID  *uuid.UUID
	Notes             *string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// IsOpen reports whether the encounter still occupies the workflow.
func (e *Encounter) IsOpen() bool { return e.Status == "active" }

// Vitals is one observation. IMMUTABLE: a second reading is a new row.
//
// The distinction matters clinically — two blood pressures taken ten minutes
// apart are two facts, and letting the second overwrite the first loses the
// trend that a doctor actually reads.
type Vitals struct {
	ID          uuid.UUID
	PatientID   uuid.UUID
	EncounterID *uuid.UUID

	RecordedAt time.Time

	BPSystolic      *int16
	BPDiastolic     *int16
	TemperatureC    *float64
	PulseRate       *int16
	RespiratoryRate *int16
	HeightCm        *float64
	WeightKg        *float64
	SpO2            *int16
	BMI             *float64
	PainScore       *int16
	Notes           *string

	RecordedBy uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// BMIFrom computes BMI from height and weight, returning nil unless both are
// usable. Computed ONCE on the client and stored, because the formula may
// change and a historical reading should keep the value it was recorded with.
func BMIFrom(heightCm, weightKg *float64) *float64 {
	if heightCm == nil || weightKg == nil || *heightCm <= 0 || *weightKg <= 0 {
		return nil
	}
	metres := *heightCm / 100
	value := *weightKg / (metres * metres)
	return &value
}
