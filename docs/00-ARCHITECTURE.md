# Architecture

> **Project:** Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres
> **Document status:** Canonical. If code disagrees with this document, one of them is a bug.
> **Last updated:** Sprint 0

---

## 1. What this system is

A web application that lets a Primary Healthcare Centre (PHC) run its **entire patient workflow with no internet connection**, and sync to a central database automatically whenever a connection becomes available.

The system covers: patient registration (individual and family), vitals capture, a triage queue, doctor consultation and diagnosis, laboratory orders and results, prescriptions, pharmacy dispensing, cashier payments, inpatient admission with daily billing, and a **referral system** in which a doctor issues a referral note that another facility can open by scanning a QR code printed on a 58–80 mm POS thermal slip.

### 1.1 The one-sentence architecture

> **The browser is the primary database. The server is a replication peer that also happens to be the authority on identity, security, and referral links.**

This single idea drives every other decision in this document. Read it again before you write code that assumes the network is available.

### 1.2 Non-goals (state these in your defence)

Explicitly out of scope, so scope creep cannot kill the project:

- **Pharmacy inventory management.** No stock levels, no batches, no expiry tracking. The `dispenses` table records *what was given to whom*, not *what remains on the shelf*.
- **HR, payroll, rostering.** `users` supports authentication and role assignment, nothing more.
- **Insurance / HMO claims processing.**
- **SMS or email notification.**
- **Native mobile apps.** Responsive web only.
- **Multi-facility / multi-tenant operation.** This is a **single-tenant system by design**. One deployment serves exactly one health centre: its own code, its own database, its own API. A second facility that wants this system deploys its own copy. There is no tenant column anywhere in the schema and no district-level UI. See §7.4.
- **HL7 / FHIR interoperability.** The referral payload is our own JSON shape. Naming ICD-10 codes in the diagnosis table is as close as we get.

---

## 2. Technology stack

Versions below are the **pinned** versions. Do not upgrade mid-sprint; version churn is how student projects lose a week.

### 2.1 Backend

| Concern | Choice | Version | Why this and not the alternative |
|---|---|---|---|
| Language | Go | 1.26 | Compiles to one static binary. No runtime to install on a PHC server. |
| HTTP framework | Gin | v1.12.0 | You asked for it; it is also the most widely documented Go framework, which matters when you are debugging alone. |
| Database driver | pgx | v5.11.0 | Native PostgreSQL protocol, real connection pooling. `database/sql` is a compatibility layer you do not need. |
| Migrations | goose | v3.29.0 | Plain `.sql` files with `-- +goose Up` / `Down`. Reviewable in a pull request, which is exactly what you want for a graded project. |
| Query layer | **hand-written SQL via pgx** | — | Deliberate: `sqlc` is excellent but adds a code-generation step and a config file to learn. For a project you are writing by hand, explicit SQL in a repository layer teaches more and debugs faster. Revisit only if query volume becomes painful. |
| Auth tokens | golang-jwt/jwt | v5.3.1 | Short-lived access tokens + rotating refresh tokens held in Postgres. |
| Password hashing | `golang.org/x/crypto/argon2` | v0.58.0 | Argon2id. Never bcrypt-md5, never SHA. |
| CORS | gin-contrib/cors | v1.7.9 | The web client is on a different origin (Cloudflare Pages) from the API (Cloud Run). |
| Config | joho/godotenv | v1.5.1 | Local `.env` only. Production reads real environment variables. |
| Logging | rs/zerolog | v1.35.1 | Structured JSON logs — Cloud Run can parse them, and you can grep them. |
| UUIDs | google/uuid | v1.6.0 | `uuid.NewV7()` for time-ordered keys. |

### 2.2 Frontend

