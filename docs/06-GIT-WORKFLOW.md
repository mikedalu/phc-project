# Git Workflow

> **Audience:** a solo developer who is also the reviewer, the release manager and the person who loses a week's work if this goes wrong.
> **Status:** deliberately lightweight. This document exists to prevent three specific disasters, not to imitate a large engineering team.

---

## 1. The three disasters this prevents

Everything below serves one of these. If a rule does not serve one of them, it is ceremony — skip it.

| # | Disaster | What it looks like | The rule that prevents it |
|---|---|---|---|
| 1 | **Losing uncommitted work** | Your laptop dies on Tuesday; Monday's offline-sync refactor existed only in the editor buffer | Commit at every Definition-of-Done tick; push daily |
| 2 | **Breaking `main` before a demo** | You "quickly fix" the payment ledger the night before the viva, and now nothing works | Work on branches; `main` is always demoable |
| 3 | **Being unable to explain a change** | An examiner asks why the conflict policy is LWW; you have 40 commits all called "update" | Conventional Commits; one logical change per commit |

---

## 2. Branching

Trunk-based with short-lived branches. **No `develop` branch, no GitFlow** — with one developer they add merge overhead and buy nothing.

```
main ──●────●─────────●──────────●────────●──▶  always deployable, always demoable
        \             \          /
         feat/…        fix/…    /     short-lived, merged within days
          ●──●──●       ●──●───●
```

### Branch naming

```
<type>/<sprint>-<short-slug>
```

| Type | Use for | Example |
|---|---|---|
| `feat/` | New functionality | `feat/s2-sync-push-endpoint` |
| `fix/` | Bug fixes | `fix/s6-double-payment-guard` |
| `refactor/` | Behaviour-preserving restructure | `refactor/s4-workflow-state-machine` |
| `chore/` | Tooling, deps, config | `chore/s0-cloud-run-deploy` |
| `docs/` | Documentation only | `docs/s8-referral-security-notes` |
| `test/` | Tests only | `test/s2-offline-soak` |
| `db/` | Migrations | `db/s6-payments-allocation` |

Include the sprint number. In six weeks you will not remember whether `fix/balance-bug` was Sprint 3 or Sprint 7, and the tracker cross-reference matters.

### Branch rules

- **Branch from `main`**, always. Never from another feature branch.
- **Merge within a few days.** A branch open for two weeks is a merge conflict you have already lost.
- **Never commit directly to `main`** for anything except a one-line typo or a docs fix — and even then, prefer a branch. The habit is what protects you.
- **Delete the branch after merging.**

---

## 3. Commit messages — Conventional Commits

```
<type>(<scope>): <subject>

<body — why, not what>

<footer — refs, breaking changes>
```

### Subject line

- Imperative mood: "add payment ledger", not "added" or "adds". It should complete the sentence *"This commit will…"*.
- Lowercase after the colon.
- No trailing full stop.
- **72 characters maximum.**

### Types

| Type | Use for |
|---|---|
| `feat` | A new feature |
| `fix` | A bug fix |
| `refactor` | Restructuring with no behaviour change |
| `perf` | Performance improvement |
| `test` | Adding or fixing tests |
| `docs` | Documentation |
| `db` | Migrations and schema |
| `chore` | Build, deps, tooling, CI |
| `style` | Formatting only, no code change |
| `revert` | Reverting a previous commit |

### Scopes

Use the module or feature area. Keep it small and consistent:

`api` · `web` · `sync` · `auth` · `patients` · `queue` · `vitals` · `billing` · `pharmacy` · `admission` · `referral` · `db` · `ci` · `docs`

### Examples — bad and good

```
❌ update
❌ fixed bug
❌ WIP
❌ changes to payment stuff and also fixed the referral thing
```

```
✅ feat(billing): add payment allocation with derived balances
✅ fix(sync): prevent duplicate payment posting on push retry
✅ db(billing): create payments and payment_allocations tables
✅ test(sync): cover conflict resolution for concurrent patient edits
✅ docs(referral): record the QR token encryption trade-off
```

### Body — the part that earns marks

Use the body of the commit to explain **why**. This is your decision log, and it is what you will mine when writing your dissertation's methodology chapter.

