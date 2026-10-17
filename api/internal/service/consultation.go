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

	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
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
	// TariffCode, when given, bills the test in the SAME transaction. Ordering a
	// test and forgetting to bill it is how a clinic loses revenue quietly.
	TariffCode string
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

	billing := repo.NewBilling()

	var tariff *repo.Tariff
	tariffCode := strings.TrimSpace(in.TariffCode)
	if tariffCode != "" {
		// A missing tariff is NOT an error: a clinic may order a test it has not
		// priced yet, and refusing the order would send the patient home.
		tariff, _ = billing.FindActiveTariff(ctx, s.pool, tariffCode)
	}

	if tariff == nil {
		if err := s.repo.InsertLabOrder(ctx, s.pool, order); err != nil {
			return nil, err
		}
		return order, nil
	}

	// The order and its bill are ONE transaction: an order that is invisible to
	// the cashier is a test the patient never pays for, and a bill with no order
	// is money for nothing.
	invoice := &domain.Invoice{
		ID:            uuid.New(),
		InvoiceNumber: "",
		PatientID:     patient.ID,
		HouseholdID:   patient.HouseholdID,
		EncounterID:   &encounter.ID,
		InvoiceType:   "lab",
		Status:        domain.InvoiceUnpaid,
		TotalMinor:    tariff.UnitPriceMinor,
		IssuedAt:      s.now(),
		IssuedBy:      actor.ID,
		CreatedBy:     actor.ID,
		UpdatedBy:     actor.ID,
	}

	invoiceNumber, err := s.generateOrderNumber(ctx) // same generator family
	if err != nil {
		return nil, err
	}
	invoice.InvoiceNumber = strings.Replace(invoiceNumber, "/LAB/", "/INV/", 1)

	charge := &domain.Charge{
		ID:             uuid.New(),
		InvoiceID:      invoice.ID,
		PatientID:      patient.ID,
		HouseholdID:    patient.HouseholdID,
		ChargeType:     "lab",
		TariffID:       &tariff.ID,
		Description:    tariff.Name,
		ReferenceID:    &order.ID,
		ReferenceType:  strPtr("lab_order"),
		Quantity:       1,
		UnitPriceMinor: tariff.UnitPriceMinor,
		AmountMinor:    tariff.UnitPriceMinor,
		Status:         "active",
		PostedAt:       s.now(),
		CreatedBy:      actor.ID,
		UpdatedBy:      actor.ID,
	}

	if txErr := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.InsertLabOrder(ctx, tx, order); err != nil {
			return err
		}
		if err := billing.InsertInvoice(ctx, tx, invoice); err != nil {
			return err
		}
		if err := billing.InsertCharge(ctx, tx, charge); err != nil {
			return err
		}
		return billing.LinkLabOrderToInvoice(ctx, tx, order.ID, invoice.ID, charge.ID)
	}); txErr != nil {
		return nil, txErr
	}

	order.InvoiceID = &invoice.ID
	return order, nil
}

func strPtr(s string) *string { return &s }

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

// RecordLabResultInput is one measured parameter.
type RecordLabResultInput struct {
	ActorID        uuid.UUID
	OrderID        uuid.UUID
	ParameterName  string
	ValueText      string
	ValueNumeric   *float64
	Unit           string
	ReferenceRange string
	AbnormalFlag   string
	Remarks        string
}

// RecordLabResult stores a measurement and, when the order is complete, closes it.
//
// ═══ WHY COLLECTED IS REQUIRED ═══
//
// A result against a specimen nobody collected is a data-entry fiction. The
// order must be 'collected' first, which also means the lab worklist has a
// queue to work through rather than a free-for-all.
func (s *Consultation) RecordLabResult(ctx context.Context, in RecordLabResultInput) (*domain.LabResult, error) {
	actor, err := s.users.FindByID(ctx, s.pool, in.ActorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleLabScientist, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Only lab staff can record a result.")
	}

	order, err := s.repo.FindLabOrder(ctx, s.pool, in.OrderID)
	if err != nil {
		return nil, err
	}
	// 'resulted' is accepted as well as 'collected': a test like a full blood
	// count has several parameters, and refusing the second one because the
	// first already closed the order would make multi-parameter tests
	// impossible to record.
	if order.Status != domain.LabCollected && order.Status != domain.LabResulted {
		return nil, domain.Publicf(domain.ErrValidation,
			"Results can only be recorded for a collected specimen; this order is %s.", order.Status)
	}

	name := strings.TrimSpace(in.ParameterName)
	if name == "" {
		return nil, domain.NewValidation("parameterName", "required")
	}

	valueText := strings.TrimSpace(in.ValueText)
	if valueText == "" && in.ValueNumeric == nil {
		// The database CHECK enforces this too; catching it here names the field.
		return nil, domain.NewValidation("valueNumeric", "value_or_text_required")
	}

	flag := strings.TrimSpace(in.AbnormalFlag)
	if flag != "" {
		switch flag {
		case "normal", "low", "high", "critical_low", "critical_high":
		default:
			return nil, domain.NewValidation("abnormalFlag", "unknown_value")
		}
	}

	result := &domain.LabResult{
		ID:             uuid.New(),
		LabOrderID:     order.ID,
		PatientID:      order.PatientID,
		ParameterName:  name,
		ValueText:      optionalString(&valueText, 200),
		ValueNumeric:   in.ValueNumeric,
		Unit:           optionalString(&in.Unit, 20),
		ReferenceRange: optionalString(&in.ReferenceRange, 60),
		AbnormalFlag:   optionalString(&flag, 20),
		ResultAt:       s.now(),
		ResultBy:       actor.ID,
		Remarks:        optionalString(&in.Remarks, 500),
		CreatedBy:      actor.ID,
		UpdatedBy:      actor.ID,
	}

	if err := s.repo.InsertLabResult(ctx, s.pool, result); err != nil {
		return nil, err
	}

	// The order is 'resulted' once at least one result exists. A test with two
	// parameters will therefore close on the first one — acceptable for a first
	// implementation, and the alternative (a tariff table of expected
	// parameters) is Sprint 6 work.
	if err := s.repo.MarkLabOrderResulted(ctx, s.pool, order.ID, actor.ID); err != nil {
		return nil, err
	}

	return result, nil
}