| Concern | Choice | Version | Why |
|---|---|---|---|
| Framework | React + TypeScript | 19.3.1 | |
| Build tool | Vite | 8.3.4 | |
| TypeScript | TypeScript | 7.0.2 | ⚠️ This is the Go-based compiler line. It is fast and fine for a new project, but pin it exactly (`"typescript": "7.0.2"`, no caret) because a minor bump here is not a minor change. |
| Local database | Dexie | 4.4.6 | IndexedDB wrapper. The heart of the offline story. |
| Local DB React binding | dexie-react-hooks | 4.4.0 | `useLiveQuery` — makes every screen re-render the instant data changes locally *or* arrives from sync. |
| Routing | TanStack Router | 1.170.44 | Type-safe routes. |
| Server state | TanStack Query | 5.104.3 | Used **only** for the sync endpoint and admin/report reads, *not* for clinical data. See §4.3. |
| Client/UI state | Zustand | 5.0.15 | Session, sidebar, active encounter, sync status. |
| Validation | Zod | 4.6.7 | One schema validates a form *and* the sync payload. |
| Forms | react-hook-form + @hookform/resolvers | 7.90.0 / 5.9.1 | |
| Styling | Tailwind CSS | 4.3.3 | v4 is CSS-first: design tokens live in `@theme` inside CSS, not in a JS config file. |
| Components | Radix UI primitives + CVA | dialog 1.3.1, select 2.4.1, CVA 0.7.1 | Accessible unstyled primitives; the *look* is ours, in `ui-registry.md`. |
| Icons | lucide-react | 1.57.0 | |
| Animation | motion (Framer Motion) | 14.3.0 | Subtle, purposeful motion only. See `03-UI-PATTERNS.md`. |
| Dates | date-fns | 4.4.0 | |
| QR generation | qrcode | 1.5.4 | Renders the referral QR to a canvas for printing. |
| Toasts | sonner | 2.0.8 | |
| PWA / service worker | vite-plugin-pwa | 2.0.0 | Caches the **app shell**. Dexie caches the **data**. Both are required — see §3.4. |
| Unit tests | Vitest + Testing Library | 5.0.5 / 16.3.3 | |
| E2E + offline tests | Playwright | 1.64.1 | Its `context.setOffline(true)` is how you *prove* the offline claims. |

### 2.3 Infrastructure

| Piece | Service | Notes |
|---|---|---|
| API hosting | Google Cloud Run | Container, scales to zero. |
| Database | Neon (serverless Postgres 18) | Branching gives you a free staging DB per feature. |
| Web hosting | Cloudflare Pages | Static SPA build. |
| Referral public view | Route inside the same SPA | `/r/:token` — a plain page, no auth, talks to the API. |

---

## 3. Deployment topology and its traps

```
┌──────────────────────────────┐          ┌──────────────────────────────┐
│  Staff workstation (browser) │          │  Receiving facility / phone  │
│  ┌────────────────────────┐  │          │  ┌────────────────────────┐  │
│  │ React SPA (CF Pages)   │  │          │  │ /r/:token  public page │  │
│  │ ┌────────────────────┐ │  │          │  └────────────────────────┘  │
│  │ │ Service Worker     │ │  │          └───────────────┬──────────────┘
│  │ │  = app shell cache │ │  │                          │
│  │ ├────────────────────┤ │  │                          │
│  │ │ Dexie / IndexedDB  │ │  │                          │
│  │ │  = THE DATABASE    │ │  │                          │
│  │ └────────────────────┘ │  │                          │
│  └───────────┬────────────┘  │                          │
└──────────────┼───────────────┘                          │
               │  POST /sync/push  (batched outbox)       │
               │  GET  /sync/pull  (cursor)               │
               ▼                                          ▼
      ┌─────────────────────┐                   ┌──────────────────┐
      │ Cloud Run: Go + Gin │◄──────────────────┤ GET /r/:token    │
      │  stateless          │                   └──────────────────┘
      └──────────┬──────────┘
                 │ pgx pool → Neon **pooled** endpoint
                 ▼
      ┌─────────────────────┐
      │ Neon Postgres 18    │
      │  truth + audit      │
      └─────────────────────┘
```

### 3.1 Trap: Cloud Run must bind to `$PORT`

Cloud Run injects a `PORT` environment variable (default `8080`) and health-checks that port. Hard-coding `:3000` produces a container that builds fine and never starts serving.

```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080" // local development default only
}
```

### 3.2 Trap: Neon's pooler breaks prepared statements

Neon gives you **two** connection strings:

- **Pooled** (`...-pooler.neon.tech`) — PgBouncer in transaction mode. Use this for the API.
- **Direct** (`...neon.tech`) — use this **only** for goose migrations.

pgx v5 uses the extended protocol by default, which relies on server-side prepared statements — and PgBouncer in transaction mode does not support them. The failure mode is intermittent: it works in dev, then throws `prepared statement "stmtcache_..." already exists` in production under concurrency.

