package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// Consultation implements the doctor's output: diagnoses today, lab orders next.
type Consultation struct {
	pool       *pgxpool.Pool
	repo       *repo.Consultation
	patients   *repo.Patients
	encounters *repo.Encounters
	users      *repo.Users
	now        func() time.Time
}

// NewConsultation wires the service.
func NewConsultation(pool *pgxpool.Pool) *Consultation {
	return &Consultation{
		pool:       pool,
		repo:       repo.NewConsultation(),
		patients:   repo.NewPatients(),
		encounters: repo.NewEncounters(),
		users:      repo.NewUsers(pool),
		now:        time.Now,
	}
}

// CreateDiagnosisInput is one diagnosis.
type CreateDiagnosisInput struct {
	ActorID     uuid.UUID
	PatientID   uuid.UUID
	EncounterID uuid.UUID
	Type        string
	ICD10Code   string
	Description string
	Notes       string
}

// CreateDiagnosis records a clinical opinion.
//
// DOCTOR ONLY. A diagnosis is the justification for treatment and for a bill;
// letting any role write one would make the record worthless as evidence.
func (s *Consultation) CreateDiagnosis(ctx context.Context, in CreateDiagnosisInput) (*domain.Diagnosis, error) {
	actor, err := s.requireDoctor(ctx, in.ActorID)
	if err != nil {
		return nil, err
	}

	patient, err := s.patients.FindPatientByID(ctx, s.pool, in.PatientID)
	if err != nil {
		return nil, err
	}

	encounter, err := s.encounters.FindEncounterByID(ctx, s.pool, in.EncounterID)
	if err != nil {
		return nil, err
	}
	// A diagnosis belongs to a visit, and the visit must be this patient's.
	// Skipping this check lets a doctor attach an opinion to the wrong chart.
	if encounter.PatientID != patient.ID {
		return nil, domain.NewValidation("encounterId", "does_not_belong_to_patient")
	}

	diagnosisType := strings.TrimSpace(in.Type)
	if diagnosisType == "" {
		diagnosisType = domain.DiagnosisPrimary
	}
	switch diagnosisType {
	case domain.DiagnosisPrimary, domain.DiagnosisSecondary,
		domain.DiagnosisDifferential, domain.DiagnosisProvisional, domain.DiagnosisFinal:
	default:
		return nil, domain.NewValidation("diagnosisType", "unknown_value")
	}

	description := strings.TrimSpace(in.Description)
	if description == "" {
		return nil, domain.NewValidation("description", "required")
	}

	diagnosis := &domain.Diagnosis{
		ID:            uuid.New(),
		PatientID:     patient.ID,
		EncounterID:   encounter.ID,
		Type:          diagnosisType,
		ICD10Code:     optionalString(&in.ICD10Code, 10),
		Description:   description,
		ClinicalNotes: optionalString(&in.Notes, 1000),
		DiagnosedAt:   s.now(),
		DiagnosedBy:   actor.ID,
		CreatedBy:     actor.ID,
		UpdatedBy:     actor.ID,
	}

	if err := s.repo.InsertDiagnosis(ctx, s.pool, diagnosis); err != nil {
		return nil, err
	}
	return diagnosis, nil
}

// DiagnosesForPatient lists a patient's diagnoses.
func (s *Consultation) DiagnosesForPatient(ctx context.Context, actorID, patientID uuid.UUID) ([]*domain.Diagnosis, error) {
	if _, err := s.requireClinician(ctx, actorID); err != nil {
		return nil, err
	}
	return s.repo.DiagnosesForPatient(ctx, s.pool, patientID)
}

func (s *Consultation) requireDoctor(ctx context.Context, actorID uuid.UUID) (*domain.User, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleDoctor, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Only a doctor can record a diagnosis.")
	}
	return actor, nil
}

func (s *Consultation) requireClinician(ctx context.Context, actorID uuid.UUID) (*domain.User, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleDoctor, domain.RoleNurse, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Your role cannot read the clinical record.")
	}
	return actor, nil
}
