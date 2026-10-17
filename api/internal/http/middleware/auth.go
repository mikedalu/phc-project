package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/platform/token"
)

// Context keys. Unexported types would be safer, but string keys are what gin
// offers; the constants below are the only approved way to reach them.
const (
	ContextUserID    = "auth_user_id"
	ContextRole      = "auth_role"
	ContextDeviceID  = "auth_device_id"
	ContextSessionID = "auth_session_id"
)

// Actor is the authenticated principal for a request.
//
// Built once by RequireAuth and read by services. Kept small: the user's full
// record is NOT stored here, because a stale copy from the token could diverge
// from the database — services that need the record re-read it.
type Actor struct {
	UserID    uuid.UUID
	Role      domain.Role
	DeviceID  uuid.UUID
	SessionID uuid.UUID
}

// RequireAuth rejects a request without a valid access token.
//
// On success it attaches an Actor to the context. Note what it does NOT do:
// verify that the user still exists, is active, or still holds the role. Those
// are database facts that can change within the token's 15-minute life, so the
// service layer re-checks them where they matter. A middleware check would
// create the illusion of freshness without the guarantee.
func RequireAuth(issuer *token.Issuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Authorization header is required.")
			return
		}

		// Case-insensitive "Bearer", because some HTTP clients normalise it.
		const prefix = "bearer "
		if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Authorization header must be 'Bearer <token>'.")
			return
		}

		raw := strings.TrimSpace(header[len(prefix):])
		if raw == "" {
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Bearer token is empty.")
			return
		}

		verified, err := issuer.Verify(raw)
		if err != nil {
			// Expiry is a distinct code so the client refreshes and retries,
			// rather than sending the user back to a login screen.
			if errors.Is(err, token.ErrExpired) {
				httpx.Unauthorized(c, "TOKEN_EXPIRED", "Access token has expired.")
				return
			}
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Access token is invalid.")
			return
		}

		deviceID, err := uuid.Parse(verified.DeviceID)
		if err != nil {
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Access token carries an invalid device.")
			return
		}
		sessionID, err := uuid.Parse(verified.SessionID)
		if err != nil {
			// A token without a session id predates the current format; treat it
			// as invalid rather than guessing.
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Access token carries an invalid session.")
			return
		}

		actor := Actor{
			UserID:    verified.UserID,
			Role:      verified.Role,
			DeviceID:  deviceID,
			SessionID: sessionID,
		}
		c.Set(ContextUserID, actor.UserID)
		c.Set(ContextRole, actor.Role)
		c.Set(ContextDeviceID, actor.DeviceID)
		c.Set(ContextSessionID, actor.SessionID)

		c.Next()
	}
}

// RequireRole admits only the listed roles.
//
// ═══ READ THIS BEFORE RELYING ON IT ═══
//
// This is a COARSE gate, and it reads the role from the TOKEN, which may be up
// to 15 minutes stale. It is fine for "only doctors reach the consultation
// route" — a convenience that keeps the error cheap.
//
// It is NOT the security boundary. The authoritative check is in the service
// layer, against the database, at the moment of the action. With a fully
// replicated offline client an attacker controls the frontend, so hiding a route
// buys nothing; what stops them is the service refusing the operation.
//
// Use this to fail fast and keep logs clean. Use service-layer checks to
// actually enforce.
func RequireRole(roles ...domain.Role) gin.HandlerFunc {
	allowed := make(map[domain.Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role, ok := RoleFrom(c)
		if !ok {
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
			return
		}
		if _, permitted := allowed[role]; !permitted {
			// Do not reveal which roles WOULD have been accepted.
			httpx.Forbidden(c, "FORBIDDEN", "Your role does not permit this action.")
			return
		}
		c.Next()
	}
}

// UserIDFrom returns the authenticated user's id.
func UserIDFrom(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ContextUserID)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// RoleFrom returns the authenticated user's role.
func RoleFrom(c *gin.Context) (domain.Role, bool) {
	v, ok := c.Get(ContextRole)
	if !ok {
		return "", false
	}
	r, ok := v.(domain.Role)
	return r, ok
}

// DeviceIDFrom returns the device that authenticated the request.
func DeviceIDFrom(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ContextDeviceID)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// SessionIDFrom returns the session (refresh-token family) id.
func SessionIDFrom(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ContextSessionID)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// ActorFrom assembles the Actor from the context.
func ActorFrom(c *gin.Context) (Actor, bool) {
	userID, ok := UserIDFrom(c)
	if !ok {
		return Actor{}, false
	}
	role, _ := RoleFrom(c)
	deviceID, _ := DeviceIDFrom(c)
	sessionID, _ := SessionIDFrom(c)
	return Actor{UserID: userID, Role: role, DeviceID: deviceID, SessionID: sessionID}, true
}

// TrustProxy publishes whether the deployment sits behind a trusted reverse
// proxy (Cloudflare) into the request context.
//
// Why it matters: behind a proxy the socket peer is the proxy itself, so gin's
// ClientIP() reports the same address for every request and per-IP rate limiting
// becomes useless. Handlers may then read CF-Connecting-IP instead.
//
// Why the flag exists at all: CF-Connecting-IP is client-controlled. Trusting it
// unconditionally would let anyone forge their address and evade the limiter, so
// it is only honoured when the operator has declared the proxy exists.
func TrustProxy(trusted bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("trust_proxy", trusted)
		c.Next()
	}
}