**Rule:** the API's pool must be built with `QueryExecMode: pgx.QueryExecModeExec` (or `QueryExecModeSimpleProtocol`).

```go
cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL")) // the -pooler URL
if err != nil {
    return nil, fmt.Errorf("parse database url: %w", err)
}
cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
cfg.MaxConns = 10 // Cloud Run instance × 10 must stay under Neon's limit
```

Note the second line's arithmetic: **max Cloud Run instances × `MaxConns` must stay below your Neon plan's connection ceiling.** Cap `--max-instances` on Cloud Run rather than letting it autoscale indefinitely.

### 3.3 Trap: cold starts and request timeouts

Cloud Run scales to zero. The first request after idle pays a cold start (roughly 1–3 s) *plus* a Neon compute wake-up. Consequences:

- The sync client must treat a slow first request as normal, not as an error. Set the client timeout to 30 s and retry with backoff.
- Set Cloud Run's `--min-instances=1` for your **defence demo**. Do not pay for it during development.
- Keep sync batch sizes bounded (§`02-SYNC-PROTOCOL.md`) so no single request approaches the timeout.

### 3.4 Trap: the service worker and Dexie solve different problems

Beginners conflate these two. They are not the same thing:

- **Service worker (`vite-plugin-pwa`)** caches HTML/JS/CSS so the *application* loads with no network. It knows nothing about patients.
- **Dexie/IndexedDB** stores the *data* so patient records are readable and writable with no network.

You need both. A cached app that cannot read patient data is useless offline; patient data you cannot load the app to read is equally useless.

**Corollary — the deployment trap:** when you deploy a new frontend build, an offline client may still be running the old shell. Version your sync payloads (§`02-SYNC-PROTOCOL.md`) and make the server tolerate a client one version behind. Prompt the user to reload when you detect a mismatch; never silently break them mid-shift.

### 3.5 Trap: CORS and cookies

The SPA origin (`*.pages.dev`, later your domain) differs from the API origin (`*.run.app`). Therefore:

- Configure `gin-contrib/cors` with an **explicit allow-list** of origins read from `ALLOWED_ORIGINS`. `AllowAllOrigins` plus credentials is rejected by browsers anyway.
- Auth uses the `Authorization: Bearer` header, **not** cookies. This sidesteps `SameSite`/`Secure` cross-site cookie pain entirely and is what the offline client needs, since it must hold the token while offline.

### 3.6 Trusting the client IP

Behind Cloudflare, `RemoteAddr` is Cloudflare's IP, which makes your audit log useless. Read `CF-Connecting-IP` **only** when `TRUST_PROXY=true`, and set `router.SetTrustedProxies(...)` explicitly. Blindly trusting `X-Forwarded-For` lets anyone forge audit entries.

### 3.7 Trap: an unquoted `&` in `.env` silently empties the variable

Neon connection strings commonly end with `...?sslmode=require&channel_binding=require`. **An unquoted `&` is a shell control operator**, so when the Makefile does `. ./.env`, everything after the `&` is taken as a background command:

```dotenv
# ❌ WRONG — the variable becomes EMPTY
MIGRATION_DATABASE_URL=postgresql://user:pass@host/db?sslmode=require&channel_binding=require
```

The failure is thoroughly misleading. `make migrate-up` does not complain about a missing variable — goose simply receives an empty connection string and falls back to libpq defaults, so you get:

```
failed to connect to `user=agent-mike database=`: /var/run/postgresql/.s.PGSQL.5432
  server error: FATAL: database "agent-mike" does not exist
```

...which sends you hunting through your local PostgreSQL configuration for a bug that is in a `.env` file. Worse, in a fresh subshell the `&` can abort the remainder of the file, so *every* variable after it is lost too.

```dotenv
# ✅ CORRECT — always double-quote connection URLs
MIGRATION_DATABASE_URL="postgresql://user:pass@host/db?sslmode=require&channel_binding=require"
```

**Note that `godotenv` parses an unquoted `&` perfectly well**, so the Go API can load the same file without trouble. Only the *shell* is confused — which is why this bites when goose runs through `make` and not when the server starts. Quote the URLs and both paths work.

