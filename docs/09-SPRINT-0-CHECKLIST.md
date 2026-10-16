# Sprint 0 — Getting Started Checklist

> **Goal:** prove that a request can travel browser → Cloudflare Pages → Cloud Run → Neon → browser, **before** any feature is written.
> **Why this first:** deployment problems are the ones that are hardest to fix under time pressure at the end. Solve them on day one, when you have slack.
> **Read first:** `00-ARCHITECTURE.md` §3 (deployment traps) and §8 (environment variables).

Work top to bottom. Do not start Sprint 1 until step 24 is green.

---

## Part A — Accounts and local tooling

### A1. Verify your toolchain

```bash
go version          # expect go1.26.x
node -v             # expect v22.x or newer
pnpm -v             # expect 10.x
psql --version      # expect 18.x
goose -version      # expect v3.x
```

Install goose if it is missing:

```bash
go install github.com/pressly/goose/v3/cmd/goose@v3.29.0
```

### A2. Create the accounts

| Service | What to create | Free tier sufficient? |
|---|---|---|
| Neon | One project, region closest to you | Yes |
| Google Cloud | One project, billing enabled (Cloud Run has a free allowance but requires a card) | Yes |
| Cloudflare | One account, Pages enabled | Yes |

### A3. Neon — get **both** connection strings

In the Neon console, open **Connection Details** and copy:

1. **Pooled** — host contains `-pooler`. This becomes `DATABASE_URL`. The API uses this one.
2. **Direct** — no `-pooler`. This becomes `MIGRATION_DATABASE_URL`. Only goose uses this.

> **This is the single most common Neon mistake.** If you run the API against the pooled endpoint without disabling prepared statements, you will get intermittent `prepared statement "stmtcache_..." already exists` errors that only appear under load — and they will appear during your demo.

Save both now. You will not remember which is which in three weeks.

---

## Part B — Repository skeleton

### B1. Create the structure

```bash
mkdir -p phc-records/{api,web,docs,scripts}
cd phc-records
git init
```

### B2. Root files

**`.gitignore`**

```gitignore
# secrets — never commit these
.env
.env.local
.env.*.local

# go
api/bin/
api/tmp/
*.exe

# node
node_modules/
web/dist/
web/.vite/

# editors / OS
.DS_Store
.idea/
.vscode/*
!.vscode/extensions.json
```

**`.env.example`** — commit this; commit the real `.env` never.

```dotenv
# ─── Environment ───────────────────────────────────────────────
APP_ENV=development
LOG_LEVEL=debug

# ─── Server ────────────────────────────────────────────────────
# Cloud Run injects PORT. This default is for local development ONLY.
PORT=8080

# ─── Database ──────────────────────────────────────────────────
# Pooled endpoint (contains "-pooler"). Used by the API.
DATABASE_URL=postgresql://user:pass@ep-xxx-pooler.region.aws.neon.tech/phc?sslmode=require
# Direct endpoint (no "-pooler"). Used ONLY by goose migrations.
MIGRATION_DATABASE_URL=postgresql://user:pass@ep-xxx.region.aws.neon.tech/phc?sslmode=require

DB_MAX_CONNS=10

# ─── Auth ──────────────────────────────────────────────────────
# Generate with: openssl rand -base64 48
JWT_SECRET=replace-me-with-48-random-bytes
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h
OFFLINE_GRACE_PERIOD=168h
# block_writes (default, safer) | warn_only (never blocks a write)
OFFLINE_POLICY=block_writes

# ─── Security ──────────────────────────────────────────────────
ALLOWED_ORIGINS=http://localhost:5173,https://your-app.pages.dev
TRUST_PROXY=false

# ─── Referrals ─────────────────────────────────────────────────
REFERRAL_TTL_HOURS=48
REFERRAL_PIN_REQUIRED=true
# Generate with: openssl rand -base64 32
REFERRAL_TOKEN_ENCRYPTION_KEY=replace-me-with-32-random-bytes

# ─── Sync ──────────────────────────────────────────────────────
SYNC_MAX_BATCH_SIZE=200
SYNC_MAX_BATCH_BYTES=1048576
SYNC_CHANGE_LOG_RETENTION_DAYS=30
```

### B3. `README.md`

Keep it short and honest — a fresh clone must be runnable by following it alone:

- What the project is (three sentences)
- Prerequisites
- Setup: `cp .env.example .env`, fill in, `make migrate-up`, `make seed`, `make api`, `make web`
- The two URLs (API, web)
- Where the docs live

