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

// Patients handles the registry endpoints.
type Patients struct {
	svc *service.Patients
}

// NewPatients builds the handler.
func NewPatients(svc *service.Patients) *Patients { return &Patients{svc: svc} }

type patientRequest struct {
	PatientNumber string  `json:"patientNumber"`
	HouseholdID   *string `json:"householdId"`

	FirstName  string  `json:"firstName"`
	MiddleName *string `json:"middleName"`
	Surname    string  `json:"surname"`

	// Exactly one of dateOfBirth or an age is required. Dates are YYYY-MM-DD
	// strings: a DATE column, not an instant, so no timezone can shift it.
	DateOfBirth *string `json:"dateOfBirth"`
	AgeYears    *int    `json:"ageYears"`
	AgeMonths   *int    `json:"ageMonths"`

	Gender        string  `json:"gender"`
	MaritalStatus *string `json:"maritalStatus"`

	Phone           *string `json:"phone"`
	AltPhone        *string `json:"altPhone"`
	Tribe           *string `json:"tribe"`
	Occupation      *string `json:"occupation"`
	Address         *string `json:"address"`
	AddressLandmark *string `json:"addressLandmark"`

	BloodGroup        *string `json:"bloodGroup"`
	Genotype          *string `json:"genotype"`
	Allergies         *string `json:"allergies"`
	ChronicConditions *string `json:"chronicConditions"`

	RegistrationDate *string `json:"registrationDate"`
}

type patientDTO struct {
	ID            string  `json:"id"`
	PatientNumber string  `json:"patientNumber"`
	HouseholdID   *string `json:"householdId,omitempty"`

	FirstName  string  `json:"firstName"`
	MiddleName *string `json:"middleName,omitempty"`
	Surname    string  `json:"surname"`

	DateOfBirth   *string `json:"dateOfBirth,omitempty"`
	AgeYears      *int16  `json:"ageYears,omitempty"`
	AgeMonths     *int16  `json:"ageMonths,omitempty"`
	AgeRecordedAt *string `json:"ageRecordedAt,omitempty"`
	AgeDisplay    string  `json:"ageDisplay,omitempty"`

	Gender        string  `json:"gender"`
	MaritalStatus *string `json:"maritalStatus,omitempty"`

	Phone           *string `json:"phone,omitempty"`
	AltPhone        *string `json:"altPhone,omitempty"`
	Tribe           *string `json:"tribe,omitempty"`
	Occupation      *string `json:"occupation,omitempty"`
	Address         *string `json:"address,omitempty"`
	AddressLandmark *string `json:"addressLandmark,omitempty"`

	BloodGroup        *string `json:"bloodGroup,omitempty"`
	Genotype          *string `json:"genotype,omitempty"`
	Allergies         *string `json:"allergies,omitempty"`
	ChronicConditions *string `json:"chronicConditions,omitempty"`

	IsDeceased       bool   `json:"isDeceased"`
	RegistrationDate string `json:"registrationDate"`
	IsActive         bool   `json:"isActive"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

func datePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02")
	return &s
}

func toPatientDTO(p *domain.Patient) patientDTO {
	var household *string
	if p.HouseholdID != nil {
		s := p.HouseholdID.String()
		household = &s
	}

	return patientDTO{
		ID:                p.ID.String(),
		PatientNumber:     p.PatientNumber,
		HouseholdID:       household,
		FirstName:         p.FirstName,
		MiddleName:        p.MiddleName,
		Surname:           p.Surname,
		DateOfBirth:       datePtr(p.DateOfBirth),
		AgeYears:          p.AgeYears,
		AgeMonths:         p.AgeMonths,
		AgeRecordedAt:     datePtr(p.AgeRecordedAt),
		AgeDisplay:        p.AgeAt(time.Now()),
		Gender:            p.Gender,
		MaritalStatus:     p.MaritalStatus,
		Phone:             p.Phone,
		AltPhone:          p.AltPhone,
		Tribe:             p.Tribe,
		Occupation:        p.Occupation,
		Address:           p.Address,
		AddressLandmark:   p.AddressLandmark,
		BloodGroup:        p.BloodGroup,
		Genotype:          p.Genotype,
		Allergies:         p.Allergies,
		ChronicConditions: p.ChronicConditions,
		IsDeceased:        p.IsDeceased,
		RegistrationDate:  p.RegistrationDate.UTC().Format("2006-01-02"),
		IsActive:          p.IsActive,
		CreatedAt:         p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (req patientRequest) toInput(actorID uuid.UUID) (service.CreatePatientInput, error) {
	in := service.CreatePatientInput{
		ActorID:           actorID,
		PatientNumber:     req.PatientNumber,
		FirstName:         req.FirstName,
		MiddleName:        req.MiddleName,
		Surname:           req.Surname,
		DateOfBirth:       req.DateOfBirth,
		AgeYears:          req.AgeYears,
		AgeMonths:         req.AgeMonths,
		Gender:            req.Gender,
		MaritalStatus:     req.MaritalStatus,
		Phone:             req.Phone,
		AltPhone:          req.AltPhone,
		Tribe:             req.Tribe,
		Occupation:        req.Occupation,
		Address:           req.Address,
		AddressLandmark:   req.AddressLandmark,
		BloodGroup:        req.BloodGroup,
		Genotype:          req.Genotype,
		Allergies:         req.Allergies,
		ChronicConditions: req.ChronicConditions,
		RegistrationDate:  req.RegistrationDate,
	}

	if req.HouseholdID != nil && *req.HouseholdID != "" {
		id, err := uuid.Parse(*req.HouseholdID)
		if err != nil {
			return in, domain.NewValidation("householdId", "must_be_a_uuid")
		}
		in.HouseholdID = &id
	}

	return in, nil
}

// Create handles POST /api/v1/patients.
func (h *Patients) Create(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	var req patientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	in, err := req.toInput(actor.UserID)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	result, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	dupes := make([]patientDTO, 0, len(result.PossibleDuplicates))
	for _, d := range result.PossibleDuplicates {
		dupes = append(dupes, toPatientDTO(d))
	}

	httpx.OK(c, gin.H{
		"patient": toPatientDTO(result.Patient),
		// Advisory only. The client shows a warning; it does not block.
		"possibleDuplicates": dupes,
	})
}

// Get handles GET /api/v1/patients/:id.
func (h *Patients) Get(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Patient id must be a UUID.", nil)
		return
	}

	patient, err := h.svc.Get(c.Request.Context(), actor.UserID, id)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, toPatientDTO(patient))
}

// Search handles GET /api/v1/patients?q=&limit=.
func (h *Patients) Search(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	limit, err := intQuery(c, "limit", 0, 0, 50)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "limit must be a number between 0 and 50.", nil)
		return
	}

	patients, err := h.svc.Search(c.Request.Context(), actor.UserID, c.Query("q"), int(limit))
	if err != nil {
		httpx.Error(c, err)
		return
	}

	out := make([]patientDTO, 0, len(patients))
	for _, p := range patients {
		out = append(out, toPatientDTO(p))
	}

	httpx.OK(c, gin.H{"patients": out, "count": len(out)})
}

// Update handles PATCH /api/v1/patients/:id.
func (h *Patients) Update(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Patient id must be a UUID.", nil)
		return
	}

	var req patientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	in, err := req.toInput(actor.UserID)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	patient, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, toPatientDTO(patient))
}
