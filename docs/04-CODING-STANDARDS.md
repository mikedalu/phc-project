# Coding Standards

> **Project:** Design and Implementation of an Offline-Capable Web-Based Patient Records and Referral Management System for Primary Healthcare Centres
> **Document status:** Canonical. If code disagrees with this document, one of them is a bug.
> **Last updated:** Sprint 0
> **Audience:** one developer who must be able to defend every line. The code is written by an
> AI agent against these rules; the review and the understanding are the developer's. See `CLAUDE.md` §1.

This document is the rulebook the code is written against, and the checklist a reviewer applies. It is deliberately short on theory and long on
"here is the line you write". Read [00-ARCHITECTURE.md](./00-ARCHITECTURE.md) first — nothing here
contradicts it. Where this document cites a rule, that rule exists to protect the offline-first design in
§1.1 of the architecture: **the browser is the primary database; the server is a replication peer.**

If you have five minutes and cannot read all of this, read §1 (principles), §4.3 (how to add a screen),
§5.2 (the migration template), and §6 (sync rules). Those four prevent the failures that are hard to
undo later.

---

## Table of contents

1. [Core principles](#1-core-principles)
2. [Repository and file organisation](#2-repository-and-file-organisation)
3. [Go standards](#3-go-standards)
4. [TypeScript / React standards](#4-typescript--react-standards)
5. [Database and migration standards](#5-database-and-migration-standards)
6. [Sync-specific coding rules](#6-sync-specific-coding-rules)
7. [Testing standards](#7-testing-standards)
8. [Git and review conventions](#8-git-and-review-conventions)
9. [Comments and documentation](#9-comments-and-documentation)
10. [Definition of Done](#10-definition-of-done-per-task)
11. [Common mistakes to avoid](#11-common-mistakes-to-avoid)
12. [Tooling configuration](#12-tooling-configuration)

---

## 1. Core principles

Ten rules. When two of them appear to conflict, the lower number wins.

| # | Principle | Why it exists |
|---|---|---|
| 1 | **The client is the source of truth for clinical facts.** The device records what happened, when it happened, and who did it. | A PHC cannot stop working because a link is down. Any design that asks the server to be present at the moment of care is disqualified. |
| 2 | **Make the offline path the default path, not a special case.** Every screen reads from Dexie and writes through the outbox, online or not. | If "online" is a code path, it is the path you test, and the offline path rots. One path means one set of bugs. |
| 3 | **Write locally, then replicate.** A mutation writes Dexie and returns. It never waits for the network. | The sync engine's job is to reconcile later. A handler that awaits `fetch` makes the app slow and fragile. |
| 4 | **One reason to change per file.** A handler changes when HTTP changes; a repo changes when SQL changes; a service changes when a rule changes. | Mixed reasons are how a 40-line file becomes a 400-line file nobody dares edit. |
| 5 | **Explicit over clever.** Write the loop. Write the `if`. Name the variable. | You will debug this at 2 a.m. before a defence. Clever code is a loan against your own future time. |
| 6 | **No silent failures.** Either handle the error, wrap and return it, or log it with context. Never swallow it. | A swallowed error in the outbox drain becomes a patient record that never reaches the server, discovered weeks later. |
| 7 | **Derive, don't store.** Balances, counts, queue positions, and statuses are computed from facts. | A stored derived value is a second source of truth, and two offline devices will always disagree about it. |
| 8 | **Every rule lives in exactly one place.** A transition table, a money helper, an ID generator, a clock — one definition each. | Two copies of a clinical rule is a bug with a delay fuse. |
| 9 | **Boring, greppable names.** `PatientID`, `amountMinor`, `listAwaitingLab`. No abbreviations you cannot spell. | Search is your debugger. `amt` finds nothing; `amountMinor` finds everything. |
| 10 | **Leave the campsite cleaner.** If you touch a file and see a violation that is cheaper to fix now than to explain later, fix it in the same commit. | Small consistent code stays consistent. Small deviations compound into a codebase you stop trusting. |

---

## 2. Repository and file organisation

### 2.1 The tree

This is the tree from [00-ARCHITECTURE.md](./00-ARCHITECTURE.md) §4.2, expanded to the file level.
Anything not listed here needs a sentence in your sprint notes explaining why it exists.

```
phc-records/
├── api/                                # Go backend
│   ├── cmd/api/main.go                 # wiring ONLY: config → pool → repos → services → handlers
│   ├── internal/
│   │   ├── config/config.go            # env parsing, one Load(), fails fast
│   │   ├── domain/                     # plain structs + enums; imports NOTHING internal
│   │   │   ├── patient.go  encounter.go  invoice.go  referral.go
│   │   │   ├── sync.go                 # SyncColumns, SyncState, OutboxOperation
│   │   │   └── errors.go               # ErrNotFound, ErrConflict, ErrForbidden, ErrValidation
│   │   ├── repo/                       # SQL ONLY. No RBAC. No HTTP.
│   │   │   ├── patient_repo.go  encounter_repo.go  invoice_repo.go
│   │   │   └── tx.go                   # withTx helper
│   │   ├── service/                    # business rules, RBAC, transactions
│   │   │   ├── patient_service.go  billing_service.go  triage_service.go
│   │   │   ├── money.go                # minor-unit arithmetic
│   │   │   └── workflow.go             # the state machine from architecture §7.5
│   │   ├── http/
│   │   │   ├── router.go               # route table, middleware order
│   │   │   ├── middleware/             # requestid.go  auth.go  recover.go
│   │   │   ├── dto/                    # request/response shapes + domain converters
│   │   │   ├── respond/respond.go      # respond.OK / respond.Error — ONLY way to write a body
│   │   │   └── handler/                # patient_handler.go  sync_handler.go
│   │   ├── sync/                       # server engine: change log, push, pull, opregistry.go
│   │   └── platform/                   # jwt/ argon2/ ids/ clock/ qr/
│   ├── db/
│   │   ├── migrations/00001_create_facilities.sql …
│   │   └── 0001_seed_facility_and_admin.sql  (see db/seed/)
│   ├── Dockerfile   go.mod   .env.example
│
├── web/                                # React frontend
│   ├── src/
│   │   ├── app/                        # router.tsx, providers.tsx, layouts/AppShell.tsx
│   │   ├── routes/                     # one thin file per screen: compose a feature
│   │   │   ├── patients.index.tsx  patients.$patientId.tsx  queue.tsx
│   │   │   └── r.$token.tsx            # public referral view (online-only)
│   │   ├── db/                         # db.ts, schema.ts, local-write.ts (THE write helper)
│   │   ├── sync/                       # outbox.ts, pusher.ts, puller.ts, engine.ts, status-store.ts
│   │   ├── features/                   # patients/ queue/ billing/ referrals/
│   │   │   └── <feature>/              # api.ts, hooks/useThing.ts, components/ThingForm.tsx
│   │   ├── components/ui/              # design system primitives (Button.tsx, Dialog.tsx)
│   │   ├── lib/                        # api-client.ts, money.ts, clock.ts, ids.ts, cn.ts, schemas/
│   │   ├── styles/                     # tokens.css (@theme), print.css (58–80 mm slip)
│   │   └── types/
│   ├── public/   package.json   vite.config.ts
│
├── docs/   scripts/   CLAUDE.md   README.md
```

### 2.2 File naming

| Kind | Convention | Example |
|---|---|---|
| Go source | `snake_case.go`, named for the thing it holds | `patient_repo.go`, `billing_service.go` |
| Go test | `<file>_test.go` beside the file under test | `bill_service_test.go` |
| React component | `PascalCase.tsx`, one component per file | `PatientForm.tsx`, `Button.tsx` |
| Hook | `useThing.ts`, camelCase after `use` | `usePatientList.ts`, `useOfflineStatus.ts` |
| Dexie / sync module | `kebab-case.ts` | `local-write.ts`, `status-store.ts` |
| Feature module (non-component) | `kebab-case.ts` | `features/patients/api.ts`, `lib/api-client.ts` |
| Zod schema file | `kebab-case-schema.ts` | `patient-schema.ts`, `payment-schema.ts` |
| SQL migration | `NNNNN_verb_noun.sql`, zero-padded 5 digits | `00014_create_referrals.sql` |
| SQL seed | `NNNN_description.sql` | `0001_seed_facility.sql` |
| Docs | `NN-KEBAB-CASE.md` | `04-CODING-STANDARDS.md` |
| Route file | mirrors the URL, TanStack convention | `patients.$patientId.tsx` → `/patients/:patientId` |

Two rules people break:

- **Do not name a file `utils.ts`.** `utils` is not a thing. Name it after what it does: `money.ts`, `dates.ts`, `cn.ts`.
- **Do not create a Go package named `util`, `common`, `helpers`, or `models`.** A package name should tell the caller what it gets. `platform/ids` does; `util` does not.

---

## 3. Go standards

### 3.1 Packages and identifiers

| Thing | Rule | Example |
|---|---|---|
| Package | short, lower-case, singular, no underscores | `repo`, `service`, `respond` |
| Exported identifier | `PascalCase`, no stutter with the package | `service.Patients`, not `service.PatientService` |
| Unexported | `camelCase` | `applyPayment` |
| Acronyms | all-caps, consistent | `PatientID`, `HTTPClient` — never `PatientId` |
| Interface | named for behaviour, `-er` when it fits | `PatientReader`, `Clock`, `TokenSigner` |
| Interface size | **1–3 methods**. Five methods is a struct, not an interface | |
| Receiver | one or two letters, consistent across the type | `func (s *Patients)` |

**`Service` struct vs free function.** Use a **struct** when the operation needs collaborators (repo,
clock, ID generator) or holds configuration. Use a **free function** when it is pure — same input, same
output, no dependencies. Money maths, workflow transition tables, and date helpers are free functions,
which is exactly why they are trivial to test with a table.

```go
func ApplyPayment(inv domain.Invoice, pays []domain.Payment) domain.Balance { /* pure */ }

type Patients struct { repo PatientReader; clock Clock }         // impure: needs a repo + a clock
func (s *Patients) Register(ctx context.Context, in RegisterInput) (domain.Patient, error)
```

**Small interfaces live where they are consumed, not where they are implemented.** The `service`
package declares what it needs; it does not import a 30-method interface defined by `repo`.

```go
// internal/service/patient_service.go — the consumer declares the contract: 2 methods, not 30.
type PatientReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.Patient, error)
	List(ctx context.Context, limit int) ([]domain.Patient, error)
}
```

### 3.2 Error handling

**Three rules, no exceptions:**

1. **Always wrap** with `fmt.Errorf("...: %w", err)`. The `%w` is what makes `errors.Is` work three layers up.
2. **Never `panic`** outside `main` and `init`. A panic in a handler dies with no useful code; return an error and let the middleware map it.
3. **Never ignore an error with `_`.** If you truly do not care, `//nolint:errcheck // reason` on the same line — the reason is mandatory.

```go
row, err := r.pool.Query(ctx, queryGetPatient, id)
if err != nil {
	return domain.Patient{}, fmt.Errorf("get patient %s: %w", id, err)      // CORRECT
}
return domain.Patient{}, fmt.Errorf("failed: %v", err)   // WRONG: no context, %v breaks errors.Is
_ = tx.Commit(ctx)                                       // WRONG: this is how a record never syncs
```

**The domain error set.** `internal/domain/errors.go` defines exactly four sentinels. Every service
returns one (wrapped) or an infrastructure error. Nothing else is a "domain failure".

```go
var (
	ErrNotFound   = errors.New("not found")          // missing or soft-deleted
	ErrConflict   = errors.New("conflict")           // duplicate, or stale revision
	ErrForbidden  = errors.New("forbidden")          // wrong role
	ErrValidation = errors.New("validation failed")  // fails a clinical/financial rule
)

// Sentinel for the KIND, wrapped %w for the CAUSE:
return domain.Balance{}, fmt.Errorf("payment %d exceeds balance %d: %w",
	amountMinor, inv.BalanceMinor(), domain.ErrValidation)
```

**Mapping to HTTP status — one table, implemented once in the middleware.**

| Domain error | HTTP | Machine code | When |
|---|---|---|---|
| `ErrValidation` | 400 | `VALIDATION_FAILED` | Fails a rule (negative amount, bad transition) |
| JSON bind failure | 422 | `INVALID_BODY` | Body malformed or a required field missing |
| missing/expired token | 401 | `UNAUTHENTICATED` | Token absent, expired, or bad signature |
| `ErrForbidden` | 403 | `FORBIDDEN` | Wrong role, or an unregistered/revoked device |
| `ErrNotFound` (incl. `pgx.ErrNoRows`) | 404 | `NOT_FOUND` | Entity missing or soft-deleted |
| `ErrConflict` | 409 | `CONFLICT` | Duplicate number, stale `server_rev`, idempotency mismatch |
| anything else | 500 | `INTERNAL` | Log the full error; return no detail to the client |

`pgx.ErrNoRows` **never** escapes the repo layer (§3.5), so this table is complete.

```go
// internal/http/respond/respond.go — the ONLY place a status code is decided.
func Error(c *gin.Context, err error) {
	status, code, msg := classify(err)
	if status >= 500 {
		log.Error().Str("request_id", RequestID(c)).Err(err).Msg("internal error")
		msg = "internal error" // never leak SQL or stack detail to a client
	}
	c.AbortWithStatusJSON(status, ErrorEnvelope{
		Error: ErrorBody{Code: code, Message: msg, RequestID: RequestID(c)},
	})
}
```

### 3.3 Context

- `ctx context.Context` is the **first parameter** of every repo and service method — even ones that "obviously" do no I/O, because they will tomorrow.
- `ctx` carries **cancellation** and **request-scoped values**: user ID, device ID, request ID. (No facility ID — this is a single-tenant deployment.)
- **Never store a context in a struct.** It is per-request; a struct outlives a request. `containedctx` enforces this.

```go
func (r *PatientRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Patient, error) // CORRECT
func (r *PatientRepo) GetByID(id uuid.UUID, ctx context.Context) (domain.Patient, error) // WRONG

type PatientRepo struct { pool *pgxpool.Pool; ctx context.Context }  // WRONG: ctx on a struct
```

Request-scoped values use typed keys, read through accessors, so no handler touches a bare string key.

```go
type ctxKey int
const (ctxKeyUserID ctxKey = iota; ctxKeyFacilityID; ctxKeyDeviceID; ctxKeyRequestID)

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyUserID).(uuid.UUID); return id, ok
}
```

### 3.4 Structs and DTOs

`domain` structs are the internal vocabulary; HTTP DTOs are the wire format. They are **separate types
with explicit converters** — never one struct doing both jobs.

| Layer | Type | JSON tags? | Validation tags? |
|---|---|---|---|
| `domain` | `Patient`, `Invoice` | **no** | no |
| `http/dto` | `PatientCreateRequest`, `PatientResponse` | yes | yes (`binding:"required"`) |
| Dexie (`web/src/db/schema.ts`) | `PatientRow` | n/a | no (Zod validates) |

```go
type Patient struct {          // internal/domain/patient.go — no tags, no HTTP knowledge
	ID uuid.UUID; FacilityID uuid.UUID; PatientNumber string
	FirstName, LastName string; DateOfBirth time.Time; Sex Sex; Phone *string
}

type PatientResponse struct {  // internal/http/dto/patient_dto.go — the wire format
	ID            string  `json:"id"`
	PatientNumber string  `json:"patient_number"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	DateOfBirth   string  `json:"date_of_birth"` // RFC3339 date
	Sex           string  `json:"sex"`
	Phone         *string `json:"phone,omitempty"`
}

func PatientFromDomain(p domain.Patient) PatientResponse {
	return PatientResponse{
		ID: p.ID.String(), PatientNumber: p.PatientNumber,
		FirstName: p.FirstName, LastName: p.LastName,
		DateOfBirth: p.DateOfBirth.Format(time.DateOnly),
		Sex: string(p.Sex), Phone: p.Phone,
	}
}
```

**`omitempty` discipline.** Use it only for genuinely optional fields — pointers and slices. Never on a
`bool`, an `int`, or a `string` with a meaningful zero, because `omitempty` drops `false` and `0` and
the client reads `undefined`.

```go
Phone    *string `json:"phone,omitempty"`  // OK: nil means "not recorded"
IsActive bool    `json:"is_active"`        // NO omitempty: false is a real value
Revision int64   `json:"revision"`         // NO omitempty: 0 is a real value
```

**Never expose** password hashes, Argon2 salts, refresh tokens, `access_token_hash`, internal counters,
`server_rev`, or device private material. Converters enforce this: if a field is not in the DTO, it
cannot leak.

### 3.5 SQL and pgx

**Parameterised queries, always.** String concatenation of user input is an injection and an automatic
fail. **`SELECT *` is banned**: column order changes silently break `Scan`, you ship PII you did not
mean to, and every added column costs bytes on a metered PHC link. List columns explicitly.

```go
const queryGetPatient = `
SELECT id, patient_number, first_name, last_name, date_of_birth, gender, phone, created_at
FROM patients WHERE id = $1 AND deleted_at IS NULL`

query := "SELECT * FROM patients WHERE id = '" + id.String() + "'"   // WRONG: injection
```

**Scanning rows — always `defer rows.Close()`, always check `rows.Err()`.**

```go
rows, err := r.pool.Query(ctx, queryListPatients, limit)
if err != nil {
	return nil, fmt.Errorf("list patients: %w", err)
}
defer rows.Close() // REQUIRED: returns the connection to the pool

patients := make([]domain.Patient, 0, limit)
for rows.Next() {
	var p domain.Patient
	if err := rows.Scan(&p.ID, &p.FacilityID, &p.PatientNumber, &p.FirstName, &p.LastName,
		&p.DateOfBirth, &p.Sex, &p.Phone, &p.CreatedAt); err != nil {
		return nil, fmt.Errorf("scan patient: %w", err)
	}
	patients = append(patients, p)
}
if err := rows.Err(); err != nil { // REQUIRED: false from Next() can mean "error", not "done"
	return nil, fmt.Errorf("iterate patients: %w", err)
}
```

**`pgx.ErrNoRows` is translated at the repo boundary**, so callers only ever see `domain.ErrNotFound`:

```go
err := r.pool.QueryRow(ctx, queryGetPatient, id).Scan( /* ... */ )
if errors.Is(err, pgx.ErrNoRows) {
	return domain.Patient{}, fmt.Errorf("patient %s: %w", id, domain.ErrNotFound)
}
if err != nil {
	return domain.Patient{}, fmt.Errorf("get patient %s: %w", id, err)
}
```

**Transactions use one shared `withTx` helper.** Nobody writes `pool.Begin` inline; that is how a
missing `Rollback` leaks a connection until the pool is exhausted.

```go
// internal/repo/tx.go
type TxFunc func(ctx context.Context, q Querier) error

// withTx runs fn in a transaction. Rollback is always deferred; a Rollback after a successful
// Commit returns pgx.ErrTxClosed, which we ignore on purpose.
func (r *Repo) withTx(ctx context.Context, fn TxFunc) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			r.log.Error().Err(rbErr).Msg("rollback failed")
		}
	}()
	if err := fn(ctx, tx); err != nil {
		return err // already wrapped by the caller's repo method
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
```

The service decides *whether* to transact; the repo decides *how*:

```go
func (s *Billing) RecordPayment(ctx context.Context, in RecordPaymentInput) (domain.Payment, error) {
	var payment domain.Payment
	err := s.repo.WithTx(ctx, func(ctx context.Context, q repo.Querier) error {
		var err error
		payment, err = s.repo.InsertPayment(ctx, q, in) // the balance is DERIVED inside the query
		if err != nil {
			return fmt.Errorf("insert payment: %w", err)
		}
		return s.repo.TouchInvoiceRevision(ctx, q, in.InvoiceID)
	})
	return payment, err
}
```

**Query naming.** One `const queryXxx` per statement, declared above the method that uses it.

| Name | Meaning |
|---|---|
| `queryGetPatient` | single row by PK |
| `queryListPatients` | many rows |
| `queryInsertPatient` | INSERT … RETURNING |
| `queryUpdatePatientRevision` | UPDATE with `revision = revision + 1` |
| `querySoftDeletePatient` | sets `deleted_at`, never `DELETE FROM` |

### 3.6 Gin handlers

**Every handler has the same five moves.** Learn the shape once; every handler after that is typing.

```
1. bind      c.ShouldBindJSON(&req) / c.Param / c.Query
2. validate  a rule the DTO tags cannot express
3. call      the service, with ctx from the request
4. respond   respond.OK(c, status, dto) or respond.Error(c, err)
5. (never)   touch a repo, a pool, or SQL
```

- Handlers do **no** RBAC. Role checks live in the service (architecture §4.1, §6).
- Handlers return the DTO, never the domain struct.
- The request ID lands in every log line for the request and in the error envelope.

```go
// internal/http/handler/patient_handler.go
type PatientHandler struct{ svc *service.Patients }

// CreatePatient handles POST /api/v1/patients. The role check (front_desk or admin) lives in
// service.Patients.Register, not here.
func (h *PatientHandler) CreatePatient(c *gin.Context) {
	var req dto.PatientCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, fmt.Errorf("bind create patient: %w: %w", domain.ErrValidation, err))
		return
	}
	if req.DateOfBirth == "" && req.EstimatedAgeYears == nil { // cross-field rule tags cannot express
		respond.Error(c, fmt.Errorf("date_of_birth or estimated_age_years required: %w",
			domain.ErrValidation))
		return
	}
	in, err := req.ToRegisterInput()
	if err != nil {
		respond.Error(c, fmt.Errorf("build register input: %w", err))
		return
	}
	patient, err := h.svc.Register(c.Request.Context(), in)
	if err != nil {
		respond.Error(c, err) // one place decides the status code
		return
	}
	respond.OK(c, http.StatusCreated, dto.PatientFromDomain(patient))
}
```

```go
// internal/http/respond/respond.go — the ONLY way to write a body.
type Envelope      struct{ Data  any       `json:"data,omitempty"` }
type ErrorEnvelope struct{ Error ErrorBody `json:"error"` }
type ErrorBody struct {
	Code      string `json:"code"`       // machine-readable: VALIDATION_FAILED, CONFLICT, …
	Message   string `json:"message"`    // human-readable, safe to show a user
	RequestID string `json:"request_id"` // correlates with the server log line
}
func OK(c *gin.Context, status int, data any) { c.JSON(status, Envelope{Data: data}) }
```

**Request-ID propagation.** The middleware reads `X-Request-ID` or generates a UUIDv7, stores it in
the context, and echoes it so a user can quote it when reporting a problem.

```go
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = ids.New() // UUIDv7
		}
		c.Request = c.Request.WithContext(middleware.WithRequestID(c.Request.Context(), id))
		c.Header("X-Request-ID", id)
		c.Next()
	}
}
```

### 3.7 Logging with zerolog

Structured JSON only. It goes to stdout and Cloud Run indexes it.

```go
log.Info().
	Str("request_id", rid).Str("user_id", userID.String()).
	Int("ops", len(push.Ops)).Msg("sync push applied")
```

| Level | Use it for | Never for |
|---|---|---|
| `Debug` | SQL timings, Dexie row counts, sync cursors. Off in production | anything you would not want grepped in prod |
| `Info` | Request completed, sync batch applied, login, device registered | per-row loops — one line per batch, not per op |
| `Warn` | Retryable failure, LWW-resolved stale revision, PIN failure, clock skew | expected 4xx validation errors from users |
| `Error` | 5xx, DB errors, outbox drain exhausted after retries | 404s — those are normal |
| `Fatal` | Only in `main`: config missing, pool unreachable | anywhere a request is in flight |

```go
fmt.Println("patient saved")                    // WRONG: unstructured, unsearchable
log.Printf("password=%s", pw)                   // WRONG: secret in the log
log.Info().Interface("patient", p).Msg("saved") // WRONG: dumps PII, and full rows are huge
```

Never log passwords or hashes, PIN values (log the *failure count*), JWTs or refresh tokens, referral
access tokens (log the referral **ID** or a token prefix), full patient rows, or national identifiers.
Log **IDs**, not records. Any log line handling something sensitive needs a `// SECURITY:` comment (§9).

---

## 4. TypeScript / React standards

### 4.1 Compiler and types

`strict: true` plus `noUncheckedIndexedAccess` (full `tsconfig.json` in §12.3). Together they mean
"indexing an array gives `T | undefined`" — annoying for one afternoon, then it prevents the classic
`undefined.map()` crash in the queue screen.

**Ban `any`.** Use `unknown` and narrow. `any` is allowed in exactly two places, each with an inline
comment: a wrong third-party type definition, and a `.d.ts` shim. Everything else is `unknown`, a
generic, or a real type.

```ts
function total(charges: any[]) { ... }                       // WRONG: disables every guarantee
function totalMinor(charges: readonly ChargeRow[]): number {  // CORRECT
  return charges.reduce((sum, c) => sum + c.amountMinor, 0);
}
function parseAmount(raw: unknown): number {                  // CORRECT: narrow at the boundary
  if (typeof raw !== 'number' || !Number.isInteger(raw)) throw new Error('amountMinor must be an int');
  return raw;
}
```

**`type` for data, `interface` for extendable contracts.**

```ts
type PatientRow = { id: string; patientNumber: string; dateOfBirth: string; gender: string };

interface ButtonProps extends React.ComponentPropsWithoutRef<'button'> { // gets spread/merged
  variant?: 'primary' | 'ghost' | 'danger';
}
```

**Naming.**

| Kind | Convention | Example |
|---|---|---|
| Component | `PascalCase` | `PatientForm`, `QueueTable` |
| Hook | `use` + `camelCase` | `usePatientList`, `useSyncStatus` |
| Type / interface | `PascalCase` | `PatientRow`, `ButtonProps` |
| Variable / function | `camelCase` | `amountMinor`, `recordPayment` |
| Constant | `SCREAMING_SNAKE` | `MAX_SYNC_BATCH`, `REFERRAL_TTL_HOURS` |
| Zod schema | `thingSchema` | `patientSchema`, `paymentSchema` |
| Inferred type | same name as schema, `PascalCase` | `type Patient = z.infer<typeof patientSchema>` |
| Dexie table | plural camelCase, matching the SQL plural | `db.patients`, `db.labOrders` |
| Boolean | `is` / `has` prefix | `isOffline`, `hasPendingOps` |
| Handler prop / impl | `on` + event / `handle` + event | `onSubmit` / `handleSubmit` |

The schema and its inferred type are the **single definition** of a shape — validated at the form
boundary, at the Dexie write boundary, and again in the sync payload builder.

```ts
// web/src/lib/schemas/patient-schema.ts
import { z } from 'zod';

export const patientSchema = z
  .object({
    firstName: z.string().trim().min(1, 'First name is required').max(80),
    lastName: z.string().trim().min(1, 'Last name is required').max(80),
    dateOfBirth: z.iso.date().optional(),
    estimatedAgeYears: z.number().int().min(0).max(120).optional(),
    sex: z.enum(['male', 'female', 'other', 'unknown']),
    phone: z.string().trim().max(20).optional(),
  })
  .refine((v) => v.dateOfBirth !== undefined || v.estimatedAgeYears !== undefined, {
    message: 'Provide a date of birth or an estimated age', path: ['dateOfBirth'],
  });

export type PatientInput = z.infer<typeof patientSchema>;
```

### 4.2 State: the three buckets (restated because it is the #1 mistake)

| Kind | Tool | Never |
|---|---|---|
| Domain data | Dexie + `useLiveQuery` | never `useState`, never TanStack Query, never HTTP fetch |
| UI state | `useState` / Zustand | never persisted as if it were data |
| Sync/session status | Zustand | never in Dexie |

Clinical data is read with `useLiveQuery` and **never mirrored into React state**. The moment you copy
a query result into `useState`, you have a second copy that `useLiveQuery` can no longer update.

```ts
const patients = useLiveQuery(          // CORRECT: Dexie pushes updates into the screen
  () => db.patients.where('surname').equalsIgnoreCase(prefix).and((p) => p.deletedAt === null).toArray(),
  [prefix],
);

const [patients, setPatients] = useState<PatientRow[]>([]);   // WRONG: frozen at mount
useEffect(() => { db.patients.toArray().then(setPatients); }, []);
```

### 4.3 How to add a new screen — the ordered checklist

Follow the numbers. Do not skip to step 5 because the route is the fun part.

| # | Step | File | Notes |
|---|---|---|---|
| 1 | **Define the shape** — Zod schema, enums as `z.enum` | `lib/schemas/<thing>-schema.ts` | Export `thingSchema` and `type Thing = z.infer<typeof thingSchema>` |
| 2 | **Check the Dexie table and index.** If new, add the table *and* a new `db.version(n+1).stores({...})` | `db/db.ts` | Never edit an existing `version()` block — devices already ran it (§5.3, same rule) |
| 3 | **Add the write helper** — row upsert + outbox append, one transaction | `features/<x>/api.ts` | Calls `localWrite`. See §4.4 |
| 4 | **Add the read hook** — a `useLiveQuery` hook per list/record | `features/<x>/hooks/useThing.ts` | Returns `{ data, isLoading }`; no fetching, no effects |
| 5 | **Add the route file** — thin: read params, call the hook, render | `routes/<path>.tsx` | Register it in the route tree if the router does not file-glob |
| 6 | **Build the form** — react-hook-form + `zodResolver(thingSchema)` | `features/<x>/components/ThingForm.tsx` | Submit calls the step-3 helper; no business logic here |
| 7 | **Wire the outbox path** — register the op name on **both** sides | `sync/pusher.ts` + `api/internal/sync/opregistry.go` | An outbox row with an unknown op is a dead letter; add both in one commit |
| 8 | **Role/route guard** — add the route to the role→route table | `app/router.tsx` | UI guard only; the service check is the real boundary |
| 9 | **Tests** — unit for pure logic, fake-indexeddb for the write path, E2E if it is one of the four flows | see §7 | A screen with no test is not done |
| 10 | **Offline + print check** — works with `setOffline(true)`; thermal layout if it prints | `styles/print.css` | Add the 58–80 mm print test if it renders a receipt |

### 4.4 The write path, as code

Every clinical mutation goes through **one** helper: a single transaction containing **both** the row
upsert and the outbox append. In separate transactions, a crash between them loses the record silently
— the row exists locally and never replicates.

```ts
// web/src/db/local-write.ts
export type SyncableTableName =
  | 'patients' | 'encounters' | 'vitals' | 'labOrders'
  | 'prescriptions' | 'invoices' | 'payments' | 'referrals';

export type LocalWriteOptions<T> = {
  table: SyncableTableName;                      // which Dexie table
  row: T & { id: string };                       // full row; id must be a client UUIDv7
  op: string;                                    // server op name, e.g. 'patient.upsert'
  actorId: string;                               // from the session store, not a prop chain
  kind?: 'create' | 'update' | 'delete';         // server-side apply semantics
};

/**
 * Upserts one syncable row and appends its outbox entry in a SINGLE Dexie transaction.
 * Returns as soon as the local transaction commits — never awaits the network.
 */
export async function localWrite<T extends { id: string }>(opts: LocalWriteOptions<T>): Promise<void> {
  const { table, row, op, actorId, kind = 'update' } = opts;
  const now = nowUtc();

  await db.transaction('rw', db.table(table), db.outbox, async () => {
    const existing = await (db.table(table) as Table<T, string>).get(row.id);

    await (db.table(table) as Table<T, string>).put({
      ...row,
      revision: existing ? (existing['revision' as keyof T] as number) + 1 : 1,
      syncState: 'pending',
      updatedAt: now, updatedBy: actorId,
      createdAt: existing ? (existing['createdAt' as keyof T] as string) : now,
      createdBy: existing ? (existing['createdBy' as keyof T] as string) : actorId,
    } as T);

    await db.outbox.add({
      id: uuidv7(),          // outbox row id
      operationId: uuidv7(), // idempotency key sent to the server — see §6.3
      entity: table, entityId: row.id, op, kind, payload: row,
      attempts: 0, nextAttemptAt: now, createdAt: now,
    });
  });
  // Nothing after this line. No fetch. No await of the pusher.
}
```

A feature never calls `db.patients.put()` directly:

```ts
// web/src/features/patients/api.ts
export async function registerPatient(input: PatientInput, actorId: string) {
  const row = patientRowSchema.parse({
    id: uuidv7(),                  // client-generated, time-ordered
    patientNumber: await nextPatientNumber(),
    ...input,
    deletedAt: null,               // soft delete only — never a hard delete (§6.4)
    syncState: 'pending',
    createdAt: nowUtc(), updatedAt: nowUtc(), createdBy: actorId, updatedBy: actorId,
  });
  await localWrite({ table: 'patients', row, op: 'patient.upsert', actorId, kind: 'create' });
  return row;
}
```

### 4.5 Component conventions

- **One component per file.** Default-export route files; named-export `components/ui/*`.
- **Type props inline**: `type ThingProps = { … }` above the component; extend DOM props with `ComponentPropsWithoutRef<'button'>` when wrapping a native element.
- **No business logic in components.** Rules, calculations, and writes live in `features/*/api.ts` or a hook. A component may format; it may not decide.
- **Lift state only when two siblings need it**, to the nearest common parent or a feature hook. Never "just in case".
- **Compose, do not prop-drill.** Beyond two levels, pass `children` or a slot, or read from a hook.
- **Radix gives behaviour and accessibility; CVA gives variants; `cn()` merges classes.**

```tsx
const button = cva(
  'inline-flex items-center justify-center rounded-md font-medium transition-colors ' +
    'focus-visible:outline-none focus-visible:ring-2 disabled:opacity-50',
  {
    variants: {
      variant: {
        primary: 'bg-primary text-primary-foreground hover:bg-primary/90',
        ghost: 'hover:bg-muted',
        danger: 'bg-danger text-danger-foreground hover:bg-danger/90',
      },
      size: { sm: 'h-8 px-3 text-sm', md: 'h-10 px-4', lg: 'h-12 px-6 text-lg' },
    },
    defaultVariants: { variant: 'primary', size: 'md' },
  },
);

type ButtonProps = React.ComponentPropsWithoutRef<'button'> & VariantProps<typeof button>;

export function Button({ className, variant, size, ...rest }: ButtonProps) {
  return <button className={cn(button({ variant, size }), className)} {...rest} />;
}
```

### 4.6 Styling

- **Utilities only from tokens.** Tokens live in `@theme` in `styles/tokens.css` (Tailwind v4 is CSS-first; there is no JS config). Raw hex in a component is a review failure.

```css
@theme {
  --color-primary: oklch(0.55 0.15 250);
  --color-danger: oklch(0.58 0.20 25);
  --radius-md: 0.5rem;
  --spacing-touch: 2.75rem;   /* minimum tap target on a clinic tablet */
}
```

```tsx
<div className="rounded-md bg-primary p-4 text-primary-foreground">…</div>  {/* CORRECT */}
<div className="bg-[#2b6cb0]">…</div>                                      {/* WRONG: raw hex */}
```

- **Utility order**: `layout → box → spacing → typography → colour → state → responsive`. Example: `flex items-center gap-2 rounded-md border p-3 text-sm font-medium text-foreground hover:bg-muted md:p-4`. `prettier-plugin-tailwindcss` sorts this for you (§12.4).
- **`cn()` for conditional classes**, never template-literal ternary soup: `cn('rounded-md p-3', isUrgent && 'border-danger bg-danger/10')`. tailwind-merge cannot resolve conflicts across concatenated strings.
- **When a component deserves CVA:** it has **two or more independent visual axes** (variant × size), or it appears in ≥3 places with different looks. Otherwise plain classes or a `cn()` conditional.
- Never override another component's internals with global CSS. Pass a `className` and merge it.

### 4.7 Forbidden frontend patterns

| Forbidden | Why | Do this instead |
|---|---|---|
| `useEffect` for data fetching | Creates a stale copy and a race | `useLiveQuery`; TanStack Query only for the 3 permitted HTTP reads |
| Domain data in `useState` | Second source of truth, never updates after sync | Read from Dexie at render time |
| `alert()` / `confirm()` | Blocks the loop, unstyled, untestable | `sonner` toast; Radix `Dialog` to confirm |
| Arbitrary `setTimeout` for sync | Races, drift, no backoff, dies when the tab sleeps | The sync engine's scheduler + `navigator.onLine` |
| `any` | Silently disables every guarantee you paid for | `unknown` + narrowing, or a real type |
| Index as `key` on a reorderable list | React reuses the wrong DOM node | A stable id: `key={p.id}` |
| `new Date()` / `Date.now()` inline | Non-injectable, and `Date.now()` jumps backwards | `nowUtc()` from `lib/clock.ts` |
| `fetch()` inside a component | Bypasses the outbox and the sync engine | Feature `api.ts` + `localWrite` |
| Decimal `number` arithmetic on money | Floating-point drift in a cash drawer | Integer minor units + `lib/money.ts` |
| `localStorage` for clinical data | Synchronous, ~5 MB, no queries, easily cleared | Dexie |

---

## 5. Database and migration standards

### 5.1 Naming

| Object | Rule | Example |
|---|---|---|
| Table | plural `snake_case` | `patients`, `lab_orders`, `referral_access_log` |
| Column | singular `snake_case` | `patient_number`, `queued_at` |
| Boolean column | `is_` / `has_` prefix | `is_active`, `has_allergy` |
| Timestamp column | `_at` suffix, `TIMESTAMPTZ` | `created_at`, `expires_at`, `device_recorded_at` |
| Money column | `_minor` suffix, `BIGINT` | `amount_minor`, `unit_price_minor` |
| Foreign key | `<singular_table>_id` | `patient_id`, `invoice_id`, `referral_id` |
| Primary key | always `id` | `id uuid PRIMARY KEY` |
| Index | `idx_<table>_<cols>` | `idx_encounters_workflow_stage` |
| Unique constraint | `uq_<table>_<cols>` | `uq_patients_number` |
| Foreign key constraint | `fk_<table>_<col>` | `fk_encounters_patient` |
| Check constraint | `ck_<table>_<rule>` | `ck_invoices_status` |

### 5.2 The migration template — copy this

Files are `NNNNN_verb_noun.sql` in `api/db/migrations/`. goose requires the `Up` and `Down` markers
exactly as below. **The Down must genuinely reverse the Up.**

```sql
-- api/db/migrations/00012_create_referrals.sql
-- +goose Up

CREATE TABLE referrals (
    -- ── sync column contract (02-SYNC-PROTOCOL.md §2) ──────────────────────
    id                 uuid        PRIMARY KEY,
    revision           bigint      NOT NULL DEFAULT 1,
    server_rev         bigint      NOT NULL DEFAULT 0,
    sync_state         text        NOT NULL DEFAULT 'synced',
    deleted_at         timestamptz NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    created_by         uuid        NOT NULL,
    updated_by         uuid        NOT NULL,
    -- ── domain columns ─────────────────────────────────────────────────────
    patient_id         uuid        NOT NULL REFERENCES patients (id)   ON DELETE RESTRICT,
    encounter_id       uuid        NOT NULL REFERENCES encounters (id) ON DELETE RESTRICT,
    from_facility_id   uuid        NOT NULL REFERENCES facilities (id) ON DELETE RESTRICT,
    to_facility_name   text        NOT NULL,
    reason             text        NOT NULL,
    priority           text        NOT NULL DEFAULT 'routine',
    access_token_hash  bytea       NOT NULL,
    pin_hash           bytea       NULL,
    expires_at         timestamptz NOT NULL,
    revoked_at         timestamptz NULL,

    CONSTRAINT ck_referrals_priority CHECK (priority IN ('routine', 'urgent', 'emergency')),
    CONSTRAINT ck_referrals_sync_state CHECK (sync_state IN ('synced', 'pending', 'conflict')),
    CONSTRAINT ck_referrals_expiry CHECK (expires_at > created_at)
);

-- Indexes are deliberate: one for the sync pull cursor, one for the public lookup.
CREATE UNIQUE INDEX uq_referrals_token_hash     ON referrals (access_token_hash);
CREATE INDEX        idx_referrals_server_rev    ON referrals (server_rev);
CREATE INDEX        idx_referrals_patient_id    ON referrals (patient_id);
-- Partial index: the public view only ever looks up live, unexpired referrals.
CREATE INDEX        idx_referrals_live_expiry   ON referrals (expires_at) WHERE deleted_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_referrals_live_expiry;
DROP INDEX IF EXISTS idx_referrals_patient_id;
DROP INDEX IF EXISTS idx_referrals_server_rev;
DROP INDEX IF EXISTS uq_referrals_token_hash;
DROP TABLE IF EXISTS referrals;
```

A Down that is empty, or that only drops the table while leaving the indexes and constraints as
implicit collateral, is not reversible in the sense that matters: you cannot read it and predict the
end state. Write the drops explicitly, in reverse dependency order.

### 5.3 Migration rules

| Rule | Detail |
|---|---|
| **One logical change per migration** | Add a table, or add a column, or add an index. Not all three unrelated things. A failing migration should tell you what it was doing. |
| **Never edit an applied migration** | Once it is on the Neon dev branch, it is history. Changing it means your machine and the branch disagree forever, with no error to tell you. Fix forward with a new migration. |
| **Always write a working Down** | You will need it when a sprint demo goes wrong. Test it: `goose -dir db/migrations down` then `up` again, and confirm the schema matches. |
| **`NOT NULL DEFAULT` on new columns** | Adding a `NOT NULL` column with no default fails on a non-empty table. Give it a default, then drop the default in a later migration if it should not persist. |
| **Add indexes deliberately** | Every index costs write throughput and storage. Justify it with a query that exists today. Never index "because it might be slow". |
| **`CHECK` for enums, plus a lookup table where the UI needs labels** | See §5.4. |
| **Explicit `ON DELETE`** | No default. Write `ON DELETE RESTRICT` for anything referencing clinical data (an invoice must not vanish because a patient row was deleted), `ON DELETE CASCADE` only for true owned children (e.g. `encounter_vitals` if you had such a table), `ON DELETE SET NULL` for optional attribution. |
| **Timestamps are `TIMESTAMPTZ`** | Never `timestamp`. Never a string. Never a local time. |
| **Money is `BIGINT`** | Column name ends `_minor`. Never `numeric`, never `double precision`, never `money`. |
| **No stored balances** | If you are typing `balance_minor`, stop: it is derived (§7.2 of the architecture). |
| **Forward-only in production** | Roll back by writing the inverse migration, not by running a Down against a database that already has newer data in it. |

### 5.4 Enum strategy: `CHECK` constraints, not native `ENUM`

Use `text` + `CHECK (col IN (...))`. Native Postgres `ENUM` types are the wrong tool here, for three
concrete reasons:

1. **Altering a native enum is painful.** `ALTER TYPE … ADD VALUE` still cannot be *used* in the same
   transaction that added it, and removing a value effectively means recreating the type and rewriting
   every dependent column. A `CHECK` constraint is dropped and recreated in one statement.
2. **Offline clients must tolerate unknown values.** An old client will eventually read a row written by
   a newer deployment. With `text`, it reads a string it does not recognise and renders "Unknown
   (pending update)". A native enum gives you a driver-level cast error instead.
3. **Migration diffs stay reviewable.** `CHECK (status IN ('a','b'))` shows the full allowed set in the
   migration file. An enum type is defined elsewhere and read by jumping files.

**The trade-off, stated honestly for your defence:** a `CHECK` constraint is a schema object just like
an enum, so *adding* a status still needs a migration — you do not get schemaless freedom. What you get
is cheap, transactional *changing* of the set, and graceful client behaviour on unknown values. If a
value set is genuinely universal and never extended (e.g. `sex`), the difference is small; consistency
across the schema is worth more than a micro-optimisation on one table.

Where the UI needs human labels or an ordering, add a small lookup table **alongside** the constraint —
the constraint protects integrity, the lookup table carries display text:

```sql
-- +goose Up
CREATE TABLE referral_priorities (
    code text PRIMARY KEY, label text NOT NULL, sort_order int NOT NULL
);
-- +goose StatementBegin
INSERT INTO referral_priorities (code, label, sort_order) VALUES
    ('routine', 'Routine', 1), ('urgent', 'Urgent', 2), ('emergency', 'Emergency', 3)
ON CONFLICT (code) DO NOTHING;
-- +goose StatementEnd
```

goose needs `-- +goose StatementBegin` / `-- +goose StatementEnd` around any statement that itself
contains a semicolon (`DO` blocks, functions, some multi-row inserts). When in doubt, wrap it.

### 5.5 Seed data

Seeds live in `api/db/seed/` and are **idempotent**: running them twice must be a no-op, because you
*will* run them twice (fresh Neon branch, then after a reset).

```sql
-- api/db/seed/0001_seed_facility.sql — safe to run on every `make seed`.
INSERT INTO facilities (id, name, code, currency_code, timezone, created_at, updated_at)
VALUES ('01920000-0000-7000-8000-000000000001', 'Demo Primary Healthcare Centre',
        'NGN', 'Africa/Lagos', now(), now())
ON CONFLICT (id) DO NOTHING;
```

Seed rules:

- **Fixed UUIDs for seeded entities**, written as literals, so tests and screenshots are reproducible.
- **`ON CONFLICT DO NOTHING`** (or `DO UPDATE SET` when you deliberately want to refresh a label).
- **No clinical data in seeds.** Seed the facility, tariffs, users, and lookup tables. Fake patients belong in a separate, clearly-labelled dev script — never in a seed that could run against staging.
- **Never seed a password hash.** Generate it through the same Argon2 path the app uses, via a script that reads the password from an environment variable.
- Seeds are **not** migrations. They do not appear in `schema_migrations` and must be re-runnable.

### 5.6 Migration review checklist

Tick every line before you commit a migration.

- [ ] Filename is `NNNNN_verb_noun.sql`, number is the next free one, no gaps reused.
- [ ] `-- +goose Up` and `-- +goose Down` present, spelled exactly.
- [ ] Every syncable table has **all ten** sync columns, in the canonical order (§6.6).
- [ ] `id uuid PRIMARY KEY` — no `DEFAULT gen_random_uuid()`, IDs come from the client.
- [ ] Money columns end `_minor` and are `BIGINT`.
- [ ] Timestamps are `TIMESTAMPTZ`, no `timestamp without time zone`.
- [ ] Every FK has an explicit `ON DELETE` and an index on the referencing column.
- [ ] Every enum-ish column has a `ck_<table>_<rule>` `CHECK` constraint.
- [ ] Indexes are named `idx_<table>_<cols>` and each one has a query that will use it.
- [ ] `Down` reverses `Up` completely, drops in reverse order, and has been run once locally.
- [ ] The migration has been applied to a **fresh** database (not just your dev branch), and `goose up` ran clean from zero.
- [ ] If it adds a table the client syncs, the Dexie `version()` bump and the op-registry entry are in the same pull request.
- [ ] If it is server-only, the PR description says so and why.

---

## 6. Sync-specific coding rules

This is where the project breaks if you improvise. Seven rules; each one has a failure story.

### 6.1 One write helper, no exceptions

Every syncable row is written through `localWrite()` (§4.4), never a bare `db.table.put()`. A bare
`put` produces a row with no outbox entry: the user sees it, the server never hears about it, and it
disappears from every other device. Ban `db.<syncableTable>.put|add|update|delete` outside
`db/local-write.ts`; enforce it with the ESLint `no-restricted-syntax` rule in §12.5.

```ts
await db.patients.put(row);                       // WRONG: no outbox entry, never replicates
await localWrite({ table: 'patients', row, op: 'patient.upsert', actorId }); // CORRECT
```

### 6.2 ID generation

One function, `uuidv7()` in `web/src/lib/ids.ts`, wrapping `uuid` v7. It is called in exactly one place
per row — the feature `api.ts`. Server-side `uuid.NewV7()` exists for server-originated records only
(users, devices, audit rows), never for a clinical fact.

```ts
// web/src/lib/ids.ts
import { v7 as uuidv7Impl } from 'uuid';
export const uuidv7 = (): string => uuidv7Impl();

// WRONG everywhere else: not time-ordered, breaks cursor ordering.
const id = crypto.randomUUID();       // v4
const id = String(Date.now());        // not an id
```

Why it matters: v7 is time-ordered, so the primary-key B-tree stays append-mostly and the sync pull
cursor can use the ID as a tiebreaker. v4 scatters writes and gives you nothing back.

### 6.3 Idempotency

Every outbox entry carries a client-generated `operation_id` (UUIDv7). The server must be safe to
receive the same operation twice — the client *will* retry after a timeout that actually succeeded.

```ts
type OutboxEntry = {
  id: string;           // outbox row id (local)
  operationId: string;  // UUIDv7 — the idempotency key sent to the server
  entity: SyncableTableName;
  entityId: string;
  op: string;           // e.g. 'payment.record'
  kind: 'create' | 'update' | 'delete';
  payload: unknown;
  attempts: number;
  nextAttemptAt: string;
  createdAt: string;
};
```

Server side, in the same transaction that applies the operation:

```sql
-- +goose Up (excerpt, from 00009_create_sync_operations.sql)
CREATE TABLE sync_operations (
    operation_id uuid        PRIMARY KEY,
    device_id    uuid        NOT NULL REFERENCES devices (id) ON DELETE RESTRICT,
    op           text        NOT NULL,
    entity_id    uuid        NOT NULL,
    applied_at   timestamptz NOT NULL DEFAULT now()
);
```

```go
// Already applied? Return success, do not re-apply. This is the whole point.
if applied, err := s.ops.WasApplied(ctx, tx, op.OperationID); err != nil {
	return fmt.Errorf("check operation %s: %w", op.OperationID, err)
} else if applied {
	return nil // idempotent: same answer as the first time
}
```

Every server-side apply function must be written as if it will run twice: `INSERT … ON CONFLICT DO
NOTHING` for append-only facts, and a `revision` comparison for mutable rows. Workflow transitions are
idempotent by design (architecture §7.5) — applying `vitals_done` twice is a no-op, not an error.

### 6.4 Never delete rows

Deletes are soft: set `deleted_at` and write an outbox entry of kind `'delete'`. A hard delete cannot
replicate — the other devices have no way to learn that a row should vanish, because absence is not an
event.

```ts
// CORRECT
await localWrite({
  table: 'patients',
  row: { ...patient, deletedAt: nowUtc() },
  op: 'patient.upsert',
  actorId,
  kind: 'delete',
});

// WRONG: nothing to send, so the row lives forever on every other device.
await db.patients.delete(patientId);
```

Server side: `UPDATE … SET deleted_at = now(), revision = revision + 1 WHERE id = $1`. `DELETE FROM` on
a syncable table is forbidden. Every read query filters `deleted_at IS NULL` — including the pull
query, which must send tombstones so clients can apply them.

### 6.5 Never store a balance or a count

No `balance_minor`, no `amount_paid_minor`, no `items_count`, no `queue_position`. Anything another
device can also change must be derived at read time.

```ts
// CORRECT — derived, one definition, in web/src/features/billing/balance.ts
export function invoiceBalanceMinor(invoice: InvoiceRow, payments: PaymentRow[]): number {
  const paid = payments
    .filter((p) => p.invoiceId === invoice.id && p.deletedAt === null && p.reversedAt === null)
    .reduce((sum, p) => sum + p.amountMinor, 0);
  return invoice.totalMinor - paid;
}
```

```sql
-- CORRECT — derived in SQL for the server-side report.
SELECT i.id,
       i.total_minor - COALESCE(SUM(p.amount_minor), 0) AS balance_minor
FROM invoices i
LEFT JOIN payments p
       ON p.invoice_id = i.id AND p.deleted_at IS NULL AND p.reversed_at IS NULL
WHERE i.deleted_at IS NULL
GROUP BY i.id;
```

Counters that a device might want for display (pending ops, today's patients) are computed in a hook or
a Zustand selector from the underlying rows.

### 6.6 The sync column contract — every syncable table, or an explicit exemption

Any new table **must** either carry the full contract or be explicitly marked server-only, with the
reason written in the migration file and the pull request. There is no third option, and "we forgot" is
how a table silently stops replicating.

| Column | Type | Purpose |
|---|---|---|
| `id` | `uuid PRIMARY KEY` | Client-generated UUIDv7 |
| `revision` | `bigint NOT NULL DEFAULT 1` | Client-side edit counter, bumped on every local write |
| `server_rev` | `bigint NOT NULL DEFAULT 0` | Server-assigned monotonic cursor for pull |
| `sync_state` | `text NOT NULL DEFAULT 'synced'` | `synced` \| `pending` \| `conflict` |
| `deleted_at` | `timestamptz NULL` | Soft delete / tombstone |
| `created_at` | `timestamptz NOT NULL DEFAULT now()` | Client clock, corrected server-side if absurd |
| `updated_at` | `timestamptz NOT NULL DEFAULT now()` | Last local edit |
| `created_by` | `uuid NOT NULL` | Actor attribution, survives offline |
| `updated_by` | `uuid NOT NULL` | Last actor |

```sql
-- +goose Up (excerpt) — server-only tables say so, loudly.
-- SYNC: server-only. audit_logs is deliberately NOT synced to clients: an audit trail that
-- replicates to the machine being audited is not an audit trail (05-SECURITY.md §6).
CREATE TABLE audit_logs (
    id         uuid        PRIMARY KEY,
    actor_id   uuid        NULL,
    action     text        NOT NULL,
    entity     text        NOT NULL,
    entity_id  uuid        NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
```

The canonical contract itself lives in `01-DATA-MODEL.md`; if the two ever disagree, that document wins
and this one gets a fix.

### 6.7 Monotonic time

One helper, `nowUtc()` in `web/src/lib/clock.ts`, and `clock.NowUTC()` in Go. It returns an ISO-8601
string in UTC. `new Date()` and `Date.now()` are banned outside those helpers — not because they are
wrong, but because co-locating them makes a future clock-skew fix a one-file change, and because
`Date.now()` is a *wall clock*: it can jump backwards when the device syncs its time, which will
scramble an ordering that relies on it.

```ts
// web/src/lib/clock.ts
export const nowUtc = (): string => new Date().toISOString();

// WRONG in feature code:
const t = new Date().toISOString();
const order = Date.now();   // jumps backwards if NTP corrects the clock
```

For **ordering** queued operations, use the outbox's auto-increment primary key or a monotonic counter —
never a timestamp. Timestamps record when care happened; they do not order work.

---

## 7. Testing standards

### 7.1 The pyramid, adapted

| Layer | Tool | What it tests | Target count |
|---|---|---|---|
| Pure logic unit | Vitest | Money maths, workflow transitions, sync conflict resolution, Zod schemas | ~60% of tests |
| Local-database unit | Vitest + `fake-indexeddb` | Dexie writes, the outbox transaction, retry/backoff maths | ~20% |
| Go unit | `go test` table-driven | Services with fake repos, money/date helpers, workflow state machine | ~15% |
| End-to-end | Playwright | The four critical flows, **including the offline ones** | ~5%, but non-negotiable |

### 7.2 The four flows that must have E2E tests

Every one of these runs twice: once online, once with `context.setOffline(true)`.

| # | Flow | Assertions that matter |
|---|---|---|
| 1 | **Register patient → vitals → queue** | Patient appears in Dexie; outbox has 2+ entries; queue list shows the patient; after `setOffline(true)` the flow still completes and the entries stay queued |
| 2 | **Doctor → lab order → payment → result** | Encounter reaches `awaiting_lab`; payment reduces the **derived** balance; the result entry is visible after the sync push |
| 3 | **Prescription → payment → dispense** | Payment is required before dispense; dispense records the actor and time; balance derives to zero |
| 4 | **Referral create → print → public view → expiry** | QR renders; `/r/:token` returns the referral JSON; a wrong PIN is rejected and the failure counter increments; an expired token returns 404/410 |

```ts
// web/e2e/offline-registration.spec.ts
import { test, expect } from '@playwright/test';

test('registers a patient and queues the operation while offline', async ({ context, page }) => {
  await page.goto('/patients/new');
  await context.setOffline(true);

  await page.getByLabel('First name').fill('Amina');
  await page.getByLabel('Last name').fill('Bello');
  await page.getByLabel('Sex').selectOption('female');
  await page.getByLabel('Estimated age').fill('34');
  await page.getByRole('button', { name: 'Save patient' }).click();

  // The UI is correct immediately — no network involved.
  await expect(page.getByRole('cell', { name: 'Amina Bello' })).toBeVisible();
  // And the write is queued, not lost.
  expect(await page.evaluate(() => window.__testHooks?.pendingOps() ?? 0)).toBeGreaterThan(0);

  await context.setOffline(false);
  await expect.poll(() => page.evaluate(() => window.__testHooks?.pendingOps() ?? 1)).toBe(0);
});
```

`window.__testHooks` is a test-only bridge, exposed only when `import.meta.env.MODE === 'test'`. It is
the sanctioned way to read Dexie/outbox state from Playwright; do not scrape IndexedDB by hand in a
spec.

### 7.3 Test naming

| Language | Pattern | Example |
|---|---|---|
| Go | `TestXxx`, table-driven with named subtests | `TestApplyPayment`, `t.Run("rejects overpayment", ...)` |
| TS | `describe` = unit under test, `it` reads "does X when Y" | `it('derives a zero balance when payments equal the total')` |

The TS pattern is a sentence: **"does X when Y"**. If you cannot finish the sentence, the test is
testing too much.

### 7.4 Example: Go table-driven test

```go
// api/internal/service/billing_test.go
func TestInvoiceBalanceMinor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		payments []domain.Payment
		want     int64
	}{
		{"returns the full total when nothing is paid", nil, 150_000},
		{"derives a partial balance after one payment",
			[]domain.Payment{{AmountMinor: 50_000}}, 100_000},
		{"ignores soft-deleted payments",
			[]domain.Payment{{AmountMinor: 150_000, DeletedAt: ptr(time.Now())}}, 150_000},
		{"ignores reversed payments",
			[]domain.Payment{{AmountMinor: 150_000, ReversedAt: ptr(time.Now())}}, 150_000},
		{"never returns a negative balance on overpayment",
			[]domain.Payment{{AmountMinor: 200_000}}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inv := domain.Invoice{TotalMinor: 150_000}
			if got := service.InvoiceBalanceMinor(inv, tt.payments); got != tt.want {
				t.Errorf("InvoiceBalanceMinor() = %d, want %d", got, tt.want)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
```

### 7.5 Example: Vitest test for money and balance

```ts
// web/src/features/billing/balance.test.ts
import { describe, it, expect } from 'vitest';
import { invoiceBalanceMinor } from './balance';
import type { InvoiceRow, PaymentRow } from '../../db/schema';

const invoice = (totalMinor: number) =>
  ({ id: 'inv-1', totalMinor, deletedAt: null }) as InvoiceRow;

const payment = (amountMinor: number, over: Partial<PaymentRow> = {}) =>
  ({ id: `pay-${amountMinor}`, invoiceId: 'inv-1', amountMinor,
     deletedAt: null, reversedAt: null, ...over }) as PaymentRow;

describe('invoiceBalanceMinor', () => {
  it('returns the full total when nothing is paid', () => {
    expect(invoiceBalanceMinor(invoice(150_000), [])).toBe(150_000);
  });

  it('derives a partial balance when a payment is recorded', () => {
    expect(invoiceBalanceMinor(invoice(150_000), [payment(50_000)])).toBe(100_000);
  });

  it('ignores soft-deleted payments when deriving the balance', () => {
    const rows = [payment(50_000, { deletedAt: '2025-01-01T00:00:00.000Z' })];
    expect(invoiceBalanceMinor(invoice(150_000), rows)).toBe(150_000);
  });

  it('never reports a negative balance when a payment exceeds the total', () => {
    expect(invoiceBalanceMinor(invoice(150_000), [payment(200_000)])).toBe(0);
  });

  it('keeps whole minor units when many small payments are summed', () => {
    const rows = Array.from({ length: 100 }, () => payment(1_000));
    expect(invoiceBalanceMinor(invoice(100_000), rows)).toBe(0);
  });
});
```

That last test is the one that catches float drift: with `0.1 + 0.2` style arithmetic it fails, with
integer minor units it passes. Keep it.

### 7.6 What NOT to test

- **Framework code.** Do not test that Gin routes, that Dexie stores, or that React re-renders. You are testing the framework's promises, not your code.
- **Trivial getters and DTO converters.** `PatientFromDomain` needs a test only if it formats or transforms (dates, rounding) — a straight field copy is not worth a test.
- **Third-party behaviour.** Do not test Zod's validation, `date-fns` formatting, or Radix focus trapping.
- **Generated or constant-equal code.** `const MAX_SYNC_BATCH = 50` needs no test.
- **Implementation details.** Test `invoiceBalanceMinor`'s *result*, not that it called `.filter()`.

**Coverage is not a goal.** A 95% coverage number achieved by asserting that a component renders is
worse than 60% coverage of the money and workflow code, because it creates maintenance work and false
confidence. Aim for: every pure function in `lib/` and `service/` has a table test; the write path has
an integration test; the four flows have E2E tests. Report coverage as information, never as a target.

---

## 8. Git and review conventions

> Depth lives in `06-GIT-WORKFLOW.md`. This is the summary that applies while you are coding.

### 8.1 Branches

| Prefix | Use | Example |
|---|---|---|
| `feat/` | New user-visible capability | `feat/referral-qr-print` |
| `fix/` | Bug fix | `fix/outbox-double-append` |
| `chore/` | Tooling, deps, config | `chore/pin-typescript-7.0.2` |
| `docs/` | Documentation only | `docs/sync-protocol-notes` |
| `refactor/` | Behaviour-preserving change | `refactor/extract-withtx-helper` |

`<prefix>/<short-kebab-description>`, lower case, under ~40 characters, no ticket-number-only names.

### 8.2 Commits — Conventional Commits

```
<type>(<scope>): <imperative summary, lower case, no full stop>

<body: why, not what — what is already in the diff>

<footer: refs, BREAKING CHANGE>
```

Scopes for this project: `api`, `web`, `db`, `sync`, `billing`, `referrals`, `auth`, `ci`, `docs`.

```
feat(billing): derive invoice balance from payments instead of a stored column

A stored balance is a second source of truth that two offline devices will
always eventually disagree about. The balance is now computed from the payment
ledger in one helper, used by both the cashier screen and the report query.
```

```
fix(sync): append the outbox entry inside the same Dexie transaction as the row

The row and its outbox entry were written in two transactions, so a tab close
between them left a patient visible locally and never replicated. Both writes
now go through localWrite().
```

```
chore(db): apply the sync column contract to the referrals table

Adds the nine sync columns plus a partial index for the public referral lookup.
Down drops the indexes and the table in reverse dependency order.
```

**A commit future-you can understand** answers three questions in the body: *what was wrong*, *why this
fix*, *what would break if someone undid it*. "Fix bug" answers none of them.

### 8.3 Small-commit discipline

- One logical change per commit. If the message needs "and", split it.
- A commit must leave the tree in a state where `go build ./...` and `pnpm typecheck` pass. A broken intermediate commit is a trap for the version of you who bisects it in six weeks.
- **Never** commit a broken migration, a `.env` file, a secret, or a generated `dist/` directory.
- Commit the Dexie `version()` bump, the migration, and the op-registry entry **together** — they are one change.
- Format and lint before committing; do not commit a formatting-only change alongside a logic change (it hides the logic in the diff).

### 8.4 Pull-request self-review checklist

- [ ] The branch contains one feature or fix, and no unrelated formatting noise.
- [ ] `make lint test` passes locally (Go: `golangci-lint run`; web: `pnpm lint && pnpm typecheck && pnpm test`).
- [ ] Every new file has a test, or the PR body says why not.
- [ ] New table → sync columns present, or explicitly marked server-only with a reason.
- [ ] New endpoint → registered in `07-API-CONTRACT.md` and in the router with the right middleware.
- [ ] New error case → mapped to a status code in the §3.2 table, and the envelope includes a machine code.
- [ ] No `fmt.Println`, no `console.log`, no commented-out code, no `TODO` without a name and a reason.
- [ ] No secrets, tokens, real patient names, or production URLs in the diff.
- [ ] The four E2E flows still pass, including the offline variant if this touches the write path.
- [ ] I re-read my own diff line by line before requesting review. If I cannot explain a line, it does not ship.

---

## 9. Comments and documentation

**Comment the *why*, never the *what*.** The code already says what. A comment earns its place by
recording a decision, a constraint, or a citation.

```go
// WRONG — restates the code.
// increment revision by 1
p.Revision++

// CORRECT — records a non-obvious rule and where it comes from.
// revision is bumped on every local write so the server can detect a stale client
// during LWW resolution (02-SYNC-PROTOCOL.md §5.2).
p.Revision++
```

**godoc on every exported Go identifier.** Start with the identifier's name; `golangci-lint`'s `revive`
checks this.

```go
// InvoiceBalanceMinor returns the outstanding balance in minor units.
//
// The balance is always derived (total minus non-reversed, non-deleted payments)
// and never stored: a stored balance is a second source of truth that offline
// devices desynchronise (00-ARCHITECTURE.md §7.2). It never returns a negative number.
func InvoiceBalanceMinor(inv domain.Invoice, payments []domain.Payment) int64 {
```

**JSDoc on every exported TS function**, and on every hook (state what it reacts to and why).

```ts
/**
 * Derives the outstanding balance for an invoice, in minor units.
 *
 * Always derived, never stored — see docs/00-ARCHITECTURE.md §7.2.
 * Soft-deleted and reversed payments are excluded.
 *
 * @param invoice  the invoice row from Dexie
 * @param payments all payments known locally for this invoice
 * @returns outstanding amount in minor units; never negative
 */
export function invoiceBalanceMinor(invoice: InvoiceRow, payments: PaymentRow[]): number {
```

**Marker comments.** Two markers, greppable, used consistently. `grep -rn "// SYNC:" web/src` should
list every line that needs care during a sync change.

| Marker | Means | Required when |
|---|---|---|
| `// SYNC:` | This code is part of the replication contract. Changing it can lose or duplicate data. | Outbox handling, revision bumps, conflict resolution, tombstones, pull cursors, the sync column contract |
| `// SECURITY:` | This code touches authentication, authorisation, secrets, or sensitive data. | Token handling, Argon2, PIN verification, referral token lookup, audit writes, anything logged that could contain PII |

```go
// SYNC: the cursor advances only after the batch commits. Advancing first would skip
// operations permanently if the commit failed.
cursor = batch.LastServerRev

// SECURITY: never log the token itself — it is a bearer credential for clinical data.
// Log the referral id and the token's first 8 characters for correlation only.
log.Warn().Str("referral_id", r.ID.String()).Str("token_prefix", token[:8]).Msg("referral lookup failed")
```

**Cite the requirement on every non-obvious clinical or financial rule.** If a rule comes from the
project brief, a supervisor's note, or a document in `docs/`, name the source in the comment. This is
what turns "I thought that was right" into a defensible answer at your defence.

```go
// Referrals expire 48 hours after issue (REFERRAL_TTL_HOURS). The expiry is enforced in the
// query, not only in the UI: an offline client may hold a stale list of live referrals.
// Source: docs/05-SECURITY.md §4.3; project brief requirement FR-7.2.
now := clock.NowUTC()
```

**Do not comment out code.** Delete it; git remembers. A commented-out block is a lie about what the
program does.

---

## 10. Definition of Done (per task)

Tick every box before moving the task to Done. A task missing one of the top six boxes is not done, no
matter how well it demos.

- [ ] **Behaviour** — the acceptance criterion is met, and I have used the feature end-to-end myself.
- [ ] **Offline** — I ran the flow with DevTools offline (or `setOffline(true)`) and it worked: the write landed in Dexie, the outbox entry exists, and the UI is already correct.
- [ ] **Write path** — every clinical write goes through `localWrite()`, with the row upsert and outbox append in one Dexie transaction.
- [ ] **Derived values** — no balance or count was stored; anything derived has exactly one helper, used everywhere.
- [ ] **Sync contract** — any new table has the nine sync columns (and no `facility_id`) or is explicitly marked server-only with a reason.
- [ ] **Security** — the role check is in the service layer, not only in the UI; no secret, PIN, or token is logged; the endpoint is in `07-API-CONTRACT.md`.
- [ ] **Migration** — one logical change, working `Down`, tested on a fresh database, review checklist (§5.6) ticked.
- [ ] **Types** — `pnpm typecheck` and `go build ./...` pass; no `any`, no `as` cast used to silence the compiler.
- [ ] **Tests** — new pure logic has a table test; the write path has a fake-indexeddb test; E2E added if this is one of the four flows.
- [ ] **Lint** — `golangci-lint run` and `pnpm lint` are clean; no `//nolint` without a reason on the same line.
- [ ] **Errors** — new failure modes return a domain error mapped in §3.2, or are genuinely impossible with a comment saying why.
- [ ] **Logging** — meaningful events are logged with structured fields; nothing sensitive appears in a log line.
- [ ] **Docs** — the API contract, data model, or this document is updated if the change contradicts it. Docs and code ship in the same PR.
- [ ] **Traceability** — the commit message and any requirement citation say which requirement this implements.
- [ ] **Demo** — I can show this working in under two minutes, including the offline half.

---

## 11. Common mistakes to avoid

These are the ones that will actually happen in *this* project. Each has a one-line antidote.

| # | Mistake | Antidote |
|---|---|---|
| 1 | Writing the row and the outbox entry in **two** Dexie transactions | One `localWrite()` call; both writes inside one `db.transaction('rw', ...)` |
| 2 | Adding a table without the sync columns | Copy the contract block from §5.2, or write "server-only because …" in the migration |
| 3 | Using `Date.now()` / `new Date()` inline to assign `created_at` | `nowUtc()` only; for ordering, use the outbox's auto-increment key |
| 4 | Storing money as a float, or as `numeric`/`money` in Postgres | `BIGINT` + `_minor` suffix + integer arithmetic in `lib/money.ts` |
| 5 | Fetching clinical data over HTTP into React state | `useLiveQuery` from Dexie; HTTP is only for the 3 permitted reads |
| 6 | Hard-coding the API port instead of `$PORT` | `os.Getenv("PORT")` with a `8080` local default only |
| 7 | Pointing the API at the Neon **direct** URL, or goose at the **pooled** URL | Pool = `DATABASE_URL` (`-pooler`); goose = `MIGRATION_DATABASE_URL` (direct) |
| 8 | Forgetting `QueryExecMode: pgx.QueryExecModeExec` | Set it in one place in `main.go`, and add a startup assertion that fails fast if it is missing |
| 9 | Editing a migration that has already been applied | Fix forward with a new numbered migration, always |
| 10 | Generating a clinical row's ID on the server | `uuidv7()` on the client; the server only ever accepts the ID it is given |
| 11 | A bare `db.patients.put()` that skips the outbox | `localWrite()`; the ESLint rule in §12.5 blocks the alternative |
| 12 | `DELETE FROM` on a syncable table | Soft delete via `deleted_at`, and push the tombstone so other devices learn about it |
| 13 | Storing `balance_minor` or `amount_paid_minor` | Derive it; one helper, used by the screen and the report |
| 14 | Checking the user's role in a handler or a repo | RBAC lives in `internal/service` only (architecture §4.1) |
| 15 | Returning a raw `err.Error()` to the client | `respond.Error(c, err)`; 5xx shows a generic message, the detail goes to the log |
| 16 | Forgetting `defer rows.Close()` or `rows.Err()` | Both, every time; a leaked connection empties the pool at demo time |
| 17 | `useEffect` + `fetch` on a screen | Declare the query in `useLiveQuery`; the sync engine owns the network |
| 18 | `alert()` / `confirm()` in a UI path | Sonner toast; Radix `Dialog` for destructive confirmation |
| 19 | `key={index}` on a reorderable list | `key={row.id}` — always a stable identifier |
| 20 | Ignoring an error with `_`, or `panic` outside `main` | Wrap with `%w` and return; the middleware maps it to a status |
| 21 | Logging a PIN, token, or full patient row | Log IDs, prefixes, and counts; mark the line `// SECURITY:` |
| 22 | Bumping Dexie by editing an existing `version()` block | Add a **new** `db.version(n+1).stores({...})`; old devices already ran the old one |

---

## 12. Tooling configuration

Pin versions. A dependency bump mid-sprint is a lost afternoon, not progress.

### 12.1 `.golangci.yml` (repo root, applies to `api/`)

```yaml
version: "2"
run:
  timeout: 5m
  tests: true
linters:
  default: none
  enable:              # each one enforces a rule from §3
    - errcheck         # no ignored errors
    - govet
    - ineffassign
    - staticcheck
    - unused
    - revive           # exported identifiers need godoc
    - errorlint        # %w, not %v, and errors.Is/As
    - bodyclose
    - rowserrcheck     # rows.Err() checked
    - sqlclosecheck    # rows.Close() called
    - noctx            # no HTTP/SQL without a context
    - containedctx     # no context.Context stored in a struct
    - gosec            # obvious security smells
    - exhaustive       # switch over enums covers every case
  settings:
    revive:
      rules:
        - { name: exported, arguments: [checkPrivateReceivers] }
        - name: package-comments
    errcheck: { check-type-assertions: true, check-blank: true }
formatters:
  enable: [gofmt, goimports]
issues:
  max-issues-per-linter: 0
  max-same-issues: 0
  exclude-rules:
    - path: _test\.go
      linters: [gosec, errcheck]   # tests may ignore errors while asserting
```

### 12.2 `.editorconfig` (repo root)

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
indent_style = space
indent_size = 2

[*.go]
indent_style = tab
indent_size = 4

[*.{sql,md}]
indent_size = 4
trim_trailing_whitespace = false   # markdown hard breaks and SQL alignment
```

### 12.3 `web/tsconfig.json`

```jsonc
{
  "compilerOptions": {
    "target": "ES2023",
    "lib": ["ES2023", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",

    /* Strictness — do not weaken any of these to make a build pass. */
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "noFallthroughCasesInSwitch": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "exactOptionalPropertyTypes": true,
    "verbatimModuleSyntax": true,

    "noEmit": true,
    "skipLibCheck": true,
    "baseUrl": ".",
    "paths": { "@/*": ["./src/*"] },
    "types": ["vite/client", "vitest/globals"]
  },
  "include": ["src", "e2e", "vite.config.ts", "playwright.config.ts"]
}
```

### 12.4 Prettier — `web/.prettierrc.json`

```json
{
  "semi": true,
  "singleQuote": true,
  "trailingComma": "all",
  "printWidth": 100,
  "tabWidth": 2,
  "arrowParens": "always",
  "plugins": ["prettier-plugin-tailwindcss"],
  "tailwindStylesheet": "./src/styles/tokens.css",
  "tailwindFunctions": ["cn", "cva"]
}
```

`prettier-plugin-tailwindcss` **sorts the utility classes for you** — that is how the ordering rule in
§4.6 stays true without discipline. `tailwindFunctions` tells it to sort inside `cn()` and `cva()` too.
`.prettierignore`: `dist`, `coverage`, `playwright-report`, `test-results`, `src/types/generated`.

### 12.5 ESLint flat config — `web/eslint.config.js` (highlights)

```js
import tseslint from 'typescript-eslint';
import reactHooks from 'eslint-plugin-react-hooks';

export default tseslint.config(
  { ignores: ['dist', 'coverage', 'playwright-report', 'src/types/generated'] },
  ...tseslint.configs.strictTypeChecked,
  {
    files: ['**/*.{ts,tsx}'],
    plugins: { 'react-hooks': reactHooks },
    rules: {
      ...reactHooks.configs.recommended.rules,

      // ── The rules that protect the offline design ──────────────────────
      'no-restricted-syntax': ['error',
        {
          selector:
            "CallExpression[callee.object.object.name='db'][callee.property.name=/^(put|add|update|delete|bulkPut)$/]",
          message: 'Write syncable rows with localWrite() so the outbox entry shares the transaction.',
        },
        { selector: "NewExpression[callee.name='Date'][arguments.length=0]",
          message: 'Use nowUtc() from lib/clock.ts.' },
        { selector: "CallExpression[callee.object.name='Date'][callee.property.name='now']",
          message: 'Use nowUtc(). Date.now() is not monotonic.' },
        { selector: "CallExpression[callee.object.name='crypto'][callee.property.name='randomUUID']",
          message: 'Use uuidv7() from lib/ids.ts. v4 IDs break cursor ordering.' },
      ],

      // ── Types: the only reviewer you have at 2 a.m. ────────────────────
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unsafe-assignment': 'error',
      '@typescript-eslint/consistent-type-imports': ['error', { prefer: 'type-imports' }],
      '@typescript-eslint/no-floating-promises': 'error', // an unawaited write is a lost record

      // ── UX: dialogs and toasts, not browser modals ─────────────────────
      'no-alert': 'error',
      'no-console': ['error', { allow: ['warn', 'error'] }],
    },
  },
  { files: ['src/db/local-write.ts', 'src/db/db.ts'], rules: { 'no-restricted-syntax': 'off' } },
  { files: ['**/*.test.ts', '**/*.test.tsx', 'e2e/**'], rules: { 'no-console': 'off' } },
);
```

### 12.6 `web/package.json` scripts

```jsonc
{
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",   // typecheck FIRST: never ship a red build
    "typecheck": "tsc --noEmit",
    "lint": "eslint . --max-warnings=0",
    "format": "prettier --write .",
    "format:check": "prettier --check .",
    "test": "vitest run",
    "test:coverage": "vitest run --coverage",
    "e2e": "playwright test",
    "e2e:offline": "playwright test --grep @offline",
    "verify": "pnpm format:check && pnpm lint && pnpm typecheck && pnpm test"
  }
}
```

`pnpm verify` is the one command to run before every commit. Wire it into `make verify` at the repo
root so the Go and web checks run together:

```makefile
verify: ## run every check the PR checklist requires
	cd api && golangci-lint run ./... && go test ./...
	cd web && pnpm verify
```

---

## Appendix: the ten lines to memorise

If you remember nothing else from this document, remember these.

```go
ctx context.Context                                     // first parameter, every repo/service method
fmt.Errorf("get patient %s: %w", id, err)               // wrap, never %v
domain.ErrNotFound | ErrConflict | ErrForbidden | ErrValidation
respond.OK(c, http.StatusCreated, dto.PatientFromDomain(p))
os.Getenv("PORT")                                       // never hard-code a port
pgx.QueryExecModeExec                                   // Neon pooler breaks prepared statements
```

```ts
uuidv7()                                                // client-side, one function
nowUtc()                                                // never new Date() inline
await localWrite({ table, row, op, actorId })           // row + outbox, ONE transaction
useLiveQuery(() => db.patients.where('surname').equalsIgnoreCase(name).toArray(), [name])
```

---

*Related documents:* [00-ARCHITECTURE.md](./00-ARCHITECTURE.md) ·
[01-DATA-MODEL.md](./01-DATA-MODEL.md) · [02-SYNC-PROTOCOL.md](./02-SYNC-PROTOCOL.md) ·
[03-UI-PATTERNS.md](./03-UI-PATTERNS.md) · [05-SECURITY.md](./05-SECURITY.md) ·
[06-GIT-WORKFLOW.md](./06-GIT-WORKFLOW.md) · [07-API-CONTRACT.md](./07-API-CONTRACT.md)