---

## Part C — Backend

### C1. Initialise the module

```bash
cd api
go mod init github.com/<your-username>/phc-records/api
```

### C2. Dependencies

```bash
go get github.com/gin-gonic/gin@v1.12.0
go get github.com/jackc/pgx/v5@v5.11.0
go get github.com/pressly/goose/v3@v3.29.0
go get github.com/golang-jwt/jwt/v5@v5.3.1
go get github.com/google/uuid@v1.6.0
go get golang.org/x/crypto@v0.58.0
go get github.com/gin-contrib/cors@v1.7.9
go get github.com/joho/godotenv@v1.5.1
go get github.com/rs/zerolog@v1.35.1
```

### C3. `internal/config/config.go`

Loads and **validates** everything at startup. Fail fast and loudly — a missing `JWT_SECRET` must crash the container on boot, not at the first login attempt.

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv   string
	LogLevel string
	Port     string

	DatabaseURL          string
	MigrationDatabaseURL string
	DBMaxConns           int32

	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	OfflineGrace    time.Duration
	OfflinePolicy   string // "block_writes" | "warn_only"

	AllowedOrigins []string
	TrustProxy     bool

	ReferralTTLHours      int
	ReferralPINRequired   bool
	ReferralEncryptionKey []byte

	SyncMaxBatchSize  int
	SyncMaxBatchBytes int
	SyncRetentionDays int
}

func Load() (*Config, error) {
	// godotenv.Load() is best-effort: in production the real environment
	// variables are already set and there is no .env file. Ignore its error.
	_ = godotenv.Load()

	c := &Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		Port:                 getEnv("PORT", "8080"), // Cloud Run overrides this
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		MigrationDatabaseURL: os.Getenv("MIGRATION_DATABASE_URL"),
		AllowedOrigins:       splitAndTrim(getEnv("ALLOWED_ORIGINS", "http://localhost:5173")),
		TrustProxy:           getEnv("TRUST_PROXY", "false") == "true",
		ReferralPINRequired:  getEnv("REFERRAL_PIN_REQUIRED", "true") == "true",
	}

	// ── required values ────────────────────────────────────────
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.AppEnv == "production" && c.MigrationDatabaseURL == "" {
		return nil, fmt.Errorf("MIGRATION_DATABASE_URL is required in production")
	}

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d)", len(secret))
	}
	c.JWTSecret = []byte(secret)

	key := os.Getenv("REFERRAL_TOKEN_ENCRYPTION_KEY")
	if len(key) < 32 {
		return nil, fmt.Errorf("REFERRAL_TOKEN_ENCRYPTION_KEY must be at least 32 characters")
	}
	c.ReferralEncryptionKey = []byte(key)

	// ── parsed values with defaults ────────────────────────────
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
	// Governs behaviour once the window expires. Default is the safer posture:
	// stop new writes, keep existing data readable. See ARCHITECTURE §5.1.
	c.OfflinePolicy = getEnv("OFFLINE_POLICY", "block_writes")
	if c.OfflinePolicy != "block_writes" && c.OfflinePolicy != "warn_only" {
		return nil, fmt.Errorf("OFFLINE_POLICY must be block_writes or warn_only, got %q", c.OfflinePolicy)
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

	return c, nil
}

func (c *Config) IsProduction() bool { return c.AppEnv == "production" }

// helpers: getEnv, splitAndTrim, parseInt, parseInt32, parseDuration
```

> **Note the `Port` comment.** Cloud Run sets `PORT` and health-checks that exact port. Hard-coding `:8080` locally and forgetting to read the variable is the classic "it works locally, the container never starts" bug.

### C4. `internal/platform/db/db.go` — the Neon pooler trap

```go
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates the connection pool for the API.
//
// The API MUST use Neon's POOLED endpoint (host contains "-pooler"), which is
// PgBouncer in transaction mode. PgBouncer does not support server-side
// prepared statements, so pgx's default extended protocol fails intermittently
// with "prepared statement stmtcache_... already exists".
//
// Setting QueryExecModeExec avoids the statement cache entirely.
// Do NOT use this pool for migrations — goose needs the direct endpoint.
func NewPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	cfg.MaxConns = maxConns
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
```

> **The arithmetic you must not forget:** `MaxConns × max Cloud Run instances` must stay below your Neon plan's connection ceiling. With `MaxConns = 10` and 10 instances you need 100 connections. Cap Cloud Run: `--max-instances=5`.

### C5. `migrate.go` — goose with the direct URL

```go
package main

