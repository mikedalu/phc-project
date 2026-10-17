package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// Households implements family registration and family views.
type Households struct {
	pool       *pgxpool.Pool
	households *repo.Households
	patients   *repo.Patients
	users      *repo.Users
	patientSvc *Patients
	now        func() time.Time
}

// NewHouseholds wires the service.
func NewHouseholds(pool *pgxpool.Pool) *Households {
	return &Households{
		pool:       pool,
		households: repo.NewHouseholds(),
		patients:   repo.NewPatients(),
		users:      repo.NewUsers(pool),
		patientSvc: NewPatients(pool),
		now:        time.Now,
	}
}

const (
	maxFamilyMembers = 20
	maxFamilyNameLen = 80
)

// FamilyMemberInput is one member of a family being registered.
type FamilyMemberInput struct {
	CreatePatientInput
	IsHead bool
}

// RegisterFamilyInput is one family registration.
type RegisterFamilyInput struct {
	ActorID    uuid.UUID
	FamilyName string
	Phone      string
	Address    string
	Notes      string
	Members    []FamilyMemberInput
}

// FamilyResult is the created household and its members.
type FamilyResult struct {
	Household *domain.Household
	Members   []*domain.Patient
}

// RegisterFamily registers a household and all its members in ONE transaction.
//
// ═══ ALL OR NOTHING ═══
//
// A family half-registered is worse than one not registered at all: the clerk
// has no way to tell which members exist, re-registering duplicates the ones
// that did land, and the household number points at an incomplete family. The
// transaction also lets us satisfy the circular FK properly — household first
// with a NULL head, members next, then the head is set.
func (s *Households) RegisterFamily(ctx context.Context, in RegisterFamilyInput) (*FamilyResult, error) {
	actor, err := s.requireFamilyWriter(ctx, in.ActorID)
	if err != nil {
		return nil, err
	}

	familyName := strings.TrimSpace(in.FamilyName)
	if familyName == "" {
		return nil, domain.NewValidation("familyName", "required")
	}
	if len(familyName) > maxFamilyNameLen {
		return nil, domain.NewValidationf("familyName", "too_long",
			"%d characters maximum", maxFamilyNameLen)
	}
	if len(in.Members) == 0 {
		return nil, domain.NewValidation("members", "at_least_one_required")
	}
	if len(in.Members) > maxFamilyMembers {
		return nil, domain.NewValidationf("members", "too_many",
			"%d members maximum", maxFamilyMembers)
	}

	headIndex := -1
	for i, member := range in.Members {
		if member.IsHead {
			if headIndex >= 0 {
				return nil, domain.NewValidation("members", "only_one_head")
			}
			headIndex = i
		}
	}
	// Defaulting to the first member is kinder than refusing: the clerk who
	// ticks nothing still gets a household, and the head can be changed later.
	if headIndex < 0 {
		headIndex = 0
	}

	now := s.now()
	householdID := uuid.New()

	familyNumber, err := s.generateFamilyNumber(ctx, now)
	if err != nil {
		return nil, err
	}

	household := &domain.Household{
		ID:           householdID,
		FamilyNumber: familyNumber,
		FamilyName:   familyName,
		Phone:        optionalFamilyText(in.Phone, maxPhoneLen),
		Address:      optionalFamilyText(in.Address, 300),
		Notes:        optionalFamilyText(in.Notes, 500),
		IsActive:     true,
		CreatedBy:    actor.ID,
		UpdatedBy:    actor.ID,
	}

	created := make([]*domain.Patient, 0, len(in.Members))

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.households.InsertHousehold(ctx, tx, household); err != nil {
			return err
		}

		for _, member := range in.Members {
			member.CreatePatientInput.ActorID = actor.ID
			member.CreatePatientInput.HouseholdID = &householdID

			patient, err := s.patientSvc.build(ctx, member.CreatePatientInput)
			if err != nil {
				return err
			}

			patient.ID = uuid.New()
			patient.CreatedBy = actor.ID
			patient.UpdatedBy = actor.ID
			patient.IsActive = true

			if err := s.patients.InsertPatient(ctx, tx, patient); err != nil {
				return err
			}
			created = append(created, patient)
		}

		// The head LAST: the row must exist before it can be referenced.
		if err := s.households.SetHeadPatient(ctx, tx, householdID, created[headIndex].ID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	household.HeadPatientID = &created[headIndex].ID

	log.Info().
		Str("household_id", householdID.String()).
		Str("family_number", familyNumber).
		Int("members", len(created)).
		Msg("family registered")

	return &FamilyResult{Household: household, Members: created}, nil
}

// HouseholdView is a family with its members.
type HouseholdView struct {
	Household *domain.Household
	Members   []*domain.Patient
	// BalanceMinor is derived, never stored. Zero until billing exists (S6).
	BalanceMinor int64
}

// Get loads a household and its members.
func (s *Households) Get(ctx context.Context, actorID, id uuid.UUID) (*HouseholdView, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}

	household, err := s.households.FindHouseholdByID(ctx, s.pool, id)
	if err != nil {
		return nil, err
	}

	members, err := s.households.PatientsForHousehold(ctx, s.pool, id)
	if err != nil {
		return nil, err
	}

	return &HouseholdView{Household: household, Members: members, BalanceMinor: 0}, nil
}

// requireFamilyWriter is narrower than patient registration: a family is a
// billing party, and the contract gives it to the front desk and admins.
func (s *Households) requireFamilyWriter(ctx context.Context, actorID uuid.UUID) (*domain.User, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleFrontDesk, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden,
			"Only front desk or an administrator can register a family.")
	}
	return actor, nil
}

func (s *Households) generateFamilyNumber(ctx context.Context, today time.Time) (string, error) {
	const sql = `SELECT code FROM facilities WHERE deleted_at IS NULL LIMIT 1`

	var code string
	if err := s.pool.QueryRow(ctx, sql).Scan(&code); err != nil {
		return "", fmt.Errorf("read facility code: %w", err)
	}

	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate family number: %w", err)
	}

	return fmt.Sprintf("%s/FAM/%d/%s", code, today.Year(), strings.ToUpper(hex.EncodeToString(buf))), nil
}

func optionalFamilyText(raw string, max int) *string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if len(value) > max {
		value = value[:max]
	}
	return &value
}