The `Makefile` now guards against this explicitly: if `MIGRATION_DATABASE_URL` is empty after sourcing, it stops and tells you to check the quoting, rather than letting goose fail obscurely.

---

## 4. Application architecture

### 4.1 Backend layering

Four layers, and the dependency arrows point **one direction only**:

```
cmd/api/main.go          wiring: config → pool → repos → services → handlers → router
   │
   ▼
internal/http/           Gin handlers, middleware, request/response DTOs.
   │                     Knows HTTP. Knows nothing about SQL.
   ▼
internal/service/        Business rules and transactions. The ONLY place
   │                     that decides "may this happen?" — RBAC, workflow
   │                     transitions, balance calculations.
   ▼
internal/repo/           SQL. Knows Postgres. Knows nothing about HTTP.
   │
   ▼
internal/domain/         Plain structs + enums. Imports nothing internal.
```

**The rule that keeps this honest:** handlers never call repos directly, and repos never contain `if user.Role == "cashier"`. If you find yourself wanting to break either, the logic belongs in a service.

### 4.2 Repository layout

```
phc-records/
├── api/                          # Go backend
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── domain/               # structs, enums, no imports
│   │   ├── repo/                 # SQL
│   │   ├── service/              # business rules
│   │   ├── http/                 # handlers, middleware, dto
│   │   ├── sync/                 # sync engine: change log, push, pull
│   │   └── platform/             # jwt, argon2, ids, clock, qr
│   ├── db/
│   │   ├── migrations/           # goose .sql files
│   │   └── seed/                 # dev seed data
│   ├── Dockerfile
│   ├── go.mod
│   └── .env.example
│
├── web/                          # React frontend
│   ├── src/
│   │   ├── app/                  # router, providers, layout shells
│   │   ├── routes/               # route components (one per screen)
│   │   ├── db/                   # Dexie schema + versioned upgrades
│   │   ├── sync/                 # outbox, pusher, puller, status store
│   │   ├── features/             # feature-sliced: patients/, queue/, billing/ ...
│   │   ├── components/ui/        # design-system primitives
│   │   ├── lib/                  # api client, zod schemas, formatters
│   │   ├── styles/               # tokens.css, print.css
│   │   └── types/                # generated/shared types
│   ├── public/
│   ├── package.json
│   └── vite.config.ts
│
├── docs/                         # this documentation set
├── scripts/                      # db reset, seed, deploy helpers
├── CLAUDE.md                     # agent working agreement
└── README.md
```

### 4.3 State management — the rule that prevents the classic mess

Three kinds of state, three tools, **no overlap**:

| State | Where it lives | Tool | Example |
|---|---|---|---|
| **Domain data** | Dexie (IndexedDB) | `useLiveQuery` | patients, encounters, vitals, invoices |
| **UI state** | React memory | `useState` / Zustand | modal open, sidebar collapsed, current tab |
| **Sync/session status** | Zustand | Zustand | online?, pending ops count, current user, last sync time |

**The rule:** clinical data is **never** fetched with TanStack Query and **never** cached in React state. It is read from Dexie and written to Dexie. The sync engine is the only code that talks to the network for domain data.

TanStack Query is permitted for exactly three things: the public referral view (`/r/:token`, which is online-by-definition), admin/user management, and reports that only an online manager would ever run.

Breaking this rule is the #1 way offline-first projects become inconsistent. A screen reading from an HTTP cache while another reads from IndexedDB will show two different truths.

### 4.4 The write path — memorise this

Every clinical mutation follows the same five steps. If you implement it once, correctly, every feature after it is easy.

```
User taps "Save"
  1. Validate with a Zod schema (shared with the form).
  2. Build the row, generating id = uuidv7() CLIENT-SIDE.
     Set sync_state = 'pending', updated_at = now, revision = 0 (new).
  3. Dexie transaction: upsert the row AND append an outbox entry. ATOMIC.
  4. Return immediately. The UI is already correct — useLiveQuery re-renders.
  5. The sync engine drains the outbox in the background.
```

Steps 3 and 4 are the whole trick. The user never waits for a network round-trip, so the app *cannot* feel slow or fail because of connectivity.

### 4.5 The read path

No `useEffect` + `fetch`. Screens declare what they need:

```ts
const waiting = useLiveQuery(
  () => db.encounters
    .where('[workflowStage+priority]')
    .between(['awaiting_doctor', -Infinity], ['awaiting_doctor', Infinity])
    .and(e => e.status !== 'cancelled')
    .sortBy('queuedAt'),
  [],   // no facility dependency — this deployment is single-tenant
);
```

When a background sync writes into Dexie, this hook fires and the list updates itself. There is no cache invalidation logic to get wrong.

---

## 5. Security architecture (summary — full detail in `05-SECURITY.md`)

**Offline authentication.** You cannot call an auth server with no network, so:

1. First login is online. The server returns an access token (15 min) and a refresh token (30 days).
2. The device stores an Argon2id hash of the user's password locally, plus the refresh token.
3. While offline, login is verified against the local hash, and a **device-bound** session is created with a configurable offline grace window (**default 7 days**). When the window expires, behaviour is governed by `OFFLINE_POLICY` — see §5.1.
4. Each device generates an **ECDSA P-256 keypair** at first run; the private key stays in IndexedDB (as an exported JWK — see `05-SECURITY.md` §4.2 for why non-extractable is impractical here). The public key is registered to the device on the server.
5. `/sync/push` requires *both* a valid access token and a valid device signature. Either one failing is sufficient to reject.

**Why device keys, and not just a JWT:** with full offline writes, a single browser holds every patient record in the facility. A stolen or lost laptop must be revocable **per device**, immediately, without locking the human out of their account or invalidating every other device.

**Referral links.** A referral note is sensitive clinical data reachable without a login. Therefore:

- The URL carries a **256-bit random token**, not the referral ID — records must not be enumerable. The database stores only `access_token_hash`.
- A **6-digit PIN** is printed on the POS slip. Verification is rate-limited and locked after 5 failures.
- `expires_at = created_at + 48 hours`, enforced in the query, not just in the UI.
- Every view writes to `referral_access_log`.

**Audit.** `audit_logs` is append-only, lives server-side, and is **never synced to clients** (otherwise the audit trail itself becomes tamperable offline). Domain rows carry `created_by`/`updated_by`; the server's audit log records what arrived in each push.

### 5.1 The offline grace policy — a deliberate, configurable trade-off

Two requirements pull in opposite directions, and only the PHC can decide which wins:

- **Credential integrity** says a device that has been offline indefinitely holds an indefinitely stale authorisation, so force re-authentication.
- **Clinical safety** says you never block a nurse from recording care because a link is down.

Rather than hard-code one answer, the window and the consequence are both configuration:

| Variable | Default | Meaning |
|---|---|---|
| `OFFLINE_GRACE_PERIOD` | `168h` (7 days) | How long a device works offline before the policy applies |
| `OFFLINE_POLICY` | `block_writes` | `block_writes` — new writes refused until one online re-auth; data stays **readable**. `warn_only` — writing continues indefinitely, with a persistent warning banner |

```dotenv
# Choose ONE. The default is the safer of the two.
OFFLINE_GRACE_PERIOD=168h
OFFLINE_POLICY=block_writes     # or: warn_only
```

**Behaviour under `block_writes` (the default):**

| Elapsed offline | Behaviour |
|---|---|
| < 60% of the window | Normal offline operation. Indicator shows `offline` |
| 60–85% of the window | Indicator gains a countdown; dismissible `info` alert on the dashboard |
| 85–100% of the window | Non-dismissible `warning` alert on every page header, showing the exact deadline |
| > 100% | Writes blocked, reads permitted. Full-screen explanatory alert. **Unlocks automatically** the moment connectivity returns and one re-auth succeeds — no reload |

**Behaviour under `warn_only`:** identical warnings, but writes are never blocked and `OFFLINE_POLICY` is recorded in every audit entry so the deviation is traceable.

**Why configurable is the right engineering answer here:** the policy is an operational decision about a specific clinic's connectivity and staffing, not a technical constant. Making it a variable costs about ten lines and lets you demonstrate *both* postures in your defence — which is a considerably stronger position than defending one arbitrary number.

**The one thing that is not configurable:** a device whose **refresh token** has been revoked server-side (lost or stolen) must stop working, and `warn_only` must never override that. Revocation is a security decision; the grace window is a convenience one. Do not conflate them.

---

## 6. Role model

Eight roles. Enforced in **two** places, which is deliberate:

