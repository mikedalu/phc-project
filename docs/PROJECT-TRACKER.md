# Project Progress Tracker

> **Project:** Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres
> **Method:** Agile, sprint-based, incremental delivery
> **How to use this file:** Update it at the **end of every working session**, not at the end of every sprint. A tracker that is only accurate once a fortnight is a tracker nobody trusts.

---

## 1. Status board

**Current sprint:** `Sprint 1 — Auth, roles & device identity`
**Sprint start:** _fill in_ · **Sprint end:** _fill in_
**Overall status:** 🟢 Sprint 0 complete and deployed. Sprint 1 in progress — schema done, password hashing done
**Last updated:** Sprint 0, after Neon connection + module path

| Legend | Meaning |
|---|---|
| ✅ | Done and verified |
| 🟡 | In progress |
| 🔴 | Blocked |
| ⬜ | Not started |
| ⏭️ | Deliberately deferred (with a reason recorded) |

---

## 2. Sprint dashboard

Update this table the moment a sprint closes. It is the first thing to show your supervisor.

| Sprint | Goal | Status | Started | Completed | Planned % | Actual % | Notes |
|---|---|---|---|---|---|---|---|
| 0 | Foundations & walking skeleton | ✅ | | | 30 | 97% | Deployed to Cloud Run + Cloudflare Pages. Only 0.24 (browser confirmation) pending |
| 1 | Auth, roles & device identity | 🟡 | | | 17 | 60% | Schema, password, JWT, domain, repos and the full auth service done. **31 tests.** Handlers + middleware + device keys next |
| 2 | Sync engine | ⬜ | | | | | |
| 3 | Patient registration (individual & family) | ⬜ | | | | | |
| 4 | Queue, vitals & triage | ⬜ | | | | | |
| 5 | Consultation, diagnosis & lab | ⬜ | | | | | |
| 6 | Billing, cashier & pharmacy | ⬜ | | | | | |
| 7 | Admission & inpatient management | ⬜ | | | | | |
| 8 | Referral system & public access | ⬜ | | | | | |
| 9 | Hardening, reporting & handover | ⬜ | | | | | |

---

## 3. Definition of Done

A task is **not** done because the code runs on your machine. It is done when every box is ticked.

### Per task
- [ ] Code written by hand, and I can explain every line without notes
- [ ] Follows `04-CODING-STANDARDS.md` (naming, layering, error handling, no `any`)
- [ ] Works with the network **off** if it touches clinical or financial data
- [ ] Sync columns present if it introduces a table; entry added to `sync_table_scopes` if syncable
- [ ] Migration written with a working `Down`; `up → down → up` verified
- [ ] Test written (unit for logic, integration for endpoints, E2E for user flows)
- [ ] Audit log written for any state change
- [ ] Relevant doc updated in the **same commit** (API contract, data model, this tracker)
- [ ] Committed with a Conventional Commit message

### Per sprint
- [ ] Every sprint deliverable in `08-ROADMAP.md` is complete
- [ ] The demo script runs end to end **without touching the database by hand**
- [ ] Demo rehearsed once, timed, and delivered to a real person
- [ ] Sprint retrospective written below
- [ ] Tracker updated, sprint marked ✅
- [ ] `main` is deployable (Cloud Run + Cloudflare Pages both green)

---

## 4. Sprint 0 — Foundations & Walking Skeleton

**Goal:** retire deployment risk on day one by proving a request can travel from a browser → Cloudflare Pages → Cloud Run → Neon → back.

