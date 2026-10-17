# PHC Patient Records & Referral Management System

**Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres**

A web application that lets a Primary Health Centre run its **entire patient workflow with no internet connection** — registration (individual and family), vitals, triage queue, doctor consultation and diagnosis, laboratory orders and results, prescriptions, pharmacy dispensing, cashier payments, inpatient admission with daily billing, and QR-code referrals printed on 58–80 mm thermal POS printers — syncing automatically to a central database whenever connectivity returns.

---

## The one-sentence architecture

> **The browser is the primary database. The server is a replication peer that also happens to be the authority on identity, security and referral links.**

Every workstation holds a complete local replica in IndexedDB (Dexie). Users always write locally and are never blocked by the network. A background engine drains a local outbox to the API and pulls changes made elsewhere.

---

## Stack

| Layer | Technology |
|---|---|
| **Frontend** | React 19.3.1 · TypeScript 7.0.2 · Vite 8.3.4 · Tailwind CSS v4.3.3 · Dexie 4.4.6 · TanStack Router · Zustand · Zod 4 · Radix UI · motion |
| **Backend** | Go 1.26 · Gin v1.12.0 · pgx v5.11.0 · goose v3.29.0 · Argon2id · JWT |
| **Database** | PostgreSQL 18 (Neon serverless) |
| **Deployment** | Cloudflare Pages (SPA) · Google Cloud Run (API) · Neon (DB) |

---

## Documentation

**Read `00-ARCHITECTURE.md` first, then `PROJECT-TRACKER.md` for where you are.**

| Document | What it covers |
|---|---|
| [`00-ARCHITECTURE.md`](docs/00-ARCHITECTURE.md) | Stack choices, offline-first topology, repo layout, deployment traps, environment variables |
| [`01-DATA-MODEL.md`](docs/01-DATA-MODEL.md) | **Canonical schema** — every table, column, index, constraint and migration |
| [`02-SYNC-PROTOCOL.md`](docs/02-SYNC-PROTOCOL.md) | Outbox, push/pull wire format, conflict resolution, idempotency, edge cases |
| [`03-UI-PATTERNS.md`](docs/03-UI-PATTERNS.md) | Design tokens, component inventory, animation rules, sync UI, POS print CSS |
| [`04-CODING-STANDARDS.md`](docs/04-CODING-STANDARDS.md) | Go and TypeScript conventions, migration rules, testing, tooling config |
| [`05-SECURITY.md`](docs/05-SECURITY.md) | Threat model, auth flows, RBAC matrix, offline device identity, referral security |
| [`06-GIT-WORKFLOW.md`](docs/06-GIT-WORKFLOW.md) | Branching, Conventional Commits, PR checks, releases |
| [`07-API-CONTRACT.md`](docs/07-API-CONTRACT.md) | Every endpoint, envelope, error code and role requirement |
| [`08-ROADMAP.md`](docs/08-ROADMAP.md) | Sprint-by-sprint delivery plan (Sprint 0 → 9) with demo scripts |
| [`09-SPRINT-0-CHECKLIST.md`](docs/09-SPRINT-0-CHECKLIST.md) | Step-by-step environment setup and walking skeleton |
| [`PROJECT-TRACKER.md`](docs/PROJECT-TRACKER.md) | **Live tracker** — sprint status, task backlogs, risks, decisions, session log |
| [`../CLAUDE.md`](../CLAUDE.md) | Working agreement for AI assistance in this repo |

---

## Scope

### In scope
Individual and family patient registration · vitals · triage queue · doctor consultation and diagnosis · laboratory orders, payment and results · prescriptions · pharmacy dispensing · cashier payments with derived balances · inpatient admission with daily charging and discharge gating · referral notes with QR access · 8 staff roles · full offline operation with background sync

### Explicitly out of scope
Pharmacy inventory (stock, batch, expiry) · HR and payroll · insurance/HMO claims · SMS/email · native mobile apps · **multi-facility / multi-tenant operation** · HL7/FHIR interoperability

Stating this list is deliberate: it stops scope creep and it is exactly what an examiner asks about.

### Single tenant, by design

**One deployment serves exactly one health centre** — its own code, its own database, its own API. A second facility that wants this system deploys its own copy from this repository. No table carries a tenant column.

`facilities` still exists, but as a **single configuration row** rather than a tenant root: it supplies the name and address printed on receipt headers and referral letterheads, the `code` that prefixes every document number (`URPHC/PAT/2026/00123`), the currency and its minor units, and the timezone used for day boundaries. One row, enforced by the database:

```sql
CREATE UNIQUE INDEX uq_facilities_single_org ON facilities ((true));
```

That constant-expression index makes single-tenancy a **database fact** rather than a convention — a second row cannot be inserted, by the application or by hand.

