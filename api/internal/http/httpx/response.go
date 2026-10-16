// Package httpx holds HTTP middleware and response helpers shared by handlers.
//
// The name is "httpx" rather than "http" so it never shadows the standard
// library's net/http inside importing files — a shadowing bug that produces
// confusing compile errors.
package httpx

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HeaderRequestID is the correlation ID echoed to the client and attached to
// every log line and audit row for a request.
const HeaderRequestID = "X-Request-Id"

// ContextKeyRequestID is the gin context key holding the request ID.
const ContextKeyRequestID = "request_id"

// RequestID assigns every request a correlation ID.
//
// If the client supplies one we reuse it, so a browser-side error report can be
// traced to the exact server logs. Otherwise we mint a UUIDv7, which is
// time-ordered and therefore sorts usefully in a log viewer.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(HeaderRequestID))
		if id == "" || len(id) > 64 {
			// Ignore absurd or missing values rather than trusting client input
			// in a header we write into logs.
			v, err := uuid.NewV7()
			if err != nil {
				id = uuid.NewString()
			} else {
				id = v.String()
			}
		}
		c.Set(ContextKeyRequestID, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

// GetRequestID returns the request ID for the current request, or "" if the
// middleware did not run.
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(ContextKeyRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ─── Error envelope ────────────────────────────────────────────────
//
// Every error response has this shape, from every endpoint:
//
//	{ "error": { "code": "...", "message": "...", "details": ..., "requestId": "..." } }
//
// `code` is stable and machine-readable — the frontend switches on it.
// `message` is human-readable and may change — never switch on it.
// See docs/07-API-CONTRACT.md §1.3 and §1.4.

// ErrorBody is the inner object of an error response.
type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

// ErrorResponse is the complete error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// Meta is attached to successful responses.
type Meta struct {
	RequestID  string `json:"requestId,omitempty"`
	ServerTime string `json:"serverTime,omitempty"`
}

// SuccessResponse is the envelope for a successful single-resource response.
type SuccessResponse struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

// OK writes a 200 success envelope.
func OK(c *gin.Context, data any) {
	c.JSON(200, SuccessResponse{
		Data: data,
		Meta: Meta{
			RequestID:  GetRequestID(c),
			ServerTime: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// Fail writes an error envelope with the given HTTP status.
//
// Prefer the named helpers below (BadRequest, Unauthorized, ...) so that the
// status and the code stay consistent across the codebase.
func Fail(c *gin.Context, status int, code, message string, details any) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			Details:   details,
			RequestID: GetRequestID(c),
		},
	})
}

// ─── Named failures — one per row of the error table ───────────────
// docs/07-API-CONTRACT.md §1.4

func BadRequest(c *gin.Context, code, message string, details any) {
	Fail(c, 400, code, message, details)
}

func Unauthorized(c *gin.Context, code, message string) {
	Fail(c, 401, code, message, nil)
}

func Forbidden(c *gin.Context, code, message string) {
	Fail(c, 403, code, message, nil)
}

func NotFound(c *gin.Context, code, message string) {
	Fail(c, 404, code, message, nil)
}

func Conflict(c *gin.Context, code, message string, details any) {
	Fail(c, 409, code, message, details)
}

// Unprocessable is for input that is well-formed but violates a business rule —
// a distinction worth preserving, because the frontend handles it differently
// from a field-level validation error.
func Unprocessable(c *gin.Context, code, message string, details any) {
	Fail(c, 422, code, message, details)
}

// InternalError never leaks the underlying error to the client. The real error
// goes to the log with the request ID, which is what makes debugging possible
// without exposing internals.
func InternalError(c *gin.Context, message string) {
	Fail(c, 500, "INTERNAL_ERROR", message, nil)
}

func ServiceUnavailable(c *gin.Context, code, message string) {
	Fail(c, 503, code, message, nil)
}