| # | Task | Type | Status | Notes |
|---|---|---|---|---|
| 0.1 | Create repo structure (`api/`, `web/`, `docs/`, `scripts/`) | INFRA | ✅ | + `.gitignore`, `.editorconfig`, `.env.example`, `Makefile` |
| 0.2 | `go mod init` + first dependencies | BE | ✅ | `github.com/mikedalu/phc-records/api` |
| 0.3 | Neon project created; pooled + direct URLs recorded | INFRA | ✅ | Project `phc_db` @ `eu-central-1`. Both URLs verified; migrations applied |
| 0.4 | `00001_create_facilities.sql` | DB | ✅ | Verified `up → down → up` |
| 0.5 | `00002_create_users.sql` | DB | ✅ | `citext` extension, partial unique indexes |
| 0.6 | `00003_seed_default_facility.sql` | DB | ✅ | Idempotent (`ON CONFLICT DO NOTHING`) |
| 0.7 | goose wired to `MIGRATION_DATABASE_URL` | BE | ✅ | Makefile sources `.env` and passes the URL positionally (`goose -env` proved unreliable). Guard added for the unquoted-`&` trap |
| 0.8 | Config loader + `.env.example` | BE | ✅ | Fail-fast validation, `Warnings()` for dev |
| 0.9 | pgx pool with `QueryExecModeExec` | BE | ✅ | The Neon pooler trap, plus `WithTx` and SQLSTATE helpers |
| 0.10 | Gin server binding `$PORT` | BE | ✅ | The Cloud Run trap, plus graceful SIGTERM shutdown |
| 0.11 | `GET /health` + `GET /ready` | BE | ✅ | Verified live: 200 / 200 with a real DB ping |
| 0.12 | `GET /meta` | BE | ✅ | Verified live: `facilityCount: 1` through a real query |
| 0.13 | CORS middleware with an explicit allow-list | BE | ✅ | Verified live: 204 allowed, **403 disallowed** |
| 0.14 | Structured logging (zerolog) + request ID | BE | ✅ | Done early — request-ID middleware + envelope in `httpx` |
| 0.15 | Dockerfile (multi-stage, distroless) | INFRA | ✅ | 20.5 MB image, static binary, runs as `nonroot`. Verified by building and running it |
| 0.16 | Deploy API to Cloud Run | INFRA | ✅ | **Deployed** to Cloud Run, `europe-west1`. Env vars via `--env-vars-file` (no Secret Manager) |
| 0.17 | Vite + React + TS + Tailwind v4 scaffold | FE | ✅ | Vite 8.3.4 build in **773 ms**; 151 KB gzipped |
| 0.18 | Design tokens in `styles/tokens.css` | FE | ✅ | All 12 custom utilities verified present in the built CSS, incl. `tnum` |
| 0.19 | TanStack Router + app shell layout | FE | ✅ | Code-based routing (no generated route tree). Light sidebar, sticky top bar |
| 0.20 | Dexie initialised at version 1 | FE | ✅ | 4 domain tables + `outbox`, `syncMeta`, `conflicts`, `numberCounters` |
| 0.21 | API client with error-envelope handling | FE | ✅ | `ApiError` with `isRetryable`; offline distinguished from a server rejection |
| 0.22 | Health-check page rendering `GET /meta` | FE | ✅ | 4 live checks incl. a Dexie probe; honest about what is not built |
| 0.23 | Deploy web to Cloudflare Pages | INFRA | ✅ | Deployed by you. Pending confirmation that the build carries the Cloud Run URL (see 0.24) |
| 0.24 | **End-to-end proof**: browser → API → Neon → browser | INFRA | 🟡 | Both halves deployed. **Needs one browser check:** Network tab must show a request to the `.run.app` host, not `localhost` |
| 0.25 | Vitest + Playwright configured | TEST | ⬜ | Batch 4 |
| 0.26 | `CLAUDE.md` + docs committed | DOCS | ✅ | Done during Sprint 0 planning |
| 0.27 | **ADDED** `00004_seed_admin_user.sql` with a real, verified Argon2id hash | DB | ✅ | Not in the original plan; a fresh install needs a usable login |
| 0.28 | **ADDED** drop the FKs on `facilities.created_by` / `updated_by` | DB | ✅ | Resolved a genuine circular FK — see decision **D13** |
| 0.29 | **ADDED** `git init` — the workspace was not a git repo, so `.gitignore` protected nothing | INFRA | ✅ | `.env` confirmed ignored; `.env.example` tracked |
| 0.30 | **ADDED** fix the unquoted-`&` `.env` trap + Makefile guard | INFRA | ✅ | See `00-ARCHITECTURE.md` §3.7 |

**Demo script:** open the deployed Pages URL, see the health page report the API version, server time, and a live row count read from Neon. Turn off the network, reload, and see the app shell still load from the service worker.

**Sprint-specific DoD:** the URLs in this file are real deployed URLs, and a fresh clone can be run locally by following `README.md` alone.

### Sprint 0 retrospective
- **Went well:**
- **Went badly:**
- **Change next sprint:**

---

## 5. Sprint backlogs (fill in as each sprint begins)

### Sprint 1 — Auth, roles & device identity

