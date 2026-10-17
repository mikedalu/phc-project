package domain

import (
	"time"

	"github.com/google/uuid"
)

// User is a staff account.
//
// Note what is NOT here: PasswordHash is deliberately excluded, so it cannot be
// accidentally serialised into a response. The repository reads it into a
// separate field only for the login path, and the HTTP layer constructs its own
// DTO from this struct. Forgetting to strip a password hash from a JSON response
// is one of the most common serious bugs in auth code; keeping it off the domain
// type makes the mistake impossible rather than merely inadvisable.
type User struct {
	ID          uuid.UUID
	StaffNumber string
	FirstName   string
	LastName    string
	MiddleName  *string
	Email       *string
	Phone       *string
	IsActive    bool

	// MustChangePassword forces a password change at next login. Set true on
	// account creation and on admin reset.
	MustChangePassword bool

	LastLoginAt      *time.Time
	FailedLoginCount int
	LockedUntil      *time.Time

	// Roles are loaded alongside the user, because every authorisation decision
	// needs them and a second round trip would be both slower and racier.
	Roles []Role

	CreatedAt time.Time
	UpdatedAt time.Time
}

// DisplayName returns "First Last".
func (u *User) DisplayName() string {
	return u.FirstName + " " + u.LastName
}

// IsLocked reports whether the account is currently locked out.
func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && now.Before(*u.LockedUntil)
}

// CanLogIn reports whether the account may authenticate at all.
func (u *User) CanLogIn(now time.Time) bool {
	return u.IsActive && !u.IsLocked(now)
}

// PrimaryRole returns the role used for the JWT `role` claim.
//
// A user may hold several roles, but the token carries one. We pick the MOST
// privileged, which is safe precisely because the token's role is used only for
// offline UI decisions — every real authorisation decision re-checks the
// database. Choosing the least privileged instead would hide capabilities from
// an admin offline, which is a usability problem with no security benefit.
func (u *User) PrimaryRole() Role {
	// Ordered most to least privileged. This is a deliberate ranking, not
	// alphabetical: `admin` outranks `doctor`, who outranks `nurse`.
	order := []Role{
		RoleAdmin, RoleDoctor, RoleNurse, RolePharmacist,
		RoleLabScientist, RoleCashier, RoleAccountant, RoleFrontDesk,
	}
	for _, r := range order {
		if HasRole(u.Roles, r) {
			return r
		}
	}
	return ""
}

// Device is a registered workstation.
type Device struct {
	ID             uuid.UUID
	RegisteredBy   *uuid.UUID
	Name           string
	Platform       *string
	PublicKey      string // base64url SPKI
	PublicKeyAlg   string
	Fingerprint    string
	AppVersion     *string
	LastSeenAt     *time.Time
	LastSyncCursor int64

	RevokedAt     *time.Time
	RevokedBy     *uuid.UUID
	RevokedReason *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsRevoked reports whether the device may no longer be used.
func (d *Device) IsRevoked() bool { return d.RevokedAt != nil }

// RefreshToken is one issued refresh token.
//
// Never carries the raw token — only the hash. See internal/platform/session.
type RefreshToken struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	DeviceID      uuid.UUID
	TokenHash     string
	FamilyID      uuid.UUID
	IssuedAt      time.Time
	ExpiresAt     time.Time
	UsedAt        *time.Time
	RevokedAt     *time.Time
	RevokedReason *string
	IP            *string
	UserAgent     *string
}

// IsUsable reports whether the token may still be exchanged.
func (t *RefreshToken) IsUsable(now time.Time) bool {
	return t.UsedAt == nil && t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// Revocation reasons, mirroring the DB CHECK constraint.
const (
	RevokeRotated         = "rotated"
	RevokeReuseDetected   = "reuse_detected"
	RevokeLogout          = "logout"
	RevokeDeviceRevoked   = "device_revoked"
	RevokeAdmin           = "admin"
	RevokePasswordChanged = "password_changed"
)
