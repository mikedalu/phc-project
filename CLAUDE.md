# Working Agreement — read this before touching any code

This file tells any AI agent (and any human collaborator) how to work in this repository. It is not optional context; it is the contract.

---

## 1. The single most important instruction

**The agent writes the code. The student reviews it, runs it, and must be able to defend it.**

This replaced the original "the student types every line by hand" agreement on the student's
explicit instruction, because hand-typing was the bottleneck against a ten-sprint plan. What did
**not** change is the reason that rule existed: every line must be explainable in a viva. Writing
code now carries a matching obligation to explain it.

That means:

- **Write files directly** under `api/` and `web/`, including migrations.
- **Explain what you wrote** in the reply: the file, what it does, why it is shaped that way, and
  the trap it avoids — enough that the student could re-derive it from the explanation.
- **Do not also paste the whole file into the reply.** Quote only the region that carries the
  reasoning. Writing it and re-printing it doubles the reading load for no benefit.
- **Migrations are code.** Same explanation, and they are only done when `up → down → up` has
  been run.
- **Never leave the tree broken.** If a change cannot be finished, revert it. A half-applied
  change is worse than no change.
- **Verify before claiming done:** `gofmt -l`, `go vet`, `go build`, the test suite, and
  `pnpm typecheck` for anything touching `web/`. State what was run and what it returned.
- **Commit** with a Conventional Commit message per `06-GIT-WORKFLOW.md`.

**Why this still matters:** this is a final-year project. Code the student cannot explain is a
liability in a viva, not an asset. The typing burden has moved to you; the explanation burden has
moved to you as well.

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
- **Give the verification step.** After writing code, say how it was verified — or how the student can re-verify it themselves: a curl command, a SQL query, a click path.
- **Do not pad.** Short and precise beats long and reassuring.

---

## 5. Working rhythm

1. Check `PROJECT-TRACKER.md` for the current sprint and task.
2. Read the relevant document section before designing anything.
3. Work in **vertical slices**: migration → repo → service → handler → route, finishing one endpoint end to end, rather than five half-built layers.
4. After each slice, run the checks and state what they returned.
5. Commit at each working slice with a Conventional Commit message (`06-GIT-WORKFLOW.md`), not once at the end of the sprint.
6. Update `PROJECT-TRACKER.md` §10 (session log), the task statuses, and the decision log whenever a choice is made that would be expensive to reverse.

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
- A change would sweep across more than a screen or two of the codebase with no intermediate state where the build still passes. Split it at a point you can verify.

---

## 8. Project summary for quick reference

**What:** an offline-first web patient records and referral system for a Primary Healthcare Centre.
**Stack:** Go 1.26 + Gin + pgx + goose + Neon Postgres 18 on Cloud Run · React 19 + TS 7 + Vite + Tailwind v4 + Dexie on Cloudflare Pages.
**Core idea:** the browser is the primary database; the server is a replication peer.
**Roles:** front_desk, nurse, doctor, lab_scientist, pharmacist, cashier, accountant, admin.
**The two headline features for the viva:** full workflow operation with the network off, and a QR referral note scannable at another facility for 48 hours.