| # | Task | Type | Status |
|---|---|---|---|
| 1.1 | `00005_create_user_roles.sql` | DB | ✅ | Verified: invalid role rejected, duplicate active role rejected, re-grant after revoke allowed, revocation requires a reason |
| 1.2 | `00006_create_devices.sql` | DB | ✅ | `registered_by` nullable (shared workstation). Duplicate fingerprint rejected |
| 1.3 | `00007_create_refresh_tokens.sql` | DB | ✅ | Server-only, no sync columns. Reuse-detection family index. Past expiry rejected |
| 1.4 | `00008_seed_admin_role.sql` | DB | ✅ | **NOT in the original plan** — the seeded admin had no role and could log in but authorise nothing |
| 1.5 | Argon2id hashing + verification helpers | BE | ✅ | **10 tests pass.** 78 ms hash / 73 ms verify at 64 MiB. Includes a 1 GiB ceiling against a DoS via tampered cost params |
| 1.6 | JWT issue/verify (access) | BE | ✅ | **9 tests**, incl. a verified `alg=none` forgery (empty signature, payload claiming admin) |
| 1.7 | Login service with dual rate limiting | BE | ✅ | Service done and tested (lockout, dummy-hash timing, no enumeration). **HTTP handler still to write** |
| 1.8 | Refresh rotation with reuse detection | BE | ✅ | **Service done, tested against real Postgres.** Reuse revokes the whole family, verified end to end |
| 1.9 | Logout service | BE | ✅ | Idempotent by design — a retried logout after a lost response must not error |
| 1.10 | `GET /auth/session` | BE | ⬜ |
| 1.11 | Auth middleware + role middleware | BE | ⬜ | Batch 4 |
| 1.12 | ECDSA P-256 device keypair + registration | FE | ⬜ |
| 1.13 | Login screen | FE | ⬜ |
| 1.14 | Session store (Zustand) + token refresh interceptor | FE | ⬜ |
| 1.15 | Role-based nav + route guards | FE | ⬜ |
| 1.16 | Configurable offline grace window + `OFFLINE_POLICY` logic | FE | ⬜ |
| 1.17 | Tests: login, refresh reuse, RBAC | TEST | 🟡 | 13 integration tests + 9 token + 9 password = **31**. RBAC tests come with the middleware |

### Sprint 2 — Sync engine

> **Build this before the features that depend on it.** Retrofitting sync onto a finished app means rewriting every screen's data access. Building it first means every subsequent feature is offline-capable for free. This is the single most important sequencing decision in the project.

| # | Task | Type | Status |
|---|---|---|---|
| 2.1 | `00008_create_sync_infrastructure.sql` (+ scope seeds) | DB | ⬜ |
| 2.2 | `localWrite` / `localDelete` helpers | FE | ⬜ |
| 2.3 | Outbox + syncMeta + conflicts Dexie stores | FE | ⬜ |
| 2.4 | Device request signing | FE | ⬜ |
| 2.5 | Push batch builder with causal ordering | FE | ⬜ |
| 2.6 | Push result handler (all six statuses) | FE | ⬜ |
| 2.7 | Backoff scheduler + single-flight guard | FE | ⬜ |
| 2.8 | Pull applier + cursor commit | FE | ⬜ |
| 2.9 | Reconnect/online detection + debounce | FE | ⬜ |
| 2.10 | Bootstrap via `/sync/snapshot` | FE | ⬜ |
| 2.11 | `SyncStatusIndicator` + per-row `SyncBadge` | FE | ⬜ |
| 2.12 | `POST /sync/push` | BE | ⬜ |
| 2.13 | `GET /sync/pull` with `REPEATABLE READ` | BE | ⬜ |
| 2.14 | `GET /sync/snapshot` | BE | ⬜ |
| 2.15 | Conflict resolver (append-only + LWW) | BE | ⬜ |
| 2.16 | Change-log writer on every mutation | BE | ⬜ |
| 2.17 | Redaction layer in the pull builder | BE | ⬜ |
| 2.18 | Change-log retention + `fullResyncRequired` | BE | ⬜ |
| 2.19 | Tests: two-browser sync, offline soak, idempotency | TEST | ⬜ |

### Sprint 3 — Patient registration
| # | Task | Type | Status |
|---|---|---|---|
| 3.1 | `00009_create_households.sql` | DB | ⬜ |
| 3.2 | `00010_create_patients.sql` | DB | ⬜ |
| 3.3 | Patient + household repositories | BE | ⬜ |
| 3.4 | Patient service (number generation, age/DOB rules) | BE | ⬜ |
| 3.5 | Patient CRUD endpoints | BE | ⬜ |
| 3.6 | Family registration endpoint (transactional) | BE | ⬜ |
| 3.7 | Duplicate detection (fuzzy match) | BE | ⬜ |
| 3.8 | Patient list + Ctrl+K search | FE | ⬜ |
| 3.9 | Registration form (single patient) | FE | ⬜ |
| 3.10 | Family registration form (dynamic member list) | FE | ⬜ |
| 3.11 | Patient profile + history timeline | FE | ⬜ |
| 3.12 | Household view with derived balance | FE | ⬜ |
| 3.13 | Tests: registration offline → sync → visible elsewhere | TEST | ⬜ |

