package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// ReferralTokenPayload is what leaves the building inside the QR code.
//
// ═══ WHAT IS DELIBERATELY ABSENT ═══
//
// No patient UUID, no phone number, no address, no NHIS number. The receiving
// facility needs enough to treat the patient standing in front of them, and
// nothing that would let the token be used to look someone up later. A referral
// that leaks a full identity is worse than no referral.
type ReferralTokenPayload struct {
	Version          int     `json:"v"`
	ReferralNumber   string  `json:"ref"`
	PatientName      string  `json:"name"`
	PatientAgeYears  *int16  `json:"age,omitempty"`
	PatientSex       *string `json:"sex,omitempty"`
	FromFacility     string  `json:"from"`
	ToFacility       string  `json:"to"`
	Reason           string  `json:"reason"`
	DiagnosisSummary *string `json:"dx,omitempty"`
	TreatmentGiven   *string `json:"tx,omitempty"`
	Urgency          string  `json:"urgency"`
	IssuedAt         string  `json:"iat"`
	ExpiresAt        string  `json:"exp"`
}

// Referrals issues and reads referral notes.
type Referrals struct {
	pool       *pgxpool.Pool
	repo       *repo.Referrals
	patients   *repo.Patients
	encounters *repo.Encounters
	users      *repo.Users
	key        []byte
	ttl        time.Duration
	now        func() time.Time
}

// NewReferrals wires the service.
func NewReferrals(pool *pgxpool.Pool, key []byte, ttl time.Duration) *Referrals {
	return &Referrals{
		pool:       pool,
		repo:       repo.NewReferrals(),
		patients:   repo.NewPatients(),
		encounters: repo.NewEncounters(),
		users:      repo.NewUsers(pool),
		key:        key,
		ttl:        ttl,
		now:        time.Now,
	}
}

// CreateReferralInput is one referral.
type CreateReferralInput struct {
	ActorID          uuid.UUID
	PatientID        uuid.UUID
	EncounterID      uuid.UUID
	ToFacility       string
	Reason           string
	DiagnosisSummary string
	TreatmentGiven   string
	Urgency          string
}

