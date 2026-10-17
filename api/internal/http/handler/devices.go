package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/http/middleware"
	"github.com/mikedalu/phc-records/api/internal/service"
)

// Devices handles workstation identity endpoints.
type Devices struct {
	svc *service.Devices
}

// NewDevices builds the handler.
func NewDevices(svc *service.Devices) *Devices { return &Devices{svc: svc} }

type registerDeviceRequest struct {
	// DeviceID is optional. When present it must equal the access token's
	// device — the server never lets a token register a key for a different
	// machine, because that is exactly what a stolen token would try.
	DeviceID   string `json:"deviceId"`
	PublicKey  string `json:"publicKey"`
	Algorithm  string `json:"algorithm"`
	Label      string `json:"label"`
	Platform   string `json:"platform"`
	AppVersion string `json:"appVersion"`
}

type revokeDeviceRequest struct {
	Reason string `json:"reason"`
}

// registeredDeviceDTO is the wire shape of a device.
//
// The public key itself is NOT returned. The caller already has it, and echoing
// key material back over the wire creates a copy of it in logs and browser
// histories for no benefit. The fingerprint is what the client displays.
type registeredDeviceDTO struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Platform    *string `json:"platform,omitempty"`
	Algorithm   string  `json:"algorithm"`
	Fingerprint string  `json:"publicKeyFingerprint"`
	// Registered is false until a public key has been bound. A device that can
	// log in but cannot sign is a real, expected state (login happens first).
	Registered bool    `json:"registered"`
	Revoked    bool    `json:"revoked"`
	LastSeenAt *string `json:"lastSeenAt,omitempty"`
	CreatedAt  string  `json:"createdAt"`
}

func toRegisteredDeviceDTO(d *domain.Device) registeredDeviceDTO {
	var lastSeen *string
	if d.LastSeenAt != nil {
		s := d.LastSeenAt.UTC().Format(time.RFC3339)
		lastSeen = &s
	}

	// Do not leak the internal placeholder fingerprint ("pending:<uuid>") used
	// while a device has no key. An empty string is the honest answer: there is
	// no key, so there is no fingerprint.
	fingerprint := d.Fingerprint
	if d.PublicKey == "" {
		fingerprint = ""
	}

	return registeredDeviceDTO{
		ID:          d.ID.String(),
		Name:        d.Name,
		Platform:    d.Platform,
		Algorithm:   d.PublicKeyAlg,
		Fingerprint: fingerprint,
		Registered:  d.PublicKey != "",
		Revoked:     d.IsRevoked(),
		LastSeenAt:  lastSeen,
		CreatedAt:   d.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// Register handles POST /api/v1/devices.
//
// Authenticated with a normal access token: this is how a device that can already
// log in earns the right to sign pushes.
func (h *Devices) Register(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	var req registerDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}
	if req.PublicKey == "" {
		httpx.BadRequest(c, "VALIDATION_FAILED", "publicKey is required.",
			gin.H{"field": "publicKey"})
		return
	}

	// An empty deviceId is tolerated (the token is authoritative). A wrong one is
	// not: silently ignoring it would hide a client bug that only shows up later
	// as pushes signed by the wrong device.
	claimed := uuid.Nil
	if req.DeviceID != "" {
		parsed, err := uuid.Parse(req.DeviceID)
		if err != nil {
			httpx.BadRequest(c, "VALIDATION_FAILED", "deviceId must be a UUID.",
				gin.H{"field": "deviceId"})
			return
		}
		claimed = parsed
	}

	device, err := h.svc.Register(c.Request.Context(), service.RegisterDeviceInput{
		TokenDeviceID:   actor.DeviceID,
		ClaimedDeviceID: claimed,
		PublicKey:       req.PublicKey,
		Algorithm:       req.Algorithm,
		Name:            req.Label,
		Platform:        req.Platform,
		AppVersion:      req.AppVersion,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	httpx.OK(c, toRegisteredDeviceDTO(device))
}

// Revoke handles POST /api/v1/admin/devices/:id/revoke.
//
// The route is gated by RequireRole(admin); the service checks the database role
// as well. The middleware check is a cheap early exit, not the boundary.
func (h *Devices) Revoke(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_FAILED", "Device id must be a UUID.", nil)
		return
	}

	var req revokeDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	device, revokedSessions, err := h.svc.Revoke(c.Request.Context(), service.RevokeDeviceInput{
		ActorID:  actor.UserID,
		DeviceID: deviceID,
		Reason:   req.Reason,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"device":          toRegisteredDeviceDTO(device),
			"revokedSessions": revokedSessions,
		},
	})
}