### Sprint 4 — Queue, vitals & triage
| # | Task | Type | Status |
|---|---|---|---|
| 4.1 | `00011_create_encounters.sql` | DB | ⬜ |
| 4.2 | `00012_create_vitals.sql` | DB | ⬜ |
| 4.3 | Workflow state machine + transition validation | BE | ⬜ |
| 4.4 | Encounter + vitals endpoints | BE | ⬜ |
| 4.5 | Queue query with role-based stage filtering | BE | ⬜ |
| 4.6 | `VitalsForm` (keyboard-first, all 5 in a row) | FE | ⬜ |
| 4.7 | Queue board (stage columns, live updates) | FE | ⬜ |
| 4.8 | Encounter detail with vitals trend | FE | ⬜ |
| 4.9 | Tests: register → vitals → queue E2E | TEST | ⬜ |

### Sprint 5 — Consultation, diagnosis & lab
| # | Task | Type | Status |
|---|---|---|---|
| 5.1 | `00013`–`00015` diagnoses, lab_orders, lab_results | DB | ⬜ |
| 5.2 | Diagnosis + lab order services | BE | ⬜ |
| 5.3 | Lab order creation with invoice coupling | BE | ⬜ |
| 5.4 | Lab worklist query (paid-only) | BE | ⬜ |
| 5.5 | Consultation split view | FE | ⬜ |
| 5.6 | ICD-10 search (bundled list) | FE | ⬜ |
| 5.7 | Lab worklist + result entry screens | FE | ⬜ |
| 5.8 | Tests: diagnosis → order → pay → result E2E | TEST | ⬜ |

### Sprint 6 — Billing, cashier & pharmacy
| # | Task | Type | Status |
|---|---|---|---|
| 6.1 | `00016`–`00023` billing, prescriptions, dispenses, audit, views | DB | ⬜ |
| 6.2 | Tariff service + price history | BE | ⬜ |
| 6.3 | Invoice/charge creation service | BE | ⬜ |
| 6.4 | Payment service with allocation + idempotency | BE | ⬜ |
| 6.5 | Payment reversal service | BE | ⬜ |
| 6.6 | Balance views + balance endpoints | BE | ⬜ |
| 6.7 | Prescription + dispense endpoints | BE | ⬜ |
| 6.8 | Audit log writer on every mutation | BE | ⬜ |
| 6.9 | Cashier screen (outstanding, collect, change) | FE | ⬜ |
| 6.10 | POS receipt print view (58/80mm) | FE | ⬜ |
| 6.11 | Pharmacist dispensing queue | FE | ⬜ |
| 6.12 | Daily collections report | FE | ⬜ |
| 6.13 | Tests: prescription → pay → dispense; double-payment | TEST | ⬜ |

### Sprint 7 — Admission & inpatient
| # | Task | Type | Status |
|---|---|---|---|
| 7.1 | `00024`–`00027` admissions, beds, ward charges, notes | DB | ⬜ |
| 7.2 | Admission service + bed allocation | BE | ⬜ |
| 7.3 | Daily accrual job (idempotent) | BE | ⬜ |
| 7.4 | Discharge with balance gate | BE | ⬜ |
| 7.5 | Admission screen + ward board | FE | ⬜ |
| 7.6 | Daily charge ledger view | FE | ⬜ |
| 7.7 | Discharge summary + print | FE | ⬜ |
| 7.8 | Tests: admit → daily charge → discharge gate | TEST | ⬜ |

### Sprint 8 — Referral system
| # | Task | Type | Status |
|---|---|---|---|
| 8.1 | `00028`–`00029` referrals, access log | DB | ⬜ |
| 8.2 | Token + PIN generation, hashing, encryption | BE | ⬜ |
| 8.3 | Referral service (draft/issue/revoke/reprint) | BE | ⬜ |
| 8.4 | Server-side QR generation | BE | ⬜ |
| 8.5 | Public endpoints with rate limiting | BE | ⬜ |
| 8.6 | Referral composer screen | FE | ⬜ |
| 8.7 | POS referral slip with QR (print CSS) | FE | ⬜ |
| 8.8 | Public `/r/:token` view + PIN entry | FE | ⬜ |
| 8.9 | Acknowledge flow | FE | ⬜ |
| 8.10 | Access log viewer for doctors | FE | ⬜ |
| 8.11 | Tests: create → print → scan → PIN → expiry | TEST | ⬜ |

