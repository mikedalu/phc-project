package service

import (
	"context"
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

// Ward implements admissions and bed-day billing.
type Ward struct {
	pool       *pgxpool.Pool
	repo       *repo.Admissions
	billing    *repo.Billing
	patients   *repo.Patients
	encounters *repo.Encounters
	users      *repo.Users
	now        func() time.Time
}

// NewWard wires the service.
func NewWard(pool *pgxpool.Pool) *Ward {
	return &Ward{
		pool:       pool,
		repo:       repo.NewAdmissions(),
		billing:    repo.NewBilling(),
		patients:   repo.NewPatients(),
		encounters: repo.NewEncounters(),
		users:      repo.NewUsers(pool),
		now:        time.Now,
	}
}

// AdmitInput is one admission.
type AdmitInput struct {
	ActorID     uuid.UUID
	PatientID   uuid.UUID
	EncounterID uuid.UUID
	Ward        string
	BedNumber   string
}

// Admit puts a patient in a bed and moves the encounter to `admitted`.
func (s *Ward) Admit(ctx context.Context, in AdmitInput) (*domain.Admission, error) {
	actor, err := s.requireWardStaff(ctx, in.ActorID)
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

	ward := strings.TrimSpace(in.Ward)
	if ward == "" {
		return nil, domain.NewValidation("ward", "required")
	}

	admission := &domain.Admission{
		ID:          uuid.New(),
		PatientID:   patient.ID,
		EncounterID: encounter.ID,
		Ward:        ward,
		BedNumber:   optionalString(&in.BedNumber, 20),
		Status:      domain.AdmissionAdmitted,
		AdmittedAt:  s.now(),
		AdmittedBy:  actor.ID,
		CreatedBy:   actor.ID,
		UpdatedBy:   actor.ID,
	}

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.InsertAdmission(ctx, tx, admission); err != nil {
			return err
		}
		if encounter.WorkflowStage != domain.StageAdmitted {
			if CanTransition(encounter.WorkflowStage, domain.StageAdmitted) {
				_, err := s.encounters.Transition(ctx, tx, encounter.ID, encounter.WorkflowStage, domain.StageAdmitted, actor.ID, false)
				return err
			}
			// A stage the map does not allow (e.g. straight from registered) is
			// not a reason to refuse the admission: the patient is in the bed.
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return admission, nil
}

// AccrueInput bills one night.
type AccrueInput struct {
	ActorID     uuid.UUID
	AdmissionID uuid.UUID
	// Night is the bed-day; empty means "today".
	Night string
	// UnitPriceMinor comes from the ward tariff when the client does not send it.
	UnitPriceMinor int64
	TariffCode     string
}

// AccrueNight bills one bed-day, at most once.
//
// ═══ THE CLOCK PROBLEM ═══
//
// Two devices can both decide that tonight is unbilled. The unique index on
// (admission_id, charge_date) makes the second insert a no-op, and this method
// reports `alreadyBilled` rather than charging twice. The night is identified by
// the DATE, so the same night is the same night no matter when it is typed.
func (s *Ward) AccrueNight(ctx context.Context, in AccrueInput) (*domain.WardCharge, bool, error) {
	actor, err := s.requireWardStaff(ctx, in.ActorID)
	if err != nil {
		return nil, false, err
	}

	admission, err := s.repo.FindAdmission(ctx, s.pool, in.AdmissionID)
	if err != nil {
		return nil, false, err
	}
	if admission.Status != domain.AdmissionAdmitted {
		return nil, false, domain.Publicf(domain.ErrValidation,
			"This admission is %s; no further bed-days can be charged.", admission.Status)
	}

	night := s.now()
	if strings.TrimSpace(in.Night) != "" {
		parsed, err := time.Parse("2006-01-02", in.Night)
		if err != nil {
			return nil, false, domain.NewValidation("night", "must_be_a_date")
		}
		night = parsed
	}
	// Normalise to midnight UTC so two devices in different timezones agree on
	// which night this is.
	night = time.Date(night.Year(), night.Month(), night.Day(), 0, 0, 0, 0, time.UTC)

	price := in.UnitPriceMinor
	if strings.TrimSpace(in.TariffCode) != "" {
		if t, err := s.billing.FindActiveTariff(ctx, s.pool, in.TariffCode); err == nil && t != nil {
			price = t.UnitPriceMinor
		}
	}
	if price <= 0 {
		return nil, false, domain.NewValidation("unitPriceMinor", "must_be_positive")
	}

	charge := &domain.WardCharge{
		ID:             uuid.New(),
		AdmissionID:    admission.ID,
		PatientID:      admission.PatientID,
		ChargeDate:     night,
		Ward:           admission.Ward,
		UnitPriceMinor: price,
		AmountMinor:    price,
		CreatedBy:      actor.ID,
		UpdatedBy:      actor.ID,
	}

	billed := false

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		invoice, err := s.invoiceFor(ctx, tx, admission, actor.ID)
		if err != nil {
			return err
		}
		charge.InvoiceID = &invoice.ID

		inserted, err := s.repo.InsertWardCharge(ctx, tx, charge)
		if err != nil {
			return err
		}
		if !inserted {
			// Another device billed this night first. Nothing to do; the caller
			// reports it as already billed.
			return nil
		}
		billed = true

		line := &domain.Charge{
			ID:             uuid.New(),
			InvoiceID:      invoice.ID,
			PatientID:      admission.PatientID,
			ChargeType:     "ward",
			Description:    fmt.Sprintf("Ward bed · %s · %s", admission.Ward, night.Format("2006-01-02")),
			Quantity:       1,
			UnitPriceMinor: price,
			AmountMinor:    price,
			Status:         "active",
			PostedAt:       s.now(),
			CreatedBy:      actor.ID,
			UpdatedBy:      actor.ID,
		}
		if err := s.billing.InsertCharge(ctx, tx, line); err != nil {
			return err
		}
		charge.ChargeID = &line.ID

		// Keep the invoice total in step with its lines.
		return s.recalcInvoiceTotal(ctx, tx, invoice.ID, actor.ID)
	}); err != nil {
		return nil, false, err
	}

	if !billed {
		return charge, false, nil
	}
	return charge, true, nil
}

