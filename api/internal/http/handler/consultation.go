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

// Consultation handles diagnoses.
type Consultation struct {
	svc *service.Consultation
}

// NewConsultation builds the handler.
func NewConsultation(svc *service.Consultation) *Consultation { return &Consultation{svc: svc} }

type diagnosisRequest struct {
	PatientID   string `json:"patientId"`
	EncounterID string `json:"encounterId"`
	Type        string `json:"diagnosisType"`
	ICD10Code   string `json:"icd10Code"`
	Description string `json:"description"`
	Notes       string `json:"clinicalNotes"`
}

type diagnosisDTO struct {
	ID            string  `json:"id"`
	PatientID     string  `json:"patientId"`
	EncounterID   string  `json:"encounterId"`
	Type          string  `json:"diagnosisType"`
	ICD10Code     *string `json:"icd10Code,omitempty"`
	Description   string  `json:"description"`
	ClinicalNotes *string `json:"clinicalNotes,omitempty"`
	DiagnosedAt   string  `json:"diagnosedAt"`
	DiagnosedBy   string  `json:"diagnosedBy"`
}

func toDiagnosisDTO(d *domain.Diagnosis) diagnosisDTO {
	return diagnosisDTO{
		ID: d.ID.String(), PatientID: d.PatientID.String(), EncounterID: d.EncounterID.String(),
		Type: d.Type, ICD10Code: d.ICD10Code, Description: d.Description,
		ClinicalNotes: d.ClinicalNotes,
		DiagnosedAt:   d.DiagnosedAt.UTC().Format(time.RFC3339),
		DiagnosedBy:   d.DiagnosedBy.String(),
	}
}

// CreateDiagnosis handles POST /api/v1/consultations/:encounterId/diagnoses.
func (h *Consultation) CreateDiagnosis(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	encounterID, err := uuid.Parse(c.Param("encounterId"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "encounterId must be a UUID.", nil)
		return
	}

	var req diagnosisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "patientId must be a UUID.", nil)
		return
	}

	diagnosis, err := h.svc.CreateDiagnosis(c.Request.Context(), service.CreateDiagnosisInput{
		ActorID: actor.UserID, PatientID: patientID, EncounterID: encounterID,
		Type: req.Type, ICD10Code: req.ICD10Code, Description: req.Description, Notes: req.Notes,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	c.JSON(201, gin.H{"data": gin.H{"diagnosis": toDiagnosisDTO(diagnosis)}})
}

// ListDiagnoses handles GET /api/v1/patients/:id/diagnoses.
func (h *Consultation) ListDiagnoses(c *gin.Context) {
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

	rows, err := h.svc.DiagnosesForPatient(c.Request.Context(), actor.UserID, patientID)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	out := make([]diagnosisDTO, 0, len(rows))
	for _, d := range rows {
		out = append(out, toDiagnosisDTO(d))
	}
	httpx.OK(c, gin.H{"diagnoses": out, "count": len(out)})
}

type labOrderRequest struct {
	PatientID   string `json:"patientId"`
	TestCode    string `json:"testCode"`
	TestName    string `json:"testName"`
	Specimen    string `json:"specimenType"`
	Indication  string `json:"clinicalIndication"`
	Priority    string `json:"priority"`
	OrderNumber string `json:"orderNumber"`
	// TariffCode bills the test in the same transaction that creates it.
	TariffCode string `json:"tariffCode"`
}

type labOrderDTO struct {
	ID          string  `json:"id"`
	PatientID   string  `json:"patientId"`
	EncounterID string  `json:"encounterId"`
	OrderNumber string  `json:"orderNumber"`
	TestCode    string  `json:"testCode"`
	TestName    string  `json:"testName"`
	Specimen    *string `json:"specimenType,omitempty"`
	Indication  *string `json:"clinicalIndication,omitempty"`
	Priority    string  `json:"priority"`
	Status      string  `json:"status"`
	OrderedAt   string  `json:"orderedAt"`
	CollectedAt *string `json:"collectedAt,omitempty"`
	// Set when the order was billed.
	InvoiceID *string `json:"invoiceId,omitempty"`
}

func toLabOrderDTO(o *domain.LabOrder) labOrderDTO {
	var collected *string
	if o.CollectedAt != nil {
		v := o.CollectedAt.UTC().Format(time.RFC3339)
		collected = &v
	}
	return labOrderDTO{
		ID: o.ID.String(), PatientID: o.PatientID.String(), EncounterID: o.EncounterID.String(),
		OrderNumber: o.OrderNumber, TestCode: o.TestCode, TestName: o.TestName,
		Specimen: o.SpecimenType, Indication: o.ClinicalIndication,
		Priority: o.Priority, Status: o.Status,
		OrderedAt: o.OrderedAt.UTC().Format(time.RFC3339), CollectedAt: collected,
		InvoiceID: uuidOrNil(o.InvoiceID),
	}
}

func uuidOrNil(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

// CreateLabOrder handles POST /api/v1/consultations/:encounterId/lab-orders.
func (h *Consultation) CreateLabOrder(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	encounterID, err := uuid.Parse(c.Param("encounterId"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "encounterId must be a UUID.", nil)
		return
	}

	var req labOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "patientId must be a UUID.", nil)
		return
	}

	order, err := h.svc.CreateLabOrder(c.Request.Context(), service.CreateLabOrderInput{
		ActorID: actor.UserID, PatientID: patientID, EncounterID: encounterID,
		TestCode: req.TestCode, TestName: req.TestName, Specimen: req.Specimen,
		Indication: req.Indication, Priority: req.Priority, OrderNumber: req.OrderNumber,
		TariffCode: req.TariffCode,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	c.JSON(201, gin.H{"data": gin.H{"labOrder": toLabOrderDTO(order)}})
}

// ListLabOrders handles GET /api/v1/patients/:id/lab-orders.
func (h *Consultation) ListLabOrders(c *gin.Context) {
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

	rows, err := h.svc.LabOrdersForPatient(c.Request.Context(), actor.UserID, patientID)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	out := make([]labOrderDTO, 0, len(rows))
	for _, o := range rows {
		out = append(out, toLabOrderDTO(o))
	}
	httpx.OK(c, gin.H{"labOrders": out, "count": len(out)})
}

// CollectLabOrder handles POST /api/v1/lab-orders/:id/collect.
func (h *Consultation) CollectLabOrder(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Order id must be a UUID.", nil)
		return
	}

	order, err := h.svc.CollectLabOrder(c.Request.Context(), actor.UserID, id)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, toLabOrderDTO(order))
}