### Sprint 9 — Hardening, reporting & handover
| # | Task | Type | Status |
|---|---|---|---|
| 9.1 | `00030`–`00031` number sequences, performance indexes | DB | ⬜ |
| 9.2 | Per-field LWW upgrade for `patients` | BE | ⬜ |
| 9.3 | Reports per role | BE/FE | ⬜ |
| 9.4 | Audit log viewer | FE | ⬜ |
| 9.5 | Admin dashboard (users, devices, conflicts, sync health) | FE | ⬜ |
| 9.6 | Conflict resolution screen | FE | ⬜ |
| 9.7 | Full offline soak test (multi-hour) | TEST | ⬜ |
| 9.8 | Security checklist from `05-SECURITY.md` | TEST | ⬜ |
| 9.9 | Performance pass on every hot query | BE | ⬜ |
| 9.10 | PWA install + stale-shell update prompt | FE | ⬜ |
| 9.11 | Backup/restore drill (Neon PITR) | INFRA | ⬜ |
| 9.12 | User manual | DOCS | ⬜ |
| 9.13 | Final demo script + rehearsal | DOCS | ⬜ |
| 9.14 | Dissertation write-up support | DOCS | ⬜ |

---

## 6. Test coverage tracking

| Test type | Sprint 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 |
|---|---|---|---|---|---|---|---|---|---|---|
| Go unit | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| Go integration | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| FE unit (Vitest) | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| E2E (Playwright) | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| Offline manual test | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |

### The four E2E flows that must exist

| # | Flow | Covered | Sprint |
|---|---|---|---|
| 1 | Register → vitals → queue | ⬜ | 4 |
| 2 | Diagnosis → lab order → payment → result | ⬜ | 5–6 |
| 3 | Prescription → payment → dispense | ⬜ | 6 |
| 4 | Referral → print → public view → expiry | ⬜ | 8 |

---

## 7. Risk log

Review this at the start of every sprint. Update probability and impact honestly — a risk you have stopped worrying about is not the same as a risk that has gone away.

| # | Risk | Prob | Impact | Mitigation | Status |
|---|---|---|---|---|---|
| R1 | Sync conflicts prove harder than expected | High | High | Build sync in Sprint 2, before features; whole-row LWW first, per-field in Sprint 9; `sync_conflicts` gives full visibility | 🟡 |
| R2 | POS printer not reachable from the browser | Med | Med | Print-CSS fallback via the OS print dialog **always** works; test on real hardware in Sprint 6, not Sprint 8 | ⬜ |
| R3 | Cloud Run cold start ruins the live demo | High | Med | `--min-instances=1` on demo day; a "warm-up" ping in the demo script | ⬜ |
| R4 | Neon connection limit hit under concurrency | Med | High | Cap `MaxConns` and Cloud Run `--max-instances`; monitor in Sprint 9 | ⬜ |
| R5 | Scope creep into pharmacy inventory | High | Med | Explicitly out of scope in `00-ARCHITECTURE.md` §1.2; deflect to "future work" | ⬜ |
| R6 | Hand-writing volume exceeds available time | High | High | Cut in priority order (`08-ROADMAP.md` §9); never cut offline demo, referral flow, or security basics | ⬜ |
| R7 | Work lost with no commit | Med | High | Commit at every DoD tick; push daily; feature branches | ⬜ |
| R8 | Examiners ask about multi-facility | Med | Low | Single-tenancy is a **stated design decision** (§`00-ARCHITECTURE.md` §7.4): each facility deploys its own instance, and `uq_facilities_single_org` enforces one row. Answer with the reasoning plus the known migration path — not an apology | ⬜ |
| R9 | Aged device clock corrupts ordering | Low | High | ±5 min signature window with an explicit `CLOCK_SKEW` error and a UI prompt | ⬜ |
| R10 | Service worker serves a stale shell after deploy | Med | Med | Versioned payloads; server tolerates one version behind; reload prompt | ⬜ |
| R11 | A migration is edited after being applied | Low | High | Discipline rule in `01-DATA-MODEL.md` §13; test `up → down → up` | ⬜ |
| R12 | **Sync built after the features instead of before** | Med | Critical | Sprint 2 is non-negotiable and comes before all feature sprints | 🔴 |

**Escalated risks:** R12 — if the schedule is squeezed and someone suggests "do sync later", that is the moment the project is most likely to fail. Retrofitting offline-first onto a network-first app means rewriting every screen.

---

## 8. Decision log

Record every decision that would be expensive to reverse. This is also the raw material for your dissertation's methodology chapter.

