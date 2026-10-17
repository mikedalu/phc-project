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