| Role | Primary screens | Notes |
|---|---|---|
| `front_desk` | Registration, queue intake, appointments | Creates patients and households |
| `nurse` | Vitals, triage, queue, admission vitals | |
| `doctor` | Consultation, diagnosis, orders, referrals, admission | Only role that can issue a referral |
| `lab_scientist` | Lab worklist, result entry | |
| `pharmacist` | Prescription queue, dispensing | |
| `cashier` | Invoice, payment collection, receipt printing | Reverses/voids with reason |
| `accountant` | Reports, reconciliation, tariff management | Read-heavy |
| `admin` | User and device management, tariff, audit, everything | Full access |

**Defence in depth:** menu visibility and route guards in the SPA decide what a user *can reach*; `service`-layer checks decide what they *may do*. The SPA guard is for usability. The service check is the security boundary. With an offline client, an attacker controls the entire frontend, so the server check is the only one that counts.

---

## 7. Cross-cutting conventions

### 7.1 Identifiers

- All primary keys are **UUIDv7** (`google/uuid` → `uuid.NewV7()`), generated **client-side**.
- v7 is time-ordered, so keys insert at the right-hand edge of the B-tree instead of scattering random writes across it — the same benefit as a serial, without exposing record counts or requiring a server round-trip.
- Human-readable numbers are **separate display columns**: `patients.patient_number`, `households.family_number`, `encounters.visit_number`, `invoices.invoice_number`. Format is defined in `02-DATA-MODEL.md`. Never use these as a foreign key.

### 7.2 Money

- Stored as **`BIGINT` minor units** (kobo/pesewas/cents). **Never** `float`, never `numeric`-as-float.
- `amount_minor = 150000` means 1500.00 in the local currency.
- Currency code lives on the facility row.
- **Balances are always derived** — `SUM(charges) - SUM(allocations)`. There is no stored balance column anywhere, because a stored balance is a second source of truth that offline devices will desynchronise.

### 7.3 Time

- All timestamps are `TIMESTAMPTZ`, stored UTC. The client renders local time.
- Never store a local-time string.
- Every row carries `created_at` and `updated_at`, set by the **client** for clinical facts (the moment of care is what matters) and corrected by the server if the device clock is wildly wrong — record `device_recorded_at` alongside when the discrepancy matters.

### 7.4 Single tenancy — one deployment, one health centre

This system is **single-tenant by design**. One installation serves one Primary Health Centre: its own code, its own PostgreSQL database, its own API, its own domain. A second facility that wants the system deploys its own copy from the same repository.

**No table carries a tenant column.** `facility_id` appears nowhere in the schema. There is no facility selector, no tenant resolution on requests, and no cross-facility reporting.

`facilities` still exists, but as a **single configuration row** rather than a tenant root — the §5.1 table in `01-DATA-MODEL.md`. It supplies the hospital name and address for receipt headers and referral letterheads, the `code` that prefixes every document number (`URPHC/PAT/2026/00123`), the currency and its minor units, and the timezone used for day boundaries. One row, seeded at install, enforced by:

```sql
CREATE UNIQUE INDEX uq_facilities_single_org ON facilities ((true));
```

That constant-expression index is what makes "single tenant" a **database fact** rather than a convention. A second row cannot be inserted, by the application or by hand.

**What this buys, beyond simplicity.** The tenant column was the source of every awkward constraint in the earlier design: per-facility uniqueness on ten columns, a tenant predicate on every index, a facility filter in the sync pull, a `facility_mismatch` rejection in the sync push, and the only genuinely **circular foreign key** in the schema (`users.facility_id → facilities` while `facilities.created_by → users`). Removing the dimension removed all of it rather than working around it.

**Why not build for multi-tenancy anyway, "in case"?** Because the requirement is one hospital, and a speculative tenant column costs real correctness now: every query must remember the predicate, every unique constraint must include the column, and forgetting either is a silent cross-tenant data leak. A second hospital is a deployment, not a feature flag.

**The one place a facility identity is still recorded** is `referrals.from_facility_id`, which is not tenancy — it is the referral note recording *which organisation issued it*, because the receiving hospital needs that on the letterhead. That FK points at the same single row.

**If multi-facility is ever genuinely required**, the migration path is: add a `facilities`-style tenant table, add the column with a backfill to the existing row, widen the unique constraints, and add the predicate to the query layer. That is a substantial change, and deferring it until it is actually needed is the correct call — recorded here so the decision is visible rather than accidental.