| # | Date | Decision | Alternatives | Rationale | Sprint |
|---|---|---|---|---|---|
| D1 | | Offline-first with a full local replica | Read-cache; online-only | A PHC cannot stop working when the link drops | 0 |
| D2 | | Client-generated UUIDv7 primary keys | Server serials; UUIDv4 | Offline inserts need no round-trip; v7 is index-friendly | 0 |
| D3 | | Append-only clinical facts, LWW mutable records | Full conflict resolution UI; optimistic locking | Vitals are observations, not edits; a resolution screen is a sprint of its own | 0 |
| D4 | | Derived balances, no stored balance column | Stored balance; paid-flag on charges | A stored balance is a second source of truth that offline devices desynchronise | 0 |
| D5 | | Pharmacy inventory out of scope | Batch/expiry tracking | Offline stock reconciliation is unsafe without a single writer | 0 |
| D6 | | Device keypairs in addition to JWTs | JWT only | A replicated client is a data store; loss must be revocable per device | 0 |
| D7 | | ECDSA P-256 for device signatures | Ed25519 | Universal `crypto.subtle` support on old PHC workstations | 0 |
| D8 | | Refresh token (Bearer) instead of cookies | HttpOnly cookies | Cross-origin SPA + API; the token must survive offline | 0 |
| D9 | | Referral token + 6-digit PIN + 48h expiry | Token only; login required | The link is unauthenticated by necessity, so defence in depth | 0 |
| D10 | | **Single tenant — no `facility_id` anywhere** | Multi-tenant with `facility_id` on every table | Each facility deploys its own copy. A speculative tenant column costs correctness *now*: every query needs the predicate and every unique constraint needs the column, and forgetting either is a silent cross-tenant leak. This also eliminated the schema's only circular FK. Superseded the earlier "facility_id everywhere" decision | 0 |
| D11 | | Hand-written SQL over `sqlc` | sqlc codegen | Explicit SQL is faster to debug while learning | 0 |
| D12 | | `CHECK` constraints over native `ENUM` types | Postgres `ENUM` | Altering a native enum is painful; offline clients must tolerate unknown values | 0 |
| D13 | | `facilities.created_by`/`updated_by` are **nullable with no FK** | NOT NULL FK; deferred constraint; seed reordering | Superseded by D10, which removed the cause. In the multi-tenant draft, `users.facility_id → facilities` and `facilities.created_by → users` were **circular** — neither table could be seeded first, and the migration failed with SQLSTATE 23503. Removing tenancy deleted `users.facility_id` and the cycle with it. Kept as a record because "we removed the constraint rather than working around it" is the stronger engineering answer | 0 |
| D14 | | Seed a **real, verified** Argon2id hash for the bootstrap admin | Placeholder hash; no seed | A fresh install needs a working login. The hash was generated and round-trip verified (accepts `ChangeMe!2026`, rejects a wrong password). `must_change_password=true` forces a change, and the committed credential is recorded as a bounded, stated risk. | 0 |
| D15 | | Offline grace window **and** consequence are both configurable | Hard-code 7 days + block | Lets both postures be demonstrated in the viva. Default stays the safer `block_writes`. Server-side revocation must override either policy — that is not configurable. | 0 |
| D16 | | **Single-tenant: one deployment, one health centre** | Multi-tenant from day one; env-var config instead of a facilities row | The requirement is one hospital; other facilities deploy their own copy. `facilities` survives as a **single configuration row** (name, code, currency, timezone) because those values print on receipts and referral letterheads, and an admin should be able to correct them without a redeploy. Enforced by `uq_facilities_single_org` — a constant-expression unique index that rejects a second row at the database level, making single-tenancy a fact rather than a convention. | 0 |
| D17 | | Cloud Run region **`europe-west1`** (Belgium) | `europe-west3` (Frankfurt, co-located with Neon) | Chosen for cost; adds roughly 5 ms per query over co-location. Measured end to end from a container: ~290 ms for `/meta` on a cold pooled connection, and ~200 ms steady state. Acceptable, and recorded so the number can be revisited if the sync soak test suffers | 0 |
| D18 | | `MIGRATION_DATABASE_URL` is **not** required by the running API | Require it in production | `config.Load` originally demanded it in production, and the container **refused to boot** without it — even though the API never uses that URL. It points at Neon's *direct* endpoint, which only goose needs. Requiring it would force a production secret onto the service for no reason and hand the API a credential it should not hold. Migrations now run as a separate step (locally `make migrate-up`, in cloud a Cloud Run Job or CI). Separating the migration credential from the runtime credential is the point | 0 |
| D19 | | Frontend uses **code-based routing**, not TanStack Router's file-based generation | `@tanstack/router-plugin` file routes | The plugin generates `routeTree.gen.ts`, which must be regenerated before the project typechecks — an extra failure mode for a four-route tree. Routes are declared explicitly in `src/router.tsx`. Revisit above ~30 screens, where autoCodeSplitting earns its keep | 0 |
| D20 | | `skipLibCheck: true`, and `baseUrl` removed | Hand-patch dependency types | TS 7 **removed `baseUrl`** (TS5102) and resolves `paths` relative to the tsconfig, so the alias is `./src/*`. `skipLibCheck` is required because `dexie-react-hooks` ships a declaration importing its optional peers (`y-dexie`, `yjs`), which we do not install. Verified that all errors were confined to `node_modules` before enabling it — our own source typechecks clean under `strict` | 0 |
| D21 | | `web/.env` is **committed** with non-secret build defaults | Leave VITE_* unset | Vite substitutes `%VITE_API_ORIGIN%` in `index.html` at build time, and when the variable is undefined it leaves the placeholder **literal**. A CSP then reads `connect-src 'self' %VITE_API_ORIGIN%`, which is a malformed source list that browsers may discard wholesale — silently disabling the protection. A committed `.env` guarantees a definite value; `.env.local` overrides it. Nothing in it is secret, because Vite inlines every `VITE_*` value into the public bundle | 0 |
| D22 | | Deploy with `--env-vars-file .env.production` (plain env vars, **no Secret Manager**) | Secret Manager + IAM bindings | One fewer service, one fewer IAM step, one fewer failure mode. gcloud accepts plain `.env` format, so the existing file works with no rewrite — and `--env-vars-file` is mutually exclusive with `--set-secrets`, so the two approaches cannot be half-mixed. **Trade-off, stated for the report:** env vars are readable in the Cloud Run revision metadata by anyone with `roles/run.viewer`, and old values persist in previous revisions after rotation. Secret Manager would isolate secret access from deploy access and allow rotation without a redeploy — the production choice, deliberately not taken here | 0 |
| D23 | | `PROJECT_ID` must be the string **project ID**, never the project number | Rely on `gcloud config` | Cloud Run's `--project` rejects a number: *"The value of --project flag was set to Project number."* Two things that look like they should convert it, and do not: `gcloud config set project <number>` succeeds but stores the number **verbatim** (verified), and `gcloud projects describe <number>` needs an active account plus `resourcemanager.projects.get`. The reliable lookup is `gcloud projects list --filter="projectNumber=N" --format="value(projectId)"`. `scripts/deploy.sh` now rejects a numeric ID before any API call and prints that command | 0 |
| D24 | | `user_roles.revoked_reason` added (not in the original schema) | `revoked_at` alone | Found while testing: revoking a role recorded *who* and *when* but not *why*. "Why did Dr X lose prescribing rights?" is a question that will be asked, so the column is now required alongside `revoked_at`, with a CHECK enforcing both or neither | 1 |
| D25 | | `devices.registered_by` is **nullable** | `user_id NOT NULL` | A PHC workstation is shared — front desk, cashier and triage nurse may all use one machine — so a device does not belong to one user. This also broke a circular FK (`devices.user_id -> users`, `refresh_tokens.device_id -> devices`). The session owner is recorded per refresh token instead | 1 |
| D26 | | Identifier is **trimmed, never lowercased**, and the email comparison uses an explicit `$1::citext` cast | One normalised comparison for both columns | Two separate bugs, both caught by integration tests. (a) Lowercasing broke every staff number, because `staff_number` is case-sensitive TEXT — the seeded admin could not log in at all. (b) A Go `string` parameter is sent as `text`, and `citext = text` falls back to the case-SENSITIVE `text = text` operator, so mixed-case emails failed. The second is nasty because the same query in `psql` with a literal works (a literal is typed `citext`). Casting `$1` rather than the column keeps the index usable | 1 |
| D27 | | In-memory, **per-instance** login rate limiter, with DB-backed account lockout as the real control | Redis-backed shared limiter | The limiter is a speed bump: with `--max-instances=5` an attacker gets 5× the allowance. The account lockout is database-backed and therefore consistent, which is why it carries the real weight. A shared store is recorded as future work rather than pretended away | 1 |

