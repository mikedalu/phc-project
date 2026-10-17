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

// Patients is the registry: registration, lookup, search and edits.
type Patients struct {
	pool     *pgxpool.Pool
	patients *repo.Patients
	users    *repo.Users
	now      func() time.Time
}

// NewPatients wires the service.
func NewPatients(pool *pgxpool.Pool) *Patients {
	return &Patients{
		pool:     pool,
		patients: repo.NewPatients(),
		users:    repo.NewUsers(pool),
		now:      time.Now,
	}
}

// Roles that may register a patient. Mirrors `patients.push_roles`.
var patientWriters = []domain.Role{
	domain.RoleFrontDesk, domain.RoleNurse, domain.RoleDoctor, domain.RoleAdmin,
}

const (
	maxNameLen  = 80
	maxPhoneLen = 20
	// A registration date more than a decade old is almost certainly a typo.
	maxBackdate = 10 * 365 * 24 * time.Hour
)

// CreatePatientInput is one registration.
type CreatePatientInput struct {
	ActorID uuid.UUID

	// PatientNumber is generated ON THE DEVICE, because registration happens
	// offline. The server accepts it and only generates one when it is absent
	// (the manual/API path).
	PatientNumber string
	HouseholdID   *uuid.UUID

	FirstName  string
	MiddleName *string
	Surname    string

	DateOfBirth *string // YYYY-MM-DD
	AgeYears    *int
	AgeMonths   *int

	Gender        string
	MaritalStatus *string

	Phone           *string
	AltPhone        *string
	Tribe           *string
	Occupation      *string
	Address         *string
	AddressLandmark *string

	BloodGroup        *string
	Genotype          *string
	Allergies         *string
	ChronicConditions *string

	RegistrationDate *string // YYYY-MM-DD
}

// PatientResult is a registration plus any near-matches the clerk should see.
type PatientResult struct {
	Patient *domain.Patient
	// PossibleDuplicates NEVER blocks the registration. A blocked clerk creates
	// a worse duplicate by hand — they invent a middle name to get past it.
	PossibleDuplicates []*domain.Patient
}

// Create registers a patient.
func (s *Patients) Create(ctx context.Context, in CreatePatientInput) (*PatientResult, error) {
	actor, err := s.requireWriter(ctx, in.ActorID)
	if err != nil {
		return nil, err
	}

	p, err := s.build(ctx, in)
	if err != nil {
		return nil, err
	}

	p.ID = uuid.New() // replaced by the client's id on the sync path
	p.CreatedBy = actor.ID
	p.UpdatedBy = actor.ID
	p.IsActive = true

	if err := s.patients.InsertPatient(ctx, s.pool, p); err != nil {
		return nil, err
	}

	dupes, err := s.patients.FindPossibleDuplicates(ctx, s.pool, p)
	if err != nil {
		// A failed duplicate check must not fail a successful registration.
		dupes = nil
	}

	return &PatientResult{Patient: p, PossibleDuplicates: dupes}, nil
}

// Update applies an edit.
func (s *Patients) Update(ctx context.Context, id uuid.UUID, in CreatePatientInput) (*domain.Patient, error) {
	actor, err := s.requireWriter(ctx, in.ActorID)
	if err != nil {
		return nil, err
	}

	existing, err := s.patients.FindPatientByID(ctx, s.pool, id)
	if err != nil {
		return nil, err
	}

	in.PatientNumber = existing.PatientNumber
	updated, err := s.build(ctx, in)
	if err != nil {
		return nil, err
	}
	updated.ID = id
	updated.CreatedAt = existing.CreatedAt
	updated.CreatedBy = existing.CreatedBy
	updated.UpdatedBy = actor.ID
	updated.RegistrationDate = existing.RegistrationDate
	updated.IsDeceased = existing.IsDeceased
	updated.DeceasedAt = existing.DeceasedAt
	updated.IsActive = existing.IsActive

	if err := s.patients.UpdatePatient(ctx, s.pool, updated); err != nil {
		return nil, err
	}

	return s.patients.FindPatientByID(ctx, s.pool, id)
}