// Run migrations with:
//   go run ./cmd/migrate up
//   go run ./cmd/migrate down
//   go run ./cmd/migrate status
//
// goose must use MIGRATION_DATABASE_URL (the DIRECT endpoint). Migrations run
// DDL inside transactions that PgBouncer's transaction pooling cannot support.
func runMigrations(direction string) error {
	dsn := os.Getenv("MIGRATION_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	// goose.SetBaseFS(embedMigrations) — or point at ./db/migrations
	// goose.SetDialect("postgres")
	// goose.Up(db, "db/migrations") etc.
}
```

### C6. `db/migrations/00001_create_facilities.sql`

Your first real migration. Study the structure — every migration follows this shape.

```sql
-- +goose Up
-- +goose StatementBegin

-- ═══ THIS DEPLOYMENT IS SINGLE-TENANT ═══
-- One installation serves exactly ONE health centre: its own code, its own
-- database, its own API. There is no tenant column anywhere in the schema.
--
-- facilities is a CONFIGURATION ROW, not a tenant root. It supplies what
-- appears on printed output and in money formatting: name and address for
-- receipt headers, the code that prefixes every document number, the currency,
-- the timezone.
CREATE TABLE facilities (
    id                   UUID PRIMARY KEY,
    name                 TEXT NOT NULL,
    code                 TEXT NOT NULL UNIQUE,     -- ⚠️ IMMUTABLE once documents exist
    lga                  TEXT,
    state                TEXT,
    address              TEXT,
    phone                TEXT,
    currency_code        CHAR(3)  NOT NULL DEFAULT 'NGN',
    currency_minor_units SMALLINT NOT NULL DEFAULT 2,
    timezone             TEXT     NOT NULL DEFAULT 'Africa/Lagos',
    is_active            BOOLEAN  NOT NULL DEFAULT true,

    -- ─── sync column contract (docs/01-DATA-MODEL.md §3) ───────
    revision    BIGINT      NOT NULL DEFAULT 1,
    server_rev  BIGINT      NOT NULL DEFAULT 0,
    sync_state  TEXT        NOT NULL DEFAULT 'synced',
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- NULLABLE on purpose: this row is inserted by the migration at install
    -- time, before any user exists to name as the actor. Every other syncable
    -- table keeps created_by/updated_by NOT NULL.
    created_by  UUID,
    updated_by  UUID,

    CONSTRAINT chk_facilities_sync_state
        CHECK (sync_state IN ('pending', 'synced', 'conflict')),
    CONSTRAINT chk_facilities_currency_units
        CHECK (currency_minor_units BETWEEN 0 AND 4),
    CONSTRAINT chk_facilities_code_upper
        CHECK (code = upper(code)),
    -- Codes sit inside document numbers like URPHC/PAT/2026/00123, so a "/" in
    -- a code would make them ambiguous.
    CONSTRAINT chk_facilities_code_charset
        CHECK (code ~ '^[A-Z0-9]{2,10}$')
);

CREATE INDEX idx_facilities_server_rev ON facilities (server_rev);

-- ★ THE SINGLE-TENANT GUARANTEE, ENFORCED BY THE DATABASE.
-- A constant expression in a unique index permits at most one row, so a second
-- INSERT fails with SQLSTATE 23505 — by the app or by hand.
CREATE UNIQUE INDEX uq_facilities_single_org ON facilities ((true));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS facilities;
-- +goose StatementEnd
```

**Three deliberate choices worth understanding:**

1. **`created_by`/`updated_by` are NULLABLE here and nowhere else.** The install migration creates this row before any user exists, so a `NOT NULL` actor would make the very first insert impossible. Every other syncable table keeps `NOT NULL`.
2. **No foreign key to `users` on those columns.** In an earlier multi-tenant draft, `users.facility_id → facilities` plus `facilities.created_by → users` formed a genuine **circular FK** — neither table could be seeded first. Single-tenancy deleted the tenant column, and with it the cycle. `users.facility_id` no longer exists.
3. **The `CHECK` constraints are named `chk_<table>_<column>`**, matching `04-CODING-STANDARDS.md`. Named constraints produce readable errors and are droppable later.

### C7. `db/migrations/00003_seed_default_facility.sql`

```sql
-- +goose Up
-- +goose StatementBegin

-- Idempotent: safe to run repeatedly on a fresh or existing environment.
--
-- created_by / updated_by are omitted (NULL) because no user exists yet.
--
-- ⚠️  Edit name/address/phone/lga/state/currency to match the real health
--     centre before going live — they print on every receipt and referral note.
-- ⚠️  Set `code` before any document is issued and never change it afterwards.
INSERT INTO facilities (
    id, name, code, lga, state, address, phone,
    currency_code, currency_minor_units, timezone
) VALUES (
    '018f0000-0000-7000-8000-000000000001',
    'Demo Primary Health Centre',
    'DEMO',
    'Demo LGA',
    'Demo State',
    '1 Health Centre Road',
    '08000000000',
    'NGN', 2, 'Africa/Lagos'
)
ON CONFLICT (id) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM facilities WHERE id = '018f0000-0000-7000-8000-000000000001';
-- +goose StatementEnd
```

### C7b. `db/migrations/00004_seed_admin_user.sql`

Not in the original task list, but a fresh install needs a usable login. The `users` table has no `facility_id`, so this seeds cleanly after 00003:

```sql
-- +goose Up
-- +goose StatementBegin

-- Bootstrap administrator. Password: ChangeMe!2026  (must_change_password = true)
-- ⚠️  Change or delete before any public deployment. This credential is in git
--     history permanently — name it as a bounded risk in your report.
-- The hash is real Argon2id, round-trip verified to accept the password above
-- and reject an incorrect one.
INSERT INTO users (
    id, staff_number, first_name, last_name, email,
    password_hash, must_change_password,
    created_by, updated_by
) VALUES (
    '018f0000-0000-7000-8000-0000000000aa',
    'DEMO/STF/0001',
    'System',
    'Administrator',
    'admin@demo.phc',
    '$argon2id$v=19$m=65536,t=3,p=4$uxVDNBtPaUyg3PsbrusV8Q$/dzeM34M2J0sW5N02s5wK07X0zmutBdPvJTC+MjhiNo',
    true,
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
)
ON CONFLICT (id) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM users WHERE id = '018f0000-0000-7000-8000-0000000000aa';
-- +goose StatementEnd
```

**Verify the round trip before committing — every time:**

```bash
goose -dir db/migrations postgres "$MIGRATION_DATABASE_URL" up
goose -dir db/migrations postgres "$MIGRATION_DATABASE_URL" down
goose -dir db/migrations postgres "$MIGRATION_DATABASE_URL" up
```

If `up → down → up` does not leave you exactly where you started, the migration is wrong.

### C8. `cmd/api/main.go` — the walking skeleton

```go
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/<you>/phc-records/api/internal/config"
	"github.com/<you>/phc-records/api/internal/platform/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}

	// Structured JSON logs in production so Cloud Run can parse them.
	if cfg.IsProduction() {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to database")
	}
	defer pool.Close()

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery(), requestIDMiddleware(), corsMiddleware(cfg))

	// Trust only the configured proxies. Blindly trusting X-Forwarded-For
	// lets anyone forge the IP in your audit log.
	if cfg.TrustProxy {
		_ = router.SetTrustedProxies(nil) // then wire Cloudflare's ranges explicitly
	} else {
		_ = router.SetTrustedProxies(nil)
	}

	// ── health and readiness ───────────────────────────────────
	// /health never touches the database: Cloud Run uses it for liveness,
	// and a Neon cold start must not cause an instance to be killed.
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": buildVersion,
			"time":    time.Now().UTC(),
		})
	})

	// /ready DOES check the database, because that is what readiness means.
	router.GET("/ready", func(c *gin.Context) {
		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(pingCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "degraded", "database": "unreachable",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "database": "ok"})
	})

	v1 := router.Group("/api/v1")
	{
		v1.GET("/meta", func(c *gin.Context) {
			var facilityCount int
			err := pool.QueryRow(c.Request.Context(),
				`SELECT count(*) FROM facilities WHERE deleted_at IS NULL`,
			).Scan(&facilityCount)
			if err != nil {
				log.Error().Err(err).Msg("count facilities")
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{"code": "INTERNAL_ERROR", "message": "database error"},
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"schemaVersion":           1,
					"serverTime":              time.Now().UTC(),
					"referralTtlHours":        cfg.ReferralTTLHours,
					"offlineGracePeriodHours": int(cfg.OfflineGrace.Hours()),
					"offlinePolicy":           cfg.OfflinePolicy,
					"syncMaxBatchSize":        cfg.SyncMaxBatchSize,
					"facilityCount":           facilityCount,
					"minClientVersion":        "0.1.0",
				},
			})
		})
	}

	// ── graceful shutdown ──────────────────────────────────────
	// Cloud Run sends SIGTERM before killing an instance. Without this,
	// in-flight sync pushes are cut off mid-transaction.
	srv := &http.Server{
		Addr:              ":" + cfg.Port, // NOT hard-coded — see §C3
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.Port).Str("env", cfg.AppEnv).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("forced shutdown")
	}
	log.Info().Msg("stopped")
}

