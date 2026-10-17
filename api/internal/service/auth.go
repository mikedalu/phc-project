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

	accessTTL  time.Duration
	refreshTTL time.Duration

	// now is injectable so lockout and expiry are testable without sleeping.
	now func() time.Time
}

// NewAuth wires the auth service.
func NewAuth(
	pool *pgxpool.Pool,
	issuer *token.Issuer,
	accessTTL, refreshTTL time.Duration,
) *Auth {
	return &Auth{
		pool:       pool,
		users:      repo.NewUsers(pool),
		tokens:     repo.NewRefreshTokens(),
		devices:    repo.NewDevices(),
		issuer:     issuer,
		byAcct:     ratelimit.New(LoginAttemptsPerAccount, LoginWindow),
		byIP:       ratelimit.New(LoginAttemptsPerIP, LoginWindow),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
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
		return nil, fmt.Errorf("%w: too many attempts, retry in %s", domain.ErrLocked, retry.Round(time.Second))
	}
	if in.IP != "" {
		if ok, _, retry := s.byIP.Allow("ip:" + in.IP); !ok {
			return nil, fmt.Errorf("%w: too many attempts from this address, retry in %s", domain.ErrLocked, retry.Round(time.Second))
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
		return nil, fmt.Errorf("%w: account is disabled", domain.ErrForbidden)
	}
	if user.IsLocked(now) {
		until := user.LockedUntil.Format(time.RFC3339)
		return nil, fmt.Errorf("%w: account locked until %s", domain.ErrLocked, until)
	}
	if len(user.Roles) == 0 {
		// The user exists and authenticated, but has no role, so has no
		// capabilities. Fail with a clear message rather than issuing a token
		// that can do nothing.
		return nil, fmt.Errorf("%w: account has no role assigned; contact an administrator", domain.ErrForbidden)
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
			return fmt.Errorf("%w: this device has been revoked; contact an administrator", domain.ErrForbidden)
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
		// The device may keep working offline until this moment. See
		// docs/00-ARCHITECTURE.md §5.1.
		OfflineUntil: now.Add(s.accessTTL),
	}, nil
}

// errInvalidCredentials is the single outward error for a failed login.
//
// One message for "no such user" and "wrong password" so a response cannot be
// used to enumerate accounts.
func errInvalidCredentials() error {
	return fmt.Errorf("%w: incorrect staff number or password", domain.ErrUnauthenticated)
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
