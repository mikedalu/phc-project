package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/http/middleware"
	"github.com/mikedalu/phc-records/api/internal/service"
)

// Ward handles admissions and bed-days.
type Ward struct {
	svc *service.Ward
}

// NewWard builds the handler.
func NewWard(svc *service.Ward) *Ward { return &Ward{svc: svc} }

type admitRequest struct {
	PatientID   string `json:"patientId"`
	EncounterID string `json:"encounterId"`
	Ward        string `json:"ward"`
	BedNumber   string `json:"bedNumber"`
}

// Admit handles POST /api/v1/admissions.
func (h *Ward) Admit(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	var req admitRequest
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

	admission, err := h.svc.Admit(c.Request.Context(), service.AdmitInput{
		ActorID: actor.UserID, PatientID: patientID, EncounterID: encounterID,
		Ward: req.Ward, BedNumber: req.BedNumber,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	c.JSON(201, gin.H{"data": gin.H{"admission": gin.H{
		"id": admission.ID.String(), "patientId": admission.PatientID.String(),
		"ward": admission.Ward, "status": admission.Status,
	}}})
}

type accrueRequest struct {
	Night          string `json:"night"`
	TariffCode     string `json:"tariffCode"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
}

// AccrueNight handles POST /api/v1/admissions/:id/bed-days.
func (h *Ward) AccrueNight(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Admission id must be a UUID.", nil)
		return
	}

	var req accrueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	charge, billed, err := h.svc.AccrueNight(c.Request.Context(), service.AccrueInput{
		ActorID: actor.UserID, AdmissionID: id, Night: req.Night,
		TariffCode: req.TariffCode, UnitPriceMinor: req.UnitPriceMinor,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	status := 201
	if !billed {
		// The night was already billed by another device. 200, not 201: nothing
		// was created, and the caller should know it was a no-op.
		status = 200
	}

	c.JSON(status, gin.H{"data": gin.H{
		"alreadyBilled": !billed,
		"bedDay": gin.H{
			"id": charge.ID.String(), "night": charge.ChargeDate.Format("2006-01-02"),
			"amountMinor": charge.AmountMinor, "invoiceId": uuidOrNil(charge.InvoiceID),
		},
	}})
}

// Discharge handles POST /api/v1/admissions/:id/discharge.
func (h *Ward) Discharge(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Admission id must be a UUID.", nil)
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	_ = c.ShouldBindJSON(&req)

	admission, err := h.svc.Discharge(c.Request.Context(), actor.UserID, id, req.Notes)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, gin.H{"id": admission.ID.String(), "status": admission.Status})
}