// Get loads one patient.
func (s *Patients) Get(ctx context.Context, actorID, id uuid.UUID) (*domain.Patient, error) {
	if _, err := s.requireReader(ctx, actorID); err != nil {
		return nil, err
	}
	return s.patients.FindPatientByID(ctx, s.pool, id)
}

// Search returns patients matching a term, or the most recent when the term is
// empty.
func (s *Patients) Search(ctx context.Context, actorID uuid.UUID, term string, limit int) ([]*domain.Patient, error) {
	if _, err := s.requireReader(ctx, actorID); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 50 {
		limit = 50
	}

	term = strings.TrimSpace(term)
	if term == "" {
		return s.patients.ListPatients(ctx, s.pool, limit)
	}
	return s.patients.SearchPatients(ctx, s.pool, term, limit)
}

// build validates the input and derives whatever is missing.
func (s *Patients) build(ctx context.Context, in CreatePatientInput) (*domain.Patient, error) {
	first, err := requiredName(in.FirstName, "firstName")
	if err != nil {
		return nil, err
	}
	surname, err := requiredName(in.Surname, "surname")
	if err != nil {
		return nil, err
	}

	gender := strings.ToLower(strings.TrimSpace(in.Gender))
	switch gender {
	case "male", "female", "other", "unknown":
	default:
		return nil, domain.NewValidation("gender", "required_one_of_male_female_other_unknown")
	}

	marital := optionalString(in.MaritalStatus, 20)
	if marital != nil {
		switch *marital {
		case "single", "married", "divorced", "widowed", "separated", "unknown":
		default:
			return nil, domain.NewValidation("maritalStatus", "unknown_value")
		}
	}

	now := s.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	dob, ageYears, ageMonths, recordedAt, err := deriveAge(today, in.DateOfBirth, in.AgeYears, in.AgeMonths)
	if err != nil {
		return nil, err
	}

	registration := today
	if in.RegistrationDate != nil && *in.RegistrationDate != "" {
		parsed, err := parseDate(*in.RegistrationDate)
		if err != nil {
			return nil, domain.NewValidation("registrationDate", "must_be_yyyy_mm_dd")
		}
		if parsed.After(today) {
			return nil, domain.NewValidation("registrationDate", "cannot_be_in_the_future")
		}
		if today.Sub(parsed) > maxBackdate {
			return nil, domain.NewValidation("registrationDate", "too_far_in_the_past")
		}
		registration = parsed
	}

	number := strings.TrimSpace(in.PatientNumber)
	if number == "" {
		generated, err := s.generateNumber(ctx, today)
		if err != nil {
			return nil, err
		}
		number = generated
	}

	return &domain.Patient{
		PatientNumber:     number,
		HouseholdID:       in.HouseholdID,
		FirstName:         first,
		MiddleName:        optionalString(in.MiddleName, maxNameLen),
		Surname:           surname,
		DateOfBirth:       dob,
		AgeYears:          ageYears,
		AgeMonths:         ageMonths,
		AgeRecordedAt:     recordedAt,
		Gender:            gender,
		MaritalStatus:     marital,
		Phone:             optionalString(in.Phone, maxPhoneLen),
		AltPhone:          optionalString(in.AltPhone, maxPhoneLen),
		Tribe:             optionalString(in.Tribe, 60),
		Occupation:        optionalString(in.Occupation, 80),
		Address:           optionalString(in.Address, 300),
		AddressLandmark:   optionalString(in.AddressLandmark, 120),
		BloodGroup:        optionalString(in.BloodGroup, 3),
		Genotype:          optionalString(in.Genotype, 6),
		Allergies:         optionalString(in.Allergies, 500),
		ChronicConditions: optionalString(in.ChronicConditions, 500),
		RegistrationDate:  registration,
	}, nil
}

