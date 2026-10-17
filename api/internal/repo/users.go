// Package repo holds all SQL. Nothing here knows about HTTP.
//
// Every method takes a pgx.Tx or a pool-like interface so the service layer can
// compose several repository calls into one transaction. Role checks live in the
// service layer, never here — a repository that enforces permissions cannot be
// reused, and with a replicated offline client the frontend is attacker-
// controlled anyway.
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Querier is the subset of pgx shared by *pgxpool.Pool and pgx.Tx.
//
// Defining it here lets a repository method run either standalone or inside a
// transaction without duplicating every query. This is the standard Go idiom for
// exactly this problem.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Users provides access to users and their roles.
type Users struct {
	pool *pgxpool.Pool
}

// NewUsers builds a Users repository.
func NewUsers(pool *pgxpool.Pool) *Users { return &Users{pool: pool} }

// userColumns is the explicit column list.
//
// `SELECT *` is banned in this project: column order changes silently break
// Scan, and every added column costs bytes over a metered PHC link. Notable
// omission: password_hash is NOT selected here. Only FindForLogin reads it.
const userColumns = `
    u.id, u.staff_number, u.first_name, u.last_name, u.middle_name,
    u.email, u.phone, u.is_active, u.must_change_password,
    u.last_login_at, u.failed_login_count, u.locked_until,
    u.created_at, u.updated_at`

// FindByID loads a user and their active roles.
func (r *Users) FindByID(ctx context.Context, q Querier, id uuid.UUID) (*domain.User, error) {
	const sql = `
        SELECT` + userColumns + `
          FROM users u
         WHERE u.id = $1 AND u.deleted_at IS NULL`

	return r.scanOne(ctx, q, sql, id)
}

// FindByStaffNumberOrEmail loads a user by either identifier.
//
// ═══ TWO SEPARATE CASE-SENSITIVITY TRAPS LIVE HERE ═══
//
//  1. `staff_number` is TEXT (case-SENSITIVE) while `email` is CITEXT
//     (case-INSENSITIVE). Applying one lowercased comparison to both therefore
//     matches emails but NEVER staff numbers, because real ones are uppercase
//     ("DEMO/STF/0001" would become "demo/stf/0001" and match nothing). So the
//     identifier is passed through with only whitespace trimmed.
//
//  2. The `$1::citext` CAST IS NOT OPTIONAL. A Go string parameter is sent as
//     `text`, and Postgres resolves `citext = text` by falling back to the
//     `text = text` operator — which is CASE-SENSITIVE. Without the explicit
//     cast, "ADMIN@DEMO.PHC" does not match "admin@demo.phc" and the login fails
//     with a misleading "incorrect staff number or password".
//
//     This one is genuinely nasty to diagnose, because testing the same query in
//     psql with a string LITERAL works: the literal is typed as citext, so it
//     compares case-insensitively. The Go driver's parameterised form does not.
//     Verified empirically — a direct repo probe returned "not found" for the
//     mixed-case email until the cast was added.
//
//     Casting $1 (rather than u.email) keeps the citext index usable, since
//     citext is binary-coercible to text.
func (r *Users) FindByStaffNumberOrEmail(ctx context.Context, q Querier, identifier string) (*domain.User, error) {
	const sql = `
        SELECT` + userColumns + `
          FROM users u
         WHERE u.deleted_at IS NULL
           AND (u.staff_number = $1 OR u.email = $1::citext)`

	return r.scanOne(ctx, q, sql, identifier)
}

