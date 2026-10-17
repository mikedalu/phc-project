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

// Households handles family registration and family views.
type Households struct {
	svc *service.Households
}

// NewHouseholds builds the handler.
func NewHouseholds(svc *service.Households) *Households { return &Households{svc: svc} }

type familyRequest struct {
	Household struct {
		FamilyName string `json:"familyName"`
		Phone      string `json:"phone"`
		Address    string `json:"address"`
		Notes      string `json:"notes"`
	} `json:"household"`

	Members []struct {
		patientRequest
		IsHead bool `json:"isHead"`
	} `json:"members"`
}

type householdDTO struct {
	ID            string  `json:"id"`
	FamilyNumber  string  `json:"familyNumber"`
	FamilyName    string  `json:"familyName"`
	HeadPatientID *string `json:"headPatientId,omitempty"`
	Phone         *string `json:"phone,omitempty"`
	Address       *string `json:"address,omitempty"`
	IsActive      bool    `json:"isActive"`
}

func toHouseholdDTO(h *domain.Household) householdDTO {
	var head *string
	if h.HeadPatientID != nil {
		s := h.HeadPatientID.String()
		head = &s
	}
	return householdDTO{
		ID:            h.ID.String(),
		FamilyNumber:  h.FamilyNumber,
		FamilyName:    h.FamilyName,
		HeadPatientID: head,
		Phone:         h.Phone,
		Address:       h.Address,
		IsActive:      h.IsActive,
	}
}

// RegisterFamily handles POST /api/v1/patients/family.
//
// All-or-nothing: the household and every member commit or none of them do.
func (h *Households) RegisterFamily(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	var req familyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	in := service.RegisterFamilyInput{
		ActorID:    actor.UserID,
		FamilyName: req.Household.FamilyName,
		Phone:      req.Household.Phone,
		Address:    req.Household.Address,
		Notes:      req.Household.Notes,
	}

	for _, member := range req.Members {
		base, err := member.toInput(actor.UserID)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		in.Members = append(in.Members, service.FamilyMemberInput{
			CreatePatientInput: base,
			IsHead:             member.IsHead,
		})
	}

	result, err := h.svc.RegisterFamily(c.Request.Context(), in)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	members := make([]patientDTO, 0, len(result.Members))
	for _, p := range result.Members {
		members = append(members, toPatientDTO(p))
	}

	c.JSON(201, gin.H{
		"data": gin.H{
			"household": toHouseholdDTO(result.Household),
			"members":   members,
		},
		"meta": gin.H{"serverTime": time.Now().UTC().Format(time.RFC3339)},
	})
}

// Get handles GET /api/v1/households/:id.
func (h *Households) Get(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Household id must be a UUID.", nil)
		return
	}

	view, err := h.svc.Get(c.Request.Context(), actor.UserID, id)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	members := make([]patientDTO, 0, len(view.Members))
	for _, p := range view.Members {
		members = append(members, toPatientDTO(p))
	}

	httpx.OK(c, gin.H{
		"household": toHouseholdDTO(view.Household),
		"members":   members,
		// Derived on every call, never stored (docs/01-DATA-MODEL.md §2.2).
		"balanceMinor": view.BalanceMinor,
	})
}
