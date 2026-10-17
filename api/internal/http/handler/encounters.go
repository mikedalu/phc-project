package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/http/middleware"
	"github.com/mikedalu/phc-records/api/internal/service"
)

// Encounters handles the queue, transitions and vitals.
type Encounters struct {
	svc *service.Encounters
}

// NewEncounters builds the handler.
func NewEncounters(svc *service.Encounters) *Encounters { return &Encounters{svc: svc} }

type encounterDTO struct {
	ID             string  `json:"id"`
	VisitNumber    string  `json:"visitNumber"`
	PatientID      string  `json:"patientId"`
	HouseholdID    *string `json:"householdId,omitempty"`
	EncounterType  string  `json:"encounterType"`
	WorkflowStage  string  `json:"workflowStage"`
	Status         string  `json:"status"`
	Priority       int16   `json:"priority"`
	ChiefComplaint *string `json:"chiefComplaint,omitempty"`
	QueuedAt       string  `json:"queuedAt"`
	StartedAt      *string `json:"startedAt,omitempty"`
	// nextStages is what the UI may offer. Sent by the server so the client can
	// never render a button the API will refuse.
	NextStages []string `json:"nextStages"`
}

func toEncounterDTO(e *domain.Encounter) encounterDTO {
	var household, started *string
	if e.HouseholdID != nil {
		s := e.HouseholdID.String()
		household = &s
	}
	if e.StartedAt != nil {
		s := e.StartedAt.UTC().Format(time.RFC3339)
		started = &s
	}
	return encounterDTO{
		ID: e.ID.String(), VisitNumber: e.VisitNumber, PatientID: e.PatientID.String(),
		HouseholdID: household, EncounterType: e.EncounterType,
		WorkflowStage: e.WorkflowStage, Status: e.Status, Priority: e.Priority,
		ChiefComplaint: e.ChiefComplaint,
		QueuedAt:       e.QueuedAt.UTC().Format(time.RFC3339),
		StartedAt:      started,
		NextStages:     service.NextStages(e.WorkflowStage),
	}
}

type startEncounterRequest struct {
	PatientID      string `json:"patientId"`
	EncounterType  string `json:"encounterType"`
	Priority       int    `json:"priority"`
	ChiefComplaint string `json:"chiefComplaint"`
	VisitNumber    string `json:"visitNumber"`
}

// Start handles POST /api/v1/encounters.
func (h *Encounters) Start(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	var req startEncounterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}
	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "patientId must be a UUID.", nil)
		return
	}

	encounter, err := h.svc.Start(c.Request.Context(), service.StartEncounterInput{
		ActorID:        actor.UserID,
		PatientID:      patientID,
		EncounterType:  req.EncounterType,
		Priority:       req.Priority,
		ChiefComplaint: req.ChiefComplaint,
		VisitNumber:    req.VisitNumber,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	c.JSON(201, gin.H{"data": gin.H{"encounter": toEncounterDTO(encounter)}})
}

type transitionRequest struct {
	To string `json:"to"`
}

// Transition handles POST /api/v1/encounters/:id/transition.
func (h *Encounters) Transition(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Encounter id must be a UUID.", nil)
		return
	}

	var req transitionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.To == "" {
		httpx.BadRequest(c, "VALIDATION_FAILED", "A target stage is required.", nil)
		return
	}

	encounter, err := h.svc.Transition(c.Request.Context(), actor.UserID, id, req.To)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, toEncounterDTO(encounter))
}

// Queue handles GET /api/v1/queue.
func (h *Encounters) Queue(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	limit, err := intQuery(c, "limit", 0, 0, 200)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "limit must be between 0 and 200.", nil)
		return
	}

	queue, err := h.svc.Queue(c.Request.Context(), actor.UserID, int(limit))
	if err != nil {
		httpx.Error(c, err)
		return
	}

	out := make([]encounterDTO, 0, len(queue))
	for _, e := range queue {
		out = append(out, toEncounterDTO(e))
	}
	httpx.OK(c, gin.H{"encounters": out, "count": len(out)})
}

type vitalsRequest struct {
	EncounterID     *string  `json:"encounterId"`
	BPSystolic      *int     `json:"bpSystolic"`
	BPDiastolic     *int     `json:"bpDiastolic"`
	TemperatureC    *float64 `json:"temperatureC"`
	PulseRate       *int     `json:"pulseRate"`
	RespiratoryRate *int     `json:"respiratoryRate"`
	HeightCm        *float64 `json:"heightCm"`
	WeightKg        *float64 `json:"weightKg"`
	SpO2            *int     `json:"spo2"`
	PainScore       *int     `json:"painScore"`
	Notes           string   `json:"notes"`
}

// RecordVitals handles POST /api/v1/patients/:id/vitals.
func (h *Encounters) RecordVitals(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	patientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Patient id must be a UUID.", nil)
		return
	}

	var req vitalsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	var encounterID *uuid.UUID
	if req.EncounterID != nil && *req.EncounterID != "" {
		parsed, err := uuid.Parse(*req.EncounterID)
		if err != nil {
			httpx.BadRequest(c, "VALIDATION_FAILED", "encounterId must be a UUID.", nil)
			return
		}
		encounterID = &parsed
	}

	vitals, err := h.svc.RecordVitals(c.Request.Context(), service.RecordVitalsInput{
		ActorID:     actor.UserID,
		PatientID:   patientID,
		EncounterID: encounterID,
		BPSystolic:  req.BPSystolic, BPDiastolic: req.BPDiastolic,
		TemperatureC: req.TemperatureC, PulseRate: req.PulseRate,
		RespiratoryRate: req.RespiratoryRate,
		HeightCm:        req.HeightCm, WeightKg: req.WeightKg,
		SpO2: req.SpO2, PainScore: req.PainScore, Notes: req.Notes,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	c.JSON(201, gin.H{"data": gin.H{
		"id": vitals.ID.String(), "patientId": vitals.PatientID.String(),
		"recordedAt": vitals.RecordedAt.UTC().Format(time.RFC3339),
		"bmi":        vitals.BMI,
	}})
}

// ListVitals handles GET /api/v1/patients/:id/vitals.
func (h *Encounters) ListVitals(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	patientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Patient id must be a UUID.", nil)
		return
	}

	rows, err := h.svc.VitalsForPatient(c.Request.Context(), actor.UserID, patientID)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	out := make([]gin.H, 0, len(rows))
	for _, v := range rows {
		out = append(out, gin.H{
			"id": v.ID.String(), "recordedAt": v.RecordedAt.UTC().Format(time.RFC3339),
			"bpSystolic": v.BPSystolic, "bpDiastolic": v.BPDiastolic,
			"temperatureC": v.TemperatureC, "pulseRate": v.PulseRate,
			"respiratoryRate": v.RespiratoryRate,
			"heightCm":        v.HeightCm, "weightKg": v.WeightKg,
			"spo2": v.SpO2, "bmi": v.BMI, "painScore": v.PainScore, "notes": v.Notes,
		})
	}
	httpx.OK(c, gin.H{"vitals": out, "count": len(out)})
}
