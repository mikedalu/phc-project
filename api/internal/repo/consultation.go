package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Consultation holds the diagnosis and lab SQL.
type Consultation struct{}

// NewConsultation builds the repository.
func NewConsultation() *Consultation { return &Consultation{} }

const diagnosisColumns = `
    id, patient_id, encounter_id, diagnosis_type, icd10_code,
    description, clinical_notes, diagnosed_at, diagnosed_by,
    created_at, updated_at, created_by, updated_by`

// InsertDiagnosis writes a diagnosis. There is no update method, by design.
func (r *Consultation) InsertDiagnosis(ctx context.Context, q Querier, d *domain.Diagnosis) error {
	const sql = `
        INSERT INTO diagnoses (
            id, patient_id, encounter_id, diagnosis_type, icd10_code,
            description, clinical_notes, diagnosed_at, diagnosed_by,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)`

	if _, err := q.Exec(ctx, sql,
		d.ID, d.PatientID, d.EncounterID, d.Type, d.ICD10Code,
		d.Description, d.ClinicalNotes, d.DiagnosedAt, d.DiagnosedBy,
		d.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert diagnosis: %w", err)
	}
	return nil
}

// DiagnosesForPatient returns a patient's diagnoses, newest first.
func (r *Consultation) DiagnosesForPatient(ctx context.Context, q Querier, patientID uuid.UUID) ([]*domain.Diagnosis, error) {
	const sql = `SELECT` + diagnosisColumns + `
          FROM diagnoses
         WHERE patient_id = $1 AND deleted_at IS NULL
         ORDER BY diagnosed_at DESC`

	rows, err := q.Query(ctx, sql, patientID)
	if err != nil {
		return nil, fmt.Errorf("list diagnoses: %w", err)
	}
	defer rows.Close()

	var out []*domain.Diagnosis
	for rows.Next() {
		var d domain.Diagnosis
		if err := rows.Scan(
			&d.ID, &d.PatientID, &d.EncounterID, &d.Type, &d.ICD10Code,
			&d.Description, &d.ClinicalNotes, &d.DiagnosedAt, &d.DiagnosedBy,
			&d.CreatedAt, &d.UpdatedAt, &d.CreatedBy, &d.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan diagnosis: %w", err)
		}
		out = append(out, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate diagnoses: %w", err)
	}
	return out, nil
}

const labOrderColumns = `
    id, patient_id, encounter_id, order_number, test_code, test_name,
    specimen_type, clinical_indication, priority, status,
    ordered_at, ordered_by, collected_at, collected_by, completed_at,
    created_at, updated_at, created_by, updated_by`

func scanLabOrder(row pgx.Row) (*domain.LabOrder, error) {
	var o domain.LabOrder
	err := row.Scan(
		&o.ID, &o.PatientID, &o.EncounterID, &o.OrderNumber, &o.TestCode, &o.TestName,
		&o.SpecimenType, &o.ClinicalIndication, &o.Priority, &o.Status,
		&o.OrderedAt, &o.OrderedBy, &o.CollectedAt, &o.CollectedBy, &o.CompletedAt,
		&o.CreatedAt, &o.UpdatedAt, &o.CreatedBy, &o.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &o, nil
}

// InsertLabOrder writes a test request.
func (r *Consultation) InsertLabOrder(ctx context.Context, q Querier, o *domain.LabOrder) error {
	const sql = `
        INSERT INTO lab_orders (
            id, patient_id, encounter_id, order_number, test_code, test_name,
            specimen_type, clinical_indication, priority, status,
            ordered_at, ordered_by, created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`

	if _, err := q.Exec(ctx, sql,
		o.ID, o.PatientID, o.EncounterID, o.OrderNumber, o.TestCode, o.TestName,
		o.SpecimenType, o.ClinicalIndication, o.Priority, o.Status,
		o.OrderedAt, o.OrderedBy, o.CreatedBy,
	); err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: order number already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert lab order: %w", err)
	}
	return nil
}

// FindLabOrder loads one order.
func (r *Consultation) FindLabOrder(ctx context.Context, q Querier, id uuid.UUID) (*domain.LabOrder, error) {
	const sql = `SELECT` + labOrderColumns + ` FROM lab_orders WHERE id = $1 AND deleted_at IS NULL`

	o, err := scanLabOrder(q.QueryRow(ctx, sql, id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("find lab order: %w", err)
	}
	return o, nil
}

// LabOrdersForPatient lists a patient's orders, newest first.
func (r *Consultation) LabOrdersForPatient(ctx context.Context, q Querier, patientID uuid.UUID) ([]*domain.LabOrder, error) {
	const sql = `SELECT` + labOrderColumns + `
          FROM lab_orders
         WHERE patient_id = $1 AND deleted_at IS NULL
         ORDER BY ordered_at DESC`

	rows, err := q.Query(ctx, sql, patientID)
	if err != nil {
		return nil, fmt.Errorf("list lab orders: %w", err)
	}
	defer rows.Close()

	var out []*domain.LabOrder
	for rows.Next() {
		o, err := scanLabOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("scan lab order: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lab orders: %w", err)
	}
	return out, nil
}

// MarkLabOrderCollected records that the specimen was taken.
//
// Pinned to status 'ordered': two stations collecting the same specimen is not
// a thing, and the second one gets 0 rows rather than silently rewriting time.
func (r *Consultation) MarkLabOrderCollected(ctx context.Context, q Querier, id, actorID uuid.UUID) (bool, error) {
	const sql = `
        UPDATE lab_orders
           SET status = 'collected', collected_at = now(), collected_by = $2,
               updated_by = $2, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND status = 'ordered' AND deleted_at IS NULL`

	tag, err := q.Exec(ctx, sql, id, actorID)
	if err != nil {
		return false, fmt.Errorf("collect lab order: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// InsertLabResult writes a measurement. No update method, by design.
func (r *Consultation) InsertLabResult(ctx context.Context, q Querier, res *domain.LabResult) error {
	const sql = `
        INSERT INTO lab_results (
            id, lab_order_id, patient_id, parameter_name, value_text, value_numeric,
            unit, reference_range, abnormal_flag, result_at, result_by, remarks,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`

	if _, err := q.Exec(ctx, sql,
		res.ID, res.LabOrderID, res.PatientID, res.ParameterName, res.ValueText, res.ValueNumeric,
		res.Unit, res.ReferenceRange, res.AbnormalFlag, res.ResultAt, res.ResultBy, res.Remarks,
		res.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert lab result: %w", err)
	}
	return nil
}

// MarkLabOrderResulted flips the order to 'resulted' once its last result lands.
func (r *Consultation) MarkLabOrderResulted(ctx context.Context, q Querier, orderID, actorID uuid.UUID) error {
	const sql = `
        UPDATE lab_orders
           SET status = 'resulted', completed_at = now(),
               updated_by = $2, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND status = 'collected' AND deleted_at IS NULL`

	if _, err := q.Exec(ctx, sql, orderID, actorID); err != nil {
		return fmt.Errorf("mark order resulted: %w", err)
	}
	return nil
}

// LabResultsForOrder lists the parameters recorded for an order.
func (r *Consultation) LabResultsForOrder(ctx context.Context, q Querier, orderID uuid.UUID) ([]*domain.LabResult, error) {
	const sql = `
        SELECT id, lab_order_id, patient_id, parameter_name, value_text, value_numeric,
               unit, reference_range, abnormal_flag, result_at, result_by,
               verified_at, verified_by, remarks,
               created_at, updated_at, created_by, updated_by
          FROM lab_results
         WHERE lab_order_id = $1 AND deleted_at IS NULL
         ORDER BY created_at`

	rows, err := q.Query(ctx, sql, orderID)
	if err != nil {
		return nil, fmt.Errorf("list lab results: %w", err)
	}
	defer rows.Close()

	var out []*domain.LabResult
	for rows.Next() {
		var r domain.LabResult
		if err := rows.Scan(
			&r.ID, &r.LabOrderID, &r.PatientID, &r.ParameterName, &r.ValueText, &r.ValueNumeric,
			&r.Unit, &r.ReferenceRange, &r.AbnormalFlag, &r.ResultAt, &r.ResultBy,
			&r.VerifiedAt, &r.VerifiedBy, &r.Remarks,
			&r.CreatedAt, &r.UpdatedAt, &r.CreatedBy, &r.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan lab result: %w", err)
		}
		out = append(out, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lab results: %w", err)
	}
	return out, nil
}

// ─── prescriptions ─────────────────────────────────────────────────

// InsertPrescription writes the header.
func (r *Consultation) InsertPrescription(ctx context.Context, q Querier, p *domain.Prescription) error {
	const sql = `
        INSERT INTO prescriptions (
            id, prescription_number, patient_id, encounter_id, prescription_type,
            status, prescribed_at, prescribed_by, invoice_id, notes,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)`

	if _, err := q.Exec(ctx, sql,
		p.ID, p.PrescriptionNumber, p.PatientID, p.EncounterID, p.PrescriptionType,
		p.Status, p.PrescribedAt, p.PrescribedBy, p.InvoiceID, p.Notes,
		p.CreatedBy,
	); err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: prescription number already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert prescription: %w", err)
	}
	return nil
}

// InsertPrescriptionItem writes one drug line.
func (r *Consultation) InsertPrescriptionItem(ctx context.Context, q Querier, item *domain.PrescriptionItem) error {
	const sql = `
        INSERT INTO prescription_items (
            id, prescription_id, drug_name, dose, frequency, duration_days,
            quantity_prescribed, route, instructions, charge_id,
            unit_price_minor, amount_minor, created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`

	if _, err := q.Exec(ctx, sql,
		item.ID, item.PrescriptionID, item.DrugName, item.Dose, item.Frequency, item.DurationDays,
		item.QuantityPrescribed, item.Route, item.Instructions, item.ChargeID,
		item.UnitPriceMinor, item.AmountMinor, item.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert prescription item: %w", err)
	}
	return nil
}

// FindPrescription loads one prescription.
func (r *Consultation) FindPrescription(ctx context.Context, q Querier, id uuid.UUID) (*domain.Prescription, error) {
	const sql = `
        SELECT id, prescription_number, patient_id, encounter_id, prescription_type,
               status, prescribed_at, prescribed_by, invoice_id, notes,
               created_at, updated_at, created_by, updated_by
          FROM prescriptions
         WHERE id = $1 AND deleted_at IS NULL`

	var p domain.Prescription
	err := q.QueryRow(ctx, sql, id).Scan(
		&p.ID, &p.PrescriptionNumber, &p.PatientID, &p.EncounterID, &p.PrescriptionType,
		&p.Status, &p.PrescribedAt, &p.PrescribedBy, &p.InvoiceID, &p.Notes,
		&p.CreatedAt, &p.UpdatedAt, &p.CreatedBy, &p.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find prescription: %w", err)
	}
	return &p, nil
}

// MarkPrescriptionsPaidForInvoice settles every prescription billed on an
// invoice that has just been paid in full.
//
// This is the link the pharmacy depends on: without it the cashier takes the
// money, the invoice reads 'paid', and the pharmacist is still refused at the
// counter with "this prescription has not been paid for".
func (r *Consultation) MarkPrescriptionsPaidForInvoice(ctx context.Context, q Querier, invoiceID, actorID uuid.UUID) error {
	const sql = `
        UPDATE prescriptions
           SET status = 'paid', updated_by = $2, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE invoice_id = $1 AND status = 'pending_payment' AND deleted_at IS NULL`

	if _, err := q.Exec(ctx, sql, invoiceID, actorID); err != nil {
		return fmt.Errorf("mark prescriptions paid: %w", err)
	}
	return nil
}

// MarkPrescriptionDispensed closes the prescription.
//
// Pinned to 'paid': dispensing an unpaid prescription is the mistake this clause
// exists to make impossible, including for a device that has been offline.
func (r *Consultation) MarkPrescriptionDispensed(ctx context.Context, q Querier, id, actorID uuid.UUID) (bool, error) {
	const sql = `
        UPDATE prescriptions
           SET status = 'dispensed', updated_by = $2, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND status = 'paid' AND deleted_at IS NULL`

	tag, err := q.Exec(ctx, sql, id, actorID)
	if err != nil {
		return false, fmt.Errorf("mark prescription dispensed: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// InsertDispense records that stock moved.
func (r *Consultation) InsertDispense(ctx context.Context, q Querier, d *domain.Dispense) error {
	const sql = `
        INSERT INTO dispenses (
            id, prescription_id, patient_id, dispensed_at, dispensed_by, notes,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$7)`

	if _, err := q.Exec(ctx, sql,
		d.ID, d.PrescriptionID, d.PatientID, d.DispensedAt, d.DispensedBy, d.Notes,
		d.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert dispense: %w", err)
	}
	return nil
}