// scanOne runs a single-row query and attaches active roles.
func (r *Users) scanOne(ctx context.Context, q Querier, sql string, args ...any) (*domain.User, error) {
	var u domain.User
	err := q.QueryRow(ctx, sql, args...).Scan(
		&u.ID, &u.StaffNumber, &u.FirstName, &u.LastName, &u.MiddleName,
		&u.Email, &u.Phone, &u.IsActive, &u.MustChangePassword,
		&u.LastLoginAt, &u.FailedLoginCount, &u.LockedUntil,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return the DOMAIN error, not pgx's. The service layer must not
			// need to know which driver is in use.
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	roles, err := r.RolesForUser(ctx, q, u.ID)
	if err != nil {
		return nil, err
	}
	u.Roles = roles
	return &u, nil
}

// PasswordHash reads a user's password hash.
//
// Kept as a separate method so the hash is only ever loaded by the login path.
// Loading it into domain.User would risk it reaching a JSON response.
func (r *Users) PasswordHash(ctx context.Context, q Querier, userID uuid.UUID) (string, error) {
	const sql = `SELECT password_hash FROM users WHERE id = $1 AND deleted_at IS NULL`

	var hash string
	if err := q.QueryRow(ctx, sql, userID).Scan(&hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("read password hash: %w", err)
	}
	return hash, nil
}

// RolesForUser returns the user's active roles.
func (r *Users) RolesForUser(ctx context.Context, q Querier, userID uuid.UUID) ([]domain.Role, error) {
	const sql = `
        SELECT role
          FROM user_roles
         WHERE user_id = $1
           AND revoked_at IS NULL
           AND deleted_at IS NULL
         ORDER BY granted_at`

	rows, err := q.Query(ctx, sql, userID)
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}
	// REQUIRED: returns the connection to the pool. Without it the pool
	// exhausts under load and every request hangs.
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, domain.Role(role))
	}
	// Checked AFTER the loop: an error mid-iteration is otherwise invisible.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roles: %w", err)
	}
	return roles, nil
}

// RecordLoginSuccess stamps the login and clears the failure counter.
func (r *Users) RecordLoginSuccess(ctx context.Context, q Querier, userID uuid.UUID, at time.Time) error {
	const sql = `
        UPDATE users
           SET last_login_at = $2,
               failed_login_count = 0,
               locked_until = NULL,
               updated_at = now()
         WHERE id = $1`

	if _, err := q.Exec(ctx, sql, userID, at); err != nil {
		return fmt.Errorf("record login success: %w", err)
	}
	return nil
}

// RecordLoginFailure increments the failure counter and locks the account once
// the threshold is reached.
//
// The whole thing is ONE statement so two concurrent wrong passwords cannot
// both read count=4 and both write 5, losing an attempt. Doing this in two
// queries is a classic race that lets an attacker double their guesses.
func (r *Users) RecordLoginFailure(ctx context.Context, q Querier, userID uuid.UUID, lockAfter int, lockFor time.Duration) (int, error) {
	const sql = `
        UPDATE users
           SET failed_login_count = failed_login_count + 1,
               locked_until = CASE
                   WHEN failed_login_count + 1 >= $2 THEN now() + $3::interval
                   ELSE locked_until
               END,
               updated_at = now()
         WHERE id = $1
         RETURNING failed_login_count`

	var count int
	if err := q.QueryRow(ctx, sql, userID, lockAfter, lockFor.String()).Scan(&count); err != nil {
		return 0, fmt.Errorf("record login failure: %w", err)
	}
	return count, nil
}

// UpdatePasswordHash sets a new hash and clears must_change_password.
func (r *Users) UpdatePasswordHash(ctx context.Context, q Querier, userID uuid.UUID, hash string) error {
	const sql = `
        UPDATE users
           SET password_hash = $2,
               password_changed_at = now(),
               must_change_password = false,
               updated_at = now()
         WHERE id = $1`

	if _, err := q.Exec(ctx, sql, userID, hash); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	return nil
}

