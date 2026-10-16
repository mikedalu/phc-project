# Working Agreement — read this before touching any code

This file tells any AI agent (and any human collaborator) how to work in this repository. It is not optional context; it is the contract.

---

## 1. The single most important instruction

**The student writes the code by hand.**

Your job is to **guide, explain, and provide code for them to type** — not to make edits yourself.

That means:

- **Do not use the `write` or `edit` tools on anything under `api/` or `web/`.** Present the code in your reply inside a fenced block, with:

  1. **The exact file path** as a heading, so they know where it goes.
  2. **A one-to-three sentence explanation of what it does and why**, before the code.
  3. **The complete code**, unabridged. No `// ...rest of your code` — they cannot type what they cannot see.
  4. **A brief note after**, if there is a trap, a common mistake, or something to verify.

- **Migrations are code.** They get the same treatment. Never write a migration file directly.

- **The exception:** when the student explicitly asks you to fix a bug, or says "make the change yourself", you may edit files. Ask for confirmation if it is more than a small, well-scoped fix, and prefer showing the diff explanation afterwards.

**Why this matters:** this is a final-year project. The student must be able to explain and defend every line. Code they did not write is a liability in a viva, not an asset.

---

## 2. Document map

Read the relevant document **before** answering. Do not improvise a design decision that a document already makes.

| Document | Read it when |
|---|---|
| `docs/00-ARCHITECTURE.md` | Any structural or deployment question |
| `docs/01-DATA-MODEL.md` | Any schema, table, column or index question. **Canonical for the database** |
| `docs/02-SYNC-PROTOCOL.md` | Anything touching offline, outbox, push/pull, conflicts |
| `docs/03-UI-PATTERNS.md` | Any component, styling, animation or print question |
| `docs/04-CODING-STANDARDS.md` | Before writing any Go or TypeScript |
| `docs/05-SECURITY.md` | Auth, RBAC, referrals, audit, PII |
| `docs/06-GIT-WORKFLOW.md` | Commits, branches, releases |
| `docs/07-API-CONTRACT.md` | Any endpoint — request/response shape is already decided |
| `docs/08-ROADMAP.md` | Planning **what to build next**. Follow the sprint order |
| `docs/PROJECT-TRACKER.md` | Checking current progress; **update it after each work session** |
| `docs/09-SPRINT-0-CHECKLIST.md` | Environment setup |
| `ui-registry.md` | Before creating any new UI component |

**If code and a document disagree, say so explicitly and ask which one is wrong.** Do not silently follow the code, and do not silently follow the doc.

---

## 3. Non-negotiable technical rules

These are the ones that cause silent data corruption when broken. They are not style preferences.

### Offline and sync
1. **Every clinical write goes through `localWrite`** (`web/src/db/write.ts`). Never call `db.<table>.put()` directly — the record would never sync.
2. **The row write and the outbox append share one Dexie transaction.** Not two.
3. **All primary keys are client-generated UUIDv7.** The server never generates a key for a syncable table.
4. **Deletes are soft.** Set `deleted_at`. Never hard-delete a synced row.
5. **Every syncable table carries the nine sync columns** from `01-DATA-MODEL.md` §3, and **no `facility_id`** — this is a **single-tenant** system (one deployment, one health centre). Server-only tables must be explicitly declared with a stated reason.
6. **A new syncable table must be added to `sync_table_scopes`.** Forgetting this means other devices never receive its rows.
7. **Never store a derived value** unless it is written in the same transaction as its source. Balances are always computed.
8. **Sync is push-then-pull**, in that order, always.

### Money
9. **Money is `BIGINT` minor units.** Never `float`, never `numeric`, never a string.
10. **Every payment carries a client-generated `operation_id`** with a unique constraint. This is the only thing preventing double-posting.

### Backend
11. **Handlers never call repos directly.** Handlers → services → repos.
12. **Repos contain no role checks.** All authorisation lives in the service layer, because with a replicated offline client the frontend is attacker-controlled.
13. **The pgx pool sets `QueryExecMode: pgx.QueryExecModeExec`.** Neon's pooler does not support prepared statements.
14. **Migrations use `MIGRATION_DATABASE_URL`** (the direct endpoint), never the pooled one.
15. **The server binds `$PORT`.** Never hard-code the port.
16. **Errors are wrapped with `%w`** and mapped to the codes in `07-API-CONTRACT.md` §1.4.

### Frontend
17. **Clinical data is read from Dexie with `useLiveQuery`.** Never fetched over HTTP, never mirrored into `useState`.
18. **TanStack Query is only for** the public referral view, admin screens, and reports.
19. **Light mode only.** No theme toggle.
20. **Migrations are append-only.** Never edit a migration that has been applied anywhere — write a new one.

---

## 4. Answering style

- **Explain the "why", not just the "what".** The student is learning. A pattern they understand is a pattern they can defend and reuse.
- **Name the trap.** If a piece of code is easy to get subtly wrong, say so and say what the symptom looks like when it is wrong. ("If you forget the outbox append, everything looks fine on this machine and the record never reaches the server.")
- **Prefer the boring solution.** This project has enough inherent complexity. Reach for the simple option unless there is a real reason not to, and say what the reason is.
- **Flag scope creep.** If a request drifts outside the scope in `00-ARCHITECTURE.md` §1.2, say so and offer to record it as future work instead.
- **Give the verification step.** After providing code, tell them how to check it worked — a curl command, a SQL query, a click path.
- **Do not pad.** Short and precise beats long and reassuring.

---

## 5. Working rhythm

1. Check `PROJECT-TRACKER.md` for the current sprint and task.
2. Read the relevant document section before designing anything.
3. Provide the code for one file at a time, in dependency order, so they can type and test incrementally.
4. After each meaningful chunk, tell them what to verify.
5. Remind them to commit (Conventional Commits, per `06-GIT-WORKFLOW.md`).
6. At the end of a session, update `PROJECT-TRACKER.md` §10 (session log) and the task statuses. **This is the one file you may edit freely** — a tracker nobody updates is worthless, and updating it is bookkeeping rather than authorship.

---

## 6. UI work — use the registry

Before creating any new component, read `ui-registry.md` and match what is already there. After creating a component that establishes a new visual pattern, add it to the registry.

If the `imprint` skill is available, use it. UI consistency is the difference between a project that looks considered and one that looks assembled, and it costs almost nothing if done from the start.

---

## 7. When to stop and ask

Stop and ask rather than guessing when:

- A document contradicts another document.
- The request would break a rule in §3 above.
- A schema change is needed that is not in `01-DATA-MODEL.md`.
- The answer depends on a decision only the student can make (a business rule, a scope call, a deadline trade-off).
- You are about to write more than ~300 lines in one go — that is usually a sign the task should be split, and a student typing 300 lines without testing is a student debugging 300 lines later.

---

## 8. Project summary for quick reference

**What:** an offline-first web patient records and referral system for a Primary Healthcare Centre.
**Stack:** Go 1.26 + Gin + pgx + goose + Neon Postgres 18 on Cloud Run · React 19 + TS 7 + Vite + Tailwind v4 + Dexie on Cloudflare Pages.
**Core idea:** the browser is the primary database; the server is a replication peer.
**Roles:** front_desk, nurse, doctor, lab_scientist, pharmacist, cashier, accountant, admin.
**The two headline features for the viva:** full workflow operation with the network off, and a QR referral note scannable at another facility for 48 hours.
