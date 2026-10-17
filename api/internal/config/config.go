// Package config loads and validates all application configuration.
//
// Design rule: fail fast and loudly at startup. A missing JWT_SECRET must crash
// the container on boot, not surface as a confusing 500 on the first login
// attempt three days later.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every setting the application needs.
//
// Fields are grouped by concern and every value is resolved once at startup, so
// no code anywhere else in the program calls os.Getenv.
type Config struct {
	// dotEnvPath records which .env file was loaded, for Warnings(). Not
	// configuration itself: it must never be printed in production responses.
	dotEnvPath string
	// envLoadWasNeeded is true when the process environment was empty before
	// the file was read — i.e. the .env actually saved the day.
	envLoadWasNeeded bool

	AppEnv   string
	LogLevel string
	Port     string

	DatabaseURL          string
	MigrationDatabaseURL string
	DBMaxConns           int32

	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	OfflineGrace  time.Duration
	OfflinePolicy string // "block_writes" | "warn_only"

	AllowedOrigins []string
	TrustProxy     bool

	ReferralTTLHours      int
	ReferralPINRequired   bool
	ReferralEncryptionKey []byte

	SyncMaxBatchSize  int
	SyncMaxBatchBytes int
	SyncRetentionDays int
}

// Offline policies. Kept as constants so the compiler catches typos, and so the
// values are greppable from the handlers that branch on them.
const (
	OfflinePolicyBlockWrites = "block_writes"
	OfflinePolicyWarnOnly    = "warn_only"
)

// Load reads the environment, applies defaults, and validates.
//
// It returns an error rather than calling log.Fatal, so tests can exercise the
// validation logic without killing the test binary.
func Load() (*Config, error) {
	// ═══ WHY TWO PATHS ═══
	//
	// `make api` runs `cd api && go run ./cmd/api`, so the working directory is
	// api/ — and godotenv.Load() with no argument looks for ./.env, which does
	// not exist there. The repository's .env is one level up. Without the
	// fallback, a perfectly good .env is silently ignored and the server dies
	// with "DATABASE_URL is required", which looks like a missing secret rather
	// than a working-directory problem.
	//
	// Best-effort either way: in production the environment is already populated
	// and there is no .env, so a missing file is not an error.
	// Only worth reporting if the file actually supplied something. Under
	// `make api` the Makefile has already exported these, so a redundant load
	// must not print a warning on every single start.
	neededFromFile := os.Getenv("DATABASE_URL") == ""
	dotEnvPath := ""
	for _, candidate := range []string{".env", "../.env"} {
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		if err := godotenv.Load(candidate); err == nil {
			dotEnvPath = candidate
			break
		}
	}

	c := &Config{
		dotEnvPath:       dotEnvPath,
		envLoadWasNeeded: neededFromFile,
		AppEnv:           getEnv("APP_ENV", "development"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		// Cloud Run injects PORT and health-checks that exact port. This default
		// is for local development only. Never hard-code a port in main.
		Port: getEnv("PORT", "8080"),

		DatabaseURL:          os.Getenv("DATABASE_URL"),
		MigrationDatabaseURL: os.Getenv("MIGRATION_DATABASE_URL"),

		AllowedOrigins: splitAndTrim(getEnv("ALLOWED_ORIGINS", "http://localhost:5173")),
		TrustProxy:     getEnv("TRUST_PROXY", "false") == "true",

		OfflinePolicy: getEnv("OFFLINE_POLICY", OfflinePolicyBlockWrites),

		ReferralPINRequired: getEnv("REFERRAL_PIN_REQUIRED", "true") == "true",
	}

	// ─── required in every environment ─────────────────────────────
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required (Neon pooled endpoint)")
	}

	// ─── numeric and duration parsing ──────────────────────────────
	var err error
	if c.DBMaxConns, err = parseInt32("DB_MAX_CONNS", 10); err != nil {
		return nil, err
	}
	if c.AccessTokenTTL, err = parseDuration("ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return nil, err
	}
	if c.RefreshTokenTTL, err = parseDuration("REFRESH_TOKEN_TTL", 720*time.Hour); err != nil {
		return nil, err
	}
	if c.OfflineGrace, err = parseDuration("OFFLINE_GRACE_PERIOD", 168*time.Hour); err != nil {
		return nil, err
	}
	if c.ReferralTTLHours, err = parseInt("REFERRAL_TTL_HOURS", 48); err != nil {
		return nil, err
	}
	if c.SyncMaxBatchSize, err = parseInt("SYNC_MAX_BATCH_SIZE", 200); err != nil {
		return nil, err
	}
	if c.SyncMaxBatchBytes, err = parseInt("SYNC_MAX_BATCH_BYTES", 1<<20); err != nil {
		return nil, err
	}
	if c.SyncRetentionDays, err = parseInt("SYNC_CHANGE_LOG_RETENTION_DAYS", 30); err != nil {
		return nil, err
	}

	// ─── enumerated values ─────────────────────────────────────────
	if c.OfflinePolicy != OfflinePolicyBlockWrites && c.OfflinePolicy != OfflinePolicyWarnOnly {
		return nil, fmt.Errorf(
			"OFFLINE_POLICY must be %q or %q, got %q",
			OfflinePolicyBlockWrites, OfflinePolicyWarnOnly, c.OfflinePolicy,
		)
	}

	// ─── secrets ───────────────────────────────────────────────────
	//
	// Required in production, optional in development. They are not used until
	// Sprint 1, and refusing to boot locally over a secret you have no use for
	// yet is friction with no benefit. The warning keeps it visible.
	c.JWTSecret = []byte(os.Getenv("JWT_SECRET"))
	c.ReferralEncryptionKey = []byte(os.Getenv("REFERRAL_TOKEN_ENCRYPTION_KEY"))

	if c.IsProduction() {
		if len(c.JWTSecret) < 32 {
			return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters in production (got %d)", len(c.JWTSecret))
		}
		if len(c.ReferralEncryptionKey) < 32 {
			return nil, fmt.Errorf("REFERRAL_TOKEN_ENCRYPTION_KEY must be at least 32 characters in production (got %d)", len(c.ReferralEncryptionKey))
		}
		// MIGRATION_DATABASE_URL is intentionally NOT required here.
		//
		// It points at Neon's DIRECT endpoint, which only goose uses. The running
		// API must use the POOLED endpoint for every query, so requiring the
		// direct URL at boot would force a production secret onto the service for
		// no reason — and hand the API a credential it does not need, widening
		// the blast radius of a compromise.
		//
		// Migrations therefore run as a separate step with their own credentials:
		// locally via `make migrate-up`, in the cloud via a Cloud Run Job or a CI
		// step. That separation is deliberate, not an inconvenience.
		if len(c.AllowedOrigins) == 0 || (len(c.AllowedOrigins) == 1 && c.AllowedOrigins[0] == "*") {
			return nil, fmt.Errorf("ALLOWED_ORIGINS must be an explicit allow-list in production, not \"*\"")
		}
	}

	// ─── production posture checks ─────────────────────────────────
	if c.IsProduction() && c.Port == "" {
		return nil, fmt.Errorf("PORT must not be empty")
	}

	return c, nil
}

