package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Households holds the family SQL.
type Households struct{}

// NewHouseholds builds the repository.
func NewHouseholds() *Households { return &Households{} }

// InsertHousehold writes a new household. `head_patient_id` is expected to be
// NULL here — the head cannot exist before its own row does.
func (r *Households) InsertHousehold(ctx context.Context, q Querier, h *domain.Household) error {
	const sql = `
        INSERT INTO households (
            id, family_number, family_name, head_patient_id,
            phone, address, notes, is_active,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`

	if _, err := q.Exec(ctx, sql,
		h.ID, h.FamilyNumber, h.FamilyName, h.HeadPatientID,
		h.Phone, h.Address, h.Notes, h.IsActive, h.CreatedBy,
	); err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: family number already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert household: %w", err)
	}
	return nil
}

// FindHouseholdByID loads one household.
func (r *Households) FindHouseholdByID(ctx context.Context, q Querier, id uuid.UUID) (*domain.Household, error) {
	const sql = `SELECT` + householdColumns + ` FROM households WHERE id = $1 AND deleted_at IS NULL`

	var h domain.Household
	err := q.QueryRow(ctx, sql, id).Scan(
		&h.ID, &h.FamilyNumber, &h.FamilyName, &h.HeadPatientID,
		&h.Phone, &h.Address, &h.Notes, &h.IsActive,
		&h.CreatedAt, &h.UpdatedAt, &h.CreatedBy, &h.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find household: %w", err)
	}
	return &h, nil
}

// SetHeadPatient closes the circular reference once the patient exists.
func (r *Households) SetHeadPatient(ctx context.Context, q Querier, householdID, patientID uuid.UUID) error {
	const sql = `
        UPDATE households
           SET head_patient_id = $2, updated_at = now(), revision = revision + 1,
               server_rev = server_rev + 1
         WHERE id = $1 AND deleted_at IS NULL`

	if _, err := q.Exec(ctx, sql, householdID, patientID); err != nil {
		return fmt.Errorf("set household head: %w", err)
	}
	return nil
}

// PatientsForHousehold returns the active members of a family.
func (r *Households) PatientsForHousehold(ctx context.Context, q Querier, householdID uuid.UUID) ([]*domain.Patient, error) {
	const sql = `
        SELECT` + patientColumns + `
          FROM patients
         WHERE household_id = $1 AND deleted_at IS NULL
         ORDER BY created_at`

	rows, err := q.Query(ctx, sql, householdID)
	if err != nil {
		return nil, fmt.Errorf("list household patients: %w", err)
	}
	defer rows.Close()

	return scanPatients(rows)
}