var buildVersion = "dev" // set with -ldflags at build time
```

### C9. Run it locally

```bash
cd api
go run ./cmd/api
curl -s localhost:8080/health | jq
curl -s localhost:8080/api/v1/meta | jq
```

`facilityCount: 1` means the whole chain works: Go → pgx → Neon → back.

### C10. `Dockerfile`

```dockerfile
# ── build ──────────────────────────────────────────────────────
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache dependencies separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.buildVersion=${VERSION}" \
    -o /out/api ./cmd/api

# ── runtime ────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api

# Cloud Run injects PORT; this is the documented default.
ENV PORT=8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/api"]
```

### C11. Deploy to Cloud Run

```bash
gcloud run deploy phc-api \
  --source . \
  --region <your-region> \
  --allow-unauthenticated \
  --max-instances=5 \
  --set-env-vars="APP_ENV=production,LOG_LEVEL=info" \
  --set-secrets="DATABASE_URL=phc-database-url:latest,JWT_SECRET=phc-jwt-secret:latest"
```

Store secrets in **Secret Manager**, never in the image or in the deploy command (which lands in your shell history and in the Cloud Run revision metadata).

> **Region matters.** Put Cloud Run in the same region as your Neon project. A cross-continent hop adds 100–200 ms to *every* query, and the sync loop issues many.

---

## Part D — Frontend

### D1. Scaffold

```bash
cd web
pnpm create vite . --template react-ts
pnpm add dexie dexie-react-hooks zustand zod react-hook-form @hookform/resolvers \
         @tanstack/react-router @tanstack/react-query date-fns uuidv7 qrcode \
         lucide-react class-variance-authority clsx tailwind-merge sonner motion
