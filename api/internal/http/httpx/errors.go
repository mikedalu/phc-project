package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Error mapping — the ONE place a domain error becomes an HTTP status.
//
// Why centralise it: if each handler invented its own status for a known
// failure, clients would need to handle the same condition in several ways, and
// the contract in docs/07-API-CONTRACT.md §1.4 would be fiction.
//
// The mapping is exhaustive over domain's sentinels. Anything unrecognised is a
// 500 and gets logged — because an unrecognised error means a bug, and hiding it
// behind a 400 would make it invisible.
func Error(c *gin.Context, err error) {
	// Order matters: check the more specific sentinels before their wrappers.
	// ValidationError wraps ErrValidation, so either branch is correct, but
	// checking it first lets us return field-level detail.
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", ve.Error(),
			gin.H{"field": ve.Field, "issue": ve.Issue})
		return
	}

	switch {
	case errors.Is(err, domain.ErrValidation):
		BadRequest(c, "VALIDATION_FAILED", err.Error(), nil)

	case errors.Is(err, domain.ErrUnauthenticated):
		// 401 means "authenticate again". The client must NOT retry with the
		// same credentials.
		Unauthorized(c, "UNAUTHENTICATED",
			domain.ClientMessage(err, "Authentication failed."))

	case errors.Is(err, domain.ErrForbidden):
		// 403 means "authenticated, but not permitted". Retrying cannot help,
		// and the client should hide the action rather than loop.
		Forbidden(c, "FORBIDDEN",
			domain.ClientMessage(err, "You do not have permission to do that."))

	case errors.Is(err, domain.ErrDeviceRevoked):
		// A distinct code because the client's response differs: wipe the local
		// replica and stop syncing, rather than hide a button.
		Forbidden(c, "DEVICE_REVOKED",
			domain.ClientMessage(err, "This device is no longer trusted. Contact an administrator."))

	case errors.Is(err, domain.ErrLocked):
		// 429 rather than 403: this is a tempoary state and the client should
		// retry later. The distinction changes the UI from "you may not" to
		// "try again in N minutes".
		//
		// Retry-After is set so a well-behaved client backs off without guessing.
		c.Header("Retry-After", "900")
		Fail(c, http.StatusTooManyRequests, "ACCOUNT_LOCKED",
			domain.ClientMessage(err, "Too many attempts. Please try again later."), nil)

	case errors.Is(err, domain.ErrNotFound):
		NotFound(c, "NOT_FOUND",
			domain.ClientMessage(err, "The requested item was not found."))

	case errors.Is(err, domain.ErrStale):
		// Optimistic-concurrency failure. Distinct from ErrConflict so the
		// client knows to reload and reapply rather than treat it as a
		// uniqueness problem.
		Conflict(c, "STALE_REVISION",
			domain.ClientMessage(err, "This record changed since you loaded it. Reload and try again."), nil)

	case errors.Is(err, domain.ErrConflict):
		Conflict(c, "CONFLICT",
			domain.ClientMessage(err, "That conflicts with an existing record."), nil)

	default:
		// Unknown error: log the full detail for the operator, return a generic
		// message to the caller. Never leak an internal error string — it can
		// contain SQL, file paths, or a DSN with credentials.
		LogInternal(c, err)
		InternalError(c, "An unexpected error occurred.")
	}
}
