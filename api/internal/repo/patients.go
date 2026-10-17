package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Patients holds the registry SQL.
type Patients struct{}

// NewPatients builds the repository.
func NewPatients() *Patients { return &Patients{} }

const patientColumns = `
    id, patient_number, household_id,
    first_name, middle_name, surname,
    date_of_birth, age_years, age_months, age_recorded_at,
    gender, marital_status,
    phone, alt_phone, tribe, occupation, address, address_landmark,
    blood_group, genotype, allergies, chronic_conditions,
    is_deceased, deceased_at,
    registration_date, is_active,
    created_at, updated_at, created_by, updated_by`

const householdColumns = `
    id, family_number, family_name, head_patient_id,
    phone, address, notes, is_active,
    created_at, updated_at, created_by, updated_by`

// InsertPatient writes a new patient.
func (r *Patients) InsertPatient(ctx context.Context, q Querier, p *domain.Patient) error {
	const sql = `
        INSERT INTO patients (
            id, patient_number, household_id,
            first_name, middle_name, surname,
            date_of_birth, age_years, age_months, age_recorded_at,
            gender, marital_status,
            phone, alt_phone, tribe, occupation, address, address_landmark,
            blood_group, genotype, allergies, chronic_conditions,
            is_deceased, deceased_at,
            registration_date, is_active,
            created_by, updated_by
        ) VALUES (
            $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,
            $19,$20,$21,$22,$23,$24,$25,$26,$27,$27
        )`

	if _, err := q.Exec(ctx, sql,
		p.ID, p.PatientNumber, p.HouseholdID,
		p.FirstName, p.MiddleName, p.Surname,
		p.DateOfBirth, p.AgeYears, p.AgeMonths, p.AgeRecordedAt,
		p.Gender, p.MaritalStatus,
		p.Phone, p.AltPhone, p.Tribe, p.Occupation, p.Address, p.AddressLandmark,
		p.BloodGroup, p.Genotype, p.Allergies, p.ChronicConditions,
		p.IsDeceased, p.DeceasedAt,
		p.RegistrationDate, p.IsActive,
		p.CreatedBy,
	); err != nil {
		if isUniqueViolation(err) {
			// A duplicate NUMBER is a normal offline outcome — two devices can
			// pick the same sequence. It must be a handleable conflict, not a 500.
			return fmt.Errorf("%w: patient number already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert patient: %w", err)
	}
	return nil
}

// FindPatientByID loads one patient.
func (r *Patients) FindPatientByID(ctx context.Context, q Querier, id uuid.UUID) (*domain.Patient, error) {
	const sql = `SELECT` + patientColumns + ` FROM patients WHERE id = $1 AND deleted_at IS NULL`

	var p domain.Patient
	err := q.QueryRow(ctx, sql, id).Scan(
		&p.ID, &p.PatientNumber, &p.HouseholdID,
		&p.FirstName, &p.MiddleName, &p.Surname,
		&p.DateOfBirth, &p.AgeYears, &p.AgeMonths, &p.AgeRecordedAt,
		&p.Gender, &p.MaritalStatus,
		&p.Phone, &p.AltPhone, &p.Tribe, &p.Occupation, &p.Address, &p.AddressLandmark,
		&p.BloodGroup, &p.Genotype, &p.Allergies, &p.ChronicConditions,
		&p.IsDeceased, &p.DeceasedAt,
		&p.RegistrationDate, &p.IsActive,
		&p.CreatedAt, &p.UpdatedAt, &p.CreatedBy, &p.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find patient: %w", err)
	}
	return &p, nil
}

// SearchPatients finds patients by number, phone, or name prefix.
//
// The name predicates are written as `lower(x) LIKE lower($) || '%'` so they hit
// `idx_patients_name`; wrapping the COLUMN in a function instead of the
// parameter (e.g. `lower(surname || '%')`) would force a sequential scan and
// quietly get slower every month.
func (r *Patients) SearchPatients(ctx context.Context, q Querier, term string, limit int) ([]*domain.Patient, error) {
	const sql = `
        SELECT` + patientColumns + `
          FROM patients
         WHERE deleted_at IS NULL
           AND is_active
           AND (
                lower(surname) LIKE lower($1) || '%'
             OR lower(first_name) LIKE lower($1) || '%'
             OR patient_number = $2
             OR phone = $2
           )
         ORDER BY surname, first_name
         LIMIT $3`

	rows, err := q.Query(ctx, sql, term, term, limit)
	if err != nil {
		return nil, fmt.Errorf("search patients: %w", err)
	}
	defer rows.Close()

	return scanPatients(rows)
}

// ListPatients returns the most recently registered patients.
func (r *Patients) ListPatients(ctx context.Context, q Querier, limit int) ([]*domain.Patient, error) {
	const sql = `
        SELECT` + patientColumns + `
          FROM patients
         WHERE deleted_at IS NULL
         ORDER BY created_at DESC
         LIMIT $1`

	rows, err := q.Query(ctx, sql, limit)
	if err != nil {
		return nil, fmt.Errorf("list patients: %w", err)
	}
	defer rows.Close()

	return scanPatients(rows)
}

// FindPossibleDuplicates looks for people who are probably the same person.
//
// Deliberately CHEAP and forgiving: the same surname, a matching first name
// initial, and either a shared date of birth or a shared phone. A fuzzy score
// would be nicer; this catches the real cases (the clerk types the name twice)
// without a trigram index and without blocking the registration.
func (r *Patients) FindPossibleDuplicates(ctx context.Context, q Querier, p *domain.Patient) ([]*domain.Patient, error) {
	const sql = `
        SELECT` + patientColumns + `
          FROM patients
         WHERE deleted_at IS NULL
           AND id <> $1
           AND lower(surname) = lower($2)
           AND lower(first_name) LIKE lower($3) || '%'
           AND (
                ($4::date IS NOT NULL AND date_of_birth = $4::date)
             OR ($5::text IS NOT NULL AND phone = $5::text)
             OR ($4::date IS NULL AND $5::text IS NULL)
           )
         ORDER BY created_at DESC
         LIMIT 5`

	var dob any
	if p.DateOfBirth != nil {
		dob = *p.DateOfBirth
	}
	var phone any
	if p.Phone != nil && *p.Phone != "" {
		phone = *p.Phone
	}

	rows, err := q.Query(ctx, sql, p.ID, p.Surname, firstInitial(p.FirstName), dob, phone)
	if err != nil {
		return nil, fmt.Errorf("find duplicates: %w", err)
	}
	defer rows.Close()

	return scanPatients(rows)
}

// UpdatePatient writes the editable columns of an existing patient.
func (r *Patients) UpdatePatient(ctx context.Context, q Querier, p *domain.Patient) error {
	const sql = `
        UPDATE patients
           SET household_id = $2, first_name = $3, middle_name = $4, surname = $5,
               date_of_birth = $6, age_years = $7, age_months = $8, age_recorded_at = $9,
               gender = $10, marital_status = $11,
               phone = $12, alt_phone = $13, tribe = $14, occupation = $15,
               address = $16, address_landmark = $17,
               blood_group = $18, genotype = $19, allergies = $20, chronic_conditions = $21,
               is_deceased = $22, deceased_at = $23, is_active = $24,
               updated_by = $25, updated_at = now(), revision = revision + 1,
               server_rev = server_rev + 1, sync_state = 'synced'
         WHERE id = $1 AND deleted_at IS NULL`

	if _, err := q.Exec(ctx, sql,
		p.ID, p.HouseholdID, p.FirstName, p.MiddleName, p.Surname,
		p.DateOfBirth, p.AgeYears, p.AgeMonths, p.AgeRecordedAt,
		p.Gender, p.MaritalStatus,
		p.Phone, p.AltPhone, p.Tribe, p.Occupation,
		p.Address, p.AddressLandmark,
		p.BloodGroup, p.Genotype, p.Allergies, p.ChronicConditions,
		p.IsDeceased, p.DeceasedAt, p.IsActive,
		p.UpdatedBy,
	); err != nil {
		return fmt.Errorf("update patient: %w", err)
	}
	return nil
}

func scanPatients(rows pgx.Rows) ([]*domain.Patient, error) {
	var out []*domain.Patient
	for rows.Next() {
		var p domain.Patient
		if err := rows.Scan(
			&p.ID, &p.PatientNumber, &p.HouseholdID,
			&p.FirstName, &p.MiddleName, &p.Surname,
			&p.DateOfBirth, &p.AgeYears, &p.AgeMonths, &p.AgeRecordedAt,
			&p.Gender, &p.MaritalStatus,
			&p.Phone, &p.AltPhone, &p.Tribe, &p.Occupation, &p.Address, &p.AddressLandmark,
			&p.BloodGroup, &p.Genotype, &p.Allergies, &p.ChronicConditions,
			&p.IsDeceased, &p.DeceasedAt,
			&p.RegistrationDate, &p.IsActive,
			&p.CreatedAt, &p.UpdatedAt, &p.CreatedBy, &p.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan patient: %w", err)
		}
		out = append(out, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate patients: %w", err)
	}
	return out, nil
}

func firstInitial(s string) string {
	if s == "" {
		return ""
	}
	return string([]rune(s)[0])
}