// CreateReferral issues a note and returns it with its token.
func (s *Referrals) CreateReferral(ctx context.Context, in CreateReferralInput) (*domain.Referral, string, error) {
	actor, err := s.users.FindByID(ctx, s.pool, in.ActorID)
	if err != nil {
		return nil, "", err
	}
	if !actor.IsActive {
		return nil, "", domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasAnyRole(actor.Roles, domain.RoleDoctor, domain.RoleNurse, domain.RoleAdmin) {
		return nil, "", domain.Public(domain.ErrForbidden, "Your role cannot issue a referral.")
	}

	if len(s.key) < 16 {
		return nil, "", domain.Public(domain.ErrValidation,
			"Referral encryption is not configured on this server.")
	}

	patient, err := s.patients.FindPatientByID(ctx, s.pool, in.PatientID)
	if err != nil {
		return nil, "", err
	}
	encounter, err := s.encounters.FindEncounterByID(ctx, s.pool, in.EncounterID)
	if err != nil {
		return nil, "", err
	}
	if encounter.PatientID != patient.ID {
		return nil, "", domain.NewValidation("encounterId", "does_not_belong_to_patient")
	}

	to := strings.TrimSpace(in.ToFacility)
	if to == "" {
		return nil, "", domain.NewValidation("toFacility", "required")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, "", domain.NewValidation("reason", "required")
	}

	urgency := strings.TrimSpace(in.Urgency)
	if urgency == "" {
		urgency = "routine"
	}
	switch urgency {
	case "routine", "urgent", "emergency":
	default:
		return nil, "", domain.NewValidation("urgency", "unknown_value")
	}

	number, err := s.nextNumber(ctx)
	if err != nil {
		return nil, "", err
	}

	now := s.now()
	expires := now.Add(s.ttl)

	name := patient.FirstName + " " + patient.Surname
	sex := string(patient.Gender)

	referral := &domain.Referral{
		ID:               uuid.New(),
		ReferralNumber:   number,
		PatientID:        patient.ID,
		EncounterID:      encounter.ID,
		PatientName:      name,
		PatientAgeYears:  patient.AgeYears,
		PatientSex:       &sex,
		FromFacility:     "Demo Primary Health Centre",
		ToFacility:       to,
		Reason:           reason,
		DiagnosisSummary: optionalString(&in.DiagnosisSummary, 500),
		TreatmentGiven:   optionalString(&in.TreatmentGiven, 500),
		Urgency:          urgency,
		Status:           "issued",
		IssuedAt:         now,
		ExpiresAt:        expires,
		IssuedBy:         actor.ID,
		CreatedBy:        actor.ID,
		UpdatedBy:        actor.ID,
	}

	payload := ReferralTokenPayload{
		Version:          1,
		ReferralNumber:   number,
		PatientName:      name,
		PatientAgeYears:  patient.AgeYears,
		PatientSex:       &sex,
		FromFacility:     referral.FromFacility,
		ToFacility:       to,
		Reason:           reason,
		DiagnosisSummary: referral.DiagnosisSummary,
		TreatmentGiven:   referral.TreatmentGiven,
		Urgency:          urgency,
		IssuedAt:         now.UTC().Format(time.RFC3339),
		ExpiresAt:        expires.UTC().Format(time.RFC3339),
	}

	token, err := s.seal(payload)
	if err != nil {
		return nil, "", err
	}

	if err := s.repo.InsertReferral(ctx, s.pool, referral); err != nil {
		return nil, "", err
	}

	// The TOKEN is returned once and never stored: the database keeps the record
	// of the referral, and the QR code carries the rest. Storing it would make
	// the encryption pointless.
	return referral, token, nil
}

// ReadReferral opens a token. Public: the receiving facility has no account.
func (s *Referrals) ReadReferral(ctx context.Context, token string) (*ReferralTokenPayload, error) {
	payload, err := s.open(token)
	if err != nil {
		// Never explain WHY a token failed: "bad signature" tells an attacker
		// they got close.
		return nil, domain.Public(domain.ErrValidation, "This referral code is not valid.")
	}

	expires, err := time.Parse(time.RFC3339, payload.ExpiresAt)
	if err != nil {
		return nil, domain.Public(domain.ErrValidation, "This referral code is not valid.")
	}
	if s.now().After(expires) {
		return nil, domain.Public(domain.ErrValidation,
			"This referral code has expired. Ask the referring clinic for a new one.")
	}

	// A revoked referral must stop working immediately, even inside its window.
	referral, err := s.repo.FindByNumber(ctx, s.pool, payload.ReferralNumber)
	if err == nil && referral.Status == "revoked" {
		return nil, domain.Public(domain.ErrValidation, "This referral was cancelled by the issuing clinic.")
	}

	return &payload, nil
}

// ─── crypto ────────────────────────────────────────────────────────

// seal encrypts the payload with AES-256-GCM.
//
// GCM, not a bare signature: the payload contains clinical data, so it must be
// unreadable to anyone without the key. GCM also authenticates, so a modified
// token fails to open rather than decrypting to nonsense.
func (s *Referrals) seal(payload ReferralTokenPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal referral: %w", err)
	}

	key := sha256.Sum256(s.key) // accept any configured length, use 32 bytes

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, raw, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (s *Referrals) open(token string) (ReferralTokenPayload, error) {
	var payload ReferralTokenPayload

	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return payload, fmt.Errorf("decode: %w", err)
	}

	key := sha256.Sum256(s.key)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return payload, fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return payload, fmt.Errorf("gcm: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return payload, fmt.Errorf("token too short")
	}

	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return payload, fmt.Errorf("open: %w", err)
	}

	if err := json.Unmarshal(plain, &payload); err != nil {
		return payload, fmt.Errorf("unmarshal: %w", err)
	}
	return payload, nil
}

func (s *Referrals) nextNumber(ctx context.Context) (string, error) {
	const sql = `SELECT code FROM facilities WHERE deleted_at IS NULL LIMIT 1`
	var code string
	if err := s.pool.QueryRow(ctx, sql).Scan(&code); err != nil {
		return "", fmt.Errorf("read facility code: %w", err)
	}
	return fmt.Sprintf("%s/REF/%d/%s", code, s.now().Year(), strings.ToUpper(uuid.NewString()[:6])), nil
}