// Discharge closes the stay.
func (s *Ward) Discharge(ctx context.Context, actorID, admissionID uuid.UUID, notes string) (*domain.Admission, error) {
	actor, err := s.requireWardStaff(ctx, actorID)
	if err != nil {
		return nil, err
	}

	admission, err := s.repo.FindAdmission(ctx, s.pool, admissionID)
	if err != nil {
		return nil, err
	}
	if admission.Status != domain.AdmissionAdmitted {
		return nil, domain.Publicf(domain.ErrValidation, "This admission is already %s.", admission.Status)
	}

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		moved, err := s.repo.Discharge(ctx, tx, admissionID, actor.ID, optionalString(&notes, 500))
		if err != nil {
			return err
		}
		if !moved {
			return domain.Public(domain.ErrStale, "Another station discharged this patient. Reload.")
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return s.repo.FindAdmission(ctx, s.pool, admissionID)
}

// invoiceFor returns the open ward invoice for an admission, creating one.
func (s *Ward) invoiceFor(ctx context.Context, tx pgx.Tx, admission *domain.Admission, actorID uuid.UUID) (*domain.Invoice, error) {
	if admission.InvoiceID != nil {
		if inv, err := s.billing.FindInvoice(ctx, tx, *admission.InvoiceID); err == nil {
			return inv, nil
		}
	}

	patient, err := s.patients.FindPatientByID(ctx, tx, admission.PatientID)
	if err != nil {
		return nil, err
	}

	number, err := s.invoiceNumber(ctx)
	if err != nil {
		return nil, err
	}

	invoice := &domain.Invoice{
		ID:            uuid.New(),
		InvoiceNumber: number,
		PatientID:     admission.PatientID,
		HouseholdID:   patient.HouseholdID,
		EncounterID:   &admission.EncounterID,
		InvoiceType:   "ward",
		Status:        domain.InvoiceUnpaid,
		IssuedAt:      s.now(),
		IssuedBy:      actorID,
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
	}

	if err := s.billing.InsertInvoice(ctx, tx, invoice); err != nil {
		return nil, err
	}
	if err := s.repo.SetAdmissionInvoice(ctx, tx, admission.ID, invoice.ID, actorID); err != nil {
		return nil, err
	}
	admission.InvoiceID = &invoice.ID
	return invoice, nil
}

func (s *Ward) recalcInvoiceTotal(ctx context.Context, tx pgx.Tx, invoiceID, actorID uuid.UUID) error {
	const sql = `
        UPDATE invoices i
           SET total_minor = COALESCE((
                   SELECT sum(amount_minor) FROM charges c
                    WHERE c.invoice_id = i.id AND c.status = 'active' AND c.deleted_at IS NULL
               ), 0),
               updated_by = $2, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE i.id = $1`

	if _, err := tx.Exec(ctx, sql, invoiceID, actorID); err != nil {
		return fmt.Errorf("recalc invoice total: %w", err)
	}
	return nil
}

func (s *Ward) invoiceNumber(ctx context.Context) (string, error) {
	const sql = `SELECT code FROM facilities WHERE deleted_at IS NULL LIMIT 1`
	var code string
	if err := s.pool.QueryRow(ctx, sql).Scan(&code); err != nil {
		return "", fmt.Errorf("read facility code: %w", err)
	}
	return fmt.Sprintf("%s/INV/%d/%s", code, s.now().Year(), strings.ToUpper(uuid.NewString()[:6])), nil
}

func (s *Ward) requireWardStaff(ctx context.Context, actorID uuid.UUID) (*domain.User, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleNurse, domain.RoleDoctor, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Your role cannot work the ward.")
	}
	return actor, nil
}
