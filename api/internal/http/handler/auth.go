// Package handler holds Gin handlers. It knows HTTP; it knows nothing about SQL.
//
// Every handler follows the same shape:
//
//	bind -> validate -> call the service -> map the result to a response
//
// No business rule lives here. If a handler starts branching on roles or
// workflow state, that logic belongs in the service.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/http/middleware"
	"github.com/mikedalu/phc-records/api/internal/service"
)

// Auth handles the authentication endpoints.
type Auth struct {
	svc *service.Auth
}

// NewAuth builds the handler.
func NewAuth(svc *service.Auth) *Auth { return &Auth{svc: svc} }

// ─── DTOs ─────────────────────────────────────────────────────────────
//
// Request and response structs are separate from domain types on purpose:
// it makes leaking an internal field impossible, and lets the wire format
// evolve without touching the domain.

type loginRequest struct {
	StaffNumber string `json:"staffNumber"`
	Password    string `json:"password"`
	DeviceID    string `json:"deviceId"`
	DeviceName  string `json:"deviceName"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
	DeviceID     string `json:"deviceId"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// userDTO is the ONLY representation of a user that ever reaches a client.
//
// Note what is absent: password_hash, failed_login_count, locked_until,
// is_active, internal timestamps. Field-by-field construction means a new column
// on domain.User cannot leak by default.
type userDTO struct {
	ID                 string   `json:"id"`
	StaffNumber        string   `json:"staffNumber"`
	FirstName          string   `json:"firstName"`
	LastName           string   `json:"lastName"`
	Email              *string  `json:"email,omitempty"`
	Roles              []string `json:"roles"`
	MustChangePassword bool     `json:"mustChangePassword"`
}

func toUserDTO(u *domain.User) userDTO {
	roles := make([]string, 0, len(u.Roles))
	for _, r := range u.Roles {
		roles = append(roles, r.String())
	}
	return userDTO{
		ID:                 u.ID.String(),
		StaffNumber:        u.StaffNumber,
		FirstName:          u.FirstName,
		LastName:           u.LastName,
		Email:              u.Email,
		Roles:              roles,
		MustChangePassword: u.MustChangePassword,
	}
}

type sessionDTO struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresIn    int       `json:"expiresIn"`
	User         userDTO   `json:"user"`
	Device       deviceDTO `json:"device"`
	SessionID    string    `json:"sessionId"`
}

type deviceDTO struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

func toSessionDTO(s *service.Session) sessionDTO {
	return sessionDTO{
		AccessToken:  s.AccessToken,
		RefreshToken: s.RefreshToken,
		ExpiresIn:    int(s.ExpiresIn.Seconds()),
		User:         toUserDTO(s.User),
		Device:       deviceDTO{ID: s.DeviceID.String()},
		SessionID:    s.SessionID.String(),
	}
}

// ─── Handlers ─────────────────────────────────────────────────────────

// Login handles POST /api/v1/auth/login.
func (h *Auth) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	// Bind then validate: gin's binding tags are convenient but cannot express
	// cross-field rules, and the domain rules belong in one place.
	if req.StaffNumber == "" || req.Password == "" {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Staff number and password are required.",
			gin.H{"field": "staffNumber"})
		return
	}

	sess, err := h.svc.Login(c.Request.Context(), service.LoginInput{
		Identifier: req.StaffNumber,
		Password:   req.Password,
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		IP:         clientIP(c),
		UserAgent:  c.GetHeader("User-Agent"),
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, toSessionDTO(sess))
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *Auth) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}
	if req.RefreshToken == "" {
		httpx.BadRequest(c, "VALIDATION_FAILED", "refreshToken is required.", nil)
		return
	}

	sess, err := h.svc.Refresh(c.Request.Context(), service.RefreshInput{
		RefreshToken: req.RefreshToken,
		DeviceID:     req.DeviceID,
		IP:           clientIP(c),
		UserAgent:    c.GetHeader("User-Agent"),
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, toSessionDTO(sess))
}

// Logout handles POST /api/v1/auth/logout.
//
// Returns 204 with no body. Idempotent: an unknown or already-revoked token is
// still a success, because the caller's desired end state is already true.
func (h *Auth) Logout(c *gin.Context) {
	var req logoutRequest
	// A malformed body is tolerated here: the endpoint's purpose is to end a
	// session, and refusing because JSON was bad would leave the client unable
	// to log out. The token is a nice-to-have, not a requirement.
	_ = c.ShouldBindJSON(&req)

	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		httpx.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Session handles GET /api/v1/auth/session.
//
// Lets a client confirm its token is still good, and re-read the user's roles
// after being offline. The roles returned here are CURRENT, unlike the token's;
// this is where an offline device catches up on a revoked capability.
func (h *Auth) Session(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	// The service re-reads the user with roles; the token is not trusted for
	// this.
	user, err := h.svc.CurrentUser(c.Request.Context(), actor.UserID)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, toUserDTO(user))
}

// ChangePassword handles POST /api/v1/auth/change-password.
func (h *Auth) ChangePassword(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	if err := h.svc.ChangePassword(c.Request.Context(), service.ChangePasswordInput{
		UserID:          actor.UserID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
		KeepSessionID:   actor.SessionID,
	}); err != nil {
		httpx.Error(c, err)
		return
	}

	// 204: the client must re-login on other devices. The session that made the
	// change is preserved so the user is not logged out of the device they are
	// standing at — an infuriating experience that leads to password reuse.
	c.Status(http.StatusNoContent)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// clientIP returns the caller's address.
//
// Behind Cloudflare the socket peer is the proxy, so c.ClientIP() would report
// the same address for everyone and make per-IP rate limiting useless. We prefer
// CF-Connecting-IP, but ONLY when the operator has declared they sit behind a
// trusted proxy — otherwise any client could forge the header and evade the
// limiter entirely.
func clientIP(c *gin.Context) string {
	if trustedProxy, ok := c.Get("trust_proxy"); ok {
		if b, _ := trustedProxy.(bool); b {
			if cf := c.GetHeader("CF-Connecting-IP"); cf != "" {
				return cf
			}
		}
	}
	return c.ClientIP()
}