// IsProduction reports whether the app is running in production.
func (c *Config) IsProduction() bool { return c.AppEnv == "production" }

// Warnings returns non-fatal configuration problems worth logging at startup.
// Keeping these separate from Load lets the caller log them with the app logger
// after it is configured.
func (c *Config) Warnings() []string {
	var w []string

	if !c.IsProduction() {
		if c.dotEnvPath == "" && os.Getenv("DATABASE_URL") == "" {
			// Say it out loud. A silently ignored .env produces an error message
			// about a missing variable, which sends people hunting for a secret
			// they already have.
			w = append(w, "no .env file found in ./.env or ../.env — using the process environment only")
		}
		if c.envLoadWasNeeded && c.dotEnvPath != "" && c.dotEnvPath != ".env" {
			w = append(w, "loaded environment from "+c.dotEnvPath+" (running from a subdirectory)")
		}
		if len(c.JWTSecret) == 0 {
			w = append(w, "JWT_SECRET is not set — authentication will not work until Sprint 1 wires it up")
		}
		if c.MigrationDatabaseURL == "" {
			w = append(w, "MIGRATION_DATABASE_URL is not set — `make migrate-up` will fail (the API does not need it)")
		}
		if len(c.ReferralEncryptionKey) == 0 {
			w = append(w, "REFERRAL_TOKEN_ENCRYPTION_KEY is not set — needed in Sprint 8")
		}
	}

	if c.OfflinePolicy == OfflinePolicyWarnOnly {
		w = append(w, "OFFLINE_POLICY=warn_only — workstations will never be blocked from writing offline; "+
			"server-side revocation must still be enforced independently")
	}

	if c.TrustProxy {
		w = append(w, "TRUST_PROXY=true — ensure trusted proxy ranges are configured before relying on client IPs in the audit log")
	}

	return w
}

// ─── helpers ───────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitAndTrim(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	if v < 0 {
		return 0, fmt.Errorf("%s must not be negative, got %d", key, v)
	}
	return v, nil
}

func parseInt32(key string, fallback int32) (int32, error) {
	v, err := parseInt(key, int(fallback))
	if err != nil {
		return 0, err
	}
	if v > 1<<31-1 {
		return 0, fmt.Errorf("%s is too large: %d", key, v)
	}
	return int32(v), nil
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 15m or 168h, got %q", key, raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %s", key, d)
	}
	return d, nil
}
