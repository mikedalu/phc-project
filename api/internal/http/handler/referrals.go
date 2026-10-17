package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/http/middleware"
	"github.com/mikedalu/phc-records/api/internal/service"
)

// Referrals handles issuing and reading referral notes.
type Referrals struct {
	svc *service.Referrals
}

// NewReferrals builds the handler.
func NewReferrals(svc *service.Referrals) *Referrals { return &Referrals{svc: svc} }

type createReferralRequest struct {
	PatientID        string `json:"patientId"`
	EncounterID      string `json:"encounterId"`
	ToFacility       string `json:"toFacility"`
	Reason           string `json:"reason"`
	DiagnosisSummary string `json:"diagnosisSummary"`
	TreatmentGiven   string `json:"treatmentGiven"`
	Urgency          string `json:"urgency"`
}

// Create handles POST /api/v1/referrals.
func (h *Referrals) Create(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	var req createReferralRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "patientId must be a UUID.", nil)
		return
	}
	encounterID, err := uuid.Parse(req.EncounterID)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "encounterId must be a UUID.", nil)
		return
	}

	referral, token, err := h.svc.CreateReferral(c.Request.Context(), service.CreateReferralInput{
		ActorID: actor.UserID, PatientID: patientID, EncounterID: encounterID,
		ToFacility: req.ToFacility, Reason: req.Reason,
		DiagnosisSummary: req.DiagnosisSummary, TreatmentGiven: req.TreatmentGiven,
		Urgency: req.Urgency,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	c.JSON(201, gin.H{"data": gin.H{
		"referral": gin.H{
			"id": referral.ID.String(), "referralNumber": referral.ReferralNumber,
			"toFacility": referral.ToFacility, "urgency": referral.Urgency,
			"expiresAt": referral.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		},
		// Returned ONCE. It is not stored, so it cannot be fetched again.
		"token": token,
	}})
}

// Read handles GET /api/v1/referrals/:token — PUBLIC, no session.
//
// The receiving facility has no account in this system and never will. Requiring
// one would make the referral useless at exactly the moment it matters.
func (h *Referrals) Read(c *gin.Context) {
	payload, err := h.svc.ReadReferral(c.Request.Context(), c.Param("token"))
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, payload)
}
