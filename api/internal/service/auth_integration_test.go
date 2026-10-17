package service_test

// Integration tests for the auth service.
//
// These run against a REAL PostgreSQL database because the behaviour that
// matters here — atomic token consumption, family revocation, lockout counters —
// lives in SQL. A mock would prove nothing about any of it.
//
// Skipped automatically when SERVICE_TEST_DATABASE_URL is unset, so `go test ./...`
// stays fast and never requires a database.
//
// Run them with:
//
//	SERVICE_TEST_DATABASE_URL="postgres://..." go test ./internal/service/ -v

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/platform/password"
	"github.com/mikedalu/phc-records/api/internal/platform/token"
	"github.com/mikedalu/phc-records/api/internal/service"
)

const testPassword = "TestPassword!2026"

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("SERVICE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SERVICE_TEST_DATABASE_URL not set — skipping auth integration tests")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// newAuthFixture creates a user with a known password and returns the service.
func newAuthFixture(t *testing.T, pool *pgxpool.Pool) (*service.Auth, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	hash, err := password.Hash(testPassword, password.DefaultParams)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	userID := uuid.New()
	staffNo := "TEST/" + userID.String()[:8]

	// Clean slate for this user id.
	if _, err := pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("clear tokens: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("clear roles: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("clear user: %v", err)
	}

	const insertUser = `
        INSERT INTO users (id, staff_number, first_name, last_name, email,
                           password_hash, must_change_password, created_by, updated_by)
        VALUES ($1,$2,'Test','Nurse',$3,$4,false,$1,$1)`
	if _, err := pool.Exec(ctx, insertUser, userID, staffNo, staffNo+"@test.local", hash); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	const insertRole = `
        INSERT INTO user_roles (id, user_id, role, granted_by, created_by, updated_by)
        VALUES ($1,$2,'nurse',$2,$2,$2)`
	if _, err := pool.Exec(ctx, insertRole, uuid.New(), userID); err != nil {
		t.Fatalf("insert role: %v", err)
	}

	iss, err := token.NewIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	auth := service.NewAuth(pool, iss, 15*time.Minute, 30*24*time.Hour)

	// Make the account discoverable by staff number in Login's lookup.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_roles WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	// Stash the staff number for the caller via the email-free lookup path.
	_ = staffNo
	return auth, userID
}