type labResultRequest struct {
	ParameterName  string   `json:"parameterName"`
	ValueText      string   `json:"valueText"`
	ValueNumeric   *float64 `json:"valueNumeric"`
	Unit           string   `json:"unit"`
	ReferenceRange string   `json:"referenceRange"`
	AbnormalFlag   string   `json:"abnormalFlag"`
	Remarks        string   `json:"remarks"`
}

type labResultDTO struct {
	ID             string   `json:"id"`
	LabOrderID     string   `json:"labOrderId"`
	PatientID      string   `json:"patientId"`
	ParameterName  string   `json:"parameterName"`
	ValueText      *string  `json:"valueText,omitempty"`
	ValueNumeric   *float64 `json:"valueNumeric,omitempty"`
	Unit           *string  `json:"unit,omitempty"`
	ReferenceRange *string  `json:"referenceRange,omitempty"`
	AbnormalFlag   *string  `json:"abnormalFlag,omitempty"`
	ResultAt       string   `json:"resultAt"`
	Remarks        *string  `json:"remarks,omitempty"`
}

func toLabResultDTO(r *domain.LabResult) labResultDTO {
	return labResultDTO{
		ID: r.ID.String(), LabOrderID: r.LabOrderID.String(), PatientID: r.PatientID.String(),
		ParameterName: r.ParameterName, ValueText: r.ValueText, ValueNumeric: r.ValueNumeric,
		Unit: r.Unit, ReferenceRange: r.ReferenceRange, AbnormalFlag: r.AbnormalFlag,
		ResultAt: r.ResultAt.UTC().Format(time.RFC3339), Remarks: r.Remarks,
	}
}

// RecordLabResult handles POST /api/v1/lab-orders/:id/results.
func (h *Consultation) RecordLabResult(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Order id must be a UUID.", nil)
		return
	}

	var req labResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	result, err := h.svc.RecordLabResult(c.Request.Context(), service.RecordLabResultInput{
		ActorID: actor.UserID, OrderID: orderID, ParameterName: req.ParameterName,
		ValueText: req.ValueText, ValueNumeric: req.ValueNumeric, Unit: req.Unit,
		ReferenceRange: req.ReferenceRange, AbnormalFlag: req.AbnormalFlag, Remarks: req.Remarks,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	c.JSON(201, gin.H{"data": gin.H{"labResult": toLabResultDTO(result)}})
}

// ListLabResults handles GET /api/v1/lab-orders/:id/results.
func (h *Consultation) ListLabResults(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Order id must be a UUID.", nil)
		return
	}

	rows, err := h.svc.LabResultsForOrder(c.Request.Context(), actor.UserID, orderID)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	out := make([]labResultDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toLabResultDTO(r))
	}
	httpx.OK(c, gin.H{"labResults": out, "count": len(out)})
}
