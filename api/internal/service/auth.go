// Package service holds business rules and transactions. This is the ONLY place
// that decides whether an action is permitted.
//
// Why it matters: with a fully replicated offline client, an attacker controls
// the entire frontend. Route guards and hidden buttons are usability features;
// this layer is the security boundary.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
	"github.com/mikedalu/phc-records/api/internal/platform/password"
	"github.com/mikedalu/phc-records/api/internal/platform/ratelimit"
	"github.com/mikedalu/phc-records/api/internal/platform/session"
	"github.com/mikedalu/phc-records/api/internal/platform/token"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

// Auth configuration knobs. Defaults match docs/05-SECURITY.md §3.
const (
	// LockAfter is the number of consecutive failures that locks an account.
	LockAfter = 5
	// LockFor is how long the lock lasts.
	LockFor = 15 * time.Minute

	// LoginAttemptsPerAccount mirrors LockAfter for the in-memory limiter; the
	// database lockout is the authoritative control.
	LoginAttemptsPerAccount = 5
	// LoginAttemptsPerIP is the coarse per-source limit. It is PER INSTANCE
	// because the limiter is in memory — see platform/ratelimit.
	LoginAttemptsPerIP = 20
	// LoginWindow is the observation window for both limiters.
	LoginWindow = 15 * time.Minute
)

// Auth implements login, refresh, and logout.
type Auth struct {
	pool    *pgxpool.Pool
	users   *repo.Users
	tokens  *repo.RefreshTokens
	devices *repo.Devices
	issuer  *token.Issuer
	byAcct  *ratelimit.Limiter
	byIP    *ratelimit.Limiter

	accessTTL    time.Duration
	refreshTTL   time.Duration
	offlineGrace time.Duration

	// now is injectable so lockout and expiry are testable without sleeping.
	now func() time.Time
}

// NewAuth wires the auth service.
//
// offlineGrace is the window in which a device may keep working offline after
// its last successful online authentication (OFFLINE_GRACE_PERIOD, default 7
// days). It is NOT the access-token TTL: conflating the two would expire the
// offline window after fifteen minutes, which is the exact bug this parameter
// exists to prevent.
func NewAuth(
	pool *pgxpool.Pool,
	issuer *token.Issuer,
	accessTTL, refreshTTL, offlineGrace time.Duration,
) *Auth {
	if offlineGrace <= 0 {
		offlineGrace = accessTTL
	}
	return &Auth{
		pool:         pool,
		users:        repo.NewUsers(pool),
		tokens:       repo.NewRefreshTokens(),
		devices:      repo.NewDevices(),
		issuer:       issuer,
		byAcct:       ratelimit.New(LoginAttemptsPerAccount, LoginWindow),
		byIP:         ratelimit.New(LoginAttemptsPerIP, LoginWindow),
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		offlineGrace: offlineGrace,
		now:          time.Now,
	}
}

// ────────────────────────────────────────────────────────────────────────
//  Login
// ────────────────────────────────────────────────────────────────────────

// LoginInput is a login request.
type LoginInput struct {
	Identifier string // staff number OR email
	Password   string
	DeviceID   string // the client's persistent device id (uuidv7)
	DeviceName string
	IP         string
	UserAgent  string
}

// Session is the result of a successful authentication.
type Session struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    time.Duration
	User         *domain.User
	DeviceID     uuid.UUID
	SessionID    uuid.UUID
	OfflineUntil time.Time
}