pnpm add -D tailwindcss @tailwindcss/vite @tanstack/router-plugin \
            vite-plugin-pwa vitest @testing-library/react @testing-library/jest-dom \
            fake-indexeddb @playwright/test typescript@7.0.2
```

> Pin TypeScript exactly (`7.0.2`, no caret). TypeScript 7 is the Go-based compiler line; it is fast and correct for a new project, but a caret range there is not a routine minor bump.

### D2. `vite.config.ts`

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { TanStackRouterVite } from '@tanstack/router-plugin/vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
  plugins: [
    TanStackRouterVite({ target: 'react', autoCodeSplitting: true }),
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'prompt',   // never 'autoUpdate' — see the stale-shell trap
      manifest: {
        name: 'PHC Patient Records',
        short_name: 'PHC Records',
        theme_color: '#0f766e',
        background_color: '#ffffff',
        display: 'standalone',
      },
      workbox: {
        // The app SHELL is cached here. Patient DATA is cached in Dexie.
        // These are different problems — see docs/00-ARCHITECTURE.md §3.4.
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
        navigateFallback: '/index.html',
        navigateFallbackDenylist: [/^\/api/, /^\/r\//],
      },
    }),
  ],
  server: { port: 5173 },
});
```

> **`registerType: 'prompt'`, not `'autoUpdate'`.** With `autoUpdate`, a workstation can swap the running app out from under a nurse mid-form. Prompt instead, and let the user choose when to reload.

### D3. `src/styles/tokens.css` — Tailwind v4 design tokens

Copy the full token set from `03-UI-PATTERNS.md` §2. The shape:

```css
@import 'tailwindcss';

@theme {
  --color-primary-50:  #f0fdfa;
  --color-primary-500: #14b8a6;
  --color-primary-600: #0d9488;
  /* ... the complete ramp from the UI patterns doc ... */

  --font-sans: 'Inter var', ui-sans-serif, system-ui, sans-serif;
  --spacing: 0.25rem;
  --radius-md: 0.5rem;
}
```

### D4. `src/db/index.ts`

Dexie version 1, exactly as specified in `02-SYNC-PROTOCOL.md` §3. Empty domain stores are fine at this stage — the schema must exist so the version is pinned before real data arrives.

### D5. `src/lib/api.ts`

One place that knows how to talk to the API, so the error envelope is handled once:

