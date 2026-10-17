package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// Encounters implements the queue and the observation record.
type Encounters struct {
	pool     *pgxpool.Pool
	repo     *repo.Encounters
	patients *repo.Patients
	users    *repo.Users
	now      func() time.Time
}

// NewEncounters wires the service.
func NewEncounters(pool *pgxpool.Pool) *Encounters {
	return &Encounters{
		pool:     pool,
		repo:     repo.NewEncounters(),
		patients: repo.NewPatients(),
		users:    repo.NewUsers(pool),
		now:      time.Now,
	}
}

var clinicalRoles = []domain.Role{
	domain.RoleFrontDesk, domain.RoleNurse, domain.RoleDoctor, domain.RoleAdmin,
}

// StartEncounterInput is one visit registration.
type StartEncounterInput struct {
	ActorID        uuid.UUID
	PatientID      uuid.UUID
	EncounterType  string
	Priority       int
	ChiefComplaint string
	// VisitNumber is generated on the device like every other document number.
	VisitNumber string
}

// Start opens a visit, which puts the patient in the queue.
func (s *Encounters) Start(ctx context.Context, in StartEncounterInput) (*domain.Encounter, error) {
	actor, err := s.requireClinical(ctx, in.ActorID)
	if err != nil {
		return nil, err
	}

	patient, err := s.patients.FindPatientByID(ctx, s.pool, in.PatientID)
	if err != nil {
		return nil, err
	}

	// One open visit at a time. Without this a clerk double-clicks and the
	// patient appears twice in the queue, then twice in the doctor's list.
	if existing, err := s.repo.FindActiveEncounter(ctx, s.pool, patient.ID); err == nil && existing != nil {
		return nil, domain.Publicf(domain.ErrConflict,
			"This patient already has an open visit (%s). Close it before starting another.",
			existing.VisitNumber)
	}

	encounterType := strings.TrimSpace(in.EncounterType)
	if encounterType == "" {
		encounterType = "opd"
	}
	switch encounterType {
	case "opd", "anc", "immunisation", "emergency", "follow_up", "inpatient":
	default:
		return nil, domain.NewValidation("encounterType", "unknown_value")
	}

	priority := in.Priority
	if priority < 0 || priority > 2 {
		return nil, domain.NewValidation("priority", "must_be_0_1_or_2")
	}

	number := strings.TrimSpace(in.VisitNumber)
	if number == "" {
		generated, err := s.generateVisitNumber(ctx)
		if err != nil {
			return nil, err
		}
		number = generated
	}

	encounter := &domain.Encounter{
		ID:             uuid.New(),
		VisitNumber:    number,
		PatientID:      patient.ID,
		HouseholdID:    patient.HouseholdID,
		EncounterType:  encounterType,
		WorkflowStage:  domain.StageRegistered,
		Status:         "active",
		Priority:       int16(priority),
		ChiefComplaint: optionalString(&in.ChiefComplaint, 500),
		QueuedAt:       s.now(),
		CreatedBy:      actor.ID,
		UpdatedBy:      actor.ID,
	}

	if err := s.repo.InsertEncounter(ctx, s.pool, encounter); err != nil {
		return nil, err
	}
	return encounter, nil
}

// Transition moves a visit to a new stage.
func (s *Encounters) Transition(
	ctx context.Context,
	actorID, encounterID uuid.UUID,
	to string,
) (*domain.Encounter, error) {
	actor, err := s.requireClinical(ctx, actorID)
	if err != nil {
		return nil, err
	}

	encounter, err := s.repo.FindEncounterByID(ctx, s.pool, encounterID)
	if err != nil {
		return nil, err
	}

	if !encounter.IsOpen() {
		return nil, domain.Public(domain.ErrValidation, "This visit is already closed.")
	}
	if !CanTransition(encounter.WorkflowStage, to) {
		// Name the legal moves: "cannot go from A to B" leaves the user stuck.
		return nil, domain.Publicf(domain.ErrValidation,
			"A visit cannot move from %s to %s. From here it can go to: %s.",
			encounter.WorkflowStage, to, strings.Join(NextStages(encounter.WorkflowStage), ", "))
	}
	// Check EVERY role the user holds, not just PrimaryRole().
	//
	// A real PHC has a nurse who also covers as doctor. PrimaryRole() picks the
	// MOST privileged role, so checking only that one refused a perfectly legal
	// move because the map happened to list the other role — and the refusal
	// message claims the user "cannot" do something they demonstrably can.
	if !StageAllowedForAnyRole(to, actor.Roles) {
		return nil, domain.Publicf(domain.ErrForbidden,
			"Your role cannot move a visit to %s.", to)
	}

	moved, err := s.repo.Transition(ctx, s.pool, encounterID, encounter.WorkflowStage, to, actor.ID, to == domain.StageClosed)
	if err != nil {
		return nil, err
	}
	if !moved {
		// Another device moved it between our read and our write.
		return nil, domain.Public(domain.ErrStale,
			"Another station moved this patient while you were looking. Reload and try again.")
	}

	return s.repo.FindEncounterByID(ctx, s.pool, encounterID)
}

