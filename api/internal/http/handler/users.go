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

// Users handles staff administration.
type Users struct {
	svc *service.Users
}

// NewUsers builds the handler.
func NewUsers(svc *service.Users) *Users { return &Users{svc: svc} }

type adminUserDTO struct {
	ID                 string   `json:"id"`
	StaffNumber        string   `json:"staffNumber"`
	FirstName          string   `json:"firstName"`
	LastName           string   `json:"lastName"`
	Email              *string  `json:"email,omitempty"`
	Roles              []string `json:"roles"`
	IsActive           bool     `json:"isActive"`
	MustChangePassword bool     `json:"mustChangePassword"`
	LastLoginAt        *string  `json:"lastLoginAt,omitempty"`
	LockedUntil        *string  `json:"lockedUntil,omitempty"`
	CreatedAt          string   `json:"createdAt"`
}

func formatTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func toAdminUserDTO(u *domain.User) adminUserDTO {
	roles := make([]string, 0, len(u.Roles))
	for _, r := range u.Roles {
		roles = append(roles, r.String())
	}

	return adminUserDTO{
		ID:                 u.ID.String(),
		StaffNumber:        u.StaffNumber,
		FirstName:          u.FirstName,
		LastName:           u.LastName,
		Email:              u.Email,
		Roles:              roles,
		IsActive:           u.IsActive,
		MustChangePassword: u.MustChangePassword,
		LastLoginAt:        formatTime(u.LastLoginAt),
		LockedUntil:        formatTime(u.LockedUntil),
		CreatedAt:          u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// List handles GET /api/v1/admin/users.
func (h *Users) List(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	users, err := h.svc.ListUsers(c.Request.Context(), actor.UserID)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	out := make([]adminUserDTO, 0, len(users))
	for _, u := range users {
		out = append(out, toAdminUserDTO(u))
	}

	httpx.OK(c, gin.H{"users": out})
}

type updateUserRequest struct {
	// Role toggles: granted when absent, revoked when present.
	Role *string `json:"role"`
	// IsActive opts an account in or out. Pointer so "absent" and "false" differ.
	IsActive *bool  `json:"isActive"`
	Reason   string `json:"reason"`
}

// Update handles PATCH /api/v1/admin/users/:id.
func (h *Users) Update(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "User id must be a UUID.", nil)
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	// An empty body is a client bug, not a no-op: answering 200 would tell the
	// client its change was applied when nothing was written.
	if req.Role == nil && req.IsActive == nil {
		httpx.BadRequest(c, "VALIDATION_FAILED",
			"Provide at least one of role or isActive.", nil)
		return
	}

	var role *domain.Role
	if req.Role != nil {
		r := domain.Role(*req.Role)
		role = &r
	}

	updated, err := h.svc.UpdateUser(c.Request.Context(), service.UpdateUserInput{
		ActorID:  actor.UserID,
		UserID:   targetID,
		Role:     role,
		IsActive: req.IsActive,
		Reason:   req.Reason,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, gin.H{"user": toAdminUserDTO(updated)})
}

type createUserRequest struct {
	StaffNumber string `json:"staffNumber"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Role        string `json:"role"`
	Password    string `json:"password"`
}

// Create handles POST /api/v1/admin/users.
func (h *Users) Create(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	created, err := h.svc.CreateUser(c.Request.Context(), service.CreateUserInput{
		ActorID:     actor.UserID,
		StaffNumber: req.StaffNumber,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Role:        domain.Role(req.Role),
		Password:    req.Password,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	c.JSON(201, gin.H{"data": gin.H{"user": toAdminUserDTO(created)}})
}