```ts
const API_BASE = import.meta.env.VITE_API_URL ?? 'http://localhost:8080/api/v1';

export class ApiError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number,
    public details?: unknown,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init.headers },
  });

  const body = await res.json().catch(() => null);

  if (!res.ok) {
    // The server always returns { error: { code, message, details } }.
    const err = body?.error;
    throw new ApiError(
      err?.code ?? 'NETWORK_ERROR',
      err?.message ?? 'Something went wrong',
      res.status,
      err?.details,
    );
  }
  return body as T;
}
```

### D6. `src/routes/index.tsx` — the health page

Render the values from `GET /meta`: server time, schema version, and the facility count. Plain, honest proof that the chain works end to end. (The count is a real query against the config row — it is the cheapest thing that proves Go → pgx → Neon → back.)

### D7. Run and deploy

```bash
pnpm dev     # http://localhost:5173
```

Set `VITE_API_URL` in the Cloudflare Pages dashboard to your Cloud Run URL, then add the resulting `*.pages.dev` origin to `ALLOWED_ORIGINS` on the API and redeploy it. **CORS will fail until you do both** — this trips up almost everyone once.

---

## Part E — Prove it

### E1. The Sprint 0 acceptance test

- [ ] `https://<your-app>.pages.dev` loads over HTTPS
- [ ] The page shows a live facility count fetched from Neon via Cloud Run
- [ ] `curl https://<your-api>.run.app/health` returns `{"status":"ok"}`
- [ ] `curl https://<your-api>.run.app/ready` returns `{"status":"ready","database":"ok"}`
- [ ] Chrome DevTools → Application → Service Workers shows the PWA registered
- [ ] DevTools → Network → **Offline**, then reload: the app shell still renders
- [ ] `make migrate-status` reports all four migrations applied
- [ ] `make migrate-verify` passes (`up → down → up`)
- [ ] `curl localhost:8080/api/v1/meta` returns `facilityCount: 1`
- [ ] `git log` shows at least five commits with Conventional Commit messages
- [ ] A fresh clone runs locally by following `README.md` alone

### E2. Commit

```
chore: scaffold repo, wire Cloud Run + Neon + Cloudflare Pages

- Go API with Gin, pgx pool (pooler-safe exec mode), goose migrations
- 4 migrations: facilities, users, facility seed, admin user seed
- GET /health, /ready, /api/v1/meta
- React SPA with Vite, Tailwind v4 tokens, Dexie v1, PWA shell
- Walking skeleton verified end to end on deployed infrastructure
```

---

## Part F — Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Container builds but never serves | Port hard-coded | Read `os.Getenv("PORT")` |
| `prepared statement "stmtcache_..." already exists` | Pooled Neon endpoint with prepared statements | `DefaultQueryExecMode: pgx.QueryExecModeExec` |
| Migrations hang or fail oddly | Using the pooled endpoint for goose | Use `MIGRATION_DATABASE_URL` (direct) |
| CORS error in console | Origin missing from `ALLOWED_ORIGINS` | Add the Pages URL **and** redeploy the API |
| `DATABASE_URL is required` on boot | Env var not set on the Cloud Run revision | Check the revision's variables, not just the service |
| Cold start > 5 s on first request | Scale-to-zero + Neon wake | Expected; set `--min-instances=1` for demos |
| `too many connections` under load | `MaxConns × instances` over the plan limit | Lower `MaxConns`, cap `--max-instances` |
| Local `.env` ignored | Wrong working directory | `godotenv.Load()` reads from the process CWD — run from `api/` |
| `database "<your-username>" does not exist` when running goose | **Unquoted `&` in `.env`** emptied `MIGRATION_DATABASE_URL`, so goose fell back to local libpq defaults | Double-quote both Neon URLs in `.env`. See `00-ARCHITECTURE.md` §3.7 |
| `make migrate-up` says the variable is empty | Same cause | The Makefile now catches this and names the fix |
| Types resolve to nothing after adding TS 7 | Mismatched `moduleResolution` | Set `"moduleResolution": "bundler"`, `"module": "ESNext"` |

---

## What Sprint 0 deliberately does **not** do

No authentication. No sync. No clinical features. That is intentional.

Sprint 0 exists to answer one question: **can I get code onto the internet and back again?** Every team that answers this in week one saves themselves a week later. Every team that assumes it will "just work" discovers the Neon pooler trap, the Cloud Run port trap and the CORS trap at 2 a.m. the night before a demo.