// Queue returns the active visits.
func (s *Encounters) Queue(ctx context.Context, actorID uuid.UUID, limit int) ([]*domain.Encounter, error) {
	if _, err := s.requireClinical(ctx, actorID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	return s.repo.Queue(ctx, s.pool, limit)
}

// RecordVitalsInput is one observation.
type RecordVitalsInput struct {
	ActorID     uuid.UUID
	PatientID   uuid.UUID
	EncounterID *uuid.UUID

	BPSystolic      *int
	BPDiastolic     *int
	TemperatureC    *float64
	PulseRate       *int
	RespiratoryRate *int
	HeightCm        *float64
	WeightKg        *float64
	SpO2            *int
	PainScore       *int
	Notes           string
}

// RecordVitals stores an observation and moves a registered visit forward.
//
// The insert and the stage change share ONE transaction: a reading recorded
// against a visit still at `registered` is invisible to the doctor, and a stage
// change without a reading sends an empty patient through.
func (s *Encounters) RecordVitals(ctx context.Context, in RecordVitalsInput) (*domain.Vitals, error) {
	actor, err := s.requireVitalsWriter(ctx, in.ActorID)
	if err != nil {
		return nil, err
	}

	patient, err := s.patients.FindPatientByID(ctx, s.pool, in.PatientID)
	if err != nil {
		return nil, err
	}

	if err := validateVitals(in); err != nil {
		return nil, err
	}

	height := in.HeightCm
	weight := in.WeightKg

	vitals := &domain.Vitals{
		ID:              uuid.New(),
		PatientID:       patient.ID,
		EncounterID:     in.EncounterID,
		RecordedAt:      s.now(),
		BPSystolic:      int16Ptr(in.BPSystolic),
		BPDiastolic:     int16Ptr(in.BPDiastolic),
		TemperatureC:    in.TemperatureC,
		PulseRate:       int16Ptr(in.PulseRate),
		RespiratoryRate: int16Ptr(in.RespiratoryRate),
		HeightCm:        height,
		WeightKg:        weight,
		SpO2:            int16Ptr(in.SpO2),
		BMI:             domain.BMIFrom(height, weight),
		PainScore:       int16Ptr(in.PainScore),
		Notes:           optionalString(&in.Notes, 500),
		RecordedBy:      actor.ID,
		CreatedBy:       actor.ID,
		UpdatedBy:       actor.ID,
	}

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.InsertVitals(ctx, tx, vitals); err != nil {
			return err
		}

		if in.EncounterID == nil {
			return nil
		}

		encounter, err := s.repo.FindEncounterByID(ctx, tx, *in.EncounterID)
		if err != nil {
			return err
		}
		if encounter.PatientID != patient.ID {
			return domain.NewValidation("encounterId", "does_not_belong_to_patient")
		}

		// Only the first observation advances the workflow; later readings during
		// the same visit are additional facts, not new stages.
		if encounter.WorkflowStage == domain.StageRegistered {
			if _, err := s.repo.Transition(ctx, tx, encounter.ID, domain.StageRegistered, domain.StageVitalsDone, actor.ID, false); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return vitals, nil
}

// VitalsForPatient lists readings newest first.
func (s *Encounters) VitalsForPatient(ctx context.Context, actorID, patientID uuid.UUID) ([]*domain.Vitals, error) {
	if _, err := s.requireClinical(ctx, actorID); err != nil {
		return nil, err
	}
	return s.repo.VitalsForPatient(ctx, s.pool, patientID, 50)
}

// validateVitals mirrors the database CHECK constraints.
//
// Both layers on purpose: the CHECK is the backstop, and this layer gives a
// field-level message the form can highlight. A bare 500 from a constraint
// violation tells a nurse nothing.
func validateVitals(in RecordVitalsInput) error {
	type bound struct {
		value *int
		min   int
		max   int
		field string
	}
	for _, b := range []bound{
		{in.BPSystolic, 40, 300, "bpSystolic"},
		{in.BPDiastolic, 20, 200, "bpDiastolic"},
		{in.PulseRate, 20, 300, "pulseRate"},
		{in.RespiratoryRate, 5, 80, "respiratoryRate"},
		{in.SpO2, 50, 100, "spo2"},
		{in.PainScore, 0, 10, "painScore"},
	} {
		if b.value != nil && (*b.value < b.min || *b.value > b.max) {
			return domain.NewValidationf(b.field, "out_of_range", "%d–%d", b.min, b.max)
		}
	}

	if in.TemperatureC != nil && (*in.TemperatureC < 25 || *in.TemperatureC > 45) {
		return domain.NewValidationf("temperatureC", "out_of_range", "25.0–45.0")
	}
	if in.HeightCm != nil && (*in.HeightCm < 20 || *in.HeightCm > 260) {
		return domain.NewValidationf("heightCm", "out_of_range", "20–260")
	}
	if in.WeightKg != nil && (*in.WeightKg < 0.5 || *in.WeightKg > 400) {
		return domain.NewValidationf("weightKg", "out_of_range", "0.5–400")
	}

	if in.BPSystolic != nil && in.BPDiastolic != nil && *in.BPSystolic <= *in.BPDiastolic {
		return domain.NewValidation("bpSystolic", "must_exceed_diastolic")
	}
	return nil
}

func (s *Encounters) requireClinical(ctx context.Context, actorID uuid.UUID) (*domain.User, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, clinicalRoles...) {
		return nil, domain.Public(domain.ErrForbidden, "Your role cannot work the queue.")
	}
	return actor, nil
}

func (s *Encounters) requireVitalsWriter(ctx context.Context, actorID uuid.UUID) (*domain.User, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleNurse, domain.RoleDoctor, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Only a nurse or doctor can record vitals.")
	}
	return actor, nil
}

func (s *Encounters) generateVisitNumber(ctx context.Context) (string, error) {
	const sql = `SELECT code FROM facilities WHERE deleted_at IS NULL LIMIT 1`
	var code string
	if err := s.pool.QueryRow(ctx, sql).Scan(&code); err != nil {
		return "", fmt.Errorf("read facility code: %w", err)
	}
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate visit number: %w", err)
	}
	return fmt.Sprintf("%s/VIS/%d/%s", code, s.now().Year(), strings.ToUpper(hex.EncodeToString(buf))), nil
}

func int16Ptr(v *int) *int16 {
	if v == nil {
		return nil
	}
	n := int16(*v)
	return &n
}
