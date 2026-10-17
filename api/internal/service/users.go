package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// Users is staff administration: listing accounts, toggling them, and granting
// or withdrawing roles.
//
// ═══ WHY THIS IS ONLINE-ONLY ═══
//
// Every other write in this system goes through the offline outbox. This one
// does not, deliberately: a role change is an authorisation decision, and an
// authorisation decision made on a device that has been offline for six days is
// a decision made against a user list that may have changed. Admin screens are
// the documented exception to the local-first rule (CLAUDE.md §3.18), and this
// service is the reason.
type Users struct {
	pool   *pgxpool.Pool
	users  *repo.Users
	tokens *repo.RefreshTokens
	now    func() time.Time
}

// NewUsers wires the user-administration service.
func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{
		pool:   pool,
		users:  repo.NewUsers(pool),
		tokens: repo.NewRefreshTokens(),
		now:    time.Now,
	}
}

const (
	// DefaultUserPageSize is the admin table size. Eight staff at a PHC; 100 is
	// a page nobody will ever scroll, and still bounds a pathological response.
	DefaultUserPageSize = 100

	// MinRoleChangeReasonLen mirrors the CHECK constraint on user_roles.
	MinRoleChangeReasonLen = 5

	// DefaultRoleChangeReason is used when the admin does not type one.
	// It is still recorded, because the column requires a reason — but a generic
	// reason is honestly better than a blank one pretending to be specific.
	DefaultRoleChangeReason = "administrator change"
)

// ListUsers returns every visible account.
func (s *Users) ListUsers(ctx context.Context, actorID uuid.UUID) ([]*domain.User, error) {
	if _, err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}

	users, err := s.users.List(ctx, s.pool, DefaultUserPageSize, 0)
	if err != nil {
		return nil, err
	}
	if users == nil {
		users = []*domain.User{}
	}
	return users, nil
}

// UpdateUserInput describes a change to one account.
//
// Role is a TOGGLE, not a target set: setting it grants the role when absent and
// revokes it when present. Modelling it as "the roles this user should have"
// would require the caller to send the full set, and a stale client would then
// silently remove a role somebody granted thirty seconds ago.
type UpdateUserInput struct {
	ActorID uuid.UUID
	UserID  uuid.UUID
	Role    *domain.Role
	// IsActive, when set, enables or disables the account.
	IsActive *bool
	// Reason is recorded when a role is revoked or an account is disabled.
	Reason string
}

// UpdateUser applies the change and returns the updated account.
func (s *Users) UpdateUser(ctx context.Context, in UpdateUserInput) (*domain.User, error) {
	if _, err := s.requireAdmin(ctx, in.ActorID); err != nil {
		return nil, err
	}

	// ═══ SELF-LOCKOUT PREVENTION ═══
	//
	// An admin who removes their own admin role, or disables their own account,
	// locks themselves out of the only screen that could undo it. With one
	// administrator in a PHC deployment that is unrecoverable without SQL — and
	// "we had to fix production with a psql session" is not a sentence anyone
	// wants in a viva.
	if in.UserID == in.ActorID && (in.Role != nil || (in.IsActive != nil && !*in.IsActive)) {
		return nil, domain.Public(domain.ErrValidation,
			"You cannot change your own role or disable your own account. Ask another administrator.")
	}

	target, err := s.users.FindByID(ctx, s.pool, in.UserID)
	if err != nil {
		return nil, err
	}

	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = DefaultRoleChangeReason
	}
	if len(reason) < MinRoleChangeReasonLen {
		return nil, domain.NewValidationf("reason", "too_short",
			"%d characters minimum", MinRoleChangeReasonLen)
	}

	if in.Role != nil && !in.Role.Valid() {
		// The database CHECK would reject it too, but a 500 from a constraint
		// violation is a worse answer than a named field error.
		return nil, domain.NewValidation("role", "unknown_role")
	}

	if in.IsActive != nil && !*in.IsActive && domain.HasRole(target.Roles, domain.RoleAdmin) {
		// Disabling the last administrator is the same lockout by another route.
		if err := s.ensureAnotherAdminExists(ctx, target.ID); err != nil {
			return nil, err
		}
	}

	now := s.now()

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if in.Role != nil {
			if domain.HasRole(target.Roles, *in.Role) {
				if _, err := s.users.RevokeRole(ctx, tx, target.ID, *in.Role, in.ActorID, reason, now); err != nil {
					return err
				}
			} else {
				if err := s.users.GrantRole(ctx, tx, target.ID, *in.Role, in.ActorID); err != nil {
					return err
				}
			}
		}

		if in.IsActive != nil {
			if err := s.users.SetActive(ctx, tx, target.ID, *in.IsActive, in.ActorID); err != nil {
				return err
			}

			// Disabling an account must also end its sessions. Otherwise a
			// dismissed employee keeps working until their refresh token
			// expires — up to thirty days.
			if !*in.IsActive {
				if _, err := s.tokens.RevokeAllForUser(ctx, tx, target.ID, domain.RevokeAdmin); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	log.Info().
		Str("actor_id", in.ActorID.String()).
		Str("user_id", target.ID.String()).
		Str("reason", reason).
		Msg("user account updated")

	updated, err := s.users.FindByID(ctx, s.pool, target.ID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// requireAdmin re-checks the actor against the DATABASE.
//
// The route middleware already gated on the token's role, but that role can be
// fifteen minutes stale — and "we demoted that admin, why can they still change
// roles?" is exactly the question a replicated-offline design invites.
func (s *Users) requireAdmin(ctx context.Context, actorID uuid.UUID) (*domain.User, error) {
	actor, err := s.users.FindByID(ctx, s.pool, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled.")
	}
	if !domain.HasRole(actor.Roles, domain.RoleAdmin) {
		return nil, domain.Public(domain.ErrForbidden, "Only an administrator can manage users.")
	}
	return actor, nil
}

// ensureAnotherAdminExists refuses to disable the last active administrator.
func (s *Users) ensureAnotherAdminExists(ctx context.Context, excluding uuid.UUID) error {
	const sql = `
        SELECT count(*)
          FROM user_roles ur
          JOIN users u ON u.id = ur.user_id
         WHERE ur.role = 'admin'
           AND ur.revoked_at IS NULL
           AND ur.deleted_at IS NULL
           AND ur.user_id <> $1
           AND u.is_active
           AND u.deleted_at IS NULL`

	var n int
	if err := s.pool.QueryRow(ctx, sql, excluding).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return domain.Public(domain.ErrValidation,
			"That is the last active administrator. Grant another account the admin role first.")
	}
	return nil
}