func loginHelper(t *testing.T, auth *service.Auth, staffNumber string) *service.Session {
	t.Helper()
	sess, err := auth.Login(context.Background(), service.LoginInput{
		Identifier: staffNumber,
		Password:   testPassword,
		DeviceID:   uuid.NewString(),
		DeviceName: "test workstation",
		IP:         "127.0.0.1",
		UserAgent:  "go-test",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return sess
}

func staffNumberFor(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(),
		`SELECT staff_number FROM users WHERE id = $1`, userID).Scan(&s); err != nil {
		t.Fatalf("read staff number: %v", err)
	}
	return s
}

// TestLoginIssuesUsableSession is the happy path, and pins that the access token
// verifies and carries the right identity and role.
func TestLoginIssuesUsableSession(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	sess := loginHelper(t, auth, staffNo)

	if sess.AccessToken == "" || sess.RefreshToken == "" {
		t.Fatal("expected both tokens")
	}
	if sess.User.ID != userID {
		t.Errorf("wrong user in session")
	}
	if sess.User.PrimaryRole() != domain.RoleNurse {
		t.Errorf("expected nurse role, got %q", sess.User.PrimaryRole())
	}

	iss, _ := token.NewIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	v, err := iss.Verify(sess.AccessToken)
	if err != nil {
		t.Fatalf("issued access token does not verify: %v", err)
	}
	if v.UserID != userID {
		t.Error("access token carries the wrong user")
	}
	if v.SessionID != sess.SessionID.String() {
		t.Error("access token sid must equal the refresh family id")
	}
}

// TestWrongPasswordRejectedAndCounted pins the failure counter, which drives
// lockout.
func TestWrongPasswordRejectedAndCounted(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	_, err := auth.Login(context.Background(), service.LoginInput{
		Identifier: staffNo,
		Password:   "wrong",
		DeviceID:   uuid.NewString(),
	})
	if err == nil {
		t.Fatal("expected login to fail with a wrong password")
	}
	if !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT failed_login_count FROM users WHERE id = $1`, userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected failed_login_count 1, got %d", count)
	}
}

// TestAccountLockoutAfterFiveFailures proves the lockout actually engages.
func TestAccountLockoutAfterFiveFailures(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	ctx := context.Background()
	for i := 0; i < service.LockAfter; i++ {
		_, _ = auth.Login(ctx, service.LoginInput{
			Identifier: staffNo, Password: "wrong", DeviceID: uuid.NewString(),
		})
	}

	// Now the CORRECT password must still be refused, because the account is
	// locked. This is the property that makes lockout meaningful.
	_, err := auth.Login(ctx, service.LoginInput{
		Identifier: staffNo, Password: testPassword, DeviceID: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("expected the locked account to refuse even a correct password")
	}
	if !errors.Is(err, domain.ErrLocked) {
		t.Fatalf("expected ErrLocked, got %v", err)
	}

	var lockedUntil *time.Time
	if err := pool.QueryRow(ctx, `SELECT locked_until FROM users WHERE id = $1`, userID).
		Scan(&lockedUntil); err != nil {
		t.Fatal(err)
	}
	if lockedUntil == nil {
		t.Error("expected locked_until to be set")
	}
}

// TestRefreshRotatesToken proves a valid token is exchanged for a NEW one and
// the old one dies.
func TestRefreshRotatesToken(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	sess := loginHelper(t, auth, staffNo)

	rotated, err := auth.Refresh(context.Background(), service.RefreshInput{
		RefreshToken: sess.RefreshToken,
		DeviceID:     sess.DeviceID.String(),
	})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if rotated.RefreshToken == sess.RefreshToken {
		t.Fatal("refresh must return a NEW token, not the same one")
	}
	if rotated.SessionID != sess.SessionID {
		t.Error("rotation must stay in the same family")
	}
	_ = userID
}

// TestRefreshReuseRevokesFamily is THE test in this file.
//
// Sequence:
//  1. log in                     -> token A
//  2. refresh with A             -> token B (A is now consumed)
//  3. refresh with A AGAIN       -> must fail
//  4. refresh with B             -> must ALSO fail, because the family died
//
// Step 4 is what distinguishes real reuse detection from a no-op. A system that
// merely rejects the reused token leaves the thief holding B, which is useless.
func TestRefreshReuseRevokesFamily(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	ctx := context.Background()
	sess := loginHelper(t, auth, staffNo)

	// 2. Legitimate rotation.
	rotated, err := auth.Refresh(ctx, service.RefreshInput{
		RefreshToken: sess.RefreshToken, DeviceID: sess.DeviceID.String(),
	})
	if err != nil {
		t.Fatalf("first refresh should succeed: %v", err)
	}

	// 3. Replay the consumed token — the attacker.
	_, err = auth.Refresh(ctx, service.RefreshInput{
		RefreshToken: sess.RefreshToken, DeviceID: sess.DeviceID.String(),
	})
	if err == nil {
		t.Fatal("SECURITY: a reused refresh token was accepted")
	}
	if !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}

	// 4. The token the legitimate client holds must now be dead too.
	_, err = auth.Refresh(ctx, service.RefreshInput{
		RefreshToken: rotated.RefreshToken, DeviceID: sess.DeviceID.String(),
	})
	if err == nil {
		t.Fatal("SECURITY: the family was not revoked — the sibling token still works")
	}

	// And the database agrees: zero live tokens remain for this user.
	var live int
	if err := pool.QueryRow(ctx, `
        SELECT count(*) FROM refresh_tokens
         WHERE user_id = $1 AND revoked_at IS NULL AND used_at IS NULL`, userID).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Errorf("expected 0 live tokens after family revocation, got %d", live)
	}
}

// TestRefreshRejectsRevokedDevice proves a revoked device stops working at once,
// even with a structurally valid token.
func TestRefreshRejectsRevokedDevice(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	ctx := context.Background()
	sess := loginHelper(t, auth, staffNo)

	// Revoke the device.
	if _, err := pool.Exec(ctx, `
        UPDATE devices SET revoked_at = now(), revoked_by = $2, revoked_reason = 'test'
         WHERE id = $1`, sess.DeviceID, userID); err != nil {
		t.Fatalf("revoke device: %v", err)
	}

	_, err := auth.Refresh(ctx, service.RefreshInput{
		RefreshToken: sess.RefreshToken, DeviceID: sess.DeviceID.String(),
	})
	if err == nil {
		t.Fatal("SECURITY: a revoked device was still able to refresh")
	}
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

// TestRefreshRejectsWrongDevice proves a token copied to another device fails.
func TestRefreshRejectsWrongDevice(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	sess := loginHelper(t, auth, staffNo)

	_, err := auth.Refresh(context.Background(), service.RefreshInput{
		RefreshToken: sess.RefreshToken,
		DeviceID:     uuid.NewString(), // a different machine
	})
	if err == nil {
		t.Fatal("SECURITY: a refresh token was accepted from a different device")
	}
}

// TestLogoutRevokesFamily proves logout actually kills the session, and that a
// second logout is idempotent rather than an error.
func TestLogoutRevokesFamily(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	ctx := context.Background()
	sess := loginHelper(t, auth, staffNo)

	if err := auth.Logout(ctx, sess.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}

	_, err := auth.Refresh(ctx, service.RefreshInput{
		RefreshToken: sess.RefreshToken, DeviceID: sess.DeviceID.String(),
	})
	if err == nil {
		t.Fatal("a logged-out session was still able to refresh")
	}

	// Idempotent second logout.
	if err := auth.Logout(ctx, sess.RefreshToken); err != nil {
		t.Errorf("logout must be idempotent, got: %v", err)
	}
}

// TestLogoutUnknownTokenIsNotAnError — a retried logout after a lost response
// must not surface an error, or the client retries forever.
func TestLogoutUnknownTokenIsNotAnError(t *testing.T) {
	pool := testPool(t)
	auth, _ := newAuthFixture(t, pool)

	if err := auth.Logout(context.Background(), "not-a-real-token"); err != nil {
		t.Errorf("logout with an unknown token should be a no-op, got: %v", err)
	}
}

// TestDisabledAccountCannotLogIn.
func TestDisabledAccountCannotLogIn(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET is_active = false WHERE id = $1`, userID); err != nil {
		t.Fatal(err)
	}

	_, err := auth.Login(context.Background(), service.LoginInput{
		Identifier: staffNo, Password: testPassword, DeviceID: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("a disabled account was able to log in")
	}
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

// TestUserWithNoRoleCannotLogIn — an account with no role can do nothing, so
// issuing it a token only produces confusing failures later.
func TestUserWithNoRoleCannotLogIn(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	if _, err := pool.Exec(context.Background(),
		`DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		t.Fatal(err)
	}

	_, err := auth.Login(context.Background(), service.LoginInput{
		Identifier: staffNo, Password: testPassword, DeviceID: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("an account with no roles was able to log in")
	}
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

// TestUnknownAccountGivesSameErrorAsWrongPassword proves the error is
// indistinguishable, so responses cannot be used to enumerate accounts.
func TestUnknownAccountGivesSameErrorAsWrongPassword(t *testing.T) {
	pool := testPool(t)
	auth, userID := newAuthFixture(t, pool)
	staffNo := staffNumberFor(t, pool, userID)

	ctx := context.Background()

	_, errWrong := auth.Login(ctx, service.LoginInput{
		Identifier: staffNo, Password: "definitely-wrong", DeviceID: uuid.NewString(),
	})
	_, errNoSuch := auth.Login(ctx, service.LoginInput{
		Identifier: "NO/SUCH/STAFF/9999", Password: "whatever", DeviceID: uuid.NewString(),
	})

	if errWrong == nil || errNoSuch == nil {
		t.Fatal("both attempts should fail")
	}
	if errWrong.Error() != errNoSuch.Error() {
		t.Errorf("SECURITY: the two errors differ, enabling account enumeration:\n  wrong password: %v\n  no such user:   %v",
			errWrong, errNoSuch)
	}
	fmt.Printf("  identical error for both: %v\n", errWrong)
}

// TestSeededAdminCanLogIn is the first thing that should have been tested: does a
// FRESH INSTALLATION actually work?
//
// It catches two real bugs that were live during development:
//
//  1. The seeded admin had no role, so it could authenticate (or not, see below)
//     but authorise nothing.
//  2. A case-sensitivity bug made EVERY login fail: the service lowercased the
//     identifier, but staff_number is case-sensitive TEXT, so "DEMO/STF/0001"
//     became "demo/stf/0001" and matched nothing. Separately, a Go string
//     parameter against a CITEXT column compared case-sensitively without an
//     explicit ::citext cast.
//
// If this test fails, a fresh deployment cannot be used at all.
func TestSeededAdminCanLogIn(t *testing.T) {
	pool := testPool(t)

	iss, err := token.NewIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	auth := service.NewAuth(pool, iss, 15*time.Minute, 30*24*time.Hour)

	// By staff number, exactly as documented in migration 00004.
	sess, err := auth.Login(context.Background(), service.LoginInput{
		Identifier: "DEMO/STF/0001",
		Password:   "ChangeMe!2026",
		DeviceID:   uuid.NewString(),
		DeviceName: "bootstrap check",
	})
	if err != nil {
		t.Fatalf("the seeded admin cannot log in by staff number: %v", err)
	}
	if sess.User.PrimaryRole() != domain.RoleAdmin {
		t.Fatalf("seeded admin role is %q, expected admin — run migration 00008", sess.User.PrimaryRole())
	}
	if !sess.User.MustChangePassword {
		t.Error("the seeded admin must be forced to change the documented password")
	}

	// By email, in a DIFFERENT CASE. This is the CITEXT path, and it only works
	// because the query casts the parameter (`$1::citext`).
	auth.ResetRateLimits()
	if _, err := auth.Login(context.Background(), service.LoginInput{
		Identifier: "ADMIN@DEMO.PHC",
		Password:   "ChangeMe!2026",
		DeviceID:   uuid.NewString(),
	}); err != nil {
		t.Fatalf("mixed-case email login failed — is the ::citext cast present in FindByStaffNumberOrEmail? %v", err)
	}

	// Whitespace tolerance.
	auth.ResetRateLimits()
	if _, err := auth.Login(context.Background(), service.LoginInput{
		Identifier: "  admin@demo.phc  ",
		Password:   "ChangeMe!2026",
		DeviceID:   uuid.NewString(),
	}); err != nil {
		t.Fatalf("email login with surrounding whitespace failed: %v", err)
	}
}
