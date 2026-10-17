package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

// CreateLabOrderInput is one test request.
type CreateLabOrderInput struct {
	ActorID     uuid.UUID
	PatientID   uuid.UUID
	EncounterID uuid.UUID
	TestCode    string
	TestName    string
	Specimen    string
	Indication  string
	Priority    string
	OrderNumber string
}

// CreateLabOrder raises a test request.
//
// ═══ NO MONEY HERE ═══
//
// The roadmap pairs this with invoice creation. Invoices arrive in Sprint 6, so
// this method deliberately does NOT take an invoice id: a lab order that
// half-bills would be worse than an order that bills a sprint later. The
// coupling lands as one transaction in Sprint 6, where the tables exist.
func (s *Consultation) CreateLabOrder(ctx context.Context, in CreateLabOrderInput) (*domain.LabOrder, error) {
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
	if encounter.PatientID != patient.ID {
		return nil, domain.NewValidation("encounterId", "does_not_belong_to_patient")
	}
	if encounter.Status != "active" {
		return nil, domain.Public(domain.ErrValidation, "This visit is closed; raise the order from a new visit.")
	}

	code := strings.TrimSpace(in.TestCode)
	name := strings.TrimSpace(in.TestName)
	if code == "" {
		return nil, domain.NewValidation("testCode", "required")
	}
	if name == "" {
		return nil, domain.NewValidation("testName", "required")
	}

	priority := strings.TrimSpace(in.Priority)
	if priority == "" {
		priority = "routine"
	}
	switch priority {
	case "routine", "urgent", "stat":
	default:
		return nil, domain.NewValidation("priority", "unknown_value")
	}

	specimen := strings.TrimSpace(in.Specimen)
	if specimen != "" {
		switch specimen {
		case "blood", "urine", "stool", "sputum", "swab", "other":
		default:
			return nil, domain.NewValidation("specimenType", "unknown_value")
		}
	}

	number := strings.TrimSpace(in.OrderNumber)
	if number == "" {
		number, err = s.generateOrderNumber(ctx)
		if err != nil {
			return nil, err
		}
	}

	order := &domain.LabOrder{
		ID:                 uuid.New(),
		PatientID:          patient.ID,
		EncounterID:        encounter.ID,
		OrderNumber:        number,
		TestCode:           code,
		TestName:           name,
		SpecimenType:       optionalString(&specimen, 20),
		ClinicalIndication: optionalString(&in.Indication, 500),
		Priority:           priority,
		Status:             domain.LabOrdered,
		OrderedAt:          s.now(),
		OrderedBy:          actor.ID,
		CreatedBy:          actor.ID,
		UpdatedBy:          actor.ID,
	}

	if err := s.repo.InsertLabOrder(ctx, s.pool, order); err != nil {
		return nil, err
	}
	return order, nil
}

// LabOrdersForPatient lists a patient's orders.
func (s *Consultation) LabOrdersForPatient(ctx context.Context, actorID, patientID uuid.UUID) ([]*domain.LabOrder, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleDoctor, domain.RoleNurse, domain.RoleLabScientist, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Your role cannot read lab orders.")
	}
	return s.repo.LabOrdersForPatient(ctx, s.pool, patientID)
}

// CollectLabOrder records that the specimen was taken.
func (s *Consultation) CollectLabOrder(ctx context.Context, actorID, orderID uuid.UUID) (*domain.LabOrder, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleLabScientist, domain.RoleNurse, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Only lab staff can collect a specimen.")
	}

	order, err := s.repo.FindLabOrder(ctx, s.pool, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.LabOrdered {
		return nil, domain.Publicf(domain.ErrValidation,
			"This order is already %s.", order.Status)
	}

	moved, err := s.repo.MarkLabOrderCollected(ctx, s.pool, orderID, actor.ID)
	if err != nil {
		return nil, err
	}
	if !moved {
		return nil, domain.Public(domain.ErrStale,
			"Another station collected this specimen. Reload and try again.")
	}
	return s.repo.FindLabOrder(ctx, s.pool, orderID)
}

func (s *Consultation) generateOrderNumber(ctx context.Context) (string, error) {
	const sql = `SELECT code FROM facilities WHERE deleted_at IS NULL LIMIT 1`
	var code string
	if err := s.pool.QueryRow(ctx, sql).Scan(&code); err != nil {
		return "", fmt.Errorf("read facility code: %w", err)
	}
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate order number: %w", err)
	}
	return fmt.Sprintf("%s/LAB/%d/%s", code, s.now().Year(), strings.ToUpper(hex.EncodeToString(buf))), nil
}