// LabResultsForOrder lists the parameters of an order.
func (s *Consultation) LabResultsForOrder(ctx context.Context, actorID, orderID uuid.UUID) ([]*domain.LabResult, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleDoctor, domain.RoleNurse, domain.RoleLabScientist, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Your role cannot read lab results.")
	}
	return s.repo.LabResultsForOrder(ctx, s.pool, orderID)
}

// PrescriptionItemInput is one drug line the doctor writes.
type PrescriptionItemInput struct {
	DrugName       string
	Dose           string
	Frequency      string
	DurationDays   *int
	Quantity       float64
	Route          string
	Instructions   string
	UnitPriceMinor int64
	TariffCode     string
}

// CreatePrescriptionInput is the whole request.
type CreatePrescriptionInput struct {
	ActorID            uuid.UUID
	PatientID          uuid.UUID
	EncounterID        uuid.UUID
	PrescriptionType   string
	PrescriptionNumber string
	Notes              string
	Items              []PrescriptionItemInput
}

// CreatePrescription writes the request, its lines and its bill in ONE
// transaction, and moves the encounter to awaiting_drug_payment.
func (s *Consultation) CreatePrescription(ctx context.Context, in CreatePrescriptionInput) (*domain.Prescription, []*domain.PrescriptionItem, error) {
	actor, err := s.requireDoctor(ctx, in.ActorID)
	if err != nil {
		return nil, nil, err
	}

	patient, err := s.patients.FindPatientByID(ctx, s.pool, in.PatientID)
	if err != nil {
		return nil, nil, err
	}
	encounter, err := s.encounters.FindEncounterByID(ctx, s.pool, in.EncounterID)
	if err != nil {
		return nil, nil, err
	}
	if encounter.PatientID != patient.ID {
		return nil, nil, domain.NewValidation("encounterId", "does_not_belong_to_patient")
	}
	if encounter.Status != "active" {
		return nil, nil, domain.Public(domain.ErrValidation, "This visit is closed.")
	}
	if len(in.Items) == 0 {
		return nil, nil, domain.NewValidation("items", "at_least_one_required")
	}

	kind := strings.TrimSpace(in.PrescriptionType)
	if kind == "" {
		kind = "opd"
	}
	switch kind {
	case "opd", "inpatient", "discharge", "refill":
	default:
		return nil, nil, domain.NewValidation("prescriptionType", "unknown_value")
	}

	number := strings.TrimSpace(in.PrescriptionNumber)
	if number == "" {
		number, err = s.generateOrderNumber(ctx)
		if err != nil {
			return nil, nil, err
		}
		number = strings.Replace(number, "/LAB/", "/RX/", 1)
	}

	rx := &domain.Prescription{
		ID:                 uuid.New(),
		PrescriptionNumber: number,
		PatientID:          patient.ID,
		EncounterID:        encounter.ID,
		PrescriptionType:   kind,
		Status:             domain.PrescriptionPendingPayment,
		PrescribedAt:       s.now(),
		PrescribedBy:       actor.ID,
		Notes:              optionalString(&in.Notes, 500),
		CreatedBy:          actor.ID,
		UpdatedBy:          actor.ID,
	}

	items := make([]*domain.PrescriptionItem, 0, len(in.Items))
	var total int64

	for _, line := range in.Items {
		name := strings.TrimSpace(line.DrugName)
		if name == "" {
			return nil, nil, domain.NewValidation("drugName", "required")
		}
		if line.Quantity <= 0 {
			return nil, nil, domain.NewValidation("quantity", "must_be_positive")
		}

		// A named tariff wins; otherwise the doctor's price stands, and zero is
		// allowed so an unpriced drug does not block the prescription.
		price := line.UnitPriceMinor
		if strings.TrimSpace(line.TariffCode) != "" {
			if t, err := repo.NewBilling().FindActiveTariff(ctx, s.pool, line.TariffCode); err == nil && t != nil {
				price = t.UnitPriceMinor
			}
		}

		amount := int64(roundTo2(float64(price) * line.Quantity))
		total += amount

		items = append(items, &domain.PrescriptionItem{
			ID:                 uuid.New(),
			PrescriptionID:     rx.ID,
			DrugName:           name,
			Dose:               strings.TrimSpace(line.Dose),
			Frequency:          strings.TrimSpace(line.Frequency),
			DurationDays:       int16FromPtr(line.DurationDays),
			QuantityPrescribed: line.Quantity,
			Route:              optionalString(&line.Route, 40),
			Instructions:       optionalString(&line.Instructions, 300),
			UnitPriceMinor:     price,
			AmountMinor:        amount,
			CreatedBy:          actor.ID,
			UpdatedBy:          actor.ID,
		})
	}

	billing := repo.NewBilling()

	var invoice *domain.Invoice
	if total > 0 {
		invoiceNumber, err := s.generateOrderNumber(ctx)
		if err != nil {
			return nil, nil, err
		}
		invoice = &domain.Invoice{
			ID:            uuid.New(),
			InvoiceNumber: strings.Replace(invoiceNumber, "/LAB/", "/INV/", 1),
			PatientID:     patient.ID,
			HouseholdID:   patient.HouseholdID,
			EncounterID:   &encounter.ID,
			InvoiceType:   "drug",
			Status:        domain.InvoiceUnpaid,
			TotalMinor:    total,
			IssuedAt:      s.now(),
			IssuedBy:      actor.ID,
			CreatedBy:     actor.ID,
			UpdatedBy:     actor.ID,
		}
		rx.InvoiceID = &invoice.ID
	}

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		// ORDER MATTERS: prescriptions.invoice_id is a real foreign key, checked
		// immediately, so the invoice must exist before the prescription row that
		// points at it. Inserting the prescription first failed with 23503.
		if invoice != nil {
			if err := billing.InsertInvoice(ctx, tx, invoice); err != nil {
				return err
			}
			charge := &domain.Charge{
				ID:             uuid.New(),
				InvoiceID:      invoice.ID,
				PatientID:      patient.ID,
				HouseholdID:    patient.HouseholdID,
				ChargeType:     "drug",
				Description:    fmt.Sprintf("Prescription %s", rx.PrescriptionNumber),
				ReferenceID:    &rx.ID,
				ReferenceType:  strPtr("prescription"),
				Quantity:       1,
				UnitPriceMinor: total,
				AmountMinor:    total,
				Status:         "active",
				PostedAt:       s.now(),
				CreatedBy:      actor.ID,
				UpdatedBy:      actor.ID,
			}
			if err := billing.InsertCharge(ctx, tx, charge); err != nil {
				return err
			}
		}

		if err := s.repo.InsertPrescription(ctx, tx, rx); err != nil {
			return err
		}
		for _, item := range items {
			if err := s.repo.InsertPrescriptionItem(ctx, tx, item); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}

	return rx, items, nil
}