---

## 9. Blockers

| # | Blocker | Raised | Owner | Impact | Status | Resolution |
|---|---|---|---|---|---|---|
| | _none yet_ | | | | | |

---

## 10. Session log

The habit that saves the project: two minutes at the end of every session.

| Date | Duration | What I did | What's next | Blockers |
|---|---|---|---|---|
| Sprint 0 batch 1 | — | Repo scaffold, Go module, config loader, pooler-safe pgx pool, `httpx` (request ID, error envelope, CORS), `main.go`, migrations 00001–00004, Makefile, `.env.example`. All verified against a throwaway PostgreSQL 18. | Batch 2: Dockerfile, Cloud Run deploy, Vite + Tailwind tokens, Dexie v1, app shell | Need your Neon project (0.3) and GCP/Cloudflare accounts (0.16, 0.23) |
| Sprint 0 refactor | — | **Removed multi-tenancy.** Rewrote migrations 00001–00004 (nothing was applied), stripped `facility_id` from the sync contract and ~25 tables, removed the `facility_mismatch` push rejection and the facility pull filter, added `uq_facilities_single_org`, reordered the admin seed to 00004. Updated all 12 docs + `CLAUDE.md` + `README.md` (137 references). Re-verified: `up→down→up` clean, 4 constraint tests pass, API builds and serves. | Batch 2 (unchanged) | — |
| Sprint 0 deploy | — | **API deployed to Cloud Run** (`europe-west1`), env vars via `--env-vars-file`. Added `scripts/deploy.sh` (9 guards) and `scripts/verify-deploy.sh` (5 checks incl. a 30-request concurrency test for the pooler trap). Debugged project-number-vs-project-ID across several rounds. | Verify deploy, then Cloudflare Pages (0.23), then the CORS loop-back (pass 2), then the end-to-end proof (0.24) | Cloudflare account for 0.23 |
| Sprint 1 batch 1 | — | Migrations 00005–00008 (user_roles, devices, refresh_tokens, admin role seed) validated on a throwaway PG18 then applied to Neon → version 8. Wrote `internal/platform/password` with 10 tests; measured 78 ms hash / 73 ms verify at production cost. | Batch 2: JWT (access + refresh with reuse detection), auth service, login/refresh/logout handlers, RBAC middleware | — |
| Sprint 1 batch 2 | — | `internal/domain`: sentinel errors + `ValidationError` + the eight roles with `HasRole`/`HasAnyRole`. `internal/platform/token`: HS256 access tokens, injectable clock, 9 tests incl. a **verified `alg=none` forgery** (empty signature, payload claiming admin) that is correctly rejected. Full suite: 19 tests green, `gofmt`/`vet`/build clean. | Batch 3: repos (users, user_roles, devices, refresh_tokens), refresh-token rotation with reuse detection, auth service | — |
| Sprint 1 batch 3 | — | Repos (users, devices, refresh_tokens) with a `Querier` interface proven by the compiler to accept both pool and tx. Auth service: login, refresh with reuse detection, logout, session revocation. 13 integration tests against real PostgreSQL. **Two live bugs found and fixed** (see D26) — neither was visible without running the code. | Batch 4: HTTP handlers + auth/role middleware, then ECDSA device identity | — |

