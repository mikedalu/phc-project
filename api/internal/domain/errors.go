// Package domain holds plain data types and sentinel errors.
//
// It imports nothing internal, so every other package may depend on it without
// creating a cycle. This is the bottom of the dependency graph:
//
//	http -> service -> repo -> domain
package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors. Services return these WRAPPED (%w) so the HTTP layer can
// classify the failure while the cause stays in the chain for the logs.
//
// The mapping to status codes lives in exactly one place — the HTTP error
// middleware — so a new endpoint cannot invent a new status for a known failure.
// See docs/04-CODING-STANDARDS.md §3.2 and docs/07-API-CONTRACT.md §1.4.
var (
	// ErrNotFound — the entity does not exist, or is soft-deleted.
	ErrNotFound = errors.New("not found")

	// ErrConflict — a uniqueness or optimistic-concurrency violation.
	ErrConflict = errors.New("conflict")

	// ErrForbidden — the actor is authenticated but lacks the capability,
	// or is acting outside their permitted scope.
	ErrForbidden = errors.New("forbidden")

	// ErrValidation — input is well-formed but violates a domain rule
	// (negative amount, illegal workflow transition, missing required pair).
	ErrValidation = errors.New("validation failed")

	// ErrUnauthenticated — no valid credentials were presented.
	ErrUnauthenticated = errors.New("unauthenticated")

	// ErrLocked — too many failed attempts; the account or referral is
	// temporarily frozen. Distinct from ErrForbidden because the client should
	// show "try again in N minutes", not "you may not do this".
	ErrLocked = errors.New("locked")

	// ErrStale — an optimistic-concurrency check failed (the row changed since
	// the client read it). Distinct from ErrConflict so the API can return the
	// server's current version and let the client reconcile.
	ErrStale = errors.New("stale revision")
)

// ValidationError carries field-level detail so a form can highlight the exact
// input that failed, rather than showing a generic banner.
//
// It wraps ErrValidation, so errors.Is(err, ErrValidation) still works and the
// HTTP layer maps it to 422 without a special case.
type ValidationError struct {
	Field  string
	Issue  string
	Detail string
}

func (e *ValidationError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Field, e.Issue, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Issue)
}

// Unwrap makes errors.Is(err, domain.ErrValidation) return true.
func (e *ValidationError) Unwrap() error { return ErrValidation }

// NewValidation builds a ValidationError.
func NewValidation(field, issue string) *ValidationError {
	return &ValidationError{Field: field, Issue: issue}
}

// NewValidationf builds a ValidationError with extra detail.
func NewValidationf(field, issue, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Issue: issue, Detail: fmt.Sprintf(format, args...)}
}

// ────────────────────────────────────────────────────────────────────────
//  The role model — eight roles, no more, no fewer.
//  Kept here rather than in the database layer so the compiler catches a typo
//  in a capability check. The DB CHECK constraint mirrors this list.
// ────────────────────────────────────────────────────────────────────────

// Role is one of the eight staff roles.
type Role string

const (
	RoleFrontDesk    Role = "front_desk"
	RoleNurse        Role = "nurse"
	RoleDoctor       Role = "doctor"
	RoleLabScientist Role = "lab_scientist"
	RolePharmacist   Role = "pharmacist"
	RoleCashier      Role = "cashier"
	RoleAccountant   Role = "accountant"
	RoleAdmin        Role = "admin"
)

// AllRoles lists every valid role, for validation and for seeding.
var AllRoles = []Role{
	RoleFrontDesk, RoleNurse, RoleDoctor, RoleLabScientist,
	RolePharmacist, RoleCashier, RoleAccountant, RoleAdmin,
}

// Valid reports whether r is one of the eight known roles.
func (r Role) Valid() bool {
	switch r {
	case RoleFrontDesk, RoleNurse, RoleDoctor, RoleLabScientist,
		RolePharmacist, RoleCashier, RoleAccountant, RoleAdmin:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (r Role) String() string { return string(r) }

// HasRole reports whether the set contains r.
//
// Used by the authorisation layer. Note it is a plain O(n) scan over at most
// eight entries — building a map here would cost more than it saves.
func HasRole(roles []Role, r Role) bool {
	for _, have := range roles {
		if have == r {
			return true
		}
	}
	return false
}

// HasAnyRole reports whether the set contains at least one of the wanted roles.
func HasAnyRole(roles []Role, wanted ...Role) bool {
	for _, w := range wanted {
		if HasRole(roles, w) {
			return true
		}
	}
	return false
}

// ────────────────────────────────────────────────────────────────────────
//  PublicError: an error that carries a message safe to show a user.
// ────────────────────────────────────────────────────────────────────────

// PublicError is a domain error with a client-facing message.
//
// ═══ WHY THIS EXISTS ═══
//
// Wrapping with fmt.Errorf("%w: detail", ErrUnauthenticated) makes errors.Is work,
// but it also prepends the sentinel's internal text to the message:
//
//	"unauthenticated: incorrect staff number or password"
//
// Shipping that to a client is sloppy — it exposes implementation vocabulary and
// tells an attacker the internal taxonomy of failures. PublicError separates the
// two: Unwrap() provides the sentinel for classification, Msg is what the user
// reads.
//
// Usage:
//
//	return domain.Public(domain.ErrUnauthenticated, "Incorrect staff number or password.")
type PublicError struct {
	kind error  // sentinel for errors.Is
	msg  string // safe to display
}

// Public builds a PublicError.
func Public(kind error, msg string) *PublicError {
	return &PublicError{kind: kind, msg: msg}
}

// Publicf builds a PublicError with a formatted message.
func Publicf(kind error, format string, args ...any) *PublicError {
	return &PublicError{kind: kind, msg: fmt.Sprintf(format, args...)}
}

// Error returns the public message, so a logged error still reads sensibly.
func (e *PublicError) Error() string { return e.msg }

// Unwrap exposes the sentinel so errors.Is(err, ErrUnauthenticated) works.
func (e *PublicError) Unwrap() error { return e.kind }

// Message returns the client-facing text.
func (e *PublicError) Message() string { return e.msg }

// ClientMessage returns the text safe to send to a user.
//
// If err is a PublicError, its message is returned. Otherwise a generic message
// is chosen from the sentinel kind — never the raw wrapped error, which may
// contain SQL, file paths or a DSN.
func ClientMessage(err error, fallback string) string {
	var pe *PublicError
	if errors.As(err, &pe) {
		return pe.Message()
	}
	return fallback
}