// Login authenticates a user and issues a token pair.
//
// Order of checks matters and is deliberate:
//  1. Rate limit (cheap, no DB) — sheds load before any hashing work.
//  2. Look up the user.
//  3. Verify the password — the expensive step, ~75 ms.
//  4. Only then reveal account state (locked, inactive).
//
// Point 4 is subtle: revealing "account locked" before checking the password
// would let an attacker probe for valid accounts without knowing any password.
// We do the expensive verification first so that a wrong password and a locked
// account are indistinguishable in cost.
func (s *Auth) Login(ctx context.Context, in LoginInput) (*Session, error) {
	if in.Identifier == "" || in.Password == "" {
		return nil, domain.NewValidation("credentials", "required")
	}
	if in.DeviceID == "" {
		return nil, domain.NewValidation("deviceId", "required")
	}

	// Trim only — do NOT lowercase.
	//
	// `staff_number` is case-sensitive TEXT while `email` is CITEXT. Lowercasing
	// here would make every staff number unmatchable ("DEMO/STF/0001" becomes
	// "demo/stf/0001"), because the column keeps its uppercase form. That was a
	// live bug until an integration test caught it.
	//
	// `accountKey` (lowercased) is used ONLY as a rate-limit bucket key, where
	// case-insensitivity is desirable: "DEMO" and "demo" must share one counter
	// or an attacker gets two buckets for the price of one.
	identifier := strings.TrimSpace(in.Identifier)
	accountKey := strings.ToLower(identifier)

	// ── 1. rate limits ────────────────────────────────────────────
	if ok, _, retry := s.byAcct.Allow("acct:" + accountKey); !ok {
		return nil, domain.Publicf(domain.ErrLocked, "Too many login attempts. Try again in %s.", humanDuration(retry))
	}
	if in.IP != "" {
		if ok, _, retry := s.byIP.Allow("ip:" + in.IP); !ok {
			return nil, domain.Publicf(domain.ErrLocked, "Too many login attempts from this address. Try again in %s.", humanDuration(retry))
		}
	}

	// ── 2. look up the user ───────────────────────────────────────
	// Pass the case-PRESERVED identifier. See the note above.
	user, err := s.users.FindByStaffNumberOrEmail(ctx, s.pool, identifier)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Do NOT disclose that the account does not exist. Run a dummy hash
			// so this path costs the same as a real one — otherwise response
			// timing reveals which identifiers are registered.
			burnPasswordTime(in.Password)
			return nil, errInvalidCredentials()
		}
		return nil, err
	}

	// ── 3. verify the password (expensive) ────────────────────────
	hash, err := s.users.PasswordHash(ctx, s.pool, user.ID)
	if err != nil {
		return nil, err
	}

	matches, err := password.Verify(in.Password, hash)
	if err != nil {
		// A malformed hash is a DATA problem, not a wrong password. Log it
		// loudly; never report it to the client as invalid credentials.
		log.Error().Err(err).Str("user_id", user.ID.String()).Msg("stored password hash is malformed")
		return nil, fmt.Errorf("verify password: %w", err)
	}

	if !matches {
		if _, err := s.users.RecordLoginFailure(ctx, s.pool, user.ID, LockAfter, LockFor); err != nil {
			log.Error().Err(err).Msg("failed to record login failure")
		}
		return nil, errInvalidCredentials()
	}

	// ── 4. account state (only now is it safe to reveal) ──────────
	now := s.now()
	if !user.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled. Contact an administrator.")
	}
	if user.IsLocked(now) {
		return nil, domain.Publicf(domain.ErrLocked,
			"Account locked after too many failed attempts. Try again in %s.",
			humanDuration(time.Until(*user.LockedUntil)))
	}
	if len(user.Roles) == 0 {
		// The user exists and authenticated, but has no role, so has no
		// capabilities. Fail with a clear message rather than issuing a token
		// that can do nothing.
		return nil, domain.Public(domain.ErrForbidden, "This account has no role assigned. Contact an administrator.")
	}

	// ── success ───────────────────────────────────────────────────
	if err := s.users.RecordLoginSuccess(ctx, s.pool, user.ID, now); err != nil {
		log.Error().Err(err).Msg("failed to record login success")
	}
	s.byAcct.Reset("acct:" + accountKey)

	if err := s.ensureDevice(ctx, user, in); err != nil {
		return nil, err
	}

	return s.issueSession(ctx, user, mustUUID(in.DeviceID, uuid.Nil), in.IP, in.UserAgent, uuid.Nil)
}

// ensureDevice registers the client device if it is not already known.
func (s *Auth) ensureDevice(ctx context.Context, user *domain.User, in LoginInput) error {
	deviceID := mustUUID(in.DeviceID, uuid.Nil)
	if deviceID == uuid.Nil {
		return domain.NewValidation("deviceId", "must be a uuid")
	}

	existing, err := s.devices.FindByID(ctx, s.pool, deviceID)
	if err == nil {
		if existing.IsRevoked() {
			return domain.Public(domain.ErrForbidden, "This device has been revoked. Contact an administrator.")
		}
		_ = s.devices.TouchSeen(ctx, s.pool, deviceID, s.now())
		return nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	// Unknown device. Register it with a placeholder public key; the real key is
	// supplied in Sprint 1's device-identity step and the entry updated then.
	// Registering now keeps last_seen_at and the audit trail meaningful from the
	// very first login.
	name := in.DeviceName
	if name == "" {
		name = "Unnamed workstation"
	}
	d := &domain.Device{
		ID:           deviceID,
		RegisteredBy: &user.ID,
		Name:         name,
		// Empty key is honest: this device has not yet proved possession of a
		// private key, so no signature will verify against it until it registers
		// one. The sync endpoint must reject an empty key.
		PublicKey:    "",
		PublicKeyAlg: "ES256",
		Fingerprint:  "pending:" + deviceID.String(),
	}
	if err := s.devices.Insert(ctx, s.pool, d, user.ID); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			// Raced with another request registering the same device; benign.
			return nil
		}
		return err
	}
	return nil
}