### Verified after the single-tenant refactor

| Check | Result |
|---|---|
| `gofmt -l .` / `go vet ./...` / `go build` | clean |
| Migrations `up → down ×4 → up` | clean; 0 tables after full down, 2 after re-up |
| `SELECT` a 2nd facility row | **rejected** — `uq_facilities_single_org` (23505) |
| Lowercase `code` | **rejected** — `chk_facilities_code_charset` |
| Duplicate `staff_number` | **rejected** — `uq_users_staff_number` |
| `users` columns | no `facility_id` present |
| Seed idempotency | `facilities=1 users=1` |
| `GET /api/v1/meta` | `200`, `facilityCount: 1` |
| `GET /ready` | `200`, `database: ok` |

---

## 11. Metrics

Fill these in at each sprint close. They take ten minutes and they make your evaluation chapter trivial to write.

| Sprint | Tasks planned | Tasks done | Migrations | Endpoints | Screens | Tests added | Bugs found | Bugs fixed | Sync latency p95 |
|---|---|---|---|---|---|---|---|---|---|
| 0 | 30 | 29 | 4 | 3 | 4 | | | | |
| 1 | 17 | | 4 | 6 | 1 | | | | |
| 2 | 19 | | 1 | 4 | 1 | | | | |
| 3 | 13 | | 2 | ~15 | 4 | | | | |
| 4 | 9 | | 2 | ~8 | 3 | | | | |
| 5 | 8 | | 3 | ~12 | 3 | | | | |
| 6 | 13 | | 8 | ~20 | 4 | | | | |
| 7 | 8 | | 4 | ~8 | 3 | | | | |
| 8 | 11 | | 2 | ~8 | 4 | | | | |
| 9 | 14 | | 2 | ~10 | 4 | | | | |

---

## 12. Weekly rhythm

Keep this simple enough that you actually do it.

| When | Ceremony | Time | Output |
|---|---|---|---|
| Sprint start | Sprint planning | 30 min | Fill in the sprint backlog in §5 |
| Daily | Standup with yourself | 5 min | Note in §10; move one task |
| Mid-sprint | Risk check | 10 min | Update §7 |
| Sprint end | Demo to a real person | 20 min | Run the demo script end to end |
| Sprint end | Retrospective | 15 min | Fill in the retro under that sprint |
| Sprint end | Tracker update | 10 min | Update §2 and §11 |

The **demo to a real person** is the part students skip and the part that matters most. Demo to a classmate who has never seen the system. Every time you do, you will discover something that seemed obvious to you and is baffling to everyone else — which is exactly what will happen in your viva.
