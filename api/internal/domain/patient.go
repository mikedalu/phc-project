package domain

import (
	"time"

	"github.com/google/uuid"
)

// Household is the billing party: a family, or a single patient's own group.
type Household struct {
	ID            uuid.UUID
	FamilyNumber  string
	FamilyName    string
	HeadPatientID *uuid.UUID
	Phone         *string
	Address       *string
	Notes         *string
	IsActive      bool

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// Patient is one registered person.
//
// ═══ BOTH date_of_birth AND age_years EXIST, ON PURPOSE ═══
//
// A PHC genuinely does not always know a date of birth — a mother knows her
// child is "about two". The service derives whichever is missing and refuses to
// leave BOTH empty, because the encounter workflow needs an age for dosing.
// Storing a derived DOB as if it were exact would be a quiet lie in the record;
// age_recorded_at marks when the estimate was taken.
type Patient struct {
	ID            uuid.UUID
	PatientNumber string
	HouseholdID   *uuid.UUID

	FirstName  string
	MiddleName *string
	Surname    string

	DateOfBirth   *time.Time
	AgeYears      *int16
	AgeMonths     *int16
	AgeRecordedAt *time.Time

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

	IsDeceased bool
	DeceasedAt *time.Time

	RegistrationDate time.Time
	IsActive         bool

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// DisplayName is "First Surname" — the form used in every list and header.
func (p *Patient) DisplayName() string {
	return p.FirstName + " " + p.Surname
}

// AgeAt renders the age the way a clinician reads it: months under two, years
// after. Returns nil when neither is known, which the service makes impossible
// for a stored patient.
func (p *Patient) AgeAt(now time.Time) string {
	if p.AgeYears == nil && p.AgeMonths == nil {
		return ""
	}
	if p.AgeMonths != nil && *p.AgeMonths < 24 {
		return plural(int(*p.AgeMonths), "month")
	}
	if p.AgeYears != nil {
		return plural(int(*p.AgeYears), "year")
	}
	return ""
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return itoa(n) + " " + unit + "s"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