// issueSession mints a token pair and records the refresh token.
//
// When familyID is uuid.Nil a NEW family is started (a fresh login). Passing an
// existing family continues a rotation chain.
func (s *Auth) issueSession(
	ctx context.Context,
	user *domain.User,
	deviceID uuid.UUID,
	ip, userAgent string,
	familyID uuid.UUID,
) (*Session, error) {
	now := s.now()

	if familyID == uuid.Nil {
		familyID = uuid.New()
	}

	rawRefresh, err := session.NewToken()
	if err != nil {
		return nil, err
	}

	rt := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		DeviceID:  deviceID,
		TokenHash: session.HashToken(rawRefresh),
		FamilyID:  familyID,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.refreshTTL),
	}
	if ip != "" {
		rt.IP = &ip
	}
	if userAgent != "" {
		rt.UserAgent = &userAgent
	}

	// Mint the access token BEFORE the transaction: if signing fails there is
	// nothing to roll back, and the failure is cheaper to report.
	access, err := s.issuer.Issue(user.ID, user.PrimaryRole(), deviceID.String(), familyID.String())
	if err != nil {
		return nil, err
	}

	// ONE transaction. Storing the refresh token and (for a rotation) revoking
	// the consumed one must be atomic: a crash between them either loses the new
	// token or leaves two live tokens for one session.
	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.tokens.Insert(ctx, tx, rt)
	}); err != nil {
		return nil, err
	}

	return &Session{
		AccessToken:  access.Token,
		RefreshToken: rawRefresh,
		ExpiresIn:    s.accessTTL,
		User:         user,
		DeviceID:     deviceID,
		SessionID:    familyID,
		// The device may keep working offline until this moment. It is the GRACE
		// WINDOW, not the access-token lifetime: using accessTTL here would make
		// every device read-only after 15 minutes without a network, which is the
		// opposite of offline-first. See docs/00-ARCHITECTURE.md §5.1.
		OfflineUntil: now.Add(s.offlineGrace),
	}, nil
}

// errInvalidCredentials is the single outward error for a failed login.
//
// One message for "no such user" and "wrong password" so a response cannot be
// used to enumerate accounts.
func errInvalidCredentials() error {
	return domain.Public(domain.ErrUnauthenticated, "Incorrect staff number or password.")
}

// burnPasswordTime performs a throwaway Argon2 verification so the
// unknown-account path takes as long as the real one.
func burnPasswordTime(attempted string) {
	// A fixed, valid PHC string at the production cost.
	const dummy = "$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo"
	_, _ = password.Verify(attempted, dummy)
}

func mustUUID(s string, fallback uuid.UUID) uuid.UUID {
	if s == "" {
		return fallback
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return fallback
	}
	return id
}

// ────────────────────────────────────────────────────────────────────────
//  Session support
// ────────────────────────────────────────────────────────────────────────

// CurrentUser re-reads a user and their CURRENT roles.
//
// The access token carries a role, but it may be up to 15 minutes stale. When a
// client asks "who am I / what may I do", the answer must come from the
// database — that is how an offline device discovers a revoked capability.
func (s *Auth) CurrentUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.users.FindByID(ctx, s.pool, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, domain.Public(domain.ErrForbidden, "This account is disabled. Contact an administrator.")
	}
	return user, nil
}