```
fix(sync): prevent duplicate payment posting on push retry

A push that timed out after the server had committed it was retried by
the client, creating a second payment row. The cashier's daily total was
therefore double the cash in the drawer.

Added a client-generated operation_id with a UNIQUE constraint on
(operation_id) on payments. A retried operation now hits the constraint and is
reported as 'duplicate' rather than inserting a second row.

Refs: docs/02-SYNC-PROTOCOL.md §5.4
```

Read that and you can reconstruct the reasoning a year later. That is the standard.

### Footers

```
Refs: #12
Closes: #14
Implements: docs/07-API-CONTRACT.md §8.5
```

`Implements:` is unusual in industry and **very** useful here — it ties a commit to the document section that specified it, which is exactly the traceability an examiner wants to see.

---

## 4. What belongs in a commit

**One logical change per commit.** A commit should be revertable on its own without dragging unrelated work with it.

### Commit together

- A migration **and** the Go code that uses it **and** the doc section it implements.
- A Dexie schema version bump **and** the sync contract change.
- A component **and** its test.

### Commit separately

- Reformatting the whole repo.
- A dependency upgrade.
- Unrelated typo fixes in files you happened to open.

> **The rule that matters:** **never commit a migration and then leave the code that depends on it for "later".** Half-applied schema changes are how a working `main` stops being deployable.

### Never commit

- `.env` or any file containing a real secret
- `node_modules/`, `dist/`, build artefacts
- Database dumps containing real patient data (**never**, under any circumstances — use synthetic seed data)
- Large binaries, PDFs, or screenshots over ~1 MB

---

## 5. The daily rhythm

```bash
# morning
git switch main
git pull
git switch -c feat/s3-patient-registration

# work in small steps; commit when the Definition of Done is met
git add api/internal/service/patient_service.go
git commit -m "feat(patients): add patient number generation"

# end of session — ALWAYS, even mid-task
git push -u origin feat/s3-patient-registration
```

### The end-of-session rule

**Push before you stop, even if the branch is unfinished and the code is broken.** An imperfect pushed branch costs you nothing. An unpushed perfect branch costs you everything when the disk fails.

If the work is genuinely broken, say so in the commit message:

```
chore(sync): WIP pull cursor (does not apply rows yet)
```

An honest WIP commit is better than a lost afternoon.

---

## 6. Commits worth making — a worked example

The same feature, committed well versus badly.

### Bad — one enormous commit

```
feat: patient registration and queue and vitals and sync stuff
```

You cannot review it, cannot revert it, and cannot explain it.

### Good — a sequence you can actually follow

```
db(patients): create households and patients tables
feat(patients): add patient repository with prefix search
feat(patients): add patient service with age/dob derivation rules
feat(patients): add POST /patients and GET /patients endpoints
feat(patients): add family registration in a single transaction
feat(patients): add fuzzy duplicate detection
docs(patients): document the registration endpoints
test(patients): cover age derivation and family registration
feat(web): add patient registration form
feat(web): add family member sub-form
feat(web): add Ctrl+K patient search
```

Eleven commits, each one understandable in isolation, each one revertable. This is also a **far** better story in a viva: it shows incremental, disciplined delivery, which is precisely what an agile methodology chapter claims.

---

## 7. Pull requests — even solo

A PR with one reviewer (you, one hour later) is worth the ten minutes. It is also evidence of process for your report.

**Keep it short.** For a solo project, a PR description of five lines is enough:

```markdown
## What
Adds patient and household registration, single and family modes.

## Why
Sprint 3 deliverable — the spine every other clinical record hangs off.

## How to verify
1. `make migrate-up && make api && make web`
2. Register a single patient → confirm the patient number format
3. Register a family of 3 → confirm one household, three patients, all linked
4. Search by surname prefix in Ctrl+K

## Checklist
- [x] Migration has a working Down (`up → down → up` verified)
- [x] Works with the network off
- [x] Audit log written for create
- [x] `07-API-CONTRACT.md` updated in this PR
- [x] Tests added
```

### Self-review checklist — run before every merge

- [ ] **Read your own diff in the GitHub/GitLab UI**, not in your editor. You will spot things the editor hid.
- [ ] No `.env`, no secrets, no real patient data.
- [ ] No leftover `console.log`, `fmt.Println`, or commented-out code.
- [ ] No `TODO` without an owner and a reason.
- [ ] Migration `up → down → up` verified.
- [ ] The relevant doc is updated **in this PR**, not "later".
- [ ] Tests pass locally.
- [ ] The app still runs with the network **off**.

---

## 8. Merging