// List returns every visible user with their ACTIVE roles, ordered by staff
// number so the admin table is stable between refreshes.
//
// One query, not one-plus-N. The roles are aggregated in the same statement
// because an admin screen that fires a second query per row is fine at eight
// users and miserable at two hundred on a metered link.
func (r *Users) List(ctx context.Context, q Querier, limit, offset int) ([]*domain.User, error) {
	const sql = `
        SELECT` + userColumns + `,
               COALESCE(
                   array_agg(ur.role ORDER BY ur.granted_at)
                       FILTER (WHERE ur.role IS NOT NULL),
                   '{}'
               ) AS roles
          FROM users u
          LEFT JOIN user_roles ur
                 ON ur.user_id = u.id
                AND ur.revoked_at IS NULL
                AND ur.deleted_at IS NULL
         WHERE u.deleted_at IS NULL
         GROUP BY u.id
         ORDER BY u.staff_number
         LIMIT $1 OFFSET $2`

	rows, err := q.Query(ctx, sql, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	// REQUIRED: without Close the connection never returns to the pool and the
	// second page of this screen hangs forever.
	defer rows.Close()

	var out []*domain.User
	for rows.Next() {
		var (
			u     domain.User
			roles []string
		)
		if err := rows.Scan(
			&u.ID, &u.StaffNumber, &u.FirstName, &u.LastName, &u.MiddleName,
			&u.Email, &u.Phone, &u.IsActive, &u.MustChangePassword,
			&u.LastLoginAt, &u.FailedLoginCount, &u.LockedUntil,
			&u.CreatedAt, &u.UpdatedAt,
			&roles,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		for _, role := range roles {
			u.Roles = append(u.Roles, domain.Role(role))
		}
		out = append(out, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return out, nil
}

// SetActive enables or disables an account.
//
// Deactivating does NOT end existing sessions here; the service decides whether
// to also revoke them, because "disable this account" and "log them out now"
// are two decisions, not one.
func (r *Users) SetActive(ctx context.Context, q Querier, userID uuid.UUID, active bool, actorID uuid.UUID) error {
	const sql = `
        UPDATE users
           SET is_active = $2, updated_by = $3, updated_at = now()
         WHERE id = $1 AND deleted_at IS NULL`

	if _, err := q.Exec(ctx, sql, userID, active, actorID); err != nil {
		return fmt.Errorf("set user active: %w", err)
	}
	return nil
}

// GrantRole adds an active role assignment.
//
// ON CONFLICT DO NOTHING covers the partial unique index: granting a role the
// user already holds is a no-op rather than an error, because the caller's
// intent ("this user should have this role") is already satisfied.
func (r *Users) GrantRole(ctx context.Context, q Querier, userID uuid.UUID, role domain.Role, actorID uuid.UUID) error {
	const sql = `
        INSERT INTO user_roles (id, user_id, role, granted_by, created_by, updated_by)
        VALUES ($1, $2, $3, $4, $4, $4)
        ON CONFLICT DO NOTHING`

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("mint role id: %w", err)
	}

	if _, err := q.Exec(ctx, sql, id, userID, role.String(), actorID); err != nil {
		return fmt.Errorf("grant role: %w", err)
	}
	return nil
}

// RevokeRole withdraws an active role assignment and records why.
//
// Returns false when the user did not hold the role, which is not an error: the
// caller wanted the role gone, and it is gone.
func (r *Users) RevokeRole(
	ctx context.Context,
	q Querier,
	userID uuid.UUID,
	role domain.Role,
	actorID uuid.UUID,
	reason string,
	at time.Time,
) (bool, error) {
	const sql = `
        UPDATE user_roles
           SET revoked_at = $4,
               revoked_by = $3,
               revoked_reason = $5,
               updated_by = $3,
               updated_at = now()
         WHERE user_id = $1
           AND role = $2
           AND revoked_at IS NULL
           AND deleted_at IS NULL`

	tag, err := q.Exec(ctx, sql, userID, role.String(), actorID, at, reason)
	if err != nil {
		return false, fmt.Errorf("revoke role: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Create inserts a staff account with its initial password.
//
// `must_change_password` is set true on purpose: an administrator chooses the
// first password, which means the administrator knows it. Forcing a change at
// first sign-in is the only thing that makes that acceptable.
func (r *Users) Create(ctx context.Context, q Querier, u *domain.User, passwordHash string, actorID uuid.UUID) error {
	const sql = `
        INSERT INTO users (
            id, staff_number, email, phone, first_name, last_name, middle_name,
            password_hash, must_change_password, is_active, created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,true,true,$9,$9)`

	if _, err := q.Exec(ctx, sql,
		u.ID, u.StaffNumber, u.Email, u.Phone, u.FirstName, u.LastName, u.MiddleName,
		passwordHash, actorID,
	); err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: staff number or email already exists", domain.ErrConflict)
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}