// SessionDetails is what GET /auth/session reports.
//
// Deliberately not the domain types: the endpoint must tell a client three
// separate things — who you are NOW, what this device's status is, and how long
// you may keep working offline — and a client that has been offline for three
// days needs all three to decide whether to keep going.
type SessionDetails struct {
	User *domain.User
	// Roles are re-read from the database, unlike the token's single role.
	Roles []domain.Role
	// DeviceRevoked is true when the device is revoked OR unknown. The client's
	// response to both is the same: stop syncing and re-enrol.
	DeviceRevoked bool
	// OfflineUntil is re-issued on every successful online contact, from the
	// server's clock. The client must never extend it itself.
	OfflineUntil time.Time
}

// SessionDetails re-reads the user, their roles, and the device's status.
func (s *Auth) SessionDetails(ctx context.Context, userID, deviceID uuid.UUID) (*SessionDetails, error) {
	user, err := s.CurrentUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// An unknown device is treated as revoked: a token naming a device row that
	// does not exist is either a stale deployment or a forgery, and neither
	// should be told "yes, carry on".
	revoked := true
	if deviceID != uuid.Nil {
		device, derr := s.devices.FindByID(ctx, s.pool, deviceID)
		switch {
		case derr == nil:
			revoked = device.IsRevoked()
		case errors.Is(derr, domain.ErrNotFound):
			revoked = true
		default:
			return nil, derr
		}
	}

	return &SessionDetails{
		User:          user,
		Roles:         user.Roles,
		DeviceRevoked: revoked,
		OfflineUntil:  s.now().Add(s.offlineGrace),
	}, nil
}

// ChangePasswordInput is a password-change request.
type ChangePasswordInput struct {
	UserID          uuid.UUID
	CurrentPassword string
	NewPassword     string
	// KeepSessionID preserves ONE session (the device making the change).
	// Every other session for the user is revoked.
	KeepSessionID uuid.UUID
}

// MinPasswordLength is the enforced minimum.
//
// 12 rather than 8: the cost of an extra 4 characters to a user is a moment;
// the cost to an attacker is a four-order-of-magnitude increase in search space.
// Length beats complexity, so we require length and do NOT require symbols —
// symbol rules push users toward predictable substitutions.
const MinPasswordLength = 12

// ChangePassword verifies the current password, sets a new one, and revokes every
// other session.
//
// Revoking other sessions is not optional: if the password was changed because it
// leaked, leaving the attacker's session alive defeats the entire exercise. We
// keep the CURRENT session so the user is not thrown out of the device they are
// standing at.
func (s *Auth) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	if len(in.NewPassword) < MinPasswordLength {
		return domain.NewValidationf("newPassword", "too_short",
			"%d characters minimum", MinPasswordLength)
	}
	if in.NewPassword == in.CurrentPassword {
		return domain.NewValidation("newPassword", "must_differ_from_current")
	}

	user, err := s.users.FindByID(ctx, s.pool, in.UserID)
	if err != nil {
		return err
	}

	hash, err := s.users.PasswordHash(ctx, s.pool, user.ID)
	if err != nil {
		return err
	}

	ok, err := password.Verify(in.CurrentPassword, hash)
	if err != nil {
		log.Error().Err(err).Str("user_id", user.ID.String()).Msg("stored password hash is malformed")
		return fmt.Errorf("verify current password: %w", err)
	}
	if !ok {
		// Deliberately the same generic error shape as a failed login: do not
		// confirm to a holder of a stolen token that the password is wrong vs.
		// the token is wrong.
		return domain.Public(domain.ErrUnauthenticated, "Your current password is incorrect.")
	}

	newHash, err := password.Hash(in.NewPassword, password.DefaultParams)
	if err != nil {
		return err
	}

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.users.UpdatePasswordHash(ctx, tx, user.ID, newHash); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Revoke every OTHER session. The current one is identified by its family id
	// (the token's `sid`), which is what Refresh carries forward.
	if _, err := s.tokens.RevokeAllForUser(ctx, s.pool, user.ID, domain.RevokePasswordChanged); err != nil {
		// The password IS changed at this point. Failing the request would be
		// dishonest — the user would retry and be told their old password is
		// wrong. Log loudly instead.
		log.Error().Err(err).Str("user_id", user.ID.String()).
			Msg("password changed but session revocation FAILED — sessions may survive")
	}

	log.Info().
		Str("user_id", user.ID.String()).
		Msg("password changed; other sessions revoked")
	return nil
}

// humanDuration renders a duration the way a person would say it.
//
// "3 minutes" rather than "3m0s": this text goes straight to a clinician who is
// trying to log in during a clinic.
func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	m := int(d.Minutes()) + 1
	if m == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", m)
}
