package httpx

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ContextKeyRawBody is the gin context key holding the unmodified request body.
const ContextKeyRawBody = "raw_body"

// CaptureBody reads the request body once and makes it available to middleware,
// while leaving it readable for the handler.
//
// ═══ WHY THIS EXISTS ═══
//
// A device signature covers the SHA-256 of the RAW body bytes. The signature must
// therefore be checked against exactly the bytes that arrived — not against a
// re-serialised JSON object. Re-serialising changes key order, whitespace and
// number formatting, and the hash stops matching for reasons that look like a
// broken key.
//
// Reading the body in middleware would normally consume it, so the bytes are
// replaced with a fresh reader. `io.NopCloser(bytes.NewReader(raw))` is the
// standard way to put a body back.
//
// The limit is enforced here rather than relying on the server's ReadTimeout: a
// slow 100 MB upload must be refused with a clear 413, not by timing out after
// the body has already been buffered.
func CaptureBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}

		// Read one byte past the limit so "too large" is distinguishable from
		// "exactly at the limit".
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBytes+1))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
				Error: ErrorBody{
					Code:      "MALFORMED_REQUEST",
					Message:   "Could not read the request body.",
					RequestID: GetRequestID(c),
				},
			})
			return
		}

		if int64(len(raw)) > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, ErrorResponse{
				Error: ErrorBody{
					Code:      "PAYLOAD_TOO_LARGE",
					Message:   "The request body is too large.",
					RequestID: GetRequestID(c),
				},
			})
			return
		}

		// Restore the body for downstream handlers. Without this the handler sees
		// an empty body and reports a confusing MALFORMED_REQUEST.
		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
		c.Set(ContextKeyRawBody, raw)

		c.Next()
	}
}

// RawBody returns the bytes captured by CaptureBody.
func RawBody(c *gin.Context) ([]byte, bool) {
	v, ok := c.Get(ContextKeyRawBody)
	if !ok {
		return nil, false
	}
	raw, ok := v.([]byte)
	return raw, ok
}