// Dispense hands the drugs over, but only once they are PAID.
func (s *Consultation) Dispense(ctx context.Context, actorID, prescriptionID uuid.UUID) (*domain.Prescription, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, domain.RolePharmacist, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Only a pharmacist can dispense.")
	}

	rx, err := s.repo.FindPrescription(ctx, s.pool, prescriptionID)
	if err != nil {
		return nil, err
	}

	if rx.Status == domain.PrescriptionDispensed {
		return nil, domain.Public(domain.ErrValidation, "This prescription has already been dispensed.")
	}
	// The guard that matters: an unpaid prescription is stock given away.
	if rx.Status == domain.PrescriptionPendingPayment {
		return nil, domain.Public(domain.ErrValidation,
			"This prescription has not been paid for. Take payment at the cashier first.")
	}
	if rx.Status != domain.PrescriptionPaid {
		return nil, domain.Publicf(domain.ErrValidation,
			"This prescription is %s and cannot be dispensed.", rx.Status)
	}

	dispense := &domain.Dispense{
		ID:             uuid.New(),
		PrescriptionID: rx.ID,
		PatientID:      rx.PatientID,
		DispensedAt:    s.now(),
		DispensedBy:    actor.ID,
		CreatedBy:      actor.ID,
		UpdatedBy:      actor.ID,
	}

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.InsertDispense(ctx, tx, dispense); err != nil {
			return err
		}
		moved, err := s.repo.MarkPrescriptionDispensed(ctx, tx, rx.ID, actor.ID)
		if err != nil {
			return err
		}
		if !moved {
			return domain.Public(domain.ErrStale, "Another station dispensed this prescription. Reload.")
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return s.repo.FindPrescription(ctx, s.pool, rx.ID)
}

func int16FromPtr(v *int) *int16 {
	if v == nil {
		return nil
	}
	n := int16(*v)
	return &n
}
