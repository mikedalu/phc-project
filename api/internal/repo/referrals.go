package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Referrals stores issued notes.
type Referrals struct{}

// NewReferrals builds the repository.
func NewReferrals() *Referrals { return &Referrals{} }

// InsertReferral writes the facility's record of a note.
func (r *Referrals) InsertReferral(ctx context.Context, q Querier, ref *domain.Referral) error {
	const sql = `
        INSERT INTO referrals (
            id, referral_number, patient_id, encounter_id, patient_name,
            patient_age_years, patient_sex, from_facility, to_facility,
            reason, diagnosis_summary, treatment_given, urgency, status,
            issued_at, expires_at, issued_by, created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$18)`

	if _, err := q.Exec(ctx, sql,
		ref.ID, ref.ReferralNumber, ref.PatientID, ref.EncounterID, ref.PatientName,
		ref.PatientAgeYears, ref.PatientSex, ref.FromFacility, ref.ToFacility,
		ref.Reason, ref.DiagnosisSummary, ref.TreatmentGiven, ref.Urgency, ref.Status,
		ref.IssuedAt, ref.ExpiresAt, ref.IssuedBy, ref.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert referral: %w", err)
	}
	return nil
}

// FindByNumber looks a referral up by its number, for revocation checks.
func (r *Referrals) FindByNumber(ctx context.Context, q Querier, number string) (*domain.Referral, error) {
	const sql = `
        SELECT id, referral_number, patient_id, encounter_id, patient_name,
               patient_age_years, patient_sex, from_facility, to_facility,
               reason, diagnosis_summary, treatment_given, urgency, status,
               issued_at, expires_at, issued_by, revoked_at, revoked_by,
               revoke_reason, created_at, updated_at, created_by, updated_by
          FROM referrals
         WHERE referral_number = $1 AND deleted_at IS NULL`

	var ref domain.Referral
	err := q.QueryRow(ctx, sql, number).Scan(
		&ref.ID, &ref.ReferralNumber, &ref.PatientID, &ref.EncounterID, &ref.PatientName,
		&ref.PatientAgeYears, &ref.PatientSex, &ref.FromFacility, &ref.ToFacility,
		&ref.Reason, &ref.DiagnosisSummary, &ref.TreatmentGiven, &ref.Urgency, &ref.Status,
		&ref.IssuedAt, &ref.ExpiresAt, &ref.IssuedBy, &ref.RevokedAt, &ref.RevokedBy,
		&ref.RevokeReason, &ref.CreatedAt, &ref.UpdatedAt, &ref.CreatedBy, &ref.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find referral: %w", err)
	}
	return &ref, nil
}
