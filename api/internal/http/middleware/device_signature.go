package middleware

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/platform/devicekey"
	"github.com/mikedalu/phc-records/api/internal/platform/nonce"
)

// DeviceLookup resolves a device by id. Declared as a function type rather than
// a repo interface so this package does not depend on the persistence layer —
// which also makes the middleware testable with a two-line fake.
type DeviceLookup func(ctx context.Context, id uuid.UUID) (*domain.Device, error)

// Headers a signed request carries.
const (
	HeaderDeviceSignature = "X-Device-Signature"
	HeaderDeviceTimestamp = "X-Device-Timestamp"
	HeaderDeviceNonce     = "X-Device-Nonce"
)

// ContextDevice holds the verified *domain.Device for the request.
const ContextDevice = "auth_device"

// MaxDeviceSkew is the acceptance window for a device timestamp.
//
// Five minutes: wide enough for a workstation whose clock drifted since the last
// NTP sync, narrow enough that a captured request is useless for long.
const MaxDeviceSkew = 5 * time.Minute

// DeviceSignature verifies that a request was signed by the device the access
// token was issued to, and that it has not been replayed.
//
// ═══ ORDERING IS A SECURITY DECISION, NOT STYLE ═══
//
//  1. RequireAuth must run first. The device id comes from the VERIFIED JWT
//     claim, never from a header the client chooses — otherwise an attacker
//     signs as their own device and claims to be someone else's.
//  2. The nonce is consumed LAST, after the signature verifies. Consuming first
//     would let unauthenticated traffic fill the cache and evict legitimate
//     nonces, turning replay protection into a denial of service.
//
// Unlike the sketch in docs/05-SECURITY.md §4.4, this middleware does NOT skip
// GET requests. Whether a route is signed is decided by WHERE this is mounted,
// not by a method check buried in here: a silent `if GET { c.Next() }` is exactly
// how a future endpoint ends up unsigned by accident.
func DeviceSignature(lookup DeviceLookup, nonces nonce.Cache, maxSkew time.Duration) gin.HandlerFunc {
	if maxSkew <= 0 {
		maxSkew = MaxDeviceSkew
	}

	return func(c *gin.Context) {
		deviceID, ok := DeviceIDFrom(c)
		if !ok || deviceID == uuid.Nil {
			// Reaching here without RequireAuth is a wiring bug, not a client
			// error. Fail closed and say so in the log.
			log.Error().Str("request_id", httpx.GetRequestID(c)).
				Msg("DeviceSignature mounted without RequireAuth")
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
			return
		}

		signature := strings.TrimSpace(c.GetHeader(HeaderDeviceSignature))
		timestamp := strings.TrimSpace(c.GetHeader(HeaderDeviceTimestamp))
		deviceNonce := strings.TrimSpace(c.GetHeader(HeaderDeviceNonce))

		if signature == "" || timestamp == "" || deviceNonce == "" {
			httpx.Unauthorized(c, "DEVICE_SIGNATURE_MISSING",
				"This endpoint requires a signed request (X-Device-Signature, X-Device-Timestamp, X-Device-Nonce).")
			return
		}

		// The timestamp must be parsed, but the ORIGINAL string is what goes into
		// the canonical form. Re-formatting it here would change the bytes the
		// client signed.
		sentAt, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			httpx.Unauthorized(c, "CLOCK_SKEW", "X-Device-Timestamp must be ISO-8601 (RFC 3339).")
			return
		}
		skew := time.Since(sentAt)
		if skew < 0 {
			skew = -skew
		}
		if skew > maxSkew {
			// A distinct code so the client can show "fix your clock" rather than
			// "signature invalid", which would send someone hunting a key bug.
			httpx.Unauthorized(c, "CLOCK_SKEW",
				"Your device clock is more than 5 minutes from the server. Correct it and retry.")
			return
		}

		rawBody, ok := httpx.RawBody(c)
		if !ok {
			// CaptureBody was not mounted. Again a wiring bug.
			log.Error().Str("request_id", httpx.GetRequestID(c)).
				Msg("DeviceSignature mounted without CaptureBody")
			httpx.InternalError(c, "An unexpected error occurred.")
			return
		}

		canonical := devicekey.CanonicalString(
			c.Request.Method,
			c.Request.URL.Path, // exactly as sent; no normalisation, no query string
			deviceID.String(),
			timestamp,
			deviceNonce,
			rawBody,
		)

		device, err := lookup(c.Request.Context(), deviceID)
		if err != nil {
			// Unknown and revoked collapse into one response on purpose: telling a
			// caller "that device exists but is revoked" confirms a device id.
			if errors.Is(err, domain.ErrNotFound) {
				httpx.Forbidden(c, "DEVICE_REVOKED", "This device is not trusted. Contact an administrator.")
				return
			}
			httpx.Error(c, err)
			return
		}

		if device.IsRevoked() {
			log.Warn().
				Str("request_id", httpx.GetRequestID(c)).
				Str("device_id", device.ID.String()).
				Msg("rejected a signed request from a revoked device")
			httpx.Forbidden(c, "DEVICE_REVOKED", "This device has been revoked. Contact an administrator.")
			return
		}

		// A device row created at login has no key yet. Its signature cannot be
		// verified against anything, and pretending otherwise would reject the
		// request with a misleading "invalid signature".
		if device.PublicKey == "" {
			httpx.Forbidden(c, "DEVICE_KEY_NOT_REGISTERED",
				"This device has no signing key. Register one with POST /api/v1/devices.")
			return
		}

		if err := devicekey.Verify(publicKeyOf(device), []byte(canonical), signature); err != nil {
			log.Warn().
				Str("request_id", httpx.GetRequestID(c)).
				Str("device_id", device.ID.String()).
				Str("fingerprint", device.Fingerprint).
				Msg("device signature did not verify")
			httpx.Unauthorized(c, "DEVICE_SIGNATURE_INVALID", "The device signature is invalid.")
			return
		}

		// Only NOW record the nonce. Keyed by device AND nonce: two devices that
		// somehow pick the same nonce must not invalidate each other.
		nonceKey := deviceID.String() + ":" + deviceNonce
		if !nonces.Consume(nonceKey, 2*maxSkew, time.Now()) {
			httpx.Conflict(c, "DEVICE_NONCE_REPLAYED",
				"This request was already received. Retrying it is not permitted.", nil)
			return
		}

		c.Set(ContextDevice, device)
		c.Next()
	}
}

// publicKeyOf re-parses the stored SPKI key for verification.
//
// It returns nil when the stored key is unparseable, which makes Verify fail
// closed. The alternative — caching a parsed *ecdsa.PublicKey on domain.Device —
// would put a crypto type in the domain package to save microseconds.
func publicKeyOf(device *domain.Device) *ecdsa.PublicKey {
	pub, _, err := devicekey.ParsePublicKey(device.PublicKey)
	if err != nil {
		log.Error().Err(err).
			Str("device_id", device.ID.String()).
			Msg("stored device public key is unparseable")
		return nil
	}
	return pub
}

// DeviceFrom returns the verified device for the request.
func DeviceFrom(c *gin.Context) (*domain.Device, bool) {
	v, ok := c.Get(ContextDevice)
	if !ok {
		return nil, false
	}
	device, ok := v.(*domain.Device)
	return device, ok
}

// ReplayTTL is how long a nonce is remembered: twice the acceptance window, so a
// nonce cannot expire from the cache while its request is still acceptable.
func ReplayTTL(maxSkew time.Duration) time.Duration {
	if maxSkew <= 0 {
		maxSkew = MaxDeviceSkew
	}
	return 2 * maxSkew
}