### 7.5 Offline-safe workflow transitions

Workflow stages advance through an explicit state machine, validated in the service layer against a single source of truth:

```
registered → vitals_done → awaiting_doctor → (awaiting_lab → lab_done → awaiting_doctor)
                          → awaiting_payment(lab) → awaiting_lab
                          → in_consultation → awaiting_payment(drugs) → awaiting_pharmacy
                          → pharmacy_done → closed
                          ↗ admitted (branches to ward management, closes via discharge)
```

Transitions are **idempotent**: applying the same transition twice is a no-op, not an error. This matters enormously when a device retries a queued operation after a flaky connection.

---

## 8. Environment variables

See `.env.example` at the repository root for the authoritative list. The non-obvious ones:

| Variable | Purpose | Trap |
|---|---|---|
| `DATABASE_URL` | Neon **pooled** endpoint | Extended protocol must be disabled (§3.2). **Keep the value double-quoted in `.env`** — see §3.7 |
| `MIGRATION_DATABASE_URL` | Neon **direct** endpoint | Used only by goose; never by the API |
| `PORT` | Injected by Cloud Run | Must not be hard-coded (§3.1) |
| `ALLOWED_ORIGINS` | Comma-separated CORS allow-list | Must include the CF Pages preview URLs |
| `TRUST_PROXY` | `true` on Cloud Run | Gates `CF-Connecting-IP` reading (§3.6) |
| `JWT_SECRET` | HS256 signing key | ≥32 random bytes; Secret Manager in production |
| `ACCESS_TOKEN_TTL` / `REFRESH_TOKEN_TTL` | e.g. `15m` / `720h` | |
| `OFFLINE_GRACE_PERIOD` | e.g. `168h` (7 days) | How long a device may work offline before `OFFLINE_POLICY` applies |
| `OFFLINE_POLICY` | `block_writes` \| `warn_only` | What happens when the grace window expires. See §5.1 |
| `REFERRAL_TTL_HOURS` | `48` | The access window from your requirements |
| `REFERRAL_PIN_REQUIRED` | `true`/`false` | Lets you demo both modes |

---

## 9. Why these choices — the short version for your defence

| Decision | Justification to give an examiner |
|---|---|
| Offline-first with client-generated UUIDv7 | A PHC's work cannot stop because a link is down. Server-generated IDs would require a round-trip per record, so IDs are generated on the device and are globally unique and time-sortable. |
| Append-only clinical facts, last-write-wins for mutable records | Vitals and lab results are timestamped observations — two of them are two facts, not a conflict. Demographics genuinely can be edited concurrently, and for those, the most recent edit is the clinically sensible winner. |
| Invoice + payment ledger, derived balances | Storing a balance duplicates truth. Offline devices would each hold a different stored balance; deriving it guarantees one answer. |
| Device keypairs in addition to user JWTs | With full offline replication, a device is a data store. Loss of a device must be revocable without disrupting the user's account. |
| Random token + PIN for referrals | The link is unauthenticated by necessity, so security comes from unguessability (256-bit token), a second factor (PIN), a hard expiry, and an access log. |
| No pharmacy inventory | Explicitly scoped out. Recording dispense events preserves the clinical and billing record without inventing a reconciliation problem that offline devices could not solve safely. |

---

## 10. Related documents

| Document | Covers |
|---|---|
| `01-DATA-MODEL.md` | Every table, column, index, and the sync-column contract |
| `02-SYNC-PROTOCOL.md` | Outbox, push/pull wire format, conflict resolution |
| `03-UI-PATTERNS.md` | Design tokens, components, animation, print styles |
| `04-CODING-STANDARDS.md` | Go and TypeScript conventions, migration rules |
| `05-SECURITY.md` | Auth flows, RBAC matrix, threat model, data protection |
| `06-GIT-WORKFLOW.md` | Branching, commits, reviews, releases |
| `07-API-CONTRACT.md` | Endpoints, envelopes, error codes |
| `09-SPRINT-0-CHECKLIST.md` | Exact setup steps and files for the first sprint |
| `08-ROADMAP.md` | Sprint-by-sprint delivery plan |
| `PROJECT-TRACKER.md` | Live progress tracker |