// deriveAge fills in whichever of DOB and age the clerk did not supply.
//
// ═══ WHY BOTH ARE ALWAYS SET ═══
//
// The encounter workflow needs an age for dosing, so "both unknown" is refused.
// A derived DOB is an APPROXIMATION and `age_recorded_at` records when it was
// taken — storing it without that marker would be a quiet lie in the record.
func deriveAge(today time.Time, dobRaw *string, years, months *int) (
	dob *time.Time, ageYears, ageMonths *int16, recordedAt *time.Time, err error,
) {
	hasDOB := dobRaw != nil && strings.TrimSpace(*dobRaw) != ""
	hasAge := years != nil || months != nil

	if !hasDOB && !hasAge {
		return nil, nil, nil, nil, domain.NewValidation("dateOfBirth",
			"either_date_of_birth_or_age_is_required")
	}

	if hasDOB {
		parsed, perr := parseDate(*dobRaw)
		if perr != nil {
			return nil, nil, nil, nil, domain.NewValidation("dateOfBirth", "must_be_yyyy_mm_dd")
		}
		if parsed.After(today) {
			return nil, nil, nil, nil, domain.NewValidation("dateOfBirth", "cannot_be_in_the_future")
		}

		y, m := wholeMonths(today, parsed)
		y16, m16 := int16(y), int16(m)
		return &parsed, &y16, &m16, &today, nil
	}

	// Age only: derive an approximate date of birth.
	totalMonths := 0
	if years != nil {
		totalMonths += *years * 12
	}
	if months != nil {
		totalMonths += *months
	}
	if totalMonths < 0 || totalMonths > 130*12 {
		return nil, nil, nil, nil, domain.NewValidation("ageYears", "out_of_range")
	}

	approx := today.AddDate(0, -totalMonths, 0)
	y := totalMonths / 12
	m := totalMonths
	y16, m16 := int16(y), int16(m)
	return &approx, &y16, &m16, &today, nil
}

// wholeMonths returns complete years and complete months between two dates.
func wholeMonths(from, to time.Time) (years, months int) {
	months = int(from.Sub(to).Hours() / (24 * 30.4375))
	if months < 0 {
		months = 0
	}
	return months / 12, months
}

func parseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(s), time.UTC)
}

func requiredName(raw, field string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", domain.NewValidation(field, "required")
	}
	if len(value) > maxNameLen {
		return "", domain.NewValidationf(field, "too_long", "%d characters maximum", maxNameLen)
	}
	return value, nil
}

func optionalString(raw *string, max int) *string {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil
	}
	if len(value) > max {
		value = value[:max]
	}
	return &value
}

// generateNumber builds a fallback number for the manual/API path.
//
// The DEVICE normally generates this (see 01-DATA-MODEL.md §14): it embeds a
// short device prefix so two offline devices cannot collide. This server-side
// fallback uses random hex for the same reason — a database sequence would be
// unavailable to the offline client anyway, so the two paths must be
// collision-free by construction rather than by luck.
func (s *Patients) generateNumber(ctx context.Context, today time.Time) (string, error) {
	const sql = `SELECT code FROM facilities WHERE deleted_at IS NULL LIMIT 1`

	var code string
	if err := s.pool.QueryRow(ctx, sql).Scan(&code); err != nil {
		return "", fmt.Errorf("read facility code: %w", err)
	}

	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate patient number: %w", err)
	}

	return fmt.Sprintf("%s/PAT/%d/%s", code, today.Year(), strings.ToUpper(hex.EncodeToString(buf))), nil
}

func (s *Patients) requireWriter(ctx context.Context, actorID uuid.UUID) (*domain.User, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, patientWriters...) {
		return nil, domain.Public(domain.ErrForbidden,
			"Your role does not permit registering patients.")
	}
	return actor, nil
}

// requireReader is deliberately permissive: every member of staff may look a
// patient up. The registry is not the sensitive part — the clinical record is.
func (s *Patients) requireReader(ctx context.Context, actorID uuid.UUID) (*domain.User, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if len(actor.Roles) == 0 {
		return nil, domain.Public(domain.ErrForbidden, "This account has no role assigned.")
	}
	return actor, nil
}
