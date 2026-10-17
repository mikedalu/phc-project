package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Admissions holds the ward SQL.
type Admissions struct{}

// NewAdmissions builds the repository.
func NewAdmissions() *Admissions { return &Admissions{} }

// InsertAdmission writes the stay.
func (r *Admissions) InsertAdmission(ctx context.Context, q Querier, a *domain.Admission) error {
	const sql = `
        INSERT INTO admissions (
            id, patient_id, encounter_id, ward, bed_number, status,
            admitted_at, admitted_by, created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`

	if _, err := q.Exec(ctx, sql,
		a.ID, a.PatientID, a.EncounterID, a.Ward, a.BedNumber, a.Status,
		a.AdmittedAt, a.AdmittedBy, a.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert admission: %w", err)
	}
	return nil
}

// FindAdmission loads one stay.
func (r *Admissions) FindAdmission(ctx context.Context, q Querier, id uuid.UUID) (*domain.Admission, error) {
	const sql = `
        SELECT id, patient_id, encounter_id, ward, bed_number, status,
               admitted_at, admitted_by, discharged_at, discharged_by,
               discharge_notes, invoice_id,
               created_at, updated_at, created_by, updated_by
          FROM admissions
         WHERE id = $1 AND deleted_at IS NULL`

	var a domain.Admission
	err := q.QueryRow(ctx, sql, id).Scan(
		&a.ID, &a.PatientID, &a.EncounterID, &a.Ward, &a.BedNumber, &a.Status,
		&a.AdmittedAt, &a.AdmittedBy, &a.DischargedAt, &a.DischargedBy,
		&a.DischargeNotes, &a.InvoiceID,
		&a.CreatedAt, &a.UpdatedAt, &a.CreatedBy, &a.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find admission: %w", err)
	}
	return &a, nil
}

// SetAdmissionInvoice records the bill a stay accrues against.
func (r *Admissions) SetAdmissionInvoice(ctx context.Context, q Querier, id, invoiceID, actorID uuid.UUID) error {
	const sql = `
        UPDATE admissions
           SET invoice_id = $2, updated_by = $3, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND deleted_at IS NULL`

	if _, err := q.Exec(ctx, sql, id, invoiceID, actorID); err != nil {
		return fmt.Errorf("set admission invoice: %w", err)
	}
	return nil
}

// Discharge closes the stay.
func (r *Admissions) Discharge(ctx context.Context, q Querier, id, actorID uuid.UUID, notes *string) (bool, error) {
	const sql = `
        UPDATE admissions
           SET status = 'discharged', discharged_at = now(), discharged_by = $2,
               discharge_notes = $3, updated_by = $2, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND status = 'admitted' AND deleted_at IS NULL`

	tag, err := q.Exec(ctx, sql, id, actorID, notes)
	if err != nil {
		return false, fmt.Errorf("discharge: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// InsertWardCharge writes one night.
//
// Returns inserted=false when the night is already billed. That is the
// double-billing guard doing its job, not an error: two devices charging the
// same night is a RACE, and the loser should be told the night is settled.
func (r *Admissions) InsertWardCharge(ctx context.Context, q Querier, c *domain.WardCharge) (bool, error) {
	const sql = `
        INSERT INTO ward_charges (
            id, admission_id, patient_id, charge_date, ward,
            unit_price_minor, amount_minor, invoice_id, charge_id,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)
        ON CONFLICT (admission_id, charge_date) DO NOTHING`

	tag, err := q.Exec(ctx, sql,
		c.ID, c.AdmissionID, c.PatientID, c.ChargeDate, c.Ward,
		c.UnitPriceMinor, c.AmountMinor, c.InvoiceID, c.ChargeID,
		c.CreatedBy,
	)
	if err != nil {
		return false, fmt.Errorf("insert ward charge: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