Removing the tenant dimension also removed the schema's only genuinely **circular foreign key** (`users.facility_id → facilities` while `facilities.created_by → users`), rather than working around it. See [`00-ARCHITECTURE.md` §7.4](docs/00-ARCHITECTURE.md) for the full reasoning and the migration path if multi-facility is ever genuinely required.

---

## Roles

`front_desk` · `nurse` · `doctor` · `lab_scientist` · `pharmacist` · `cashier` · `accountant` · `admin`

---

## Demo accounts

All eight demo accounts share the password **`ChangeMe!2026`** and are seeded with
`must_change_password = true`. Seeded by migration `00009_seed_demo_users.sql`, so a
fresh install has one account per role without any manual SQL.

| Staff number | Role | Why it exists |
|---|---|---|
| `DEMO/STF/0001` | `admin` | Staff administration; the only account that can see `/admin/users` |
| `DEMO/STF/0002` | `front_desk` | Registration and the queue start point |
| `DEMO/STF/0003` | `nurse` | Vitals and triage |
| `DEMO/STF/0004` | `doctor` | Consultation, diagnosis, referral |
| `DEMO/STF/0005` | `lab_scientist` | Lab worklist |
| `DEMO/STF/0006` | `pharmacist` | Dispensing |
| `DEMO/STF/0007` | `cashier` | Payments |
| `DEMO/STF/0008` | `accountant` | Reports |

**This is seed data, not a deployment.** A real installation must sign in as `DEMO/STF/0001`,
change the password, and disable or delete the other seven. Migration `00009`'s `Down` step
removes them all.

Signing in as different roles is how the RBAC is demonstrated: the sidebar for a cashier shows
no triage board, and `/admin/users` redirects away for everyone except an administrator.

---

## Repository layout

```
phc-records/
├── api/                  # Go backend
│   ├── cmd/api/          # entry point
│   ├── internal/
│   │   ├── config/       # env loading + validation
│   │   ├── domain/       # plain structs and enums
│   │   ├── repo/         # SQL only
│   │   ├── service/      # business rules + RBAC (the security boundary)
│   │   ├── http/         # Gin handlers, middleware, DTOs
│   │   ├── sync/         # sync engine
│   │   └── platform/     # jwt, argon2, ids, clock, qr
│   └── db/migrations/    # goose .sql files
├── web/                  # React frontend
│   └── src/
│       ├── db/           # Dexie schema + the localWrite helper
│       ├── sync/         # outbox, pusher, puller, status
│       ├── features/     # feature-sliced modules
│       ├── components/ui # design-system primitives
│       └── styles/       # tokens.css, print.css
├── docs/                 # this documentation set
└── scripts/              # db reset, seed, deploy helpers
```

---

## Getting started

See [`docs/09-SPRINT-0-CHECKLIST.md`](docs/09-SPRINT-0-CHECKLIST.md) for the full walkthrough. The short version:

```bash
cp .env.example .env          # then fill in your Neon URLs and secrets

cd api
goose -dir db/migrations postgres "$MIGRATION_DATABASE_URL" up
go run ./cmd/api              # http://localhost:8080

cd ../web
pnpm install
pnpm dev                      # http://localhost:5173
```

### Verify it works

```bash
curl -s localhost:8080/health              # {"status":"ok",...}
curl -s localhost:8080/ready               # {"status":"ready","database":"ok"}
curl -s localhost:8080/api/v1/meta | jq    # schema version, server time, facility count
```

> `facilityCount: 1` means the whole chain works: Go → pgx → Neon → back.

---

## The two features that matter most in a viva

1. **The complete clinical workflow runs with the network off** — register, triage, consult, order labs, prescribe, pay, dispense — and every record arrives intact when connectivity returns.
2. **A referral note printed as a QR code on a thermal slip can be opened at another facility for 48 hours**, with a PIN, an expiry enforced in SQL, and a full access log.

Practise demonstrating both until they are boring.

---

## Traps worth knowing before you start

Four things reliably cost people a day each. All are covered in the docs.

| Trap | Symptom | Where it is explained |
|---|---|---|
| Neon's pooler breaks prepared statements | `prepared statement "stmtcache_..." already exists`, intermittently, under load | `00-ARCHITECTURE.md` §3.2 |
| Cloud Run injects `$PORT` | Container builds fine, never serves | `00-ARCHITECTURE.md` §3.1 |
| CORS between Pages and Cloud Run | Console errors, and adding the origin is not enough — you must redeploy the API | `00-ARCHITECTURE.md` §3.5 |
| A write that skips the outbox never syncs | Everything looks correct on that one machine, forever | `02-SYNC-PROTOCOL.md` §4.1 |