Use **squash merge** for feature branches with messy history, **rebase merge** when the commits are already clean and individually meaningful.

| Situation | Use | Why |
|---|---|---|
| Clean, well-described commits | Rebase merge | Preserves the incremental history — valuable for your report |
| "WIP", "fix typo", "argh" littered through | Squash merge | Keeps `main`'s history readable |
| Never | Merge commit | Adds noise for no benefit in a solo project |

**Never force-push to `main`.** Force-push to your own feature branch is fine, but if you have already pushed and someone (or a CI run) may have pulled, prefer a new commit.

---

## 9. Versioning and releases

Tag at the end of each sprint:

```bash
git tag -a v0.2.0 -m "Sprint 2: sync engine complete"
git push origin v0.2.0
```

Semantic versioning, mapped to sprints:

| Change | Version bump | Example |
|---|---|---|
| Sprint complete, new features | Minor | `v0.2.0` → `v0.3.0` |
| Bug fix, no new feature | Patch | `v0.3.0` → `v0.3.1` |
| Breaking sync payload change | **Minor while pre-1.0**, and bump `schemaVersion` | `v0.3.0` → `v0.4.0` |

The `v1.0.0` tag is your final submission. Everything before it is pre-release, which honestly reflects a project built to a deadline.

> **The sync-relevant rule:** if a release changes the sync payload shape, bump the integer `schemaVersion` in `GET /meta` **in the same release**. A client one version behind must keep working — a PHC cannot pause admissions for a browser update. Record the compatibility note in the tag annotation.

---

## 10. Handling mistakes

| Mistake | Fix |
|---|---|
| Committed a secret | **Rotate the secret first.** Then remove it from history (`git filter-repo`). Assume it is compromised the moment it is pushed — the commit is not the problem, the leak is. |
| Committed to `main` by accident | `git reset --soft HEAD~1`, create a branch, commit there |
| Bad commit already pushed | `git revert <sha>` — a revert commit is honest history. Do not rewrite public history. |
| Wrong commit message on the tip | `git commit --amend` **only if unpushed** |
| Need to undo an unpushed commit but keep the changes | `git reset --soft HEAD~1` |
| Merge conflict | Resolve it, then **run the tests before committing the merge** — a conflict resolved wrongly can compile and still be wrong |
| Broke `main` | Revert immediately to restore a demoable state, then fix properly on a branch. Never debug forward on `main`. |

---

## 11. `.gitignore`

```gitignore
# ─── secrets ───────────────────────────────────────────────────
.env
.env.*
!.env.example

# ─── go ────────────────────────────────────────────────────────
api/bin/
api/tmp/
*.test
*.out

# ─── node ──────────────────────────────────────────────────────
node_modules/
web/dist/
web/.vite/
web/coverage/

# ─── playwright ────────────────────────────────────────────────
test-results/
playwright-report/
blob-report/

# ─── data — NEVER commit real patient data ─────────────────────
*.dump
*.sql.gz
seed-data/real/

# ─── editors / OS ──────────────────────────────────────────────
.DS_Store
Thumbs.db
.idea/
*.swp
```

> `!.env.example` after `.env.*` re-includes the example file. Order matters in `.gitignore`, and getting it wrong means either committing secrets or losing the template you need.

---

## 12. Commit message template

Save this as `.gitmessage` and run `git config commit.template .gitmessage`:

```
# <type>(<scope>): <subject — imperative, lowercase, no full stop, ≤72 chars>
#
# Why this change was needed (the problem, not the code):
#
#
# What changed:
#
#
# How it was verified:
#
#
# Refs: docs/XX-DOC.md §N
# Implements: Sprint N deliverable
```

---

## 13. Quick reference

```bash
git switch -c feat/s3-patient-registration   # branch from main
git add -p                                    # stage hunks deliberately, not everything
git commit -m "feat(patients): add registration endpoint"
git push -u origin HEAD                       # push before you stop, always
git switch main && git merge --rebase feat/…  # merge back
git tag -a v0.3.0 -m "Sprint 3: patient registration"
```

### The five habits that matter most

1. **Commit at every Definition-of-Done tick** — not at the end of the day.
2. **Push before you stop** — even broken, even unfinished.
3. **Write the "why" in the body** — your dissertation will thank you.
4. **Keep `main` demoable** — always, without exception.
5. **Never commit `.env` or real patient data** — not once, not "just to test".
