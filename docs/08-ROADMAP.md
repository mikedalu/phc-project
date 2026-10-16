# Roadmap — Sprint-by-Sprint Delivery Plan

> **Project:** Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres
> **Document status:** Living plan. Update it at the end of every sprint retrospective — a roadmap nobody edits is a roadmap nobody follows.
> **Companion documents:** `00-ARCHITECTURE.md` (canonical), `01-DATA-MODEL.md`, `02-SYNC-PROTOCOL.md`, `03-UI-PATTERNS.md`, `04-CODING-STANDARDS.md`, `05-SECURITY.md`, `06-GIT-WORKFLOW.md`, `07-API-CONTRACT.md`, `PROJECT-TRACKER.md`
> **Audience:** one final-year student, working part-time around other coursework.

---

## Table of contents

| § | Section |
|---|---|
| 1 | [How to use this roadmap](#1-how-to-use-this-roadmap) |
| 2 | [Definition of Ready and Definition of Done](#2-definition-of-ready-and-definition-of-done) |
| 3 | [Estimation guidance](#3-estimation-guidance) |
| 4 | [The sprint plan](#4-the-sprint-plan) |
| 5 | [Cross-sprint dependency graph](#5-cross-sprint-dependency-graph) |
| 6 | [Risk register](#6-risk-register) |
| 7 | [Testing strategy per sprint](#7-testing-strategy-per-sprint) |
| 8 | [Milestones for the academic calendar](#8-milestones-for-the-academic-calendar) |
| 9 | [Buffer and contingency guidance](#9-buffer-and-contingency-guidance) |
| 10 | [Post-project / future work](#10-post-project--future-work) |

---

## 1. How to use this roadmap

### 1.1 The method: short sprints, one developer, always demoable

This project is delivered with **agile, incremental delivery**: the system is built in a sequence of short time-boxed sprints, each ending in something that runs, and each building only on work that is already finished and proven.

The cycle inside every sprint is fixed:

```
   ┌─────────────────────────────────────────────────────────────────┐
   │                                                                 │
   ▼                                                                 │
PLAN ──────► BUILD ──────► DEMO ──────► RETRO ──────► (next sprint) ─┘
 │             │             │            │
 │             │             │            └─ 15 min, solo. Three
 │             │             │               questions: what worked,
 │             │             │               what didn't, one change.
 │             │             │               Write the change into
 │             │             │               PROJECT-TRACKER.md.
 │             │             │
 │             │             └─ Non-negotiable. Run the click-through in
 │             │                the sprint's "Demo script". If a step
 │             │                fails, the sprint is NOT done.
 │             │
 │             └─ Hand-written code only. Small units. Commit at least
 │                every working day (06-GIT-WORKFLOW.md).
 │
 └─ 30–45 min at sprint start. Pull the next sprint section from this
    document, re-check each task against the Definition of Ready,
    and move them into PROJECT-TRACKER.md. If a task is not Ready,
    it does not enter the sprint.
```

### 1.2 Sprint length: **two weeks** (recommended)

Two weeks, not one. Here is the reasoning, and it is worth repeating in your defence when an examiner asks why you did not "do proper Scrum".

| Factor | 1-week sprint | 2-week sprint | Verdict |
|---|---|---|---|
| Setup overhead (plan, demo, retro, release) | ~3 h of an 8–15 h week — **20–35 % overhead** | ~3 h of a 16–30 h fortnight — **10–20 %** | 2-week wins |
| Context reload cost | You re-learn your own code from scratch every Monday | Code stays "warm" in your head | 2-week wins |
| Realistic throughput | You cannot finish a feature that spans migrations + repo + service + handler + screen + test in 8 focused hours | That feature fits comfortably | 2-week wins |
| Time to first deployment | Slightly earlier | Two weeks later — irrelevant, since Sprint 0 is small | Tie |
| Feedback frequency | Higher | Still frequent: a demo every fortnight | 1-week wins marginally |
| Resilience to a bad week (coursework deadline, illness) | One bad week kills a whole sprint | One bad week is absorbed; you still demo | 2-week wins |

**Decision: two-week sprints, with a mid-sprint checkpoint.** Around day 5–6 of each sprint, spend 15 minutes asking: *"Is the demo script still achievable?"* If not, cut scope now (see §9) rather than discovering it on demo day. `PROJECT-TRACKER.md` should carry a `mid-sprint checkpoint` row for this.

**Exception — Sprint 0 only.** Sprint 0 is a *walking skeleton*: it either deploys end-to-end or it does not. If you have never deployed to Cloud Run before, treat Sprint 0 as a hard two weeks and accept that your first migration may be your only migration that sprint. That is the correct trade: retiring deployment risk early is worth more than any feature.

### 1.3 The rule that keeps this honest

> **Every sprint ends with something demoable — even if it is small.**

A sprint that produces "refactored the repo layer" and nothing a human can click is a failed sprint, no matter how good the code is. If a sprint's work is genuinely invisible (for example, hardening), then the *demo* is a test run, a dashboard, a printed report, or a log — still a thing you can show on a screen.

Corollaries:

- **No big-bang integration.** Nothing is "wired up later". Sprint 9 does not contain an integration phase, because there is nothing left to integrate.
- **No sprint depends on unmerged work.** If Sprint *n*'s demo needs half of Sprint *n+1*, the scope was wrong at planning time.
- **Vertical slices, not layers.** Prefer "patients can be registered offline and sync, from form to Neon and back" over "the repo layer is complete".
- **Sync is built early, before the features that need it** — see §4.3 for the justification. This is the single most important sequencing decision in the plan.

### 1.4 Working rhythm

| When | What | Time |
|---|---|---|
| Sprint day 1 | Planning: choose tasks, confirm Ready, copy into `PROJECT-TRACKER.md` | 30–45 min |
| Days 2–9 | Build. Commit at least once per day. Push at least weekly. | core hours |
| Day 5–6 | Mid-sprint checkpoint | 15 min |
| Day 10 | Freeze: no new tasks. Finish, test, write migration `Down`. | core hours |
| Day 10–11 | **Demo to self (or mentor)** — run the demo script exactly as written | 20 min |
| Day 11 | Retro + update `PROJECT-TRACKER.md` + tag the release | 20 min |
| Day 12–14 | **Buffer.** Absorb slippage, or pull the next sprint forward. | — |

That trailing buffer is deliberate. It is the mechanism by which §9's ~20 % slippage allowance actually gets absorbed instead of silently eaten.

---

## 2. Definition of Ready and Definition of Done

### 2.1 Definition of Ready (DoR) — a task may enter a sprint only if…

- [ ] The task is described in **one sentence** with a verb ("Add `POST /patients` handler that writes a patient row and returns 201"), not a noun ("patients API").
- [ ] **Files to touch are named in advance** (e.g. `api/internal/service/patient.go`, `web/src/features/patients/PatientForm.tsx`).
- [ ] Any **migration needed is drafted** — table, columns, indexes, and the sync-column contract from `01-DATA-MODEL.md`.
- [ ] Any **endpoint needed is already specified** in `07-API-CONTRACT.md` (request shape, response envelope, error codes).
- [ ] The task **fits in one working session** (≤ 3 focused hours). If not, split it.
- [ ] **Prerequisites are merged and deployed** — the previous sprint's work is on `main` and live on Cloud Run/Cloudflare Pages.
- [ ] The **acceptance check is written down** ("saving a patient with no surname shows a field error and writes nothing").
- [ ] It is **in scope** — it is not pharmacy inventory, HR, SMS, insurance, or multi-facility administration (`00-ARCHITECTURE.md` §1.2). This is a single-tenant system: one deployment, one health centre.
- [ ] If it is a **new table**, it has been checked against the sync contract: `id`, `revision`, `server_rev`, `sync_state`, `deleted_at`, `created_at`, `updated_at`, `created_by`, `updated_by`.
- [ ] **No new dependency** is required, or the new dependency is already pinned in `00-ARCHITECTURE.md` §2.

### 2.2 Definition of Done (DoD) — global, applies to every sprint

A sprint is Done only when **all** of the following are true.

**Code**

- [ ] All sprint tasks in `PROJECT-TRACKER.md` are `done` or explicitly moved to the next sprint **and named in the retro**.
- [ ] Code is **hand-written and understood** by the student. No unexamined generated code. Nothing is committed that you cannot explain line by line at the viva — if you cannot explain it, delete it and write it again from the mentor's example.
- [ ] Layering respected: handlers → services → repos → domain. No SQL in handlers. No `if user.Role == …` in a repo (`00-ARCHITECTURE.md` §4.1).
- [ ] Clinical writes go through **the one write path**: Zod validate → build row with client UUIDv7 → Dexie transaction (row + outbox) → return immediately (`00-ARCHITECTURE.md` §4.4).
- [ ] Clinical reads use `useLiveQuery` against Dexie. TanStack Query is used **only** for sync/admin/reports/public referral (`00-ARCHITECTURE.md` §4.3).
- [ ] `gofmt`/`go vet` clean; `tsc --noEmit` clean; no `any` introduced without a comment saying why.

**Database**

- [ ] Migrations written with goose `-- +goose Up` **and a working `-- +goose Down`**. The `Down` has been shown to run: `goose -dir api/db/migrations down` then `up` again.
- [ ] Migration file name follows `NNNNN_verb_noun.sql` with a sequence number continuing from the previous sprint (never reuse or renumber).
- [ ] Every new syncable table carries the full sync column contract.
- [ ] Migrations run against the **Neon direct** URL; the API uses the **pooled** URL with `QueryExecModeExec` (`00-ARCHITECTURE.md` §3.2).
- [ ] indexes exist for every query the new screen makes (a `.where()` in Dexie and a `WHERE` in SQL both deserve one).
- [ ] Balances are never stored: any money total shown is `SUM(charges) − SUM(allocations)` computed on read.

**Tests**

- [ ] Unit tests for new business rules (Go: service layer; TS: Zod schemas, calculations, state-machine transitions).
- [ ] Integration tests for new endpoints/repos against a real Postgres (Neon branch or local).
- [ ] E2E tests added or extended per §7 for the sprint's flows.
- [ ] All tests pass locally **and** the full suite passes in CI.
- [ ] **Manual offline check**: the sprint's feature was exercised with the network off (DevTools *Offline*, or Playwright `context.setOffline(true)`) wherever the feature is offline-capable.
- [ ] No test is skipped to make the suite green. A quarantined test is a task in the next sprint.

**Evidence and docs**

- [ ] The sprint's **demo script ran end-to-end** and was recorded (30–90 s screen capture is enough; it is also perfect material for your dissertation appendix).
- [ ] `docs/` updated: any new endpoint in `07-API-CONTRACT.md`, any new table in `01-DATA-MODEL.md`, any new sync rule in `02-SYNC-PROTOCOL.md`, any new screen pattern in `03-UI-PATTERNS.md`.
- [ ] `PROJECT-TRACKER.md` updated with what shipped, what slipped, and why.
- [ ] `.env.example` updated if new configuration was introduced.
- [ ] Seed data updated so a fresh clone can reproduce the demo.

**Release**

- [ ] Work committed with **conventional commit messages** (`feat(patients): …`, `fix(sync): …`, `docs(roadmap): …`, `chore(infra): …`) per `06-GIT-WORKFLOW.md`.
- [ ] Branch merged to `main`; `main` deployed to Cloud Run + Cloudflare Pages.
- [ ] The deployed URL was exercised (not just localhost) — including the CORS path, which only exists in the deployed environment.
- [ ] Git tag `sprint-N` pushed; release notes written in one paragraph.
- [ ] **Not a single uncommitted change remains** at sprint end. Losing work is the most avoidable failure in this plan (§6, R7).

---

## 3. Estimation guidance

### 3.1 Sizing scheme — T-shirt sizes, converted to points

Story points are used only to compare effort *within this project*, never across projects. T-shirt sizes are easier to feel honestly about than raw numbers.

| Size | Points | Meaning for a solo part-time developer | Examples |
|---|---|---|---|
| **XS** | 1 | ≤ 30 min. Mechanical, no design decisions. | Add a route stub; add a Tailwind token; add a `.where()` index to the Dexie schema |
| **S** | 2 | ~1–2 h. One file, one idea, obvious test. | `GET /patients/:id` handler + repo query; a Zod schema; a print-CSS block |
| **M** | 3–5 | ~2–4 h. Several files, one layer boundary crossed. | Migration + repo + service + handler + test for `POST /vitals`; a form screen wired to Dexie |
| **L** | 8 | ~4–6 h. A vertical slice with a real decision in it. | The outbox drain with backoff; invoice creation from tariff lines; the POS slip layout |
| **XL** | 13 | **Must be split.** > 1 working session, so it violates the DoR. | "The sync engine" → split into push endpoint, pull endpoint, outbox, applier, status UI |

Two rules that matter more than the numbers:

1. **If a task grows past L during the sprint, stop and re-split it.** The estimate is not the problem; the task boundary is.
2. **Never carry an XL into a sprint.** It will not finish, and an unfinished task blocks the demo.

### 3.2 Planned velocity — be honest with yourself

| Input | Value |
|---|---|
| Focused hours available per week (part-time, alongside coursework) | **8–15 h** |
| Realistic efficiency inside those hours (an undergraduate writing unfamiliar code by hand) | 60–75 % |
| Effective productive hours per week | **≈ 6–11 h** |
| Effective productive hours per 2-week sprint | **≈ 12–22 h** |
| Points per productive hour (from the scale above, where 1 point ≈ 30 min of *estimated* work) | ≈ 1.5–2 |
| **Planned velocity** | **20–30 points per sprint**, i.e. roughly 10–15 points per week |

What that means concretely, per sprint:

- **2–3 L tasks, or 1 L + 3–5 M tasks, or 6–9 S/M tasks.** That is the whole sprint.
- **One vertical slice per sprint is the target**, not three. Sprint 4's "vitals + queue" is a good sprint. Sprint 4 plus "and also start referrals" is not.
- **You will lose one week in five** to illness, a coursework deadline, a family event, or a broken environment. That is already priced into the 20 % buffer in §9; do not treat it as a personal failure.
- **Do not plan 40-hour sprints.** Plan 15 productive hours and finish; an over-planned sprint fails its demo, and a failed demo is the one thing that compounds across sprints.

### 3.3 Estimation hygiene

- Re-estimate **only** tasks you have not started. Re-estimating in-flight work hides the learning.
- At each retro, record *planned points* vs *completed points* in `PROJECT-TRACKER.md`. After three sprints you will have a real velocity number; use it instead of this table.
- If completed velocity is consistently below 20 points, **cut scope** (§9) — do not extend the sprint. Extending sprints is how the whole calendar slips.

---

## 4. The sprint plan

Ten sprints: **Sprint 0 through Sprint 9**, two weeks each (Sprint 0 may run long — see §1.2). Migration numbers continue monotonically across sprints; never renumber a migration that has touched Neon.

### Sprint 0 — Foundations & walking skeleton

**Sprint goal.** Stand up the entire delivery pipeline — repo, Go API on Cloud Run, Neon Postgres with goose migrations, Vite SPA on Cloudflare Pages — and prove it end-to-end with a trivial round-trip so that deployment risk is retired on day one.

**Duration.** 2 weeks (allow 3 if this is your first Cloud Run deploy).
**Prerequisites.** None. This is the first sprint.

**Why this sprint exists.** Most student projects spend their first month on features and hit deployment in week 12, then lose two weeks to Docker, CORS, ports and connection URLs. This sprint inverts that: the *first* thing that works is the pipeline, and every sprint afterwards just adds to it.

#### Deliverables

- [ ] Git repository `phc-records` with the directory layout from `00-ARCHITECTURE.md` §4.2, plus `README.md`, `CLAUDE.md`, `.gitignore`, `.gitattributes`.
- [ ] Go module `api/` with `cmd/api/main.go` wiring config → pgx pool → router; server binds `$PORT` (default `8080` only for local dev).
- [ ] `internal/config` reading `DATABASE_URL`, `MIGRATION_DATABASE_URL`, `PORT`, `ALLOWED_ORIGINS`, `TRUST_PROXY`, `JWT_SECRET`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `OFFLINE_GRACE_PERIOD`, `REFERRAL_TTL_HOURS`, `REFERRAL_PIN_REQUIRED`; `.env.example` committed.
- [ ] pgx pool configured with `DefaultQueryExecMode = pgx.QueryExecModeExec` and `MaxConns = 10`.
- [ ] `GET /healthz` returning `{status, version, commit, db: "ok"}` and genuinely pinging Neon.
- [ ] `GET /api/v1/meta` returning `{serverTime, schemaVersion, facility: {name, code, currency}, facilityCount}` — the trivial round-trip target for the frontend.
- [ ] goose wired with `api/db/migrations/`, runnable via `goose -dir api/db/migrations up` against the **direct** Neon URL; `scripts/db-up.sh`, `scripts/db-down.sh`, `scripts/db-status.sh`.
- [ ] Vite + React 19 + TypeScript 7 app in `web/` (TypeScript pinned exactly, no caret), TanStack Router with one route, Zustand store skeleton.
- [ ] Tailwind v4 with `@theme` tokens in `web/src/styles/tokens.css` (colour, spacing, radius, type scale) per `03-UI-PATTERNS.md`; `web/src/styles/print.css` created but empty save a comment.
- [ ] Dexie initialised at version 1 in `web/src/db/database.ts` with a `meta` table; a thin `web/src/db/meta.ts` with `getMeta`/`setMeta`.
- [ ] `web/src/lib/api-client.ts` — one place that knows the API base URL, sets `Authorization` later, applies a 30 s timeout and a single retry.
- [ ] `Dockerfile` (multi-stage, distroless/static final image) and `cloudbuild.yaml` or a `scripts/deploy-api.sh` using `gcloud run deploy`.
- [ ] Cloudflare Pages project connected to the repo, with `VITE_API_BASE_URL` set per environment.
- [ ] CI (GitHub Actions): `go vet`, `go test ./...`, `tsc --noEmit`, `vitest run`, and migration `up`/`down` check against an ephemeral Neon branch.
- [ ] `docs/` directory present with `00-ARCHITECTURE.md` and this file; `PROJECT-TRACKER.md` created.

#### Tasks

- [ ] `[INFRA]` Create the GitHub repo, branch protection on `main`, and the `main` + `feat/*` + `fix/*` branch convention from `06-GIT-WORKFLOW.md`.
- [ ] `[INFRA]` Create the Neon project. Copy **both** connection strings: pooled for the API, direct for goose. Store as GitHub secrets and Cloud Run env vars.
- [ ] `[INFRA]` Create the Cloud Run service with `--max-instances=3` and `--min-instances=0` (raise to 1 only for the defence — `00-ARCHITECTURE.md` §3.3).
- [ ] `[INFRA]` Create the Cloudflare Pages project; add `pages.dev` preview + production origins to `ALLOWED_ORIGINS`.
- [ ] `[BE]` `go mod init`, then add exactly the pinned versions from `00-ARCHITECTURE.md` §2.1. Commit `go.sum`.
- [ ] `[BE]` `internal/config/config.go` — load and validate; fail fast with a clear message if `DATABASE_URL` is missing.
- [ ] `[BE]` `internal/platform/db/pool.go` — pgxpool with `QueryExecModeExec`; a `Ping` used by `/healthz`.
- [ ] `[BE]` `internal/platform/logging` — zerolog JSON logger with a request-ID middleware.
- [ ] `[BE]` `internal/http/router.go` — `gin.New()`, recovery, request ID, zerolog access logs, `gin-contrib/cors` with the explicit allow-list, `SetTrustedProxies` gated by `TRUST_PROXY`.
- [ ] `[BE]` `internal/http/handlers/health.go` and `about.go`; a shared response envelope `{data, error, meta}` matching `07-API-CONTRACT.md`.
- [ ] `[DB]` Migration `00001_create_facilities.sql` — the single configuration row: `facilities(id, name, code, currency_code, timezone, ...)` plus `uq_facilities_single_org`; seed it in 00003 row in `api/db/seed/0001_facility.sql`.
- [ ] `[DB]` Migration `00002_create_users.sql` — `users` with sync columns, `email CITEXT UNIQUE`, `password_hash`, `role TEXT NOT NULL` with a CHECK against the eight roles, `is_active BOOLEAN`.
- [ ] `[FE]` `npm create vite@latest` with the React+TS template (Vite 8.3.4, React 19.3.1); pin `typescript` to `7.0.2` exactly.
- [ ] `[FE]` `web/src/app/router.tsx` with TanStack Router, a root layout (`AppShell`) and one route.
- [ ] `[FE]` `web/src/styles/tokens.css` — Tailwind v4 `@theme` block: brand colour, clinical status colours (waiting / in-progress / done / alert), spacing, radii, font stack.
- [ ] `[FE]` `web/src/db/database.ts` — `class PhcDatabase extends Dexie` with `version(1).stores({ meta: 'key' })`.
- [ ] `[FE]` `web/src/routes/__root.tsx` + `web/src/routes/index.tsx` — the walking-skeleton screen: shows `/healthz` status, the `/about` server time, and a button that writes the current timestamp to `meta` and reads it back.
- [ ] `[TEST]` Go: `config_test.go` (missing env fails), `health_test.go` with `httptest` asserting 200 and the envelope shape.
- [ ] `[TEST]` TS: Vitest configured; `database.test.ts` opening Dexie in `fake-indexeddb` and round-tripping `meta`.
- [ ] `[TEST]` Playwright configured with `webServer`; a smoke test that loads the page, asserts "API: ok", clicks the round-trip button and sees a local timestamp.
- [ ] `[DOCS]` `README.md` — clone → install → migrate → run, in under 20 lines.
- [ ] `[DOCS]` `docs/00-ARCHITECTURE.md` committed; `docs/08-ROADMAP.md` (this file) committed; `PROJECT-TRACKER.md` with the Sprint 0 task list.
- [ ] `[DOCS]` `docs/06-GIT-WORKFLOW.md` — branch naming, conventional commits, when to tag.
- [ ] `[DOCS]` Screenshot the deployed app + the `/healthz` JSON into `docs/evidence/sprint-0/`.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00001_create_facilities.sql` | `facilities` — the single config row: name, code, currency, timezone |
| `00002_create_users.sql` | `users` — sync columns, unique email, role CHECK constraint, `is_active` |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `GET /healthz` | Pings Neon; returns version + commit SHA |
| `GET /api/v1/meta` | Trivial round-trip payload: server time, schema version, facility name + code + currency |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/` | Walking-skeleton page: API health, server time, local-write round-trip button |

#### Demo script

1. Open the **deployed** Cloudflare Pages URL (not localhost) on a laptop.
2. Point at the green "API: ok" indicator and say: "this is a Go binary on Cloud Run talking to Neon Postgres over pgx."
3. Press F12 → Application → IndexedDB → show the `phc-records` database with a `meta` table.
4. Click **Write local timestamp**. Show the new row inside IndexedDB and the value rendered on the page.
5. Go to DevTools → Network → **Offline**. Reload the page. The app still loads and the timestamp is still there.
6. Click **Write local timestamp** again → it still succeeds. Say the sentence that defines the whole project: *"the browser is the primary database; the server is a replication peer."*
7. Restore the network, click **Check API**, show `/about` returning the server time.
8. Show the GitHub Actions run that deployed this build, and `goose status` showing both migrations applied.

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | `config` fails fast on missing `DATABASE_URL`; `PORT` defaults to 8080 |
| Integration (Go) | `GET /healthz` with `httptest` + a real pool returns 200 and `db: "ok"` |
| Migration | `goose up` then `goose down` then `goose up` succeeds on a Neon branch |
| Unit (TS) | Dexie v1 opens and `meta` round-trips under `fake-indexeddb` |
| E2E (Playwright) | Page loads, reports API ok, local write visible, and the same page works with `context.setOffline(true)` |
| Manual | Cold-start timing: note how long the first request after idle takes (baseline for §6, R4) |

#### Risks / watch-outs

- **Cloud Run "container failed to start"** — almost always a hard-coded port. Bind `os.Getenv("PORT")`.
- **CORS failure on the deployed origin only** — the preview URL changes per deployment; add a wildcard-safe entry per `06-GIT-WORKFLOW.md` rather than `AllowAllOrigins`.
- **`prepared statement already exists`** — you used the pooled URL without `QueryExecModeExec`. Fix the pool config, not the query (`00-ARCHITECTURE.md` §3.2).
- **goose pointed at the pooled URL** — migrations must use the **direct** endpoint. Two env vars, two purposes.
- **Over-scoping Sprint 0.** The temptation is to add login here. Do not; login is Sprint 1 and it has a sprint's worth of work in it.
- **Spending a week on the design system.** Tokens and one button, that is all. The design system grows in `03-UI-PATTERNS.md` as screens appear.

#### Definition of Done for this sprint

- [ ] The deployed Pages URL renders the skeleton screen with a green API indicator.
- [ ] `curl https://<run-url>/healthz` returns 200 with `db: "ok"` from a cold start.
- [ ] Both migrations have been run **up and down** at least once on Neon.
- [ ] CI is green on `main` and blocks merges when red.
- [ ] `README.md` lets a stranger (your supervisor) run the app locally in under 10 minutes.
- [ ] Git tag `sprint-0` pushed, with release notes.

---

### Sprint 1 — Auth, roles & device identity

**Sprint goal.** Let a real user log in with an Argon2id-verified password, receive a short-lived access token and a rotating refresh token, and have that identity travel with every request — plus establish each workstation's ECDSA P-256 device identity and the 7-day offline login window that the sync engine will require.

**Duration.** 2 weeks.
**Prerequisites.** Sprint 0 complete and deployed.

**Why device identity is in *this* sprint, not later.** The sync endpoint in Sprint 2 requires *both* a valid access token and a valid device signature (`00-ARCHITECTURE.md` §5). If device keys arrive after the sync engine, you will retrofit a security requirement into a working protocol — the worst kind of change. Build the keypair now, while there is nothing to break.

#### Deliverables

- [ ] `internal/platform/argon2` — Argon2id hash + constant-time verify, versioned parameters encoded in the hash string (`$argon2id$v=19$m=…,t=…,p=…$salt$hash`) so parameters can be raised later without breaking old rows.
- [ ] `internal/platform/jwt` — HS256 access tokens (15 min) with claims `sub`, `role`, `device_id`, `jti`, `exp`. (No `facility_id` — single-tenant.)
- [ ] Refresh tokens: 30 days, **stored hashed** in Postgres, **rotated on every use**, old token revoked in the same transaction (reuse of a revoked token revokes the whole chain and is logged).
- [ ] `internal/http/middleware/auth.go` — bearer parsing, token validation, context injection of `UserID`, `Role`, `FacilityID`, `DeviceID`.
- [ ] `internal/http/middleware/rbac.go` — `RequireRole("doctor", "admin")` per-route; returns 403 with a stable error code.
- [ ] `internal/http/middleware/device_signature.go` — verifies `X-Device-Signature` over the canonical request string; **defined and unit-tested here, enforced on `/sync/*` from Sprint 2**.
- [ ] `internal/service/auth.go` — `Login`, `Refresh`, `Logout`, `ChangePassword`, `RecordFailedAttempt` with lockout after 5 failures in 15 minutes.
- [ ] Seed users: one per role (eight accounts), same demo password, documented in `README.md`, plus a `scripts/seed-dev.sh`.
- [ ] Frontend `features/auth/` — login form (react-hook-form + Zod), `useSession` store in Zustand with `persist` to IndexedDB (not `localStorage`) so it survives offline reloads.
- [ ] Frontend `lib/device-keys.ts` — generate an **ECDSA P-256** keypair via WebCrypto on first run, export the public key as **SPKI**, store the private key as an exported JWK in IndexedDB (re-imported non-extractable on load — see `05-SECURITY.md` §4.2), and provide `signRequest()`.
- [ ] Offline login: on successful online login, store an Argon2id-derived verifier of the user's password locally; while offline, verify against it and mint a **device-bound local session** that expires `OFFLINE_GRACE_PERIOD` (default 168h) after the last online authentication. On expiry, honour `OFFLINE_POLICY` — `block_writes` (default) or `warn_only`. See `00-ARCHITECTURE.md` §5.1.
- [ ] Route guards: `beforeLoad` on protected routes checking session presence, role, and offline-grace expiry; a friendly "session expired — reconnect to continue" screen rather than a redirect loop.
- [ ] Role-aware `AppShell` sidebar: menu items filtered by role from a single `ROLE_NAV` table in `web/src/app/nav.ts`.
- [ ] `/admin/users` list showing users, roles, active state, last login (admin only).
- [ ] `docs/05-SECURITY.md` — auth flows, RBAC matrix (role × capability), token lifetimes, threat notes for offline sessions.

#### Tasks

- [ ] `[DB]` Migration `00003_create_devices.sql` — `devices` (sync columns + `device_fingerprint TEXT UNIQUE`, `public_key TEXT`, `label`, `last_seen_at`, `revoked_at`).
- [ ] `[DB]` Migration `00004_create_refresh_tokens.sql` — `refresh_tokens(id, user_id, device_id, token_hash, family_id, expires_at, revoked_at, replaced_by, created_at)`; no sync columns (server-only, never replicated).
- [ ] `[DB]` Migration `00005_create_login_attempts.sql` — `login_attempts(id, email, ip, user_agent, succeeded, created_at)` for lockout and audit.
- [ ] `[BE]` `internal/repo/user_repo.go` — `FindByEmail`, `FindByID`, `List`, `SetPasswordHash`, `TouchLastLogin`.
- [ ] `[BE]` `internal/repo/device_repo.go` — `Register`, `FindByFingerprint`, `Revoke`, `TouchLastSeen`.
- [ ] `[BE]` `internal/repo/token_repo.go` — `Insert`, `FindByHash`, `RevokeFamily` inside a transaction with row locking.
- [ ] `[BE]` `internal/service/auth.go` — the whole login/refresh/logout state machine; the only place `argon2` and `jwt` are called.
- [ ] `[BE]` Handlers: `auth_handler.go` with login/refresh/logout/me; `device_handler.go` with register/list/revoke.
- [ ] `[BE]` Rate-limit login and refresh by IP + email (gin middleware, in-memory token bucket is sufficient; note in the docs that Cloud Run instances are per-instance and multi-instance limits are approximate).
- [ ] `[BE]` Audit: write `audit_logs` rows for `login_succeeded`, `login_failed`, `token_refreshed`, `token_reuse_detected`, `device_registered`, `device_revoked`.
- [ ] `[FE]` `routes/login.tsx` with the form, error states (bad credentials, locked, offline), and the device-registration step on first login.
- [ ] `[FE]` `features/auth/session-store.ts` (Zustand) — `accessToken`, `refreshToken`, `user`, `role`, `deviceId`, `offlineGraceExpiresAt`, `lastOnlineAuthAt`.
- [ ] `[FE]` `lib/api-client.ts` — attach `Authorization`, refresh once on 401 then retry the original request, attach `X-Device-Signature` for `/sync/*`.
- [ ] `[FE]` Silent refresh on app start and 60 s before access-token expiry, with a single-flight guard so ten parallel requests do not trigger ten refreshes.
- [ ] `[FE]` `routes/admin.users.tsx` — table + role badge + active toggle (frontend only for now; the PATCH endpoint ships here too since it is trivial).
- [ ] `[FE]` Logout: revoke the refresh family server-side **and** clear the local session; if offline, queue the revocation and clear locally anyway.
- [ ] `[TEST]` Argon2 hash/verify round trip; verify fails on a tampered hash; parameters parse from the encoded string.
- [ ] `[TEST]` JWT: valid token accepted, expired rejected, wrong-secret rejected, `none` algorithm rejected.
- [ ] `[TEST]` Refresh rotation: old token becomes unusable; reusing a revoked token revokes the family.
- [ ] `[TEST]` RBAC: each role's denied routes return 403 (table-driven test over the route list).
- [ ] `[TEST]` Playwright: log in as `nurse`, see the nurse nav, navigate directly to `/admin/users` and get the forbidden screen.
- [ ] `[TEST]` Playwright offline: log in online, go offline, reload, still logged in; simulate 8 days offline (clock injection) and confirm re-auth is required.
- [ ] `[DOCS]` `05-SECURITY.md` + `07-API-CONTRACT.md` auth section + the demo-accounts table in `README.md`.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00003_create_devices.sql` | `devices` — sync columns, unique fingerprint, ECDSA P-256 public key, `revoked_at` |
| `00004_create_refresh_tokens.sql` | `refresh_tokens` — hashed token, rotation family, expiry, revocation (server-only) |
| `00005_create_login_attempts.sql` | `login_attempts` — lockout counter source and audit trail |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `POST /api/v1/auth/login` | Email + password + device fingerprint → access token, refresh token, user profile |
| `POST /api/v1/auth/refresh` | Rotates the refresh token; detects reuse |
| `POST /api/v1/auth/logout` | Revokes the refresh family |
| `GET /api/v1/auth/me` | Current user, role, offline grace deadline |
| `POST /api/v1/auth/change-password` | Requires the current password; revokes other devices' sessions |
| `POST /api/v1/devices` | Registers an ECDSA P-256 public key against a device fingerprint |
| `GET /api/v1/devices` | Admin: list devices |
| `DELETE /api/v1/devices/:id` | Revoke a device (takes effect on the next sync push) |
| `GET /api/v1/users` | Admin: list users |
| `PATCH /api/v1/users/:id` | Admin: role / active state (audited) |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/login` | Credentials + first-run device registration |
| `/admin/users` | User list, role assignment, activation |
| `/session-expired` | Offline-grace exceeded; prompts reconnection |

#### Demo script

1. Open the deployed app. Register the device (show the generated public key in the network tab).
2. Log in as `frontdesk@phc.test`. Land on the role-appropriate home. Note the "8 menu items", not 20.
3. Log out, log in as `doctor@phc.test`. Show that the sidebar changed and `/admin/users` now returns the forbidden screen.
4. F12 → Application → IndexedDB → show the stored device key and session, and that no plaintext password is anywhere.
5. Show `curl -X POST /auth/login` with a wrong password five times → 423 locked with a retry-after.
6. Show `curl` calling a protected endpoint with no token (401) and with a `cashier` token on a doctor-only route (403).
7. **Offline login:** go offline in DevTools, log out, log back in with the same credentials → success, with an "offline — N days remaining" badge. Then flip `OFFLINE_GRACE_PERIOD=30s` and `OFFLINE_POLICY=block_writes`, and confirm new writes are refused while existing records stay readable.
8. Log in as `admin@phc.test` → `/admin/users` → revoke the device you are using → show that the next online request is rejected.

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | Argon2id hash/verify; JWT validation including algorithm-confusion attempts |
| Unit (TS) | Session store transitions; offline-grace expiry maths; single-flight refresh |
| Integration (Go) | `login → refresh → refresh with old token` sequence against a real DB |
| Integration (Go) | Lockout after 5 failures; successful login resets the counter |
| E2E (Playwright) | Login per role, nav filtering, route-guard 403 screen |
| Manual offline | Offline login, offline reload, offline grace expiry (clock injected) |
| Security | Token in IndexedDB not `localStorage`; no password or refresh token in any log line |

#### Risks / watch-outs

- **Refresh-token races.** Two tabs refreshing at once burn each other's tokens. Fix with a single-flight guard in `api-client.ts` and a short grace window on the server that accepts a token rotated seconds ago from the same device.
- **Non-extractable keys vs. signing.** ECDSA P-256 is universally available in `crypto.subtle`, so there is no feature-detection fallback branch to write — one less thing to get wrong. The real trade-off is key persistence: export the private key as a JWK once so it survives reloads, then re-import it non-extractable on every load. See `05-SECURITY.md` §4.2 for the alternatives and why Option B is right at this scale.
- **Storing a local password verifier is the riskiest thing in the project.** Derive it with Argon2id over the *server-issued* credential, never store the raw password, and never let the verifier be exportable into the sync payload.
- **Clock tampering defeats the grace window.** Store `lastOnlineAuthAt` **and** a monotonic counter; refuse a local session if the wall clock moved backwards by more than a few minutes. Note that the window is deliberately short to test — set `OFFLINE_GRACE_PERIOD=30s` rather than waiting a week.
- **Scope creep into user management UIs.** An admin list and a role toggle is all this sprint needs.
- **Locking yourself out during development.** Keep a `scripts/reset-admin.sh` that re-hashes and resets the admin password from the CLI.

#### Definition of Done for this sprint

- [ ] Every protected endpoint is behind both `auth` and an explicit `RequireRole`.
- [ ] A table-driven test asserts the full role × route matrix, so a new route cannot be added without a role.
- [ ] No secret (password, refresh token, device private key) appears in any log line — verified by grepping a full-day CI log.
- [ ] Offline login works after a **hard reload** with the network off.
- [ ] Device revocation takes effect within one request.
- [ ] `05-SECURITY.md` documents the offline-auth threat model honestly, including what an attacker with the laptop can do.

---

### Sprint 2 — The sync engine

**Sprint goal.** Build the outbox/push/pull replication engine end-to-end so that two browsers converge through the server and one browser keeps working, and stays internally correct, while completely offline.

**Duration.** 2 weeks. **Expect this to be the hardest sprint in the project.** Plan for it, protect the time, and do not schedule anything else alongside it.

**Prerequisites.** Sprint 1 complete (auth + device signatures), Sprint 0 pipeline live.

#### 4.3.1 Why sync is built *before* the features that depend on it — read this twice

> **Every clinical feature in Sprints 3–8 is a variation of one pattern: "write a row locally, atomically, with an outbox entry, and let the engine replicate it." If the engine exists first, each feature is a form, a table, and a `useLiveQuery`. If the engine does not exist, each feature invents its own half-solution — a `fetch()` here, a hopeful `navigator.onLine` check there — and you end up with six inconsistent write paths that you must later unify. That unification is the "big-bang integration" this plan forbids.**

Concretely, building sync at Sprint 2 buys you five things:

1. **One write path, designed once.** `db.write()` with the Dexie transaction that upserts the row *and* appends the outbox entry (architecture §4.4) is written once, tested once, and reused by patients, vitals, lab results, invoices, payments, dispenses and referrals. Feature sprints then contain almost no sync code.
2. **Schema discipline from the first table.** Because the engine must replicate `households`, `patients`, `encounters`… every table in Sprints 3–8 is born with the sync column contract. Retrofitting `revision`, `server_rev`, `sync_state` and `deleted_at` onto ten tables later is a migration nightmare *and* it invalidates any data you have already collected.
3. **Conflict policy is testable in isolation.** Last-write-wins per field and append-only facts (architecture §5, §9) are abstract rules. With the engine first, you prove them with two browser contexts and a synthetic table *before* a real clinical record is at stake.
4. **Offline behaviour can be demonstrated from day one.** The examiner's central question — "show me it works with no network" — becomes answerable from Sprint 2 onwards, and every later sprint demo inherits it.
5. **You retire the unknown early.** Sync is the part of this project most likely to be harder than expected (§6, R1). Discovering that at Sprint 2 leaves seven sprints to adapt. Discovering it at Sprint 8 leaves none.

The cost of this ordering is that Sprints 3–8 *feel* less exciting than they might: no screen at the end of Sprint 2 shows a patient. Accept that. **A sync engine with no features is worth more than features with no sync engine.**

#### Deliverables

- [ ] `internal/sync/` — `push.go`, `pull.go`, `changes.go`, `conflict.go`, `apply.go`, `cursor.go`.
- [ ] Server change log: every applied mutation appends one row to `sync_change_log` with a globally monotonic `seq`.
- [ ] `POST /api/v1/sync/push` — accepts a bounded batch (≤ 200 ops) of outbox operations; per-operation result (`applied` | `duplicate` | `conflict_lww` | `rejected`); idempotent under replay.
- [ ] `GET /api/v1/sync/pull?since=<server_rev>&limit=<n>` — returns changes in `server_rev` order (≤ 500 rows), plus `has_more` and `next_cursor`.
- [ ] Conflict policy implemented and logged: **append-only** tables reject in-place updates to an existing `id`; **mutable** tables use last-write-wins with per-field resolution against `updated_at` (row-level LWW fallback documented).
- [ ] Idempotency: **client-generated operation IDs**. `sync_operations(op_id TEXT PRIMARY KEY, device_id, user_id, applied_at, result)` — a replayed op returns its recorded result without re-applying (`00-ARCHITECTURE.md` §9: payments are protected by idempotency, not by conflict resolution).
- [ ] Device signature enforcement on `/sync/push` and `/sync/pull` (middleware built in Sprint 1, switched on here).
- [ ] `sync_conflicts` table — every `conflict_lww` decision recorded with both versions, so Sprint 9's conflict dashboard has data.
- [ ] Frontend `web/src/db/write.ts` — `write(table, row)` and `writeMany(ops)`; a single Dexie transaction that upserts and appends to `outbox`; generates UUIDv7, sets `sync_state='pending'`, `revision=0` for new rows.
- [ ] Frontend `web/src/sync/outbox.ts` — enqueue, peek-next-batch, mark-done, mark-failed with attempt count and `next_attempt_at`.
- [ ] Frontend `web/src/sync/pusher.ts` — drains the outbox in order, per-table order preserved, exponential backoff with jitter (1 s → 2 → 4 → … capped at 5 min), 30 s request timeout, stops on 401 and triggers refresh.
- [ ] Frontend `web/src/sync/puller.ts` — pulls by cursor, applies inside Dexie transactions, advances the stored cursor only after a successful transaction, and never overwrites a locally `pending` row's fields with older server data.
- [ ] Frontend `web/src/sync/applier.ts` — one function that knows all tables: upsert, tombstones (`deleted_at`), and skip-if-locally-pending.
- [ ] `web/src/sync/status-store.ts` (Zustand) — `online`, `pendingCount`, `lastPullAt`, `lastPushAt`, `lastError`, `activeConflicts`.
- [ ] `SyncStatusIndicator` component in the app shell: online/offline dot, pending count, "last synced 2 min ago", a manual **Sync now** button, and an expandable error panel.
- [ ] Offline detection: `navigator.onLine` **plus** a reachability probe to `/healthz` (the flag is a hint, never the truth) and a "flaky" state distinct from "offline".
- [ ] `web/src/db/upgrade.ts` — a documented pattern for Dexie version upgrades, with a test that opens v1 data under a v2 schema.
- [ ] `docs/02-SYNC-PROTOCOL.md` — wire format, batch limits, ordering guarantees, conflict rules, cursor semantics, version negotiation.

#### Tasks

- [ ] `[DB]` Migration `00008_create_sync_infrastructure.sql` — `sync_change_log(seq BIGSERIAL PK, table_name, row_id, op, server_rev, changed_at, ...vice_id, user_id, changed_at)`, indexes on `(seq)` and `(table_name, row_id)`.
- [ ] `[DB]` Migration `00007_create_sync_operations.sql` — `sync_operations(op_id TEXT PK, device_id, user_id, table_name, row_id, result, applied_at)`, index on `(device_id, applied_at)`; a retention job documented (not built) for pruning older than 90 days.
- [ ] `[DB]` `sync_conflicts(id, table_name, row_id, conflict_type, losing_payload JSONB, winning_payload JSONB, ...)` — same migration_value JSONB, losing_updated_at, winning_updated_at, device_id, resolved_at)`.
- [ ] `[DB]` Migration `00009_create_sync_cursors.sql` — per-device `last_pulled_rev`, `last_pushed_at` (server-side visibility of who is behind).
- [ ] `[BE]` `internal/sync/protocol.go` — the request/response DTOs, versioned (`sync_version: 1`); the server must tolerate a client one version behind (`00-ARCHITECTURE.md` §3.4).
- [ ] `[BE]` `internal/sync/push.go` — validate the batch, check `sync_operations` for each `op_id`, apply in a transaction, append to `sync_changes`, return per-op results. Reject the whole batch **only** on auth failure; per-op failures are reported per op.
- [ ] `[BE]` `internal/sync/apply.go` — table registry: table name → {syncable, conflict mode (`append_only` | `lww_field`), allowed columns}. An unknown table or unknown column is `rejected`, not a 500.
- [ ] `[BE]` `internal/sync/conflict.go` — field-level LWW using `updated_at`; equal timestamps break ties by `device_id` (deterministic, documented) and record the losing value.
- [ ] `[BE]` `internal/sync/pull.go` — keyset pagination on `seq` inside a `REPEATABLE READ` transaction, role-scoped via `sync_table_scopes` (`00-ARCHITECTURE.md` §3.4)E.md` §7.4), `has_more` + `next_cursor`.
- [ ] `[BE]` Handlers `POST /sync/push`, `GET /sync/pull`, `GET /sync/status`, `GET /sync/conflicts`.
- [ ] `[BE]` Append-only guard: `UPDATE` on a `vitals`-class table to an existing `id` from a **different** `device_id` is `rejected` with code `append_only_violation`.
- [ ] `[FE]` `web/src/db/schema.ts` — the single source of truth for Dexie stores **and** the TS row types; a `Syncable` base interface mirroring the column contract exactly.
- [ ] `[FE]` `web/src/db/write.ts` — `write()` / `writeMany()`; the outbox row carries `op_id` (UUIDv7), `table`, `row_id`, `op`, `payload`, `created_at`, `attempts`, `next_attempt_at`, `last_error`.
- [ ] `[FE]` `web/src/sync/pusher.ts` — order-preserving drain, batch cap 200, no concurrent drains (module-level mutex), resume on `online`/`visibilitychange`/interval.
- [ ] `[FE]` `web/src/sync/puller.ts` + `applier.ts` — cursor persisted in Dexie; applying a change for a row that is locally `pending` **does not** clobber it.
- [ ] `[FE]` `web/src/sync/engine.ts` — start/stop, a 30 s idle poll, jittered backoff on failure, and a `syncNow()` manual trigger.
- [ ] `[FE]` `SyncStatusIndicator` in `AppShell`; a `useSyncStatus()` hook; a dev-only debug panel showing the raw outbox.
- [ ] `[FE]` A temporary **scratch table** (`db.sync_probe` containing free-text notes) used *only* to prove replication in this sprint. It stays in the codebase as the sync engine's test fixture.
- [ ] `[TEST]` Go: push idempotency — the same batch twice yields `duplicate` on the second pass and exactly one row.
- [ ] `[TEST]` Go: field-level LWW — device A edits `phone`, device B edits `address` from the same base revision; both survive; a conflict on the *same* field resolves to the later `updated_at`.
- [ ] `[TEST]` Go: append-only rejection with the correct error code and a `sync_conflicts` row.
- [ ] `[TEST]` Go: pull pagination across the 500-row boundary, with no duplicates and no gaps.
- [ ] `[TEST]` Go: device signature required — a valid JWT without a signature is 401.
- [ ] `[TEST]` TS: `write()` is atomic — force a failure mid-transaction and assert **no** row and **no** outbox entry exist.
- [ ] `[TEST]` TS: backoff schedule is monotonic and jittered within bounds.
- [ ] `[TEST]` Playwright **two-context test**: context A and context B, different device fingerprints; A writes, B pulls and sees it; B writes, A pulls and sees it.
- [ ] `[TEST]` Playwright **offline test**: A goes offline, writes three probe rows (badge shows "3 pending"), stays correct across a reload, comes back online, drains to zero, and B receives all three in order.
- [ ] `[DOCS]` `02-SYNC-PROTOCOL.md` fully written — this sprint's main written deliverable, and a chapter of your dissertation.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00006_create_sync_changes.sql` | `sync_changes` — append-only change log with `BIGSERIAL server_rev` |
| `00007_create_sync_operations.sql` | `sync_operations` — idempotency ledger keyed by client `op_id` |
| `00008_create_sync_conflicts.sql` | `sync_conflicts` — every LWW decision with both versions, for the Sprint 9 dashboard |
| `00009_create_sync_cursors.sql` | `sync_cursors` — per-device pull position and last push time |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `POST /api/v1/sync/push` | Batch ≤ 200 ops; per-op results; requires token **and** device signature |
| `GET /api/v1/sync/pull` | `since`, `limit` ≤ 500; returns `changes`, `next_cursor`, `has_more` |
| `GET /api/v1/sync/status` | Server-side view: pending conflicts, device cursor lag |
| `GET /api/v1/sync/conflicts` | Paginated conflict log (admin/accountant) |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/dev/sync` | Development-only inspector: outbox contents, cursor value, conflict list, manual drain |

#### Demo script

1. Open **Browser A** (Chrome) and **Browser B** (Firefox) side by side, both logged in as `front_desk` on separate device fingerprints.
2. In A, open `/dev/sync` and write a probe row: "A says hello".
3. Watch A's badge go `1 pending` → `0 pending` → "synced just now". Switch to B; within the poll interval the row appears. Show in A's network tab exactly one `POST /sync/push`.
4. Do the reverse from B, and show A receiving it.
5. **Kill the server** (or go offline in A). In A, write three rows. Point at the badge: `3 pending`, and the rows are visible and correct in A's IndexedDB.
6. **Hard-reload A with the network still off.** The app loads from the service worker, the session survives, and the three rows are still there in the right order.
7. Bring A back online. Watch the badge drain `3 → 2 → 1 → 0`. Switch to B and show all three rows.
8. **Idempotency:** in A's DevTools console, re-send the last `POST /sync/push` body (a saved cURL). Show the response returns `duplicate` for every op and B still has exactly three rows, not six.
9. **LWW:** with A offline, edit probe row 1's `title` to "A edit". In B (online), edit the same row's `title` to "B edit" **later**. A comes online; show the surviving value is B's, and a `sync_conflicts` row recording A's losing value.
10. Open `/dev/sync` and show the conflict list — the evidence for your dissertation chapter on conflict resolution.

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | Conflict resolution rules; op-result mapping; batch validation rejections |
| Unit (TS) | Atomic write; backoff monotonicity; cursor advance only after a committed apply |
| Integration (Go) | Push replay → `duplicate` ×N and one row; pull paging at boundaries; append-only rejection |
| Integration (Go) | 1000-op batch rejected at the cap with a clear error, not a timeout |
| E2E (Playwright) | Two-context convergence; offline-queue-then-drain; reload while offline |
| Manual offline | The full 10-step demo above, with the server stopped for step 5–7 |
| Performance | Time a 500-row pull and a 200-op push against the deployed API; record cold vs warm |

#### Risks / watch-outs

- **This sprint will overrun if you let it.** Timebox the *engine* to two weeks and move any remaining polish (the debug panel, cursor-lag metrics) to Sprint 9. A working push/pull is the bar; dashboards are not.
- **Concurrent pulls double-applying.** Use one in-flight pull per tab, and an IndexedDB lock or `navigator.locks` across tabs so two tabs do not fight over the cursor.
- **Outbox ordering.** Do not parallelise the drain in this sprint. Sequential, ordered, boring, correct.
- **Clock skew between devices.** LWW compares `updated_at` from different machines. Cap acceptance: record `device_recorded_at` and treat edits older than the last accepted edit as losing, but log loudly if skew exceeds 5 minutes (`00-ARCHITECTURE.md` §7.3).
- **Dexie transactions spanning non-Dexie code.** `await fetch()` inside a Dexie transaction silently commits it. Keep network calls strictly outside transactions.
- **Growing `sync_changes` forever.** Full-table pulls will eventually be slow. Note a snapshot/compaction strategy in `02-SYNC-PROTOCOL.md` as future work; do not build it.
- **Temptation to "just add patients while I'm here".** No. Sprint 3 is patients. Sprint 2 is the engine.

#### Definition of Done for this sprint

- [ ] `02-SYNC-PROTOCOL.md` specifies the wire format precisely enough that a second developer could implement a client from it.
- [ ] A replayed push of any batch is provably a no-op (test, not belief).
- [ ] The app is fully usable (write + read) with the network off, and converges when it returns.
- [ ] Conflict handling is *logged*, not silent, and the log is visible in the UI.
- [ ] No feature screen in the codebase calls `fetch` for domain data — the sync engine is the only network path for clinical rows.
- [ ] The two-browser Playwright test runs in CI.

---

### Sprint 3 — Patient registration, individual & family

**Sprint goal.** Register a patient — individually or as part of a family whose members share one family account — entirely offline, with generated patient numbers, duplicate detection, and search that survives a reload.

**Duration.** 2 weeks.
**Prerequisites.** Sprint 2 complete (sync engine live and proven).

#### Deliverables

- [ ] `households` table with `family_number`, family name, head-of-household reference, and the full sync column contract.
- [ ] `patients` table with the full demographic set: `first_name`, `middle_name` (nullable), `surname`, `age`, `date_of_birth`, `registration_date`, `phone`, `tribe`, `occupation`, `gender`, `marital_status`, `address`, plus `patient_number`, `household_id` (nullable), `is_head_of_household`.
- [ ] Registration form with **two modes** (Individual / Family), a shared Zod schema used by both the form and the sync payload, and cross-field validation (`age` XOR/OR `date_of_birth` — decide, document, enforce; at least one required, DOB wins when both are present).
- [ ] Family mode: one household form + a repeatable member sub-form; all members get `household_id` set atomically in one Dexie transaction.
- [ ] Patient number generation: `<CODE>/PAT/<yyyy>/<zero-padded sequence>` where `<CODE>` is `facilities.code` — allocated offline from a **local counter row** in Dexie, then reconciled on sync; collisions resolved by the server with a suffix and the client notified.
- [ ] Family number generation: `<CODE>/FAM/<yyyy>/<zero-padded sequence>`, same mechanism and the same reconciliation path.
- [ ] Duplicate detection at save time: block nothing, warn clearly — exact match on `(surname, first_name, date_of_birth)` or `phone` triggers a "possible duplicate" panel listing the candidates with a link, plus a required "register anyway" confirmation with a reason.
- [ ] Patient search: debounced, matches name fragments, phone, patient number, and family number; results in < 150 ms from Dexie over 5 000 seeded rows.
- [ ] Patient profile screen: demographics, household membership with siblings, visit history placeholder, current balance placeholder (wired to real data in Sprint 6).
- [ ] Household screen: the family roster, one aggregate account summary, and a per-member charge roll-up that will be driven by invoices in Sprint 6.
- [ ] `patients` and `households` registered in the sync table registry as **mutable / LWW-per-field**.
- [ ] `01-DATA-MODEL.md` sections for `households` and `patients` written with every index and the reasoning for each.

#### Tasks

- [ ] `[DB]` Migration `00010_create_households.sql` — sync columns, `family_number TEXT NOT NULL`, `family_name`, `head_patient_id UUID NULL`, unique `family_number`.
- [ ] `[DB]` Migration `00011_create_patients.sql` — sync columns, all demographics, `patient_number TEXT NOT NULL`, `household_id UUID NULL REFERENCES households(id)`, unique `patient_number`, CHECK on `gender` and `marital_status`.
- [ ] `[DB]` Migration `00012_add_patient_indexes.sql` — trigram index on `lower(surname || ' ' || first_name)`, index on `(phone)`, on `(household_id)`, and on `(registration_date)`.
- [ ] `[DB]` Migration `00013_create_number_sequences.sql` — `number_sequences(scope, year, last_value, updated_at)` for server-side reconciliationion of `patient_number` / `family_number`.
- [ ] `[BE]` `internal/domain/patient.go` — structs, gender/marital enums, `PatientNumber` value type with `Parse`/`String`.
- [ ] `[BE]` `internal/repo/patient_repo.go` — `Create`, `Update`, `FindByID`, `FindByNumber`, `Search` (paged), `ListByHousehold`, `FindPossibleDuplicates`.
- [ ] `[BE]` `internal/repo/household_repo.go` — `Create`, `AddMember`, `FindByID`, `FindByNumber`, `RollupCharges` (a `SUM` over charges − allocations, no stored balance).
- [ ] `[BE]` `internal/service/patient.go` — demographics validation, number allocation, duplicate policy, household linking rules (a patient belongs to at most one household; members are not soft-deleted).
- [ ] `[BE]` Handlers + DTOs: `patient_handler.go`, `household_handler.go`, all wrapped in the standard envelope.
- [ ] `[BE]` Server-side number reconciliation in `internal/service/numbering.go` — on conflict, allocate the next server value, append a `sync_changes` correction row, and let the sync response carry the reassignment back.
- [ ] `[FE]` `web/src/features/patients/schema.ts` — Zod schema (shared), with `superRefine` for the DOB/age rule and phone format.
- [ ] `[FE]` `web/src/db/patients.ts` + `households.ts` — typed Dexie tables, `.stores()` entries, and a v2 upgrade in `database.ts`.
- [ ] `[FE]` `web/src/features/patients/usePatientNumber.ts` — local counter allocation with hourly server reconciliation.
- [ ] `[FE]` `routes/patients.new.tsx` — the two-mode registration form; **writes through `db.write()`**, never `fetch`.
- [ ] `[FE]` `features/patients/FamilyMemberList.tsx` — add/remove members, inline validation, head-of-household selection.
- [ ] `[FE]` `features/patients/DuplicateWarning.tsx` — candidate list, "register anyway" with a mandatory reason.
- [ ] `[FE]` `routes/patients.index.tsx` — search + paginated list with a "created offline" badge for `sync_state='pending'` rows.
- [ ] `[FE]` `routes/patients.$patientId.tsx` — profile, edit in place, household panel.
- [ ] `[FE]` `routes/households.$householdId.tsx` — family roster + aggregate account panel.
- [ ] `[FE]` Optimistic UX: the new patient appears in the list **before** the outbox drains, with no spinner blocking the form.
- [ ] `[TEST]` Go: number allocation under two concurrent devices → both get distinct numbers, one corrected by the server.
- [ ] `[TEST]` Go: duplicate detection returns the expected candidates for tricky cases (same phone, different surname; same name, different DOB).
- [ ] `[TEST]` Go: household linking rejects a soft-deleted patient, and a patient cannot belong to two households.
- [ ] `[TEST]` TS: Zod schema — DOB/age rules, required fields, phone normalisation.
- [ ] `[TEST]` TS: family registration writes N patients + 1 household + N+1 outbox rows in one atomic transaction.
- [ ] `[TEST]` Playwright: register a family of three **offline**, reload, search for each member, open the household, then come online and see all four rows on the server.
- [ ] `[TEST]` Performance: seed 5 000 patients and assert search p95 < 150 ms in-browser.
- [ ] `[DOCS]` `01-DATA-MODEL.md` + `07-API-CONTRACT.md` + a short `docs/` note on the numbering scheme and its collision policy.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00010_create_households.sql` | `households` — family number, family name, head patient, sync columns |
| `00011_create_patients.sql` | `patients` — full demographics, patient number, household FK, sync columns |
| `00012_add_patient_indexes.sql` | Trigram name index, phone index, household index, registration-date index |
| `00013_create_number_sequences.sql` | `number_sequences` — server-side reconciliation for human-readable numbers |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `POST /api/v1/patients` | Direct create (used by the API and by tests; the SPA normally writes locally and syncs) |
| `GET /api/v1/patients` | Paged search: `q`, `household_id`, `limit`, `cursor` |
| `GET /api/v1/patients/:id` | Single patient with household summary |
| `PATCH /api/v1/patients/:id` | LWW-per-field update, audited |
| `GET /api/v1/patients/duplicates` | Candidate duplicates for a proposed demographic tuple |
| `POST /api/v1/households` | Create a family with optional members in one transaction |
| `GET /api/v1/households/:id` | Household with members and derived charge roll-up |
| `POST /api/v1/households/:id/members` | Link an existing patient to a household |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/patients` | Search + list, with pending-sync badges |
| `/patients/new` | Individual / Family registration (two modes, one schema) |
| `/patients/:patientId` | Profile, edit, household panel |
| `/households/:householdId` | Family roster + single family account summary |

#### Demo script

1. Go **offline** in DevTools before touching anything, and say so out loud.
2. `/patients/new` → **Family** mode. Enter household "Okonkwo". Add three members: father (head), mother, child. Deliberately give the child only an `age`, no DOB, and show the form accepting it; then try to give **both** a nonsense combination and show the field error.
3. Save. Point at the three patient numbers and the family number generated locally, and at the sync badge reading `4 pending`.
4. `/patients` → search "Oko" → all three appear instantly. Search the child's patient number → found.
5. Reload the browser **with the network still off**. Show the records still present.
6. Try to register "Okonkwo / Chidi / same phone" again → the duplicate warning appears with the candidate; register anyway with the reason "different person, same household phone".
7. Go **online**. Watch the badge drain to `0`.
8. Open a second browser logged in as `nurse`; search "Oko" and show all four records arrived by sync.
9. Open `/households/:id` and show the family roster and the single family account panel — say the sentence: *"every family member's bills belong to this one account."*
10. Show `SELECT patient_number, household_id FROM patients` in Neon, confirming the server received exactly what the device generated.

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | Patient number parse/format; gender & marital enums; duplicate scoring |
| Unit (TS) | Zod schema; local number allocation increments and never reuses |
| Integration (Go) | Create-with-members transaction rollback on a bad member; household soft-delete guard |
| Integration (Go) | Number collision: two devices allocate the same number, the server corrects one |
| E2E (Playwright) | **Flow (a) part 1**: register → search → reload offline → reconnect → visible on a second context |
| Manual offline | Family registration, search, duplicate warning, and reload all with the network off |
| Performance | 5 000-row seed; search latency recorded |

#### Risks / watch-outs

- **Human-readable number collisions** are guaranteed once two devices register patients offline. This is designed for, not hoped away: the server reconciles and the client must handle a reassigned `patient_number` gracefully (show a small "number updated" toast, never break a link).
- **DOB vs age drift.** A stored `age` is wrong the day after it is recorded. Store DOB when known; when only age is known, store the age **and** an `age_recorded_at` date, and display "age as at <date>". Document this in the data model — an examiner will ask.
- **Free-text tribe/occupation** produces mess. Use a datalist with a suggestion list, stored as text.
- **PII in logs.** Never log a full name together with a phone number. Log IDs.
- **`useLiveQuery` re-render storms** on the search screen while the outbox drains. Memoise the query function, keep the dependency array narrow.
- **Scope creep: medical history, allergies, next-of-kin.** Not in this sprint. Write them on the backlog and leave them there.

#### Definition of Done for this sprint

- [ ] A family of five can be registered, searched, and reopened with the network off, from a cold browser start.
- [ ] Every patient row created on a device is traceable to the device and user that created it (`created_by`, and the originating `sync_changes` row).
- [ ] Patient number collisions are covered by an automated test, not just a comment.
- [ ] `01-DATA-MODEL.md` documents every index and says why it exists.
- [ ] No screen fetches patients over HTTP.

---

### Sprint 4 — Queue, vitals & triage

**Sprint goal.** Capture vitals at the nurse's station and drive every patient through an explicit, validated workflow queue with clear handoffs between front desk, nurse and doctor.

**Duration.** 2 weeks.
**Prerequisites.** Sprint 3 complete.

#### Deliverables

- [ ] `vitals` table — **append-only clinical facts**: `systolic`, `diastolic`, `temperature_c`, `pulse_rate`, `respiration_rate`, `height_cm`, `weight_kg`, computed `bmi` (derived on write, stored as a fact), `spo2` (optional but free to add), `recorded_at`, `recorded_by`, full sync columns.
- [ ] `encounters` table — the visit record: `visit_number`, `patient_id`, `workflow_stage`, `queued_at`, `closed_at`, `attending_doctor_id`, sync columns.
- [ ] `encounter_events` table — an append-only transition log (`from_stage`, `to_stage`, `by_user`, `at`, `reason`), which doubles as the triage timeline and the audit trail for stage changes.
- [ ] The workflow state machine in one place, shared conceptually and duplicated deliberately: `internal/domain/workflow.go` for the server, `web/src/features/queue/state-machine.ts` for the client, with a **shared table of transitions** and a test that asserts both agree.
- [ ] Transitions are **idempotent**: applying the same transition twice is a no-op, not an error — critical when a device retries a queued operation (`00-ARCHITECTURE.md` §7.5).
- [ ] Vitals capture screen with sensible clinical ranges: BP sanity checks, temperature range warnings, BMI computed live, "repeat vitals" support (a second row, never an overwrite).
- [ ] Queue board: columns per stage (`registered`, `vitals_done`, `awaiting_doctor`, `awaiting_payment`, `awaiting_lab`, `lab_done`, `awaiting_pharmacy`, `pharmacy_done`, `admitted`, `closed`), with per-column counts, waiting time since `queued_at`, and a "my patients" filter.
- [ ] Handoff actions per role: front desk → `registered`; nurse → vitals → `vitals_done`; nurse → `awaiting_doctor`; doctor pulls into `in_consultation`.
- [ ] `QueueCard` showing name, patient number, age/sex, latest vitals summary, and stage age — the single most-used component in the app, so `03-UI-PATTERNS.md` gets its pattern written here.
- [ ] Vitals history on the patient profile (append-only list, newest first).
- [ ] `vitals` registered as **append-only** in the sync table registry; `encounters` and `encounter_events` registered appropriately (`encounters` mutable/LWW for `workflow_stage` with a guard; `encounter_events` append-only).
- [ ] Guard: `workflow_stage` changes are rejected by the server if the transition is not in the legal table — the client validates for UX, the server for truth (`00-ARCHITECTURE.md` §6).

#### Tasks

- [ ] `[DB]` Migration `00014_create_vitals.sql` — append-only table, index `(patient_id, recorded_at DESC)` and `(encounter_id)`.
- [ ] `[DB]` Migration `00015_create_encounters.sql` — `visit_number`, `workflow_stage` with a CHECK against the stage enum, `queued_at`, index `(workflow_stage, priority DESC, queued_at)` — the queue board's index.
- [ ] `[DB]` Migration `00016_create_encounter_events.sql` — append-only transition log, index `(encounter_id, at)`.
- [ ] `[DB]` Migration `00017_seed_workflow_stages.sql` — a reference table `workflow_stages(code, label, role_owner, sort_order)` so the board's columns are data, not hard-coded strings.
- [ ] `[BE]` `internal/domain/workflow.go` — the transition table, `CanTransition(from, to, role)`, `Apply` (idempotent).
- [ ] `[BE]` `internal/repo/encounter_repo.go` — `Create`, `FindByID`, `ListByStage`, `QueueSnapshot` (one grouped query for the board), `Advance`.
- [ ] `[BE]` `internal/repo/vitals_repo.go` — `Create` (insert-only), `ListByPatient`, `LatestForEncounter`.
- [ ] `[BE]` `internal/service/triage.go` — vitals validation, BMI, abnormal-flag computation, stage transitions, and the rule that only the owning role may advance a stage.
- [ ] `[BE]` Handlers: `encounter_handler.go`, `vitals_handler.go`, `queue_handler.go`.
- [ ] `[FE]` `web/src/features/queue/state-machine.ts` — mirrors `workflow.go`; a Vitest test reads `workflow.go`'s generated JSON fixture and asserts equality (single source of truth, enforced by test).
- [ ] `[FE]` `routes/queue.tsx` — the board, responsive (columns scroll horizontally on a tablet), with per-stage counts and a stage filter.
- [ ] `[FE]` `routes/encounters.$encounterId.vitals.tsx` — vitals form, live BMI, range warnings, "record again" flow.
- [ ] `[FE]` `routes/nurse.queue.tsx` — the nurse's worklist: patients awaiting vitals, with quick vitals entry.
- [ ] `[FE]` `components/ui/QueueCard.tsx` + `VitalsBadge.tsx` (colour-coded abnormal values) + `StageChip.tsx`.
- [ ] `[FE]` Waiting-time ticker (updates every 30 s from `queued_at`, no server involvement).
- [ ] `[FE]` Front-desk intake: from a registered patient, "start visit" creates the encounter with `visit_number` allocated locally and stage `registered`.
- [ ] `[TEST]` Go: every legal transition succeeds and every illegal one is rejected with `invalid_transition`.
- [ ] `[TEST]` Go: idempotent transition — the same transition applied twice yields one `encounter_events` row.
- [ ] `[TEST]` Go: append-only `vitals` — an update attempt to an existing vitals `id` from another device is rejected.
- [ ] `[TEST]` Go: BMI computation and abnormal-flag thresholds across boundary values.
- [ ] `[TEST]` TS: state machine parity with `workflow.go`; vitals Zod ranges.
- [ ] `[TEST]` E2E: **Flow (a)** — register → start visit → capture vitals → hand to doctor, all offline, then reconnect and verify on a second context.
- [ ] `[TEST]` Manual offline: two devices record vitals for the same patient independently → both rows survive, neither overwrites the other.
- [ ] `[DOCS]` `03-UI-PATTERNS.md` — the `QueueCard` pattern, status colour tokens, and the waiting-time convention.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00014_create_vitals.sql` | `vitals` — append-only measurements with derived `bmi`, sync columns |
| `00015_create_encounters.sql` | `encounters` — visit number, workflow stage CHECK, queue index |
| `00016_create_encounter_events.sql` | `encounter_events` — append-only transition log / triage timeline |
| `00017_seed_workflow_stages.sql` | `workflow_stages` reference data: code, label, owning role, sort order |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `POST /api/v1/encounters` | Start a visit; allocates `visit_number`; stage `registered` |
| `GET /api/v1/encounters/:id` | Encounter with patient, latest vitals, timeline |
| `GET /api/v1/encounters` | Filter by `stage`, `patient_id`, `from`, `to` |
| `POST /api/v1/encounters/:id/transition` | Idempotent stage advance; `{to, reason?}`; role-checked |
| `GET /api/v1/queue` | One query returning the whole board snapshot, grouped by stage |
| `POST /api/v1/vitals` | Append a vitals fact (never an update) |
| `GET /api/v1/patients/:id/vitals` | Vitals history, newest first |
| `GET /api/v1/encounters/:id/vitals` | Latest vitals for the queue card |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/queue` | Full triage board across all stages |
| `/nurse/queue` | Nurse worklist: awaiting vitals |
| `/encounters/:encounterId/vitals` | Vitals capture with live BMI and range flags |

#### Demo script

1. As `front_desk`, register a patient (or reuse one), then click **Start visit**. Show the `visit_number` and the card appearing in the `registered` column of `/queue`.
2. Switch to `nurse`. Open `/nurse/queue` — the patient is there.
3. Capture vitals: BP 138/88, temp 37.8, pulse 92, resp 18, height 168, weight 71. Point at the live BMI (25.2) and the amber abnormal badge on the temperature.
4. Click **Save and send to doctor**. Show the card move to `awaiting_doctor` with the vitals summary on the card.
5. **Go offline.** Capture a *repeat* vitals set for the same patient (BP 132/84). Show both rows in the history — the second appended, the first untouched.
6. Try an illegal move: as `nurse`, attempt to drag the card into `in_consultation`. Show it refused, with the reason.
7. Reload offline → the board still renders with correct column counts and waiting times.
8. Come online → badge drains. Switch to `doctor` in a second browser → the patient is in `awaiting_doctor` with the latest vitals.
9. Doctor pulls the patient: stage becomes `in_consultation`, and the timeline shows every transition with who did it and when.
10. Open the patient profile → vitals history shows both readings, newest first.

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | Transition table completeness; role ownership; idempotency |
| Unit (TS) | BMI, range flags, waiting-time formatting, state-machine parity fixture |
| Integration (Go) | Append-only vitals under concurrent writes from two devices |
| Integration (Go) | Queue snapshot query returns correct counts with 200 mixed encounters |
| E2E (Playwright) | **Flow (a)** complete, offline, with reconnect verification |
| Manual offline | Board reload offline; repeat vitals offline; illegal transition rejected offline |
| Performance | Queue board renders 200 encounters < 200 ms |

#### Risks / watch-outs

- **Stage drift between client and server.** The parity test exists so the two state machines cannot diverge silently. Keep it in CI.
- **The queue board is a performance trap.** Do not query per column. One query, one `useLiveQuery`, group in memory.
- **Append-only vitals will surprise you.** A "fix the typo in the temperature" request is a *new* vitals row with a note, not an edit. Decide this now, put it in `01-DATA-MODEL.md`, and tell the examiner it is deliberate.
- **`in_consultation` concurrency.** Two doctors can pull the same patient. The server's transition guard makes whichever arrives second a no-op, and the client shows "already being seen by Dr X".
- **Stage enum churn.** Adding a stage later means a CHECK-constraint migration. Decide the full stage list now (it is in `00-ARCHITECTURE.md` §7.5) and resist edits.

#### Definition of Done for this sprint

- [ ] Every stage transition in the workflow is reachable and tested, including the lab branch and the admission branch (stubbed behaviour is fine; the transitions must exist).
- [ ] The queue board is fast with 200 encounters and correct with the network off.
- [ ] Illegal transitions are rejected server-side with a stable error code.
- [ ] Vitals are provably append-only (a test asserts an update is rejected).
- [ ] The client and server state machines are proven identical by a test.

---

### Sprint 5 — Doctor consultation, diagnosis & lab orders

**Sprint goal.** Let a doctor record a diagnosis against an ICD-10 code, order lab tests, and later review the lab results — with the lab able to work and enter results entirely offline.

**Duration.** 2 weeks.
**Prerequisites.** Sprint 4 complete.

**A deliberate sequencing note on payment.** The clinical order `diagnosis → lab order → **payment at cashier** → lab result` requires money to move. Full billing (tariffs, invoices, allocations, receipts) is Sprint 6. Therefore **this sprint introduces a provisional payment gate**: `lab_orders.payment_status` with a placeholder price constant and a cashier "mark lab paid" action that writes a minimal `payments` row. Sprint 6 replaces that constant with a tariff lookup and the real ledger, and the `lab_orders.payment_status` gate stays exactly where it is. This is a *deliberate, documented* migration path, not a shortcut that gets forgotten.

#### Deliverables

- [ ] `diagnoses` table — `encounter_id`, `icd10_code`, `icd10_label`, `notes`, `diagnosed_by`, `diagnosed_at`, `is_primary`, sync columns (append-only: a revised diagnosis is a **new** diagnosis row with `supersedes_id`).
- [ ] `icd10_codes` reference table with a seeded subset (~500 common PHC codes) plus a search endpoint; the seed is a small SQL file, not a 70 000-row dump.
- [ ] `lab_orders` table — `encounter_id`, `order_number`, `status` (`ordered` | `awaiting_payment` | `paid` | `sample_collected` | `resulted` | `reviewed` | `cancelled`), `ordered_by`, `ordered_at`, `priority`, `payment_status`, sync columns.
- [ ] `lab_order_items` table — one row per test: `lab_order_id`, `test_code`, `test_name`, `specimen`, `price_minor`, `status`.
- [ ] `lab_results` table — **append-only**: `lab_order_item_id`, `result_value`, `unit`, `reference_range`, `interpretation` (`normal` | `abnormal` | `critical`), `result_text`, `entered_by`, `entered_at`, `verified_by`, sync columns.
- [ ] `lab_tests` reference table: code, name, specimen type, price placeholder, so the order screen is data-driven.
- [ ] Consultation screen: patient header with vitals + history, diagnosis entry with ICD-10 autocomplete, lab order builder, prescription placeholder (Sprint 6), referral placeholder (Sprint 8).
- [ ] Lab worklist for `lab_scientist`: ordered → awaiting payment → paid → sample collected → resulted, with per-stage counts and urgent-first ordering.
- [ ] Result entry form with unit, reference range, interpretation flagging, and a "critical result" visual alert.
- [ ] Doctor review: pending-results inbox, result acknowledgement (`reviewed`), and a doctor's note attached to the review.
- [ ] Payment gate on the clinical path: a lab order cannot be marked `sample_collected` unless `payment_status = 'paid'`. Enforced in the **service layer** (`00-ARCHITECTURE.md` §6) and mirrored in the UI.
- [ ] Result-linking: results appear on the encounter timeline and the patient's record; a critical result raises a badge on the doctor's inbox.

#### Tasks

- [ ] `[DB]` Migration `00018_create_icd10_codes.sql` — reference table + seed (`api/db/seed/0002_icd10.sql`).
- [ ] `[DB]` Migration `00019_create_diagnoses.sql` — append-only, `supersedes_id UUID NULL`, index `(encounter_id, diagnosed_at)`.
- [ ] `[DB]` Migration `00020_create_lab_tests.sql` — reference table: `code`, `name`, `specimen`, `price_minor`, `is_active`.
- [ ] `[DB]` Migration `00021_create_lab_orders.sql` — order header, `order_number`, status CHECK, index `(status, ordered_at)ed_at)`.
- [ ] `[DB]` Migration `00022_create_lab_order_items.sql` — per-test lines with price snapshots (prices are copied at order time, never re-read).
- [ ] `[DB]` Migration `00023_create_lab_results.sql` — append-only results, index `(lab_order_item_id, entered_at DESC)`.
- [ ] `[BE]` `internal/repo/diagnosis_repo.go`, `lab_repo.go`, `reference_repo.go` (ICD-10 search, lab tests).
- [ ] `[BE]` `internal/service/diagnosis.go` — diagnosis recording, supersede rules, ICD-10 validation (code must exist).
- [ ] `[BE]` `internal/service/lab.go` — order creation, status machine, payment gate, result entry, critical-result flagging, review.
- [ ] `[BE]` `internal/service/workflow.go` — extend the state machine with the lab branch: `awaiting_lab → lab_done → awaiting_doctor`.
- [ ] `[BE]` Handlers + DTOs: `diagnosis_handler.go`, `lab_handler.go`, `icd10_handler.go`.
- [ ] `[BE]` Provisional cashier action `POST /lab-orders/:id/mark-paid` writing a minimal `payments` row tagged `source='provisional'`, so Sprint 6 can find and supersede it.
- [ ] `[FE]` `routes/encounters.$encounterId.consult.tsx` — the consultation screen (the most complex screen so far; build it in pieces: header, diagnosis, orders).
- [ ] `[FE]` `features/consultation/Icd10Combobox.tsx` — keyboard-first autocomplete over the local `icd10_codes` Dexie table (works offline).
- [ ] `[FE]` `features/consultation/LabOrderBuilder.tsx` — multi-test selection with running total, priority, clinical notes.
- [ ] `[FE]` `routes/lab.index.tsx` — lab worklist with stage tabs and counts.
- [ ] `[FE]` `routes/lab.$orderId.tsx` — order detail + result entry, per-item forms, critical flag with a confirmation.
- [ ] `[FE]` `routes/doctor.inbox.tsx` — awaiting-results inbox with critical badges.
- [ ] `[FE]` `routes/cashier.lab.tsx` — provisional lab payment screen (replaced in Sprint 6; keep it small and clearly marked).
- [ ] `[FE]` Encounter timeline component: merged, chronologically sorted events across tables (`encounter_events`, `vitals`, `diagnoses`, `lab_*`) — one `useLiveQuery`, sorted client-side.
- [ ] `[TEST]` Go: lab status machine — legal and illegal transitions, including the payment gate.
- [ ] `[TEST]` Go: append-only results — a corrected result is a new row, the original remains.
- [ ] `[TEST]` Go: ICD-10 search returns the expected top hits for abbreviations ("mal" → malaria).
- [ ] `[TEST]` TS: ICD-10 combobox keyboard navigation; order builder total arithmetic in minor units only.
- [ ] `[TEST]` E2E: **Flow (b)** part 1 — diagnosis → lab order → provisional payment → sample collected → result → doctor review, offline until the payment step, then reconnect.
- [ ] `[TEST]` Manual offline: lab scientist enters three results with the network off; all three sync without loss, in order.
- [ ] `[DOCS]` `01-DATA-MODEL.md` for the six new tables; `07-API-CONTRACT.md` for the new endpoints; a note in `PROJECT-TRACKER.md` recording the provisional payment gate and its Sprint 6 removal.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00018_create_icd10_codes.sql` | `icd10_codes` reference table + ~500-code seed |
| `00019_create_diagnoses.sql` | `diagnoses` — append-only, `supersedes_id` for revisions |
| `00020_create_lab_tests.sql` | `lab_tests` — catalogue with specimen type and price |
| `00021_create_lab_orders.sql` | `lab_orders` — order number, status machine, payment gate |
| `00022_create_lab_order_items.sql` | `lab_order_items` — per-test lines with price snapshots |
| `00023_create_lab_results.sql` | `lab_results` — append-only values, units, ranges, interpretation |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `GET /api/v1/icd10?q=` | Code/label search over the seeded subset |
| `POST /api/v1/encounters/:id/diagnoses` | Append a diagnosis (or a superseding diagnosis) |
| `GET /api/v1/encounters/:id/diagnoses` | Diagnosis history |
| `POST /api/v1/lab-orders` | Create an order with items from the catalogue |
| `GET /api/v1/lab-orders` | Worklist: `status`, `priority`, `from`, `to` |
| `GET /api/v1/lab-orders/:id` | Order with items and results |
| `POST /api/v1/lab-orders/:id/mark-paid` | Provisional cashier gate (superseded in Sprint 6) |
| `POST /api/v1/lab-orders/:id/collect` | Sample collected; requires `paid` |
| `POST /api/v1/lab-order-items/:id/results` | Append a result |
| `POST /api/v1/lab-orders/:id/review` | Doctor acknowledgement |
| `GET /api/v1/lab-tests` | Test catalogue |
| `GET /api/v1/encounters/:id/timeline` | Merged chronological event list |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/encounters/:encounterId/consult` | Consultation: diagnosis + lab orders |
| `/lab` | Lab worklist by stage |
| `/lab/:orderId` | Result entry per item |
| `/doctor/inbox` | Awaiting results, critical-result badges |
| `/cashier/lab` | Provisional lab payment (Sprint 6 successor: `/cashier/invoices`) |

#### Demo script

1. As `doctor`, open the patient in `awaiting_doctor` and start the consultation. Point at the vitals summary and the visit timeline.
2. Search ICD-10 for "malaria", select **B50.9 Plasmodium falciparum malaria, unspecified**, add a note, save. Show it on the timeline.
3. Build a lab order: Malaria RDT, Full Blood Count, priority **urgent**. Point at the total in minor units.
4. Click **Send to cashier**. Show the encounter move to `awaiting_payment(lab)`, and the lab worklist show the order under "awaiting payment".
5. **Go offline.** As `lab_scientist`, try to mark the sample collected → refused with "payment required". Say: *"the payment gate is enforced in the service layer, not just the UI."*
6. Back online as `cashier`, mark the order paid on `/cashier/lab`. Show the amount and the provisional payment row.
7. As `lab_scientist` (offline), collect the sample and enter results: RDT positive, Hb 9.2 g/dL (abnormal, below range). Mark the Hb **critical** and show the confirmation.
8. Come online. As `doctor`, open `/doctor/inbox` — the order appears with a red critical badge.
9. Open the result, review it, add "start artemether-lumefantrine; repeat Hb in 48 h", and acknowledge. Show the encounter return to `awaiting_doctor`.
10. Open the patient's record and show the complete timeline: registration → vitals → diagnosis → order → payment → collection → result → review.

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | Lab status machine; payment-gate enforcement; critical-result thresholds |
| Unit (Go) | ICD-10 search ranking; diagnosis supersede chain |
| Unit (TS) | Order total arithmetic; combobox keyboard handling |
| Integration (Go) | Result append under replay — a replayed push does not duplicate a result |
| Integration (Go) | `collect` without payment returns 409 `payment_required` with a stable code |
| E2E (Playwright) | **Flow (b)** part 1: diagnosis → order → pay → collect → result → review |
| Manual offline | Lab result entry for three items offline; sync in order; no loss |
| Data | ICD-10 seed covers the 20 conditions the demo will use (asserted by a test) |

#### Risks / watch-outs

- **ICD-10 scope explosion.** 500 codes is enough. If a code is missing during the demo you have a "free text + search" fallback; do not import the full WHO dataset.
- **The provisional payment gate is debt.** It is *planned* debt with a named repayment sprint (Sprint 6). Put a `TODO(sprint-6)` comment on the constant and a line in `PROJECT-TRACKER.md`, or it will survive to the viva.
- **Result corrections.** Insist on append-only. A doctor who wants a different value gets a new row; the UI shows the latest and the history.
- **The consultation screen is big.** Build it top-down in three commits (header → diagnosis → orders) so a bad day never loses the whole screen.
- **Critical results must be loud.** A missed critical result is the most serious usability failure in this sprint. Spend real time on the visual and the inbox badge.

#### Definition of Done for this sprint

- [ ] A complete lab cycle runs offline except for the payment step, which requires connectivity by design.
- [ ] Results are append-only and the history is visible in the UI.
- [ ] The payment gate is enforced server-side with a test that proves the UI cannot bypass it.
- [ ] The encounter timeline merges at least five source tables correctly and in order.
- [ ] The provisional payment gate is documented as Sprint 6 debt in both `PROJECT-TRACKER.md` and a code comment.

---

### Sprint 6 — Billing, cashier & pharmacy dispensing

**Sprint goal.** Turn the whole clinical path into a money path: tariffs, invoices and charge lines, payments with explicit allocation, derived balances, 58/80 mm receipts, prescriptions, and pharmacist dispensing gated on payment.

**Duration.** 2 weeks. **This is the second-hardest sprint.** Money correctness is unforgiving.
**Prerequisites.** Sprint 5 complete.

#### Deliverables

- [ ] `tariffs` table — `code`, `description`, `category` (`consultation` | `lab` | `drug` | `procedure` | `bed_day`), `price_minor BIGINT`, `effective_from`, `effective_to`, `is_active`, sync columns.
- [ ] `invoices` table — `invoice_number`, `patient_id`, `household_id` (**nullable — set for family accounts**), `encounter_id`, `admission_id` (nullable, Sprint 7), `status` (`open` | `part_paid` | `paid` | `void`), `issued_at`, sync columns.
- [ ] `invoice_lines` table — the **charges**: `invoice_id`, `tariff_code` snapshot, `description`, `quantity`, `unit_price_minor`, `line_total_minor BIGINT`, `category`, `source_table`, `source_id`, sync columns.
- [ ] `payments` table — `payment_number`, `patient_id`, `household_id`, `amount_minor BIGINT`, `method` (`cash` | `transfer` | `pos` | `waiver`), `reference`, `received_by`, `received_at`, `op_id TEXT UNIQUE NOT NULL`, `reversed_by_payment_id`, sync columns.
- [ ] `payment_allocations` table — the **allocations**: `payment_id`, `invoice_line_id`, `amount_minor BIGINT`, sync columns. Every allocation must be traceable to one payment and one charge.
- [ ] **Balances are derived, always:** `SUM(invoice_lines.line_total_minor) − SUM(payment_allocations.amount_minor)`, per patient and per household. There is no stored balance column anywhere (`00-ARCHITECTURE.md` §7.2) — assert this in a test that greps the migrations for a forbidden column name.
- [ ] Idempotency on money: `payments.op_id` is a client-generated operation ID with a UNIQUE constraint; a replayed push returns the original payment rather than creating a second one.
- [ ] Prescriptions: `prescriptions` (`prescription_number`, `encounter_id`, `prescribed_by`, `status`) and `prescription_items` (`drug_code`, `drug_name`, `dose`, `frequency`, `duration`, `quantity`, `unit_price_minor`, `line_total_minor`).
- [ ] `drugs` reference table: code, name, form, strength, unit price, `is_active`.
- [ ] `dispenses` table — **append-only fact**: `prescription_item_id`, `quantity_dispensed`, `batch_note` (free text only — no stock maths), `dispensed_by`, `dispensed_at`, sync columns.
- [ ] Pharmacist queue gated on payment: a prescription appears in the queue as `awaiting_payment`, becomes `ready` when the drug invoice is fully paid (derived, not a flag), and `dispensed` after the dispense event.
- [ ] Receipt printing: `web/src/styles/print.css` with `@page { size: 58mm auto; margin: 0 }` and an 80 mm variant, a `ReceiptSlip` component, `window.print()` from the cashier screen, and a browser print-preview screenshot in `docs/evidence/`.
- [ ] Void and reversal: a payment can be **reversed** by a compensating payment (negative amount, `reversed_by_payment_id`), never deleted. Reason is mandatory and audited.
- [ ] Tariff admin screen (`/admin/tariffs`) — accountant/admin CRUD with `effective_from`, never destructive edits to a price already used on an invoice.
- [ ] Money formatting utilities: `lib/money.ts` with `formatMinor(bigint, currency)` and **no floating-point arithmetic anywhere**; a lint rule or test forbidding `parseFloat` in `features/billing/**`.

#### Tasks

- [ ] `[DB]` Migration `00024_create_tariffs.sql` — price list with effective dating, unique `(code, effective_from)`.
- [ ] `[DB]` Migration `00025_create_invoices.sql` — invoice header with `household_id`, status CHECK, `invoice_number` globally unique.
- [ ] `[DB]` Migration `00026_create_invoice_lines.sql` — charges with `source_table`/`source_id` provenance and `line_total_minor`.
- [ ] `[DB]` Migration `00027_create_payments.sql` — payments with `op_id TEXT UNIQUE NOT NULL` and reversal linkage.
- [ ] `[DB]` Migration `00028_create_payment_allocations.sql` — allocations with a CHECK that `amount_minor <> 0` and a partial index for fast balance derivation.
- [ ] `[DB]` Migration `00029_create_drugs.sql` — drug catalogue + seed of ~120 common PHC drugs.
- [ ] `[DB]` Migration `00030_create_prescriptions.sql` — prescriptions with `prescription_number` and status.
- [ ] `[DB]` Migration `00031_create_prescription_items.sql` — items with price snapshots and quantity.
- [ ] `[DB]` Migration `00032_create_dispenses.sql` — append-only dispense events.
- [ ] `[DB]` Migration `00033_add_money_indexes.sql` — indexes for balance queries: `(patient_id)`, `(household_id)`, `(invoice_id)` on lines and allocations.
- [ ] `[BE]` `internal/repo/billing_repo.go` — `CreateInvoice`, `AddLines`, `RecordPayment`, `Allocate`, `PatientBalance`, `HouseholdBalance`, `OpenInvoices`, `ReversePayment`.
- [ ] `[BE]` `internal/repo/pharmacy_repo.go` — `CreatePrescription`, `PendingQueue`, `Dispense`, `ListDispenses`.
- [ ] `[BE]` `internal/service/billing.go` — the money rules: line totals, allocation must not exceed either the payment or the outstanding line, no negative allocations except reversals, all arithmetic in `int64` minor units, all multi-row writes in one transaction.
- [ ] `[BE]` `internal/service/pharmacy.go` — prescription creation, the derived "paid" check, dispense rules (cannot dispense more than prescribed; partial dispense allowed and recorded), and the rule that dispensing **never** touches a stock number.
- [ ] `[BE]` Handlers: `invoice_handler.go`, `payment_handler.go`, `prescription_handler.go`, `dispense_handler.go`, `tariff_handler.go`.
- [ ] `[BE]` `GET /api/v1/patients/:id/balance` and `GET /api/v1/households/:id/balance` — derived sums only, with the SQL visible in the repo for the viva.
- [ ] `[BE]` Retire the provisional gate: `POST /lab-orders/:id/mark-paid` now creates a **real** invoice line from the tariff and a real payment; remove the placeholder constant and the `source='provisional'` marker; note the removal in `PROJECT-TRACKER.md`.
- [ ] `[FE]` `web/src/lib/money.ts` — `formatMinor`, `sumMinor`, `allocateMinor` (largest-remainder split so allocation never loses a minor unit).
- [ ] `[FE]` `routes/cashier.invoices.tsx` — search by patient/household/invoice number, list open invoices, show **derived** outstanding.
- [ ] `[FE]` `routes/cashier.invoices.$invoiceId.tsx` — invoice detail: lines, payments, allocations, outstanding, **Collect payment**, **Print receipt**, **Reverse**.
- [ ] `[FE]` `routes/cashier.household.$householdId.tsx` — the family account view: all members' charges in one ledger.
- [ ] `[FE]` `features/billing/PaymentDialog.tsx` — amount, method, reference, auto-allocation oldest-first with manual override, change/overpayment handling (overpayment becomes an unallocated credit, shown clearly).
- [ ] `[FE]` `features/billing/ReceiptSlip.tsx` + `print.css` — 58 mm and 80 mm layouts, facility header, patient, lines, total, amount paid, outstanding, cashier, timestamp, receipt number.
- [ ] `[FE]` `routes/pharmacist.queue.tsx` — awaiting payment / ready / dispensed columns with the derived payment state.
- [ ] `[FE]` `routes/pharmacist.dispense.$prescriptionId.tsx` — per-item dispensing with quantity, partial dispensing, and a printed/stored dispensing note.
- [ ] `[FE]` `routes/admin.tariffs.tsx` — tariff CRUD with effective dating.
- [ ] `[FE]` Doctor's prescription builder on the consultation screen, with drug autocomplete from the local `drugs` table (works offline).
- [ ] `[TEST]` Go: allocation arithmetic — partial payments across three invoices, oldest-first, exactly reconciled to the minor unit.
- [ ] `[TEST]` Go: idempotency — replay the same `POST /sync/push` containing a payment twice → one payment, one allocation set, identical balance.
- [ ] `[TEST]` Go: **balance is derived** — a migration-scan test asserting no `balance` column exists on any table, plus a balance test after a reversal.
- [ ] `[TEST]` Go: `parseFloat`/`float64` never appears in billing packages (static check).
- [ ] `[TEST]` Go: dispense guard — cannot dispense an unpaid prescription; cannot dispense more than prescribed.
- [ ] `[TEST]` TS: `formatMinor` with large values, zero, and negatives; `allocateMinor` sums to the total.
- [ ] `[TEST]` E2E: **Flow (c)** — prescription → drug payment at cashier → dispense, offline until payment.
- [ ] `[TEST]` Manual: print a receipt on 58 mm and 80 mm paper (or a PDF printer with a custom page size); photograph both for `docs/evidence/`.
- [ ] `[DOCS]` `01-DATA-MODEL.md` billing + pharmacy sections; `03-UI-PATTERNS.md` receipt pattern; a short `docs/` note on the derived-balance rule.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00024_create_tariffs.sql` | `tariffs` — effective-dated price list per category |
| `00025_create_invoices.sql` | `invoices` — patient **and** household accounts, status |
| `00026_create_invoice_lines.sql` | `invoice_lines` — charges with source provenance |
| `00027_create_payments.sql` | `payments` — `op_id UNIQUE`, reversal linkage, methods |
| `00028_create_payment_allocations.sql` | `payment_allocations` — the allocation half of the ledger |
| `00029_create_drugs.sql` | `drugs` — catalogue + seed |
| `00030_create_prescriptions.sql` | `prescriptions` — prescription number, status |
| `00031_create_prescription_items.sql` | `prescription_items` — dose, quantity, price snapshot |
| `00032_create_dispenses.sql` | `dispenses` — append-only dispense events |
| `00033_add_money_indexes.sql` | Indexes backing derived balance and queue queries |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `GET /api/v1/tariffs` | Active price list, filterable by category |
| `POST /api/v1/tariffs` · `PATCH /api/v1/tariffs/:id` | Accountant/admin price management |
| `POST /api/v1/invoices` | Create an invoice with charge lines (patient or household account) |
| `GET /api/v1/invoices` | Search: `q`, `status`, `patient_id`, `household_id` |
| `GET /api/v1/invoices/:id` | Invoice with lines, payments, allocations, derived outstanding |
| `POST /api/v1/payments` | Record a payment; `op_id` in the body; allocates oldest-first unless told otherwise |
| `POST /api/v1/payments/:id/reverse` | Compensating reversal with a mandatory reason |
| `GET /api/v1/patients/:id/balance` | Derived: `SUM(charges) − SUM(allocations)` |
| `GET /api/v1/households/:id/balance` | Derived family-account balance |
| `GET /api/v1/drugs` | Drug catalogue search |
| `POST /api/v1/prescriptions` | Create a prescription with items |
| `GET /api/v1/prescriptions` | Pharmacist queue with derived payment state |
| `POST /api/v1/prescriptions/:id/dispense` | Record dispense events (partial allowed) |
| `GET /api/v1/dispenses` | Dispense history, filterable |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/cashier/invoices` | Find and list open invoices |
| `/cashier/invoices/:invoiceId` | Invoice detail, collect payment, print receipt, reverse |
| `/cashier/household/:householdId` | Family account: all members' charges in one ledger |
| `/pharmacist/queue` | Awaiting payment / ready / dispensed |
| `/pharmacist/dispense/:prescriptionId` | Dispense items, partial dispensing |
| `/admin/tariffs` | Tariff management |

#### Demo script

1. As `doctor`, finish a consultation for the malaria patient and write a prescription: artemether-lumefantrine 20/120 mg, 2 tablets twice daily for 3 days, quantity 24.
2. Point at the pharmacist queue (second browser, `pharmacist`): the prescription is there as **awaiting payment**, greyed out, with "not dispensed" and a reason on hover.
3. **Go offline** as `pharmacist` and try to dispense → refused, with "payment outstanding: 4 500.00". Say: *"dispensing is gated on a derived balance, not a flag anyone can set."*
4. As `cashier`, open `/cashier/invoices`, find the drug invoice, click **Collect payment**, enter 4 500.00 cash, confirm the allocation is oldest-first, and click **Print receipt**.
5. Print the 58 mm slip (or export the PDF). Show the patient name, line, total, amount paid, **outstanding 0.00**, cashier name and receipt number.
6. **Idempotency:** in DevTools, re-POST the exact same payment payload with the same `op_id`. Show the response is the *original* payment and the invoice is still paid exactly once.
7. Back to `pharmacist` (still offline-capable): the queue now shows **ready**. Dispense all 24 tablets, and note the dispense record has no stock maths — say: *"pharmacy inventory is explicitly out of scope; we record the dispense event, not the shelf."*
8. Go online; show the balance on the patient profile: charges 4 500.00, allocations 4 500.00, outstanding 0.00 — computed, not stored.
9. **Family account:** register a charge against the child in the Okonkwo household, then open `/cashier/household/:id` and show every member's charges in one ledger with one outstanding total.
10. Reverse the payment with reason "wrong patient selected" and show the outstanding return to 4 500.00 with both the original and the compensating payment visible.

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | Allocation arithmetic incl. overpayment, partial payment, multi-invoice split |
| Unit (TS) | `formatMinor`, `allocateMinor` largest-remainder split, receipt line totals |
| Integration (Go) | Payment replay idempotency end-to-end through `/sync/push` |
| Integration (Go) | Reversal restores the derived balance exactly |
| Integration (Go) | **No stored balance**: scan migrations for a `balance` column (must fail the build if one appears) |
| E2E (Playwright) | **Flow (c)** complete; plus receipt DOM assertions (amounts, no `NaN`) |
| Manual offline | Take a payment offline, reload, print the receipt offline, then sync |
| Manual hardware | Print on 58 mm and 80 mm; record paper/DPI settings in `docs/` |

#### Risks / watch-outs

- **Floating point creeping into money.** The single most common and most damaging bug in a billing system. `BIGINT` minor units end to end, `int64` in Go, no `number` division in TS, and a static test.
- **Receipt printing is where browser hardware support bites.** `window.print()` with print CSS is the guaranteed path. WebUSB/Web Bluetooth thermal printers have uneven browser support and are **not** the demo path — see §6, R3.
- **Overpayment semantics.** Decide now: an overpayment becomes an unallocated credit on the household, visible in the UI, never silently swallowed. Write it in `01-DATA-MODEL.md`.
- **Price snapshots.** Copy `unit_price_minor` into `invoice_lines` and `prescription_items` at creation. Never re-read the current tariff for an old invoice.
- **Allocation deadlocks.** Allocate inside one transaction with `SELECT … FOR UPDATE` on the affected lines, or accept retries with a clear error. Test concurrency with two parallel payments.
- **Scope creep into inventory.** Any sentence containing "stock level", "batch", or "expiry" is a scope violation. The dispense note is free text, and that is the whole of it.
- **Household billing ambiguity.** A charge belongs to a patient *and* optionally to a household. Decide the rule (patient-level charge, household-level settlement) and never let a single invoice mix both funding sources silently.

#### Definition of Done for this sprint

- [ ] Every figure shown anywhere in the UI is derived at read time from `invoice_lines` and `payment_allocations`.
- [ ] A payment replayed through `/sync/push` cannot double-charge — proven by test.
- [ ] A 58 mm receipt prints correctly from the deployed app on a real printer.
- [ ] The provisional lab-payment gate from Sprint 5 has been fully replaced and the debt note closed.
- [ ] A grep of the entire schema shows no `balance` column.
- [ ] Nothing in the pharmacy code counts stock.

---

### Sprint 7 — Admission & inpatient management

**Sprint goal.** Admit a patient from a diagnosis, manage the ward with bed assignment and daily charge accrual, keep nursing notes, and discharge only when the inpatient balance is fully cleared.

**Duration.** 2 weeks.
**Prerequisites.** Sprint 6 complete (charges, invoices, derived balances).

#### Deliverables

- [ ] `wards` table — `name`, `code`, `ward_type` (`general` | `maternity` | `paediatric` | `isolation`), `is_active`, sync columns.
- [ ] `beds` table — `ward_id`, `bed_number`, `status` (`free` | `occupied` | `maintenance`), sync columns, unique `(ward_id, bed_number)`.
- [ ] `admissions` table — `admission_number`, `encounter_id`, `patient_id`, `ward_id`, `bed_id`, `admitted_by`, `admitted_at`, `reason`, `diagnosis_id`, `status` (`admitted` | `discharged` | `transferred` | `absconded`), `discharged_at`, `discharged_by`, `discharge_summary`, `daily_rate_minor BIGINT` (**snapshot of the tariff at admission**), sync columns.
- [ ] `nursing_notes` table — **append-only**: `admission_id`, `note`, `observation_json` (optional vitals snapshot), `recorded_by`, `recorded_at`, sync columns.
- [ ] Daily charge accrual: for every day the patient is admitted, exactly **one** `bed_day` invoice line. Made idempotent with a UNIQUE constraint on `(admission_id, charge_date)` in a `daily_charges` table that points at the invoice line it produced.
- [ ] Admission screen launched from a diagnosis in the consultation, with ward/bed picker showing live availability and the daily rate from the tariff.
- [ ] Inpatient board `/admissions`: bed grid by ward, occupancy, length of stay, outstanding balance, and a discharge button gated on the balance.
- [ ] Discharge flow: outstanding balance must be 0.00 (enforced in the service layer), discharge summary required, bed released to `free`, encounter closed via the `discharged` branch of the state machine, discharge summary printable.
- [ ] Balance clearance at the bedside: the cashier can take a payment against an admission's invoice from the inpatient board (a nurse calls the cashier; the cashier does not need to leave the desk to compute the total, because it is derived).
- [ ] Accrual job runs on the **client** (a local scheduled task at clinic-local midnight, plus a catch-up sweep on app start) so it works offline; the server validates and deduplicates on sync, so a device that was switched off for three days still accrues correctly when it wakes up.
- [ ] Admission, discharge and daily accrual register in the encounter timeline.

#### Tasks

- [ ] `[DB]` Migration `00034_create_wards.sql` — wards with type and active flag.
- [ ] `[DB]` Migration `00035_create_beds.sql` — beds with status and uniqueness per ward.
- [ ] `[DB]` Migration `00036_seed_wards_beds.sql` — a realistic PHC layout: 1 general ward (8 beds), 1 maternity (4), 1 paediatric (4), 1 isolation (2).
- [ ] `[DB]` Migration `00037_create_admissions.sql` — admission header with `daily_rate_minor` snapshot and status CHECK.
- [ ] `[DB]` Migration `00038_create_nursing_notes.sql` — append-only notes, index `(admission_id, recorded_at DESC)`.
- [ ] `[DB]` Migration `00039_create_daily_charges.sql` — `(admission_id, charge_date)` UNIQUE, `invoice_line_id` FK, `amount_minor`, sync columns.
- [ ] `[BE]` `internal/repo/admission_repo.go` — `Admit`, `AssignBed`, `ReleaseBed`, `ListActive`, `WardOccupancy`, `AccrueDaily`, `Discharge`.
- [ ] `[BE]` `internal/service/admission.go` — admission rules (one active admission per patient; bed must be `free`; ward must match the patient's age band for paediatric), accrual idempotency, discharge gate on the derived balance, and bed release in the same transaction as the discharge.
- [ ] `[BE]` `internal/service/accrual.go` — given an admission and a date range, produce the missing daily charge lines; safe under replay and across devices.
- [ ] `[BE]` Handlers: `admission_handler.go` (`POST /admissions`, `POST /admissions/:id/transfer`, `POST /admissions/:id/discharge`), `ward_handler.go`, `nursing_note_handler.go`, `GET /admissions/:id/balance`.
- [ ] `[BE]` Bed double-booking guard: `AssignBed` uses `UPDATE beds SET status='occupied' WHERE id=$1 AND status='free'` and treats zero rows affected as a conflict → the loser gets `bed_unavailable` and the UI offers the next free bed.
- [ ] `[FE]` `routes/encounters.$encounterId.admit.tsx` — admission form with ward/bed availability, daily rate, reason, diagnosis link.
- [ ] `[FE]` `routes/admissions.tsx` — inpatient board: bed grid grouped by ward, occupancy stats, LOS, outstanding balance per bed.
- [ ] `[FE]` `routes/admissions.$admissionId.tsx` — admission detail: timeline, nursing notes (add + list), daily charges, balance, **Discharge** and **Collect payment** actions.
- [ ] `[FE]` `features/admissions/BedGrid.tsx` — colour-coded bed tiles, click to open the admission, drag-free (click-to-assign only — no drag-and-drop in this project).
- [ ] `[FE]` `features/admissions/DailyAccrualRunner.tsx` — a headless component running the catch-up sweep on app start and at local midnight, with a visible "accrued through <date>" indicator so a demo never shows a stale total.
- [ ] `[FE]` `features/admissions/DischargeSummary.tsx` + print styles for an A5 discharge summary (separate from the 58 mm receipt).
- [ ] `[FE]` Bed status reflected live across devices via sync (two browsers, one assigns, the other sees the bed occupied).
- [ ] `[TEST]` Go: accrual idempotency — run `AccrueDaily` twice for the same range and for a 3-day catch-up → exactly one line per day.
- [ ] `[TEST]` Go: discharge blocked with a non-zero balance; allowed at exactly zero; bed released in the same transaction.
- [ ] `[TEST]` Go: bed double-booking under two concurrent requests → one wins, one gets `bed_unavailable`.
- [ ] `[TEST]` Go: `daily_rate_minor` snapshot — changing the tariff mid-stay does not alter accrued lines.
- [ ] `[TEST]` TS: local accrual runner computes the correct date range across a month boundary and a DST-less local timezone.
- [ ] `[TEST]` E2E: admit → 3 days of accrual (clock injected) → partial payments → discharge blocked → settle → discharge → bed free.
- [ ] `[TEST]` Manual offline: accrue three days offline on two devices simultaneously, then sync → no duplicate lines.
- [ ] `[DOCS]` `01-DATA-MODEL.md` admission section; a `docs/` note explaining why accrual is client-driven and server-deduplicated.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00034_create_wards.sql` | `wards` — ward definitions with type |
| `00035_create_beds.sql` | `beds` — per-ward beds with status |
| `00036_seed_wards_beds.sql` | Seed data: 4 wards, 18 beds |
| `00037_create_admissions.sql` | `admissions` — admission header, daily-rate snapshot, status |
| `00038_create_nursing_notes.sql` | `nursing_notes` — append-only observations |
| `00039_create_daily_charges.sql` | `daily_charges` — one row per admission-day, unique, links to the invoice line |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `GET /api/v1/wards` | Wards with occupancy counts |
| `GET /api/v1/wards/:id/beds` | Bed grid with current occupant |
| `POST /api/v1/admissions` | Admit from an encounter/diagnosis; assigns a bed atomically |
| `GET /api/v1/admissions` | Active admissions; `ward_id`, `status` filters |
| `GET /api/v1/admissions/:id` | Admission detail with notes, charges, balance |
| `POST /api/v1/admissions/:id/transfer` | Move ward/bed |
| `POST /api/v1/admissions/:id/notes` | Append a nursing note |
| `POST /api/v1/admissions/:id/accrue` | Idempotent accrual for a date range |
| `POST /api/v1/admissions/:id/discharge` | Discharge; 409 if the balance is non-zero |
| `GET /api/v1/admissions/:id/balance` | Derived inpatient balance |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/encounters/:encounterId/admit` | Admit from a diagnosis |
| `/admissions` | Inpatient board: bed grid, occupancy, balances |
| `/admissions/:admissionId` | Admission detail, notes, daily charges, discharge |

#### Demo script

1. As `doctor`, open a consultation for a patient with a diagnosis of severe malaria and click **Admit**. Choose the general ward, show only **free** beds offered, and pick bed G-04. Show the daily rate pulled from the tariff.
2. Open `/admissions`. Show bed G-04 occupied, with day 1 and an outstanding balance equal to one day's rate.
3. As `nurse`, add a nursing note ("BP 100/60, febrile, tepid sponging given"). Show it appended, never editable.
4. **Accrual:** use the dev clock control (or the catch-up sweep on a seeded admission) to advance three days. Run **Accrue daily charges**. Show exactly four `bed_day` lines — one per day, no duplicates — and the balance at four days.
5. Run the accrual again immediately and show the line count unchanged. Say: *"accrual is idempotent by `(admission_id, charge_date)`; two offline devices cannot double-charge a bed."*
6. **Go offline** as `nurse`. Try to discharge → refused: "outstanding balance 12 000.00".
7. As `cashier`, from the inpatient board click **Collect payment** for the admission. Take a partial payment of 6 000.00 and show the outstanding drop to 6 000.00.
8. Try to discharge again → still refused, now with the smaller figure. Take the remaining 6 000.00.
9. Discharge. Fill in the summary, confirm, and show: bed G-04 back to **free**, encounter closed, and the discharge summary on the timeline.
10. Open a **second browser** and show the bed grid updated there too — the second device learned the bed was released by sync, not by a refresh.

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | Accrual date-range logic incl. same-day admission, month boundary, catch-up |
| Unit (Go) | Discharge gate: 0 allowed, 1 minor unit blocked |
| Unit (TS) | Local accrual runner scheduling and date maths under a non-UTC locale |
| Integration (Go) | Concurrent bed assignment; concurrent accrual from two devices |
| Integration (Go) | Admission uniqueness: a second active admission for the same patient is rejected |
| E2E (Playwright) | Admit → accrue → blocked discharge → settle → discharge → bed free |
| Manual offline | Two devices accrue offline, sync, no duplicate charges |
| Print | Discharge summary on A5 and receipt on 58 mm both render correctly |

#### Risks / watch-outs

- **Day-boundary ambiguity.** "One day" must be defined: a bed-day is charged for each local calendar date on which the patient occupies the bed, including the discharge date only if discharge is after the ward's cutoff. Write the rule in `01-DATA-MODEL.md`, implement it in one function, and test it.
- **Two devices accruing the same day.** This is the whole reason for the composite unique constraint. Do not implement accrual as "check then insert" — insert and handle the conflict.
- **Time-zone drift.** Facility-local dates, UTC timestamps. Derive the local date once, in one helper, and use it everywhere.
- **Bed status as derived truth.** `beds.status` is a cached convenience; the truth is "is there an active admission for this bed?". Add a reconciliation query and run it in tests; mention it in the defence.
- **Absconded/transferred patients.** Both release the bed and stop accrual. Implement the statuses even if the UI is minimal — leaving them out creates a stuck-bed bug you will hit in the demo.
- **The nightly accrual job is not a server cron.** Server-side cron would break the offline story. Client-driven with server deduplication is the correct design; be ready to defend it.

#### Definition of Done for this sprint

- [ ] Discharge is impossible with a non-zero derived balance, enforced in the service layer.
- [ ] A four-day admission produces exactly four charge lines under replay and under two-device concurrency.
- [ ] Bed availability is correct across two synced browsers within one poll interval.
- [ ] The ward layout is seeded data, so a fresh clone can demo admissions with one command.
- [ ] The day-charging rule is written down, tested, and defensible.

---

### Sprint 8 — Referral system & public access

**Sprint goal.** A doctor composes a referral note, prints a 58–80 mm POS thermal slip carrying a QR code, and an external facility opens a read-only public view with the token plus a 6-digit PIN within a 48-hour window that is enforced in the query, logged, and revocable.

**Duration.** 2 weeks.
**Prerequisites.** Sprint 7 complete. (Sprint 6 at minimum — referrals need only the encounter and diagnosis, but the plan assumes 7.)

#### Deliverables

- [ ] `referrals` table — `referral_number`, `encounter_id`, `patient_id`, `diagnosis_id`, `from_facility_id`, `to_facility_name`, `to_facility_contact`, `reason`, `clinical_summary`, `urgency` (`routine` | `urgent` | `emergency`), `created_by`, `created_at`.
- [ ] **Server-only columns on `referrals`** (never replicated to devices): `access_token_hash TEXT`, `pin_hash TEXT`, `pin_attempts INT`, `pin_locked_until`, `expires_at`, `revoked_at`, `revoked_by`, `first_viewed_at`, `view_count`.
- [ ] `referral_access_log` table — **server-only, never synced**: `referral_id`, `viewed_at`, `ip`, `user_agent`, `outcome` (`success` | `bad_pin` | `expired` | `revoked` | `locked`), `failure_count`.
- [ ] Referral composer on the consultation screen: destination facility, reason, clinical summary (pre-filled from the diagnosis and latest results, editable), urgency, and a required destination contact.
- [ ] QR generation with `qrcode` 1.5.4 → canvas → data URL, encoding `https://<pages-domain>/r/<token>` (a 256-bit token, base64url, **never** the referral ID).
- [ ] POS thermal slip layout in `print.css`: `@page { size: 58mm auto }` (and an 80 mm variant), monospace, facility header, patient name/number/age/sex, referral number, destination, reason, urgency, QR block, the 6-digit PIN, an expiry line "Valid until <local datetime> (48 hours)", and a footer "Contains confidential patient information".
- [ ] **Print paths, in order of preference:** (1) browser `window.print()` with the thermal print CSS — the guaranteed demo path; (2) WebUSB / Web Bluetooth ESC-POS when the browser and printer both support it, behind a feature detection and a settings toggle; (3) manual fallback — display the slip and let the user print from the OS dialog. The UI must state which path is in use, and the fallback must never be a dead end.
- [ ] `GET /r/:token` public view route in the SPA — a plain page, no auth, no app shell chrome, `noindex`, with a 6-digit PIN entry.
- [ ] `POST /api/v1/public/referrals/:token/verify` — rate-limited (5 attempts, then locked for 15 minutes), constant-time PIN comparison, expiry checked **in the query** (`WHERE expires_at > now() AND revoked_at IS NULL`), and every attempt logged.
- [ ] Read-only referral payload: patient name, patient number, age, sex, diagnosis code and label, clinical summary, urgency, referring facility, contact, referral date. **No** full history, **no** billing, **no** identifiers beyond what a receiving clinician needs.
- [ ] Expiry UX: a live countdown on the public page, a clear "this referral link has expired" state, and the same for revoked links.
- [ ] Revocation: the issuing doctor can revoke from the referral detail screen; a revoked token returns the revoked state and is logged.
- [ ] Referral register (`/referrals`) for the doctor/admin: all referrals with status (pending view / viewed / expired / revoked), view counts, and a link to the access log.
- [ ] Referral detail screen with a **Reprint slip** action and the full access log.

#### Tasks

- [ ] `[DB]` Migration `00040_create_referrals.sql` — referral content + the server-only token/PIN/expiry columns, unique index on `access_token_hash`, index on `(created_at DESC)`.
- [ ] `[DB]` Migration `00041_create_referral_access_log.sql` — server-only access log with outcome and IP.
- [ ] `[DB]` Migration `00042_add_referral_indexes.sql` — partial index for live referrals (`WHERE revoked_at IS NULL`), plus an index for the expiry sweeper.
- [ ] `[BE]` `internal/platform/token` — 256-bit crypto-random token generation (base64url) and a 6-digit PIN generator using `crypto/rand` with a modulo-bias-free method; PIN hashed with Argon2id.
- [ ] `[BE]` `internal/repo/referral_repo.go` — `Create`, `FindByTokenHash` (expiry-conditional), `RecordView`, `RecordFailure`, `Lock`, `Revoke`, `List`, `AccessLog`.
- [ ] `[BE]` `internal/service/referral.go` — creation (token + PIN generated **server-side**), TTL from `REFERRAL_TTL_HOURS`, PIN verification with lockout, revocation, and the rule that only a `doctor` may issue.
- [ ] `[BE]` `internal/service/referral_public.go` — the read-only projection: an explicit allow-list of fields, so a future column cannot leak by default.
- [ ] `[BE]` Handlers: `referral_handler.go` (authenticated CRUD + revoke + access log), `public_referral_handler.go` (unauthenticated verify + fetch, its own rate limiter, no request logging of the token).
- [ ] `[BE]` **Token redaction in logs:** a zerolog hook or an explicit rule that URL paths under `/public/referrals/` are never logged in full. Add a test that asserts a request log line contains no token.
- [ ] `[BE]` Nightly sweeper (documented, optional to run): mark expired referrals and record an `expired` access-log entry on the first post-expiry attempt.
- [ ] `[FE]` `features/referrals/ReferralComposer.tsx` — the doctor's form, pre-filled from the encounter.
- [ ] `[FE]` `features/referrals/ReferralSlip.tsx` — the 58/80 mm layout, QR canvas, PIN block, expiry line.
- [ ] `[FE]` `features/referrals/useThermalPrint.ts` — feature-detect WebUSB/Web Bluetooth, expose `printSlip()`, fall back to `window.print()` and report which path was used.
- [ ] `[FE]` `routes/referrals.index.tsx` — register with status, view count, expiry countdown.
- [ ] `[FE]` `routes/referrals.$referralId.tsx` — detail, reprint, revoke, access log table.
- [ ] `[FE]` `routes/public.referral.$token.tsx` — **outside** the auth guard, minimal layout, PIN entry, countdown, read-only rendering, print-friendly.
- [ ] `[FE]` The public route must work in a **fresh browser profile with no session and no IndexedDB** — test it that way, because that is exactly how the receiving facility opens it.
- [ ] `[TEST]` Go: token entropy and uniqueness; a token is not the referral ID and contains no patient data.
- [ ] `[TEST]` Go: expiry enforced at the query level — an expired referral returns 410 even if the token and PIN are correct.
- [ ] `[TEST]` Go: PIN lockout after 5 failures; a locked referral recovers after the lock window; a successful verify resets the counter.
- [ ] `[TEST]` Go: access log rows written for success, bad PIN, expired, revoked and locked outcomes.
- [ ] `[TEST]` Go: the public projection contains exactly the allow-listed fields (a reflection-based test that fails when a field is added).
- [ ] `[TEST]` Go: log redaction — no request log line contains a referral token.
- [ ] `[TEST]` TS: QR payload is the public URL and nothing else; PIN is six digits with leading zeros preserved.
- [ ] `[TEST]` E2E: **Flow (d)** — issue referral → slip renders with QR → open `/r/:token` in a fresh context → wrong PIN ×5 → locked → correct PIN after the lock → data visible → access log shows every attempt.
- [ ] `[TEST]` E2E: expiry — issue a referral with a 1-second TTL via a test-only env override → the public view reports expired and logs it.
- [ ] `[TEST]` Manual: print the slip on a real 58 mm POS printer, scan the QR with a phone, and open the public page on that phone (mobile network, not the dev machine). Photograph the slip for `docs/evidence/`.
- [ ] `[TEST]` Manual offline: compose a referral with the network off → it is queued, the doctor sees "will be issued when online", and after reconnecting the QR and PIN appear.
- [ ] `[DOCS]` `05-SECURITY.md` referral section (token, PIN, expiry, logging, revocation, threats); `03-UI-PATTERNS.md` thermal slip pattern; a `docs/` note on browser printer compatibility and the fallback order.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00040_create_referrals.sql` | `referrals` — content plus server-only token/PIN/expiry/revocation columns |
| `00041_create_referral_access_log.sql` | `referral_access_log` — every view attempt with outcome (never synced) |
| `00042_add_referral_indexes.sql` | Live-referral partial index and expiry-sweep index |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `POST /api/v1/referrals` | Create; server generates the 256-bit token and the 6-digit PIN, returns both once |
| `GET /api/v1/referrals` | Register: status filter, paging |
| `GET /api/v1/referrals/:id` | Detail including the access log |
| `POST /api/v1/referrals/:id/revoke` | Immediate revocation, audited |
| `GET /api/v1/referrals/:id/slip` | Server-rendered slip data (for reprint) |
| `GET /api/v1/public/referrals/:token` | Unauthenticated metadata: requires a valid, unexpired, unrevoked token; returns only a challenge (no clinical data) |
| `POST /api/v1/public/referrals/:token/verify` | PIN verification; returns the read-only payload; rate-limited and logged |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/referrals` | Referral register with status and view counts |
| `/referrals/:referralId` | Detail, reprint slip, revoke, access log |
| `/r/:token` | **Public**, unauthenticated, read-only referral view with PIN and 48-hour countdown |

#### Demo script

1. As `doctor`, open the consultation for the admitted malaria patient and click **Refer**. Show the form pre-filled with the diagnosis (B50.9) and the latest lab summary.
2. Choose destination "St. Mary's District Hospital", urgency **urgent**, add the contact, and issue the referral.
3. Show the **58 mm POS slip** preview: facility header, patient name/number/age/sex, referral number, destination, reason, urgency, a QR code, and a 6-digit PIN. Point at "Valid until <time> (48 hours)".
4. Print it on the thermal printer (or the PDF fallback) and hold the physical slip up to the camera. **Scan the QR with your phone** and open the URL on the phone, on mobile data, in a fresh browser with no login.
5. Enter a **wrong** PIN five times. Show the lockout message and that the API returned 429s. Show the access log (on the laptop) recording all five failures with timestamps.
6. Wait out (or reset) the lock, enter the **correct** PIN, and show the referral: patient name, age, sex, diagnosis code and label, summary, urgency, referring facility, contact. Confirm there is **no** billing data and no full history.
7. Try `/r/<token-plus-one-character>` → "invalid or expired". Try an expired seeded referral → "this referral link has expired". Show the same message and the `expired` log entry.
8. Back on the laptop, open `/referrals/:id` and show `view_count = 6` and the full access log.
9. Click **Revoke**. On the phone, reload the public page → revoked, and a new access-log line with outcome `revoked`.
10. **Offline referral:** go offline, compose a second referral, and show it queued with "will be issued when online"; reconnect and show the QR and PIN appear — while saying clearly: *"the token is issued by the server, so a referral is the one clinical action that needs connectivity to complete."*

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | PIN generation has no modulo bias and preserves leading zeros; constant-time comparison |
| Unit (Go) | Expiry arithmetic against `REFERRAL_TTL_HOURS`; urgency enum; token base64url round-trip |
| Unit (TS) | QR payload URL construction; countdown formatting; slip line wrapping at 32 characters |
| Integration (Go) | Verify flow with every outcome; lockout and recovery; revocation |
| Integration (Go) | Public projection field allow-list (fails when a field is added) |
| Integration (Go) | Log redaction: no token appears in any log line |
| E2E (Playwright) | **Flow (d)** full: issue → print DOM → fresh-context public view → lockout → success → revoke → expiry |
| Manual offline | Compose offline, queue, issue on reconnect |
| Manual hardware | Real 58 mm print, real phone scan, real mobile-network access; evidence photographed |

#### Risks / watch-outs

- **Browser thermal-printer support is genuinely uneven.** Detect, never assume; keep `window.print()` as the path you demo and mention WebUSB as an enhancement (see §6, R3). If the printer will not talk to the browser on demo day, the PDF/print-preview fallback plus a scanned QR on the phone still proves the feature — rehearse that fallback **before** the defence.
- **Leaking the token.** It appears in URLs, therefore in browser history, referrer headers, and logs. Redact it in server logs, send `Referrer-Policy: no-referrer` on the public page, and use a redirect (`/r/:token` → session-scoped view) only if you can do it without breaking the QR target.
- **The PIN is on the same slip as the QR.** That is the requirement, and it means the slip *is* the credential — say so in `05-SECURITY.md`, and never email or SMS it.
- **Server-only columns must actually stay server-only.** `access_token_hash` and `pin_hash` must never enter `sync_changes`. Add a pull-filter test asserting the public/authenticated pull for `referrals` returns only the allowed columns.
- **Offline creation cannot mint a token.** The honest design is "compose offline, issue on reconnect". Do not invent a client-generated token that the server might reject later.
- **Rate limiting behind Cloudflare/Cloud Run.** Per-instance in-memory limits are approximate. Be honest in `05-SECURITY.md`; note a shared store as future work.
- **`noindex` and caching.** The public page must not be cached by the service worker or a CDN edge with a token in the path. Exclude `/r/*` from the PWA precache manifest and set `Cache-Control: no-store` on the public API.

#### Definition of Done for this sprint

- [ ] A referral issued on the deployed app prints on a real 58 mm printer and opens on a phone over mobile data.
- [ ] Expiry is enforced in the SQL query, not in the UI, and is proven by test.
- [ ] Every access attempt (success or failure) writes exactly one access-log row.
- [ ] `access_token_hash` and `pin_hash` never appear in any sync payload — proven by test.
- [ ] A revoked referral is dead within one request, and the reason is visible in the register.
- [ ] The print fallback chain is documented and rehearsed.

---

### Sprint 9 — Hardening, reporting, offline validation & handover

**Sprint goal.** Prove the system is trustworthy under real conditions — long offline soaks, conflicting edits, cold starts, and restore drills — and hand it over with reports, an audit view, a user manual, and a defence-ready demo script.

**Duration.** 2 weeks (3 if the buffer from §9 is available).
**Prerequisites.** Sprints 0–8 complete and deployed.

#### Deliverables

- [ ] Role dashboards: front desk (registrations today, queue depth), nurse (awaiting vitals, outstanding observations), doctor (my consultations, awaiting results, critical results), lab (worklist ageing), pharmacist (awaiting payment, ready to dispense), cashier (collections today, outstanding by patient/household), accountant (daily collection summary, revenue by category, outstanding ageing), admin (devices, sync health, conflicts).
- [ ] Reports: daily activity, collections by method, revenue by tariff category, outstanding balances (patient and household), admissions census, referral register with view status. Each report exportable to CSV **client-side** (so it works offline).
- [ ] `/admin/audit` — audit log viewer with filters (actor, action, entity, date range). `audit_logs` remains **server-only and append-only** (`00-ARCHITECTURE.md` §5).
- [ ] `/admin/sync-health` — per-device cursor lag, pending op counts, push/pull success rates, and the `sync_conflicts` list with a side-by-side differ showing the losing and winning values.
- [ ] Full **offline soak test**: a Playwright scenario simulating two full working days of activity across all roles with the network off (register, vitals, diagnose, order, pay, dispense, admit, accrue, discharge, refer), then reconnect and assert convergence with zero data loss and zero duplication.
- [ ] Load/soak: 10 000 patients, 30 000 encounters, 50 000 charges seeded locally; measure app start, queue board render, patient search, balance query, and pull time. Record the numbers in `docs/`.
- [ ] Security checklist pass: OWASP-style review of the API (input validation, authz on every route, rate limits, error leakage, SQL injection via the table registry, CORS, headers), secret handling, dependency audit (`go list -m all` + `npm audit`), and a written residual-risk section in `05-SECURITY.md`.
- [ ] Performance pass: indexes verified with `EXPLAIN ANALYZE` for the ten heaviest queries; Cloud Run `--min-instances=1` for the defence and documented cost; pull batch tuning; Dexie index audit against every `useLiveQuery`.
- [ ] PWA install: `vite-plugin-pwa` manifest, icons, an install prompt, an offline shell verified by loading from a stopped network, and an **update prompt** when a new build is detected (never silently swap the shell mid-shift — `00-ARCHITECTURE.md` §3.4).
- [ ] Backup/restore drill: document Neon's restore/branching procedure, perform a real restore into a branch, point the API at it, and verify the app works. Record the RPO/RTO you actually achieved.
- [ ] `docs/USER-MANUAL.md` — one page per role, with screenshots, written for a PHC staff member who has never seen the system.
- [ ] `docs/DEMO-SCRIPT.md` — the 12-minute defence walkthrough with timings, the exact clicks, the pre-seeded fixtures, and the three fallbacks (network down, printer dead, Neon slow).
- [ ] Dissertation support: architecture diagrams exported as SVG, the sync sequence diagram, the ER diagram, the testing matrix, and a **limitations and future work** section drawn from §10 below.
- [ ] `README.md` final pass: prerequisites, setup, run, test, deploy, demo accounts, troubleshooting table.
- [ ] Tag `v1.0.0`.

#### Tasks

- [ ] `[DB]` Migration `00043_add_report_indexes.sql` — indexes for the reporting queries (`(created_at)` on the high-volume tables, plus covering indexes where `EXPLAIN` says so).
- [ ] `[DB]` Migration `00044_create_report_views.sql` — SQL views for the heavy reports (`v_daily_collections`, `v_revenue_by_category`, `v_outstanding_balances`, `v_admissions_census`) so the report SQL is reviewable in one place and reusable.
- [ ] `[DB]` Migration `00045_add_audit_indexes.sql` — audit-log filter indexes; confirm retention policy.
- [ ] `[BE]` `internal/service/reports.go` — one function per report, each returning a typed struct, each with a test that seeds and asserts known totals.
- [ ] `[BE]` Handlers: `report_handler.go`, `audit_handler.go`, `sync_health_handler.go`.
- [ ] `[BE]` `GET /api/v1/sync/health` — per-device lag, pending conflicts, error rates.
- [ ] `[BE]` Hardening: request body size limits, strict JSON decoding (`DisallowUnknownFields` where safe), consistent error envelopes, no stack traces in 500s, `Retry-After` on 429s.
- [ ] `[BE]` Verify every route appears in the RBAC test matrix; add a startup assertion that every registered route has an explicit role.
- [ ] `[FE]` `routes/index.tsx` becomes a real role-aware dashboard (redirect to the role's home screen).
- [ ] `[FE]` `routes/reports.*.tsx` — one route per report, each with a date range, a table, a total row and a CSV export button (client-side).
- [ ] `[FE]` `routes/admin.audit.tsx` — filterable audit viewer with a detail drawer.
- [ ] `[FE]` `routes/admin.sync-health.tsx` — device lag table + conflict differ.
- [ ] `[FE]` `features/sync/UpdatePrompt.tsx` — detects a new service worker, prompts, and reloads on demand.
- [ ] `[FE]` Accessibility pass: keyboard navigation on the queue board and forms, focus traps in dialogs, labels on every input, contrast check on status colours (Radix primitives already help).
- [ ] `[INFRA]` Cloud Run: `--min-instances=1` for the defence window, `--max-instances=3`, memory/CPU tuned from the load test, and a note on the cost delta.
- [ ] `[INFRA]` Neon: connection-limit audit (max instances × `MaxConns` vs the plan ceiling); record the number in `docs/`.
- [ ] `[INFRA]` Restore drill executed and written up with the exact commands.
- [ ] `[TEST]` Playwright offline soak: two simulated working days, all roles, network off, then converge — the single most important test in the project.
- [ ] `[TEST]` Conflict soak: two devices edit the same patients, vitals and invoices concurrently; assert convergence and that **no append-only fact was ever merged**.
- [ ] `[TEST]` Chaos: kill the API mid-push, verify the client retries and neither loses nor duplicates an operation.
- [ ] `[TEST]` Performance: record p50/p95 for the ten heaviest queries and the four heaviest screens; write the numbers into `docs/`.
- [ ] `[TEST]` Recovery drill: restore to a branch, run the smoke suite against it, document the outcome.
- [ ] `[TEST]` Full E2E suite green: **all four flows (a)–(d) in one script**, offline where applicable.
- [ ] `[DOCS]` `USER-MANUAL.md`, `DEMO-SCRIPT.md`, final `README.md`, `PROJECT-TRACKER.md` closure, thesis diagrams exported.
- [ ] `[DOCS]` `05-SECURITY.md` residual risks + `LIMITATIONS.md` (or a dissertation appendix) covering the honest gaps.

#### Database migrations introduced

| Migration | Contents |
|---|---|
| `00043_add_report_indexes.sql` | Reporting indexes justified by `EXPLAIN ANALYZE` |
| `00044_create_report_views.sql` | `v_daily_collections`, `v_revenue_by_category`, `v_outstanding_balances`, `v_admissions_census` |
| `00045_add_audit_indexes.sql` | Audit-log filter indexes and retention notes |

#### API endpoints introduced

| Endpoint | Notes |
|---|---|
| `GET /api/v1/reports/daily-activity` | Registrations, encounters, admissions for a date range |
| `GET /api/v1/reports/collections` | Payments by method and cashier, with reversals netted |
| `GET /api/v1/reports/revenue-by-category` | Consultation / lab / drug / bed-day split, derived |
| `GET /api/v1/reports/outstanding` | Patient and household balances, aged buckets |
| `GET /api/v1/reports/admissions-census` | Current inpatients, LOS, ward occupancy |
| `GET /api/v1/reports/referrals` | Referral register with view status |
| `GET /api/v1/audit-logs` | Filtered audit viewer (admin) |
| `GET /api/v1/sync/health` | Device lag, conflicts, error rates |
| `GET /api/v1/audit-logs/:id` | Single audit entry detail |

#### Screens introduced

| Route | Purpose |
|---|---|
| `/` | Role-aware dashboard (redirects to the role home) |
| `/reports/daily-activity` | Daily activity report + CSV |
| `/reports/collections` | Collections by method/cashier + CSV |
| `/reports/revenue` | Revenue by tariff category + CSV |
| `/reports/outstanding` | Outstanding balances, aged + CSV |
| `/reports/admissions` | Admissions census + CSV |
| `/reports/referrals` | Referral register with view status + CSV |
| `/admin/audit` | Audit log viewer |
| `/admin/sync-health` | Device lag and conflict differ |

#### Demo script

This is the **defence rehearsal**. Time-box to 12 minutes.

1. **Minute 0–1 — the problem.** Show the app installed as a PWA, then turn the network off completely (DevTools *Offline* **and** a phone hotspot off, if possible). Say the one-sentence architecture.
2. **Minute 1–4 — the clinical day, entirely offline.** Register a family → vitals → queue → diagnosis (B50.9) → lab order → lab result → prescription → *(payment requires the network by design: say so)* — do as much as possible offline and point at the pending badge growing.
3. **Minute 4–5 — reconnect and converge.** Bring the network back. Show the badge drain. Open a second browser and show the same records arrived. Read out the pending count going to zero.
4. **Minute 5–6 — the money path.** As cashier, collect the lab and drug payments, print a 58 mm receipt, and reverse one payment to show the derived balance move. Replay a payment with the same `op_id` to show idempotency.
5. **Minute 6–8 — admission.** Admit the patient, accrue three days, show the discharge blocked on the balance, settle it, discharge, and show the bed released on the second browser.
6. **Minute 8–10 — the referral.** Compose and print the thermal slip, scan the QR with a phone over mobile data, enter a wrong PIN five times to show the lockout and the access log, then the correct PIN to show the read-only view. Revoke it and reload.
7. **Minute 10–11 — the evidence.** Open `/admin/sync-health`: device lag, the conflict log with a real LWW decision, and the audit log. Say: *"conflicts are logged, not hidden."*
8. **Minute 11–12 — the honest limits.** Show `LIMITATIONS.md`: no pharmacy inventory, single-tenant by design, in-memory rate limits, client-side reports. Then §10's future work.
9. Have these three fallbacks loaded **before** you start: a saved screen recording of the full run, a PDF of the thermal slip with a scannable QR, and `--min-instances=1` already warm.

#### Tests to write in this sprint

| Type | Test |
|---|---|
| Unit (Go) | Each report's totals against a hand-computed seeded fixture |
| Unit (TS) | CSV export escaping (commas, quotes, newlines, unicode names) |
| Integration (Go) | Audit-log filters; startup route/role assertion |
| Integration (Go) | `EXPLAIN ANALYZE` regression: the four heaviest queries stay on index scans |
| E2E (Playwright) | **Offline soak**: two simulated working days, all roles, network off, then full convergence |
| E2E (Playwright) | **All four flows (a)–(d) in a single script** |
| Chaos | API killed mid-push; client retries without loss or duplication |
| Performance | p50/p95 recorded for the ten heaviest queries and four heaviest screens |
| Recovery | Restore drill into a Neon branch; smoke suite green against the restored database |
| Manual | PWA install on a desktop and an Android device; offline launch from a cold start |
| Manual | Accessibility: keyboard-only pass through registration, vitals, payment and dispense |

#### Risks / watch-outs

- **Sprint 9 is where unfinished work hides.** If Sprints 3–8 slipped, this sprint becomes the catch-up sprint and you lose the hardening. Protect it by cutting scope earlier (§9), not by shrinking this sprint.
- **Fixing bugs found by the soak test may need schema changes.** Treat any such migration as a *problem*: prefer a code fix to a migration this late, and if a migration is unavoidable, run the restore drill immediately afterwards.
- **Reports tempt you into TanStack Query for clinical data.** Reports are the one place where server reads are legitimate (`00-ARCHITECTURE.md` §4.3), but keep clinical screens on Dexie. Do not blur it now.
- **PWA update prompts can break a demo.** Test the update prompt on a *stale* client explicitly; a silent swap mid-demo is a self-inflicted wound.
- **Cloud Run cold start on demo day.** `--min-instances=1` for the defence window only, and warm the API 10 minutes before you present.
- **Neon connection ceiling.** Re-check `max-instances × MaxConns` after the load test; a burst during the soak can exhaust it and produce baffling timeouts.
- **Writing the dissertation is part of this sprint.** Budget real days for it. The code is not the deliverable; the dissertation is.

#### Definition of Done for this sprint

- [ ] All four E2E flows (a)–(d) pass in a single automated script, and the offline soak passes.
- [ ] The offline soak shows **zero data loss and zero duplication** — stated with numbers in `docs/`.
- [ ] A restore drill has been performed and written up.
- [ ] `USER-MANUAL.md` is usable by someone who has never seen the system.
- [ ] `DEMO-SCRIPT.md` has been rehearsed end-to-end at least twice, including one run with a deliberate failure.
- [ ] `v1.0.0` tagged, deployed, and the deployed URL exercised with the network off.
- [ ] `PROJECT-TRACKER.md` closed with an honest list of what was not delivered.

---

## 5. Cross-sprint dependency graph

```
                          ┌───────────────────────────────────────────┐
                          │  S0  Foundations & walking skeleton       │
                          │  repo · goose · Neon · Cloud Run · Pages  │
                          │  /healthz · /about · Dexie v1             │
                          └────────────────────┬──────────────────────┘
                                               │ blocks everything
                                               ▼
                          ┌───────────────────────────────────────────┐
                          │  S1  Auth, roles & device identity        │
                          │  Argon2id · JWT+refresh · RBAC · ECDSA    │
                          └────────────────────┬──────────────────────┘
                                               │  (device signature +
                                               │   session required by sync)
                                               ▼
        ╔══════════════════════════════════════════════════════════════╗
        ║  S2  THE SYNC ENGINE            ◄── CRITICAL PATH            ║
        ║  outbox · push · pull · change log · LWW · idempotency       ║
        ║                                                              ║
        ║  Every clinical feature below writes through this engine.    ║
        ║  Build it first, or build six inconsistent write paths.      ║
        ╚═══════════════╤═══════════╤═══════════╤═══════════╤═══════════╝
                        │           │           │           │
        ┌───────────────┘           │           │           └───────────────┐
        │                           │           │                           │
        ▼                           ▼           ▼                           ▼
┌───────────────────┐   ┌───────────────────┐   ┌───────────────────┐   ┌───────────────────┐
│ S3 Patients &     │   │ S4 Queue, vitals  │   │ S5 Consultation,  │   │ (S2 also blocks   │
│    households     │──►│    & triage       │──►│    diagnosis,     │   │  the schema       │
│    (mutable/LWW)  │   │    (append-only   │   │    lab orders     │   │  discipline of    │
│                   │   │     vitals)       │   │                   │   │  S3–S8)           │
└─────────┬─────────┘   └─────────┬─────────┘   └─────────┬─────────┘   └───────────────────┘
          │                       │                       │
          │ household account     │ encounter + stage     │ diagnosis + order
          │                       │                       │
          └───────────┬───────────┴───────────┬───────────┘
                      │                       │
                      ▼                       ▼
        ┌───────────────────────────┐   ┌───────────────────────────┐
        │ S6 Billing, cashier &     │   │ S7 Admission & inpatient  │
        │    pharmacy               │──►│    management             │
        │ charges · allocations ·   │   │ daily accrual · discharge │
        │ derived balance · dispense│   │ gate on derived balance   │
        └─────────────┬─────────────┘   └─────────────┬─────────────┘
                      │                               │
                      │  (S6 provides the ledger a    │
                      │   referral's billing view      │
                      │   must NOT expose)             │
                      └───────────────┬───────────────┘
                                      ▼
                      ┌───────────────────────────────────┐
                      │ S8 Referral & public access       │
                      │ token+QR+PIN · 48h · /r/:token    │
                      └─────────────────┬─────────────────┘
                                        │
                                        ▼
                      ┌───────────────────────────────────┐
                      │ S9 Hardening, reporting, offline  │
                      │    validation & handover          │
                      └───────────────────────────────────┘
```

**Reading the graph.**

| Relationship | Meaning |
|---|---|
| S0 → everything | No sprint can start without a working deploy pipeline. |
| S1 → S2 | Sync requires both a session token and a device signature. Device keys must exist before the protocol that uses them. |
| **S2 → S3, S4, S5, S6, S7, S8** | **The critical edge.** Every clinical table, every write path, and every conflict rule is inherited from the engine. Building a feature before the engine means rewriting that feature afterwards. |
| S3 → S4 | An encounter belongs to a patient. |
| S4 → S5 | A consultation begins from the queue. |
| S5 → S6 | Billing charges originate from diagnoses, lab orders and prescriptions. |
| S5 → S7 | Admission is triggered by a diagnosis. |
| S6 → S7 | Discharge is gated on a derived balance, which is a Sprint 6 construct. |
| S6 → S8 | The public referral view is defined partly by what it must *not* show (billing). |
| S6 → S9 | Reports summarise the ledger. |

**Parallelisable work if time allows** (a second developer, or a spare week): S8 can be built alongside S6 once S5 exists, because referrals depend on the encounter and diagnosis, not on money — only the "no billing in the public view" rule touches S6. Everything else is a chain.

---

## 6. Risk register

Probability and impact: **L**ow / **M**edium / **H**igh. "Bites in" names the sprint where this risk is most likely to materialise — put a reminder in `PROJECT-TRACKER.md` for that sprint.

| # | Risk | Prob. | Impact | Mitigation | Contingency | Bites in |
|---|---|---|---|---|---|---|
| **R1** | **Sync conflicts are harder than expected** — field-level LWW, clock skew, replay, and append-only guards interact in ways that are hard to debug | H | H | Build sync first (S2) with a scratch probe table; write the conflict policy down before coding; log every decision into `sync_conflicts`; keep the drain sequential and boring; test with two real browser contexts from day one | Fall back to **row-level** LWW on `updated_at` (one page of code instead of five) and document the simplification honestly in `02-SYNC-PROTOCOL.md`; clinical facts stay append-only either way, so no clinical data is ever at risk | **S2**, resurfaces S4 (concurrent vitals), S7 (concurrent accrual) |
| **R2** | **Offline auth edge cases** — 7-day grace, clock tampering, expired refresh while offline, device revoked mid-shift, multiple tabs refreshing | H | M | Single-flight refresh; store `lastOnlineAuthAt` plus a monotonic counter; a visible "N days offline remaining" badge; device revocation checked on every push; explicit tests for each edge | Shrink the offline grace to 24 h for the defence and re-authenticate on every reconnect; a shorter grace is defensible and still demonstrates the concept | **S1**, resurfaces S2 and S9 (soak) |
| **R3** | **POS thermal printer compatibility with browsers is uneven** — WebUSB is Chromium-only and often blocked on Android; Web Bluetooth needs a BLE-capable printer; ESC/POS over USB varies by vendor | H | M | Demo path is **browser print with `@page { size: 58mm auto }` print CSS** — works everywhere. WebUSB/Web Bluetooth is a feature-detected enhancement behind a settings toggle. Detect support at runtime and always offer the print-dialog fallback. Test on the actual printer in S6 and again in S8 | Print to PDF at 58 mm and show that, plus a photo of a genuine thermal print from an earlier run; or use the vendor's own driver as a system printer (not a browser API). **Never** promise "the browser talks to the printer" in the dissertation | **S8** (slip), **S6** (receipt) |
| **R4** | **Cloud Run cold starts affecting the demo** — 1–3 s container start plus a Neon compute wake-up, right when the examiner is watching | H | M | Client timeout 30 s with retry and backoff; bounded batches; `--min-instances=1` during the defence window; warm the API 10 minutes before presenting; measure and record the cold-start time in S0 and again in S9 | Pre-recorded screen capture of the full demo as an offline fallback; keep a warm tab open and never let the app go idle before the live run; if the network at the venue is the problem, do the entire demo offline (which is the project's own thesis) | **S0** (measure), **S9** (demo) |
| **R5** | **Neon connection limits** — `max Cloud Run instances × MaxConns` exceeding the plan ceiling causes intermittent, baffling connection failures under burst | M | H | `MaxConns = 10`, `--max-instances=3` → a hard ceiling of 30; use the **pooled** endpoint for the API with `QueryExecModeExec`; a startup log line printing the computed ceiling; an alert if the pool waits | Drop `max-instances` to 1 and `MaxConns` to 5 for the demo (a single examiner's traffic is trivial); Neon's pooler absorbs the rest | **S0** (configure), **S9** (load test) |
| **R6** | **Scope creep into pharmacy inventory** — "just a stock column", then batches, then expiry, then a dispensing-reconciliation problem that offline devices cannot solve safely | H | H | `00-ARCHITECTURE.md` §1.2 states it as a non-goal; the `dispenses` table has no quantity-on-hand; the `batch_note` is free text; a code review rule that any PR mentioning "stock level" is rejected | If an examiner insists, answer with the design rationale: recording the dispense event preserves the clinical and billing record, while inventory across offline devices is an unsolved reconciliation problem *and* explicitly future work (§10) | **S6**, resurfaces every later sprint and at the viva |
| **R7** | **Hand-writing volume exceeds available time** — every file written by hand from the mentor's examples, at 8–15 h/week, against a ten-sprint plan | H | H | Vertical slices only; DoR requires tasks ≤ 3 h; T-shirt sizing with a hard XL split rule; the 20 % buffer in §9; cut features in the documented order rather than compressing sprints; start S2 early | Execute the cut list in §9 in order, down to the **never cut** floor (offline sync demo, referral flow, security basics). Deliver S0–S5 beautifully rather than S0–S9 badly | **Every sprint**, most acutely **S5–S8** |
| **R8** | **Losing work with no commit** — a laptop failure, a bad `git reset`, a Dexie schema wipe, or an uncommitted week | M | H | Commit at least daily, push weekly (a rule in `06-GIT-WORKFLOW.md`, not a habit); `main` always deployable; Neon branching for risky migrations; the backup/restore drill rehearsed in S9; never `git push --force` to `main` | Restore from the last push; Neon point-in-time restore into a branch; re-run the sprint's migrations and seeds. Local-only clinical demo data is regenerable from seed scripts — keep seed scripts current | **Continuous**, drill in **S9** |
| **R9** | **Examiners ask about multi-facility** — "what if a district has five PHCs?" | H | M | Single-tenancy is a **stated design decision**, not an oversight (`00-ARCHITECTURE.md` §7.4). Each facility deploys its own instance: own code, own database, own API. `facilities` holds exactly one row, enforced by `uq_facilities_single_org` | Answer with the reasoning, not an apology: a speculative tenant column costs correctness *now* — every query must remember the predicate and every unique constraint must include the column, and forgetting either is a silent cross-tenant leak. A second hospital is a deployment, not a feature flag. Name the migration path for the future | **Viva**, prepared from **S0** onwards |
| **R10** | **The offline soak reveals data loss nobody expected** (a dropped outbox entry, a clobbered pending row, a cursor advanced past an unapplied change) | M | H | Cursor advances only after a committed apply; the applier never overwrites a locally `pending` row; the outbox is transactional with the data write; two-context E2E from S2 | Treat any loss as a release blocker: reduce the pull batch, force a full re-pull by resetting the cursor (a documented recovery action), and add the failing case to the automated suite before fixing it | **S9** (and S2 if the tests are good) |
| **R11** | **Money arithmetic drifts** — a float sneaks in, allocation loses a minor unit, overpayment is swallowed | M | H | `BIGINT` minor units end to end; `int64` in Go; no division on money in TS; `formatMinor` only; largest-remainder allocation; static test forbidding `parseFloat` in billing; allocation reconciles to the payment exactly | Refuse the allocation rather than approximate it; show an explicit unallocated credit; write a one-off SQL reconciliation query and run it in the soak test | **S6**, resurfaces **S7** |
| **R12** | **The service worker serves a stale shell** and the examiner sees an old build or, worse, a broken mix of old shell and new sync protocol | M | M | Version the sync payloads; the server tolerates a client one version behind; an explicit update prompt (never a silent swap); exclude `/r/*` from precache | Hard-reload with "Update on reload" enabled in DevTools before the demo; clear the cache in front of the examiner and let it reinstall from the network — it demonstrates the PWA in the process | **S9** (and any deploy during S3–S8) |
| **R13** | **Referral token or PIN leaks** (URL in logs, in browser history, on a shared slip, in a screenshot) | L | H | Redact tokens in logs with a test asserting it; `Referrer-Policy: no-referrer`; `no-store` on the public API; the public projection is an explicit allow-list; access logging on every attempt; revocation always available | Revoke the affected referral immediately, keep the access log as the forensic record, and disclose it in `05-SECURITY.md`'s residual-risk section rather than hiding it | **S8** |
| **R14** | **Doing the dissertation at the end instead of throughout** — the code works but there is no write-up, no data, no diagrams | M | H | Write the chapter that each sprint enables *in that sprint* (see the milestone table in §8); keep `docs/` current as part of the DoD; export diagrams as you build them; keep an evidence folder per sprint | Reconstruct from `PROJECT-TRACKER.md`, git history, and `docs/evidence/` — which is exactly why every sprint requires recorded evidence and a written demo script | **Every sprint**, critical from **S5** |
| **R15** | **The mentor's examples do not cover a case you hit** (an unfamiliar Go pattern, a Dexie transaction subtlety, a Gin middleware ordering bug) and you stall for days | M | M | Keep a `QUESTIONS.md` and batch questions to the mentor; timebox any block to 2 hours, then write the smallest possible reproduction and move to another task; prefer the boring solution you already understand | Descope the task (see §9) and implement the simplest correct version; an explained simple solution scores better than a copied clever one you cannot defend | **Continuous** |

---

## 7. Testing strategy per sprint

Four test types are used throughout. The mix shifts from unit-heavy early to E2E- and manual-heavy late, because the interesting failures in an offline system only appear when real browsers, real networks and real devices are involved.

| Symbol | Type | Tooling | What it proves |
|---|---|---|---|
| **U** | Unit | `go test`, Vitest | One function's rules: state machines, money arithmetic, validation, formatting |
| **I** | Integration | `go test` against a real Postgres, Vitest against `fake-indexeddb` | The database contract: migrations, indexes, transactions, idempotency, concurrency |
| **E** | E2E | Playwright (multi-context, `setOffline`) | The user's journey across both tiers, including two devices converging |
| **O** | Manual offline | DevTools *Offline* / Playwright + a real laptop with Wi-Fi off | The claim the whole project rests on, verified by a human, not a mock |

### 7.1 Test-type matrix

| Sprint | U | I | E | O | Required E2E flows covered | Coverage focus |
|---|---|---|---|---|---|---|
| **S0** | ✅ config, health handler | ✅ pool + `/healthz` + migrate up/down | ✅ smoke + offline load | ✅ reload offline | — | Pipeline works; the app loads with no network |
| **S1** | ✅ argon2, JWT, session store | ✅ login/refresh/lockout | ✅ per-role nav + guard | ✅ offline login, grace expiry | — | Auth correct in both modes; no secret leaks to logs |
| **S2** | ✅ conflict rules, backoff, atomic write | ✅ push replay, pull paging, append-only | ✅ **two-context convergence**, offline drain | ✅ server stopped mid-demo | — (probe table stands in) | Replication is exactly-once and order-preserving |
| **S3** | ✅ Zod, numbering, duplicates | ✅ number collision, household rules | ✅ register family offline → second context | ✅ family registration + reload offline | **(a) part 1** | Records survive offline and converge |
| **S4** | ✅ state machine + parity fixture, BMI | ✅ append-only vitals, queue snapshot | ✅ register → vitals → queue | ✅ board reload offline, repeat vitals | **(a) complete** | The clinical queue is correct with no network |
| **S5** | ✅ lab state machine, ICD-10 ranking | ✅ replay of results, payment gate | ✅ diagnosis → order → pay → result → review | ✅ three results offline | **(b) part 1** | Orders, results and the payment gate hold |
| **S6** | ✅ allocation, `formatMinor` | ✅ payment idempotency, reversal, derived balance | ✅ prescription → pay → dispense; receipt DOM | ✅ payment + print offline | **(c) complete** | Money is exact, idempotent, and never stored |
| **S7** | ✅ accrual ranges, discharge gate | ✅ concurrent accrual, concurrent bed assignment | ✅ admit → accrue → settle → discharge | ✅ two devices accrue offline | (b)(c) regression | Inpatient charges cannot duplicate or leak |
| **S8** | ✅ PIN, expiry maths, QR payload | ✅ verify outcomes, projection allow-list, log redaction | ✅ **referral → print → public view → expiry → revoke** | ✅ compose offline, issue on reconnect | **(d) complete** | The public door is locked, logged and time-boxed |
| **S9** | ✅ reports, CSV escaping | ✅ audit filters, plan regressions | ✅ **all four flows in one script** + two-day offline soak | ✅ full working day offline, cold PWA start | **(a)(b)(c)(d) all together** | The system is trustworthy under real conditions |

### 7.2 The four end-to-end flows and when each is first provable

| Flow | Path | First provable | Fully automated by |
|---|---|---|---|
| **(a) register → vitals → queue** | `/patients/new` → `/encounters/:id/vitals` → `/queue` | Sprint 3 (register), Sprint 4 (vitals + queue) | Sprint 4 |
| **(b) diagnosis → lab order → payment → result** | `/encounters/:id/consult` → `/lab` → `/cashier` → `/lab/:orderId` → `/doctor/inbox` | Sprint 5 (provisional payment gate) | Sprint 6 (real tariff and ledger) |
| **(c) prescription → payment → dispense** | `/encounters/:id/consult` → `/cashier/invoices` → `/pharmacist/queue` | Sprint 6 | Sprint 6 |
| **(d) referral → print → public view → expiry** | `/referrals` → thermal slip → `/r/:token` → expiry → revoke | Sprint 8 | Sprint 8 |

### 7.3 Rules for the test suite

- **Every flow runs offline where the feature allows it.** If a test only passes with the network on, it is not testing an offline-first system.
- **A failing test stops the sprint.** No green-washing on the day before the demo.
- **Regression suite grows, never shrinks.** Flows (a)–(d) stay in CI from the sprint that introduced them until the end.
- **The offline soak is the acceptance test for the whole project.** It is the single artefact that proves the central claim.
- **Record the numbers.** Cold-start time, search p95, queue render time, pull duration, soak loss/duplication counts — these tables become the evaluation chapter of the dissertation.

---

## 8. Milestones for the academic calendar

Assumes a two-semester final-year project: roughly 22 working weeks, a proposal early, a progress presentation mid-first-semester, a mid-point demo, final submission, and a viva. Shift the weeks to your own calendar, but keep the **ordering** — the dependencies in §5 do not move.

| Milestone | Typical week | Sprints done | What must be demonstrable | Written work due alongside |
|---|---|---|---|---|
| **Project proposal / topic approval** | 1–2 | S0 in progress | A working deployed skeleton: a Pages URL that loads, a `/healthz` that reports a live Neon connection, one migration applied. Nothing clinical yet — and that is the point: *"the pipeline is proven before the features"* | Proposal: problem statement (PHC connectivity, paper records, referral loss), objectives, scope + explicit non-goals, methodology (agile, incremental), tech stack justification |
| **Literature review** | 2–5 | S0, S1 | Login as three roles with different menus, offline login with the 7-day grace, device registration visible in `/admin/users` | Literature review: offline-first architectures (local-first, CRDT vs LWW, outbox patterns), DHIS2/OpenMRS comparisons, mHealth in low-resource PHCs, referral-system literature, the gap this addresses |
| **Progress presentation (first semester)** | 6–8 | S2 (± S3 start) | **The sync demo.** Two browsers converging through Neon, one browser staying correct with the network off, and the conflict log showing a real LWW decision. This is the strongest possible progress demo because it is the research contribution | Methodology chapter: sync protocol, conflict policy justified (append-only facts vs LWW mutable records), derived balances, the write path. Include the sequence diagram |
| **Mid-point demo / interim evaluation** | 9–11 | S3, S4 | The offline clinical core: register a family → vitals → queue → handoff, all with the network off, then converge. Data model walkthrough of `patients`, `households`, `encounters`, `vitals` | Design chapter: ER diagram, sync-column contract, index rationale, the workflow state machine, UI patterns. Testing chapter started with the S0–S4 matrix |
| **Semester break / exams** | 11–13 | buffer | No new features promised. Use it to absorb slippage, or to write up | Keep the tracker updated; no new scope |
| **Second-semester progress check** | 14–16 | S5, S6, S7 | The full clinical and financial loop: diagnosis → lab order → payment → result; prescription → payment → dispense; admission → daily accrual → blocked discharge → settle → discharge. Receipt printing on 58 mm | Implementation chapter: lab and billing modules, idempotency design, the derived-balance rule, admission accrual idempotency. Evaluation chapter started |
| **Referral feature sign-off** | 17–18 | S8 | The referral slip printed on thermal paper, the QR scanned by a phone over mobile data, the 48-hour expiry enforced, the PIN lockout, the access log, revocation | Security chapter: token/PIN/threat model, access logging, rate limiting, offline-auth risks and residual risks |
| **Final submission** | 19–21 | S9 | Everything: all four flows in one automated run, the offline soak results with numbers, dashboards and reports per role, the audit viewer, the conflict dashboard, the PWA installed, and the restore drill written up | Full dissertation: abstract, introduction, literature review, methodology, design, implementation, **evaluation with measured results**, limitations, future work, references, appendices (screenshots, demo script, user manual, API contract) |
| **Viva / defence** | 22 | — | The 12-minute demo in `DEMO-SCRIPT.md`, all three fallbacks loaded, one deliberate failure shown and explained (a revoked referral, a payment replay, an illegal transition) | Defence slides: the one-sentence architecture, the offline demo, the conflict evidence, the honest limitations, and the future-work list from §10 |

**Two calendar rules that save projects.**

1. **Never let a milestone week be the first time a feature is demoed.** Every milestone demo is a sprint demo you have already performed. If it is new on milestone day, it will fail on milestone day.
2. **Write the chapter in the sprint that produces the evidence.** The design chapter belongs to S2–S4, the implementation chapter to S5–S7, the security chapter to S8, the evaluation chapter to S9. Reconstructing them at week 20 costs more than writing them as you go, and the writing is what is actually graded.

---

## 9. Buffer and contingency guidance

### 9.1 Assume ~20 % slippage

The plan assumes you will lose about **one week in five** to illness, coursework deadlines, a broken environment, a mentor unavailable for a question, or simply an estimate that was wrong. That is 20 % of the calendar — roughly **two extra weeks** across a 22-week project — and it is already reflected in the trailing buffer inside each sprint (§1.4) and in the cut list below. It is not a sign of failure; it is the plan working.

Also assume **one sprint's worth of work will be lost to a single bad decision** — a sync design that has to be redone, a schema that has to be migrated twice. The mitigation is not more buffer; it is building the risky thing first (S2) while there is still time to recover.

### 9.2 What to cut first — in priority order

Cut from the **top** of this list. Each item is genuinely nice to have, and none of them is required for the project's core claim.

| Order | Cut | Why it can go | Sprint affected |
|---|---|---|---|
| 1 | **Role-specific dashboards** beyond two or three roles; the accountant's and admin's views drop first | Reports are presentation; the data is already there and derivable | S9 |
| 2 | **CSV export** on reports | A screen showing the numbers is enough evidence | S9 |
| 3 | **PWA install/update prompt** | Nice demonstration, not a claim. The service worker offline shell must stay (that is required); the install prompt need not | S9 |
| 4 | **Audit log viewer UI** | The `audit_logs` table is the requirement; a filterable screen is a convenience. Show the table via SQL | S9 |
| 5 | **WebUSB / Web Bluetooth thermal printing** | The print-CSS fallback is the real demo path anyway (R3). Cut the enhancement, keep the fallback | S6, S8 |
| 6 | **Ward/transfer/absconded flows** | Keep admit + accrue + discharge; cut transfer and absconded to future work | S7 |
| 7 | **Partial dispensing and partial payments** | Keep the simple full-payment and full-dispense paths; partials are an enhancement. **The schema should still support them** (allocations and dispense quantities exist) even if the UI does not | S6 |
| 8 | **Duplicate detection** | Excellent feature, not a requirement. Keep a simple exact-match warning if possible; cut the fuzzy scoring | S3 |
| 9 | **Lab result interpretation flags / reference ranges** | Keep the value and unit; interpretation is a presentation nicety | S5 |
| 10 | **Referral urgency levels, referral register filters, reprint history** | Keep issue → print → view → verify → expire → revoke | S8 |
| 11 | **ICD-10 code search depth** | Keep the seeded 500 codes and a free-text fallback; do not import the full dataset | S5 |

Two cuts that **look** attractive and should be refused:

- **Cutting the conflict log** (`sync_conflicts`). It is your evidence. It is also cheap.
- **Cutting tests to save time.** Cut a feature instead. An untested billing path is a demo that dies.

### 9.3 What must never be cut

These are the project. Losing any of them loses marks that no amount of polish recovers.

1. **The offline sync demonstration.** Write locally with the network off, reload offline, reconnect, converge across two devices, show the conflict log. This is the research contribution and the answer to the first question an examiner will ask. If only one thing works at the viva, it must be this.
2. **The referral flow.** Doctor composes → thermal slip with QR → public read-only view with the PIN → 48-hour expiry enforced in the query → access log → revocation. This is the named feature in the project title.
3. **Security basics.** Argon2id password hashing, short-lived JWT + rotating refresh, device keypairs, RBAC enforced **server-side** in the service layer, the referral token + PIN + expiry + access log, and no secret in a log line. Simplify *nothing* here.
4. **The sync column contract** (`id`, `revision`, `server_rev`, `sync_state`, `deleted_at`, `created_at`, `updated_at`, `created_by`, `updated_by`) on every syncable table, and **client-generated UUIDv7** primary keys. Note there is no `facility_id` — this is a single-tenant system.
5. **Append-only clinical facts and last-write-wins for mutable records**, with idempotency for payments. This is the design argument, not an implementation detail.
6. **Derived balances — no stored balance column anywhere.** One source of truth for money.
7. **Migrations with working `Down`.** An examiner may ask you to roll back. Be able to.
8. **Hand-written, understood code.** A working feature you cannot explain is worth less than a smaller one you can defend line by line.

### 9.4 Contingency playbooks

| Situation | Response |
|---|---|
| **A sprint is 50 % behind at the mid-sprint checkpoint** | Cut the sprint's last deliverable immediately (it is already ordered by priority), finish the first two, demo them, and move the cut item to the next sprint *explicitly*, naming it in the retro. Never let a cut item become invisible. |
| **A sprint is entirely lost** (illness, crisis) | Do not compress the next sprint. Take the trailing buffer, then cut from §9.2 top-down, then — if still short — merge the two remaining small sprints (S6 and S7 are the mergeable pair, since both are billing-shaped). Never merge S8 into anything: the referral flow needs a full sprint on its own. |
| **Sync proves harder than a two-week sprint** | Ship the row-level LWW fallback at the end of S2, demo it, and schedule field-level LWW as the **first task of S3**. The protocol document describes the intended design; the tracker records the temporary simplification. |
| **The printer will not work on demo day** | Use the print-to-PDF 58 mm fallback, show a photograph of a real thermal print, and scan the on-screen QR with the phone. The feature is the token/PIN/expiry/QR flow, not the hardware. Say so plainly. |
| **Neon is slow, down, or rate-limited at the viva** | Run the entire demo **offline** on the already-synced client. That is the project's own thesis, and it turns an outage into the most persuasive two minutes of the defence. |
| **You are asked for something out of scope** (stock levels, insurance, SMS, multi-facility UI) | Answer with §10 and the design rationale, not an apology: the schema is ready, the reconciliation problem is real, and it is explicitly future work. |
| **A migration has already run on Neon and is wrong** | Never edit a migration that has run. Add a new one that fixes it forward, and run the restore drill if data is affected. |

---

## 10. Post-project / future work

Suitable verbatim for the dissertation's *Future Work* section. Each item should be stated with why it is future work, not merely that it is missing.

| # | Item | Why it is future work | What the current design already provides |
|---|---|---|---|
| 1 | **Multi-facility / district deployment** | The system deliberately serves **one** facility — a single-tenant design, each site deploying its own instance. A district tier raises tenant administration, cross-facility patient identity resolution, and district-level reporting; it is a project in itself | The migration path is understood and documented (`00-ARCHITECTURE.md` §7.4): add a tenant table, add the column with a backfill, widen the unique constraints, add the predicate to the query layer. **Deliberately deferred** — a speculative tenant column costs real correctness today for a requirement that does not exist |
| 2 | **Pharmacy inventory management** | Offline devices cannot safely reconcile a shared stock count; a wrong stock number is a clinical safety issue that the current scope does not attempt to solve | The `dispenses` table records every dispense event with drug, quantity, prescriber, dispenser and time — the raw material any future stock ledger would need |
| 3 | **FHIR / HL7 interoperability** | The referral payload is a bespoke JSON shape. Standards compliance requires terminology mapping (ICD-10 → SNOMED CT), resource modelling (Patient, Encounter, DiagnosticReport, MedicationRequest), and a conformance statement | Diagnoses already carry ICD-10 codes; every clinical table is a normalised, well-identified resource with UUIDv7 keys and audit columns — a natural mapping source |
| 4 | **SMS / email appointment and referral reminders** | Requires an external gateway, per-message cost, consent handling, and a delivery-status model — plus a decision about patient phone-number quality | Patient phone numbers are captured at registration; referral destination contacts are recorded; the `referral_access_log` already shows whether a referral was ever opened, which is exactly the signal a reminder would use |
| 5 | **Offline-capable mobile application** (native or installable PWA with background sync) | The current client is a responsive web app; background sync that survives the tab closing, camera-based QR capture, and Bluetooth printing all need a native or deeper PWA capability | The sync protocol is transport-agnostic and versioned, and the client is already a PWA with a service worker and IndexedDB — a second client would speak the same `/sync/push` and `/sync/pull` contract |
| 6 | **AI-assisted triage and decision support** | Requires a validated model, a labelled dataset from the facility, clinical governance, and explainability. Shipping an unvalidated model into a PHC would be irresponsible | Structured, timestamped vitals, diagnoses with ICD-10 codes, lab results with units and reference ranges, and outcomes (admission, discharge, referral) — a clean, growing training set in the exact shape a triage model consumes |
| 7 | **NHIS / health-insurance claims integration** | Depends on an external scheme's API, eligibility checks, pre-authorisation workflows, claim adjudication and reconciliation — none of which is under this project's control | The invoice/charge/allocation ledger with immutable price snapshots and per-patient and per-household accounts is precisely the claim-line structure an insurer requires; derived balances mean an eligibility check would not require a schema change |
| 8 | **Analytics dashboards and population health reporting** | Needs an aggregate/warehouse layer, de-identification, and district-level indicators (DHIS2-style) that only make sense with many facilities and months of data | Every clinical event is append-only and server-side in Postgres with `created_at` and actor columns — a straightforward extract, transform and load into an analytics store without touching the operational schema |
| 9 | **Snapshot/compaction for the sync change log** | A brand-new device currently pulls the full change history; that is acceptable at one facility for one year and will not be at scale | `sync_changes` is append-only with a monotonic `server_rev` per facility, so a snapshot-at-revision plus a delta from that revision is a contained addition to the existing pull protocol |
| 10 | **Shared rate-limit and lockout store** | Login and PIN rate limits are per Cloud Run instance and therefore approximate across a scaled deployment | The limit points are already isolated in middleware and in `referral_service`, so swapping the in-memory bucket for Redis or a Postgres-backed counter is a contained change |
| 11 | **Server-driven report generation and scheduled exports** | Reports are computed client-side today so they work offline; a scheduled district report needs a server job | The report SQL already lives in named views (`v_daily_collections`, `v_revenue_by_category`, `v_outstanding_balances`, `v_admissions_census`), so a scheduled job can reuse it unchanged |
| 12 | **Full offline-first device provisioning and de-provisioning workflow** | Today a device is registered on first login and revoked by an admin. Fleet-scale provisioning (enrolment codes, remote wipe, device health reporting) is an operational concern beyond one PHC | ECDSA P-256 device keypairs are already registered, revocable, and required on every sync push — the enforcement point a provisioning workflow would build on |

### 10.1 One-paragraph summary for the dissertation

> This project delivers an offline-capable patient records and referral management system for a single primary healthcare centre, in which every workstation holds a full local replica and continues to function — registration, triage, consultation, laboratory, pharmacy, billing, admission and referral — with no network connection, reconciling with a central PostgreSQL database through a custom push/pull sync engine with append-only semantics for clinical facts and last-write-wins resolution for mutable records. Future work extends the system in four directions: **scale** (multi-facility deployment, sync log compaction, shared rate limiting), **interoperability** (FHIR/HL7, NHIS claims), **reach** (native mobile clients, SMS reminders), and **intelligence** (inventory reconciliation, AI-assisted triage, population-health analytics) — each of which the present schema and protocol were deliberately designed not to preclude.

---

*End of roadmap. Update it at the end of every sprint; the version in `PROJECT-TRACKER.md` is the one that reflects reality.*
