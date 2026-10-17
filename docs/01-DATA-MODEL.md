# Data Model

> **Document status:** Canonical. Every migration must conform to this document.
> **Database:** PostgreSQL 18 (Neon serverless), `TIMESTAMPTZ` in UTC, money as `BIGINT` minor units.
> **Last updated:** Sprint 0

---

## 1. How to read this document

Each table below gives you: its purpose, its columns, its indexes, and its sprint. Type it into a goose migration exactly as specified — do not improvise column names, because the sync engine, the API contract and the UI all key off them.

**Notation**

| Marker | Meaning |
|---|---|
| 🔄 | **Syncable** — carries the full sync column contract (§3). The sync engine generates code for it automatically. |
| 🔒 | **Server-only** — never sent to clients. Must NOT carry `sync_state`. |
| 📅 | The sprint in which this table's migration is written. |

---

## 2. The five modelling decisions

Read these before the tables. They explain *why* the schema looks the way it does, and they are the answers you give when an examiner asks "why did you model it this way?"

### 2.1 Clinical facts are append-only; mutable records are last-write-wins

| Kind | Examples | Conflict behaviour |
|---|---|---|
| **Immutable fact** | `vitals`, `lab_results`, `dispenses`, `payments`, `audit_logs`, `bed_assignments` | Never updated after insert (except a soft-delete/void). Two rows from two devices are **two facts**, not a conflict. This is why vitals record `recorded_at` — a second BP reading is not a correction of the first. |
| **Mutable record** | `patients`, `households`, `users`, `drug_products`, `tariffs` | Genuinely editable. Concurrent edits resolve **last-write-wins per changed field**, using `updated_at` as the tiebreaker (see `02-SYNC-PROTOCOL.md`). |
| **Workflow state** | `encounters.workflow_stage`, `encounters.status` | Last-write-wins, but transitions are validated against the state machine and are **idempotent**. |
| **Money** | `payments`, `payment_allocations` | Protected by **idempotency**, not conflict resolution. Every payment carries a client-generated `operation_id` with a unique constraint. A double-queued payment cannot double-post. |

### 2.2 Balances are derived, never stored

There is **no** `balance` column anywhere in this schema. A patient's or household's outstanding balance is always:

```sql
-- patient balance
SELECT COALESCE(SUM(c.amount_minor), 0)
     - COALESCE((SELECT SUM(a.amount_minor)
                 FROM payment_allocations a
                 JOIN payments p ON p.id = a.payment_id
                 WHERE a.charge_id IN (SELECT id FROM charges WHERE patient_id = $1)
                   AND p.status = 'posted'), 0)
FROM charges c
WHERE c.patient_id = $1 AND c.status = 'active' AND c.deleted_at IS NULL;
```

**Reason:** a stored balance is a second source of truth. Offline devices would each compute a different stored balance, and every one of them would be right. Deriving it guarantees one answer. This costs a slightly more expensive query — a price worth paying for correctness, and one you can index your way out of later if needed.

**Performance note:** for the family account screen and the discharge check, wrap this in a `VIEW` (`v_patient_balance`, `v_household_balance`) so the SQL is written once. Views are in the Sprint 6 migration.

### 2.3 Age is recorded, not just derived

The registration form captures **age** *and* date of birth, because in a PHC many adult patients genuinely do not know their DOB and the clerk records an approximate age instead. Both columns are stored:

- `date_of_birth DATE NULL`
- `age_years SMALLINT NULL`, `age_months SMALLINT NULL`
- `age_recorded_at DATE NOT NULL` — the date the age was observed.

This lets you store "about 34" honestly and derive a usable DOB while still showing "34 yrs (approx)" on screen. `age_recorded_at` converts a recorded age into a birth year estimate when needed for reports.

### 2.4 `workflow_stage` lives on `encounters`, and the queue is derived

There is no separate "queue position" concept to keep in sync. An `encounter` **is** the queue entry. The queue board is a query:

```sql
SELECT * FROM encounters
WHERE status = 'active'
  AND workflow_stage = ANY($1)      -- the stages the caller's role works on
ORDER BY priority DESC, queued_at ASC;
```

**Reason:** a separate queue table would duplicate encounter state and could drift out of sync with it. Deriving the queue from the encounter means there is exactly one row to update when a patient moves forward.

### 2.5 Households are a billing construct, and clinical records never merge

A `household` groups patients for **billing only**. Each patient keeps their own `encounters`, `vitals`, `diagnoses`, `lab_orders`, `prescriptions` and `admissions`, keyed by `patient_id`. The household relationship is used for exactly three things:

1. Resolving who pays for a charge (`charges.household_id`).
2. Displaying outstanding family balance.
3. Letting a clerk register several members in one sitting.

**There is no query in this system that returns one family member's clinical history to another family member.** Be ready to say that in your defence — it is the privacy question an examiner is most likely to ask.

---

## 3. The sync column contract 🔄

**Every syncable table carries exactly these nine columns.** The sync engine relies on their uniformity: it enumerates tables from `sync_table_scopes`, reads the same column names from each, and needs no per-table special-casing.

```sql
id          UUID        PRIMARY KEY,                                  -- UUIDv7, generated CLIENT-side
revision    BIGINT      NOT NULL DEFAULT 1,                           -- client-side edit counter
server_rev  BIGINT      NOT NULL DEFAULT 0,                           -- server-assigned, monotonic per row
sync_state  TEXT        NOT NULL DEFAULT 'synced',                    -- pending | synced | conflict
deleted_at  TIMESTAMPTZ NULL,                                         -- soft delete ONLY
created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
created_by  UUID        NOT NULL REFERENCES users(id),
updated_by  UUID        NOT NULL REFERENCES users(id),

CONSTRAINT chk_<table>_sync_state CHECK (sync_state IN ('pending','synced','conflict'))
```

> **There is no `facility_id`.** This deployment serves **one** health centre — its own code, its own database, its own API. A second facility deploys its own copy. Tenancy is therefore not a dimension of this schema, and no clinical table carries a tenant column. See §3.2 for what that simplifies.

**Rules that are not negotiable:**

1. **IDs are generated on the client** with `uuid.NewV7()` / `uuidv7()` in the browser. The server never generates a primary key for a synced table. This is what makes offline inserts possible at all.
2. **`server_rev` is assigned by the server** on every accepted write, and is authoritative. A client's `revision` is advisory and used for conflict *detection*, never for conflict *resolution*.
3. **Rows are never hard-deleted.** `deleted_at` is set instead, so the deletion can replicate as a normal update. Every read query must filter `deleted_at IS NULL`.
4. **`created_by`/`updated_by` are the acting user's ID**, taken from the device's authenticated session — not from the server.
5. **`created_at`/`updated_at` are set on the device**, because the moment of care is what matters clinically. The server additionally records `device_recorded_at` inside the sync change log so a wildly wrong device clock is detectable.

**Two documented exceptions:**

- **`facilities`** (§5.1) allows NULL `created_by`/`updated_by`, because the single configuration row is inserted by the install migration before any user exists.
- **Server-only tables (🔒)** omit `sync_state` and `server_rev` entirely, and get a DB-level grant that the API's role cannot `UPDATE` or `DELETE` where append-only.

### 3.1 Indexing

With no tenant column, the standard index set for a syncable table is simply:

```sql
CREATE INDEX idx_<table>_server_rev ON <table> (server_rev);        -- the /sync/pull cursor
CREATE INDEX idx_<table>_updated    ON <table> (updated_at);        -- "what changed recently"
CREATE INDEX idx_<table>_patient    ON <table> (patient_id) WHERE deleted_at IS NULL;  -- if it has patient_id
```

`idx_<table>_server_rev` exists specifically to make `GET /sync/pull` fast — it is the cursor column. Every hot query that used to be `WHERE facility_id = $1 AND ...` is now `WHERE ...` and gets its own targeted index (for example `patients (lower(surname), lower(first_name))` for the patient search).

### 3.2 What single-tenancy simplifies

Removing the tenant dimension is not just fewer columns. These things **stop existing**:

| Gone | Why it was there |
|---|---|
| `facility_id` on ~25 tables | Tenant scoping |
| `facility_id` as a leading index column on every index | Tenant-scoped query plans |
| `UNIQUE (facility_id, patient_number)` and ~10 siblings | Per-tenant uniqueness — now simply `UNIQUE (patient_number)` |
| The `facility_mismatch` rejection in `POST /sync/push` | Defending against one PHC writing into another's records |
| Facility filtering in `GET /sync/pull` | One of three pull filters |
| The circular FK workaround between `facilities` and `users` | `users.facility_id → facilities` while `facilities.created_by → users` |
| "Which facility am I?" resolution on every request | Deriving the tenant from the session |

That last one is worth stating plainly in your report: **the tenant column was the source of the only genuinely circular constraint in the schema, and removing it removed the problem rather than working around it.**

---

## 4. Table inventory

| # | Table | Kind | Sprint | Purpose |
|---|---|---|---|---|
| 1 | `facilities` | 🔄 mutable | 0 | The health centre itself |
| 2 | `users` | 🔄 mutable | 0 | Staff accounts |
| 3 | `user_roles` | 🔄 mutable | 1 | Role assignments (a user may hold several) |
| 4 | `devices` | 🔄 mutable | 1 | Registered workstations and their public keys |
| 5 | `refresh_tokens` | 🔒 | 1 | Rotating refresh token families |
| 6 | `households` | 🔄 mutable | 3 | Family billing account |
| 7 | `patients` | 🔄 mutable | 3 | Demographics |
| 8 | `encounters` | 🔄 workflow | 4 | A visit; also the queue entry |
| 9 | `vitals` | 🔄 fact | 4 | Observations |
| 10 | `drug_products` | 🔄 mutable | 6 | Drug catalogue (no stock) |
| 11 | `diagnoses` | 🔄 fact | 5 | Doctor's diagnosis |
| 12 | `lab_orders` | 🔄 workflow | 5 | Lab test request |
| 13 | `lab_results` | 🔄 fact | 5 | Result values |
| 14 | `prescriptions` | 🔄 workflow | 6 | Prescription header |
| 15 | `prescription_items` | 🔄 workflow | 6 | Prescribed drugs |
| 16 | `dispenses` | 🔄 fact | 6 | Dispensing event |
| 17 | `tariffs` | 🔄 mutable | 6 | Price list |
| 18 | `invoices` | 🔄 workflow | 6 | Bill for one or more services |
| 19 | `charges` | 🔄 workflow | 6 | Invoice line items |
| 20 | `payments` | 🔄 fact | 6 | Money received |
| 21 | `payment_allocations` | 🔄 fact | 6 | Which charge a payment settles |
| 22 | `admissions` | 🔄 workflow | 7 | Inpatient episode |
| 23 | `bed_assignments` | 🔄 fact | 7 | Bed occupancy history |
| 24 | `ward_charges` | 🔄 fact | 7 | Daily accrual ledger |
| 25 | `admission_notes` | 🔄 fact | 7 | Ward progress notes |
| 26 | `referrals` | 🔄 workflow | 8 | Referral note + access token |
| 27 | `referral_access_log` | 🔒 | 8 | Every view of a public referral |
| 28 | `audit_logs` | 🔒 | 6 | Append-only audit trail |
| 29 | `sync_table_scopes` | 🔒 config | 2 | Which tables sync, to which roles |
| 29b | `sync_probe` | 🧪 dev | 2 | Development probe table; not clinical, not deployed long-term |
| 30 | `sync_batches` | 🔒 | 2 | Applied push batches (idempotency) |
| 31 | `sync_operations` | 🔒 | 2 | Applied individual operations (idempotency) |
| 32 | `sync_change_log` | 🔒 | 2 | Server change sequence for pull |
| 33 | `sync_conflicts` | 🔒 | 2 | Recorded conflicts for review |
| 34 | `device_sync_state` | 🔒 | 2 | Per-device cursor and diagnostics |

---

## 5. Identity and access

### 5.1 `facilities` 🔄 — 📅 Sprint 0

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | Fixed constant: `018f0000-0000-7000-8000-000000000001` |
| `name` | TEXT NOT NULL | e.g. "Ungwan Rimi Primary Health Centre". Printed on every receipt and referral note |
| `code` | TEXT NOT NULL UNIQUE | **⚠️ IMMUTABLE.** Prefixes every document number, e.g. `URPHC`. Must match `^[A-Z0-9]{2,10}$` |
| `lga`, `state` | TEXT NULL | |
| `address` | TEXT NULL | |
| `phone` | TEXT NULL | |
| `currency_code` | CHAR(3) NOT NULL DEFAULT 'NGN' | ISO 4217 |
| `currency_minor_units` | SMALLINT NOT NULL DEFAULT 2 | 100 kobo = 1 naira |
| `timezone` | TEXT NOT NULL DEFAULT 'Africa/Lagos' | Used for day-boundary calculations |
| `is_active` | BOOLEAN NOT NULL DEFAULT true | |
| `created_by`, `updated_by` | UUID **NULL** | NULL until an admin first edits the settings — the row is created at install, before any user exists |
| sync columns | | minus `facility_id`; nine columns as per §3 |

```sql
-- ★ THE SINGLE-TENANT GUARANTEE
-- A constant expression in a unique index permits at most one row. A second
-- INSERT fails with SQLSTATE 23505. This makes "single tenant" a database fact
-- rather than a convention that a stray seed could quietly break.
CREATE UNIQUE INDEX uq_facilities_single_org ON facilities ((true));

CONSTRAINT chk_facilities_code_charset CHECK (code ~ '^[A-Z0-9]{2,10}$')
```

**Why `code` is immutable.** It appears inside `URPHC/PAT/2026/00123`, `URPHC/RCP/2026/01007` and every other document number. Changing it would not renumber documents already printed on thermal paper, leaving two competing number series and a uniqueness collision on the next sync push. The service layer must refuse to update it; the comment in the migration is the second line of defence.

**Why the actors are nullable here, and nowhere else.** `facilities` is inserted by the install migration, at which point no user exists to name as the actor. A `NOT NULL` actor would make the very first insert impossible. Every other syncable table keeps `NOT NULL`, because by the time those rows are written a real user is always logged in.

`currency_minor_units` is what makes the money model portable — never hard-code `100` in the application. It is also the reason this table is worth keeping even in a single-tenant system: currency, timezone, code and letterhead all come from here rather than from env vars, so an admin can correct them without a redeploy.

### 5.2 `users` 🔄 — 📅 Sprint 0

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `staff_number` | TEXT NOT NULL | **Globally** unique (no facility scoping) |
| `first_name`, `last_name` | TEXT NOT NULL | |
| `middle_name` | TEXT NULL | |
| `email` | CITEXT NULL | Globally unique when present |
| `phone` | TEXT NULL | |
| `password_hash` | TEXT NOT NULL | Argon2id encoded string, PHC format. **Never synced to clients** — see §9.4 |
| `password_changed_at` | TIMESTAMPTZ NULL | Forces re-auth after admin reset |
| `must_change_password` | BOOLEAN NOT NULL DEFAULT true | |
| `is_active` | BOOLEAN NOT NULL DEFAULT true | |
| `last_login_at` | TIMESTAMPTZ NULL | |
| `failed_login_count` | SMALLINT NOT NULL DEFAULT 0 | For lockout |
| `locked_until` | TIMESTAMPTZ NULL | |
| sync columns | | |

```sql
CREATE UNIQUE INDEX uq_users_staff_number ON users (staff_number) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_users_email        ON users (email) WHERE email IS NOT NULL AND deleted_at IS NULL;
```

> `CITEXT` requires `CREATE EXTENSION IF NOT EXISTS citext;` in the Sprint 0 migration. If you would rather avoid the extension, use `TEXT` with `LOWER(email)` in a unique index — either is fine, but decide now and be consistent.
>
> There is **no** `facility_id` column. Users belong to the one facility implicitly, which is what makes the uniqueness constraints above global rather than per-facility — and what removed the circular FK that `facilities.created_by → users` would otherwise have created.

### 5.3 `user_roles` 🔄 — 📅 Sprint 1

Roles live in their own table rather than a `users.role` column, because a real PHC has staff who are both a nurse and a midwife, or a doctor who also administers.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID NOT NULL REFERENCES users(id) | |
| `role` | TEXT NOT NULL | `front_desk` \| `nurse` \| `doctor` \| `lab_scientist` \| `pharmacist` \| `cashier` \| `accountant` \| `admin` |
| `granted_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `granted_by` | UUID NOT NULL REFERENCES users(id) | |
| `revoked_at` | TIMESTAMPTZ NULL | Soft revoke; keeps the audit trail |
| `revoked_by` | UUID NULL REFERENCES users(id) | Required alongside `revoked_at` (CHECK-enforced) |
| `revoked_reason` | TEXT NULL | Required alongside `revoked_at`, min 5 chars. "Why did Dr X lose prescribing rights?" will be asked |
| sync columns | | |

```sql
CONSTRAINT chk_user_roles_role CHECK (role IN
  ('front_desk','nurse','doctor','lab_scientist','pharmacist','cashier','accountant','admin'))
CREATE UNIQUE INDEX uq_user_roles_active ON user_roles (user_id, role) WHERE revoked_at IS NULL;
```

### 5.4 `devices` 🔄 — 📅 Sprint 1

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | Generated by the browser on first run |
| `registered_by` | UUID **NULL** REFERENCES users(id) | Who registered it, for the audit trail. **Nullable** because a PHC workstation is shared — front desk, cashier and triage nurse may all use one machine — so a device does not belong to one user. The session owner is recorded per refresh token |
| `name` | TEXT NOT NULL | Human label: "Reception Desk 1" |
| `public_key` | TEXT NOT NULL | Base64url **SPKI**-encoded ECDSA P-256 public key (WebCrypto exports P-256 as `spki`, not `raw`) |
| `public_key_alg` | TEXT NOT NULL DEFAULT 'ES256' | `ES256` (ECDSA P-256 + SHA-256). Column exists so a future algorithm change needs no migration |
| `fingerprint` | TEXT NOT NULL | SHA-256 of the public key, for display to the admin |
| `platform` | TEXT NULL | From `navigator.userAgentData` where available |
| `app_version` | TEXT NULL | Which frontend build registered it |
| `last_seen_at` | TIMESTAMPTZ NULL | Updated on every sync push |
| `last_sync_cursor` | BIGINT NOT NULL DEFAULT 0 | Server's view of where this device has read to |
| `revoked_at` | TIMESTAMPTZ NULL | Set on loss/theft — **revocation is immediate** |
| `revoked_by` | UUID NULL REFERENCES users(id) | |
| `revoked_reason` | TEXT NULL | |
| sync columns | | |

Unique: `UNIQUE (fingerprint)`.

### 5.5 `refresh_tokens` 🔒 — 📅 Sprint 1

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID NOT NULL REFERENCES users(id) | |
| `device_id` | UUID NOT NULL REFERENCES devices(id) | A token is bound to one device |
| `token_hash` | TEXT NOT NULL | SHA-256 of the token. Never store the token itself |
| `family_id` | UUID NOT NULL | Rotation family — reuse of a rotated token revokes the whole family |
| `issued_at`, `expires_at` | TIMESTAMPTZ NOT NULL | |
| `used_at` | TIMESTAMPTZ NULL | Set on rotation |
| `revoked_at` | TIMESTAMPTZ NULL | |
| `revoked_reason` | TEXT NULL | `rotated` \| `reuse_detected` \| `logout` \| `device_revoked` \| `admin` |
| `ip`, `user_agent` | INET / TEXT NULL | |

Index: `idx_refresh_tokens_family ON refresh_tokens (family_id)`, `idx_refresh_tokens_hash ON refresh_tokens (token_hash)`.

**No sync columns** — this table is server-only. But **the device does cache its own refresh token locally** in IndexedDB so it can refresh when connectivity returns; that cached copy is what enables the 7-day offline grace period.

---

## 6. Patient registry

### 6.1 `households` 🔄 — 📅 Sprint 3

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `family_number` | TEXT NOT NULL | Display number, e.g. `URPHC/FAM/2026/00042`. Unique |
| `family_name` | TEXT NOT NULL | The billing name — "the Adeyemi family" |
| `head_patient_id` | UUID NULL REFERENCES patients(id) | Who is considered the head. Nullable and **circular** — see note |
| `phone` | TEXT NULL | Primary contact for the family |
| `address` | TEXT NULL | Shared family address, prefilled into member registrations |
| `notes` | TEXT NULL | |
| `is_active` | BOOLEAN NOT NULL DEFAULT true | |
| sync columns | | |

**Circular FK note:** `households.head_patient_id → patients.id` while `patients.household_id → households.id`. Both are `NULLABLE`, so insert the household first (with `head_patient_id = NULL`), then the head patient, then `UPDATE households SET head_patient_id = ...`. Alternatively add the FK in a later migration with `ALTER TABLE`. **Do not** make both `NOT NULL` — you will create an insert deadlock with no way out.

**Orphan rule:** if a household has no active patients left, keep the household row (`is_active = false`). Never cascade-delete a family because its last member died or transferred.

### 6.2 `patients` 🔄 — 📅 Sprint 3

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `patient_number` | TEXT NOT NULL | e.g. `URPHC/2026/00123`. Unique |
| `household_id` | UUID NULL REFERENCES households(id) | NULL = independent/single patient |
| `first_name` | TEXT NOT NULL | |
| `middle_name` | TEXT NULL | Optional per the requirements |
| `surname` | TEXT NOT NULL | |
| `date_of_birth` | DATE NULL | May be unknown — see §2.3 |
| `age_years` | SMALLINT NULL | |
| `age_months` | SMALLINT NULL | For infants under 2 |
| `age_recorded_at` | DATE NULL | When the age was observed |
| `gender` | TEXT NOT NULL | `male` \| `female` \| `other` \| `unknown` |
| `marital_status` | TEXT NULL | `single` \| `married` \| `divorced` \| `widowed` \| `separated` \| `unknown` |
| `phone` | TEXT NULL | |
| `alt_phone` | TEXT NULL | Next of kin / family contact |
| `tribe` | TEXT NULL | Kept as free text with a suggestion list — see note |
| `occupation` | TEXT NULL | |
| `address` | TEXT NULL | |
| `address_landmark` | TEXT NULL | Common and genuinely useful in PHC settings |
| `blood_group` | TEXT NULL | `A+`…`O-`, optional |
| `genotype` | TEXT NULL | Optional; clinically relevant in Nigeria |
| `allergies` | TEXT NULL | Free text, surfaced prominently in the consultation view |
| `chronic_conditions` | TEXT NULL | |
| `is_deceased` | BOOLEAN NOT NULL DEFAULT false | |
| `deceased_at` | DATE NULL | |
| `registration_date` | DATE NOT NULL | Captured explicitly — may differ from `created_at` for back-entry |
| `is_active` | BOOLEAN NOT NULL DEFAULT true | |
| `photo_document_id` | UUID NULL | Reserved for a future document store |
| sync columns | | |

```sql
CONSTRAINT chk_patients_gender CHECK (gender IN ('male','female','other','unknown')),
CONSTRAINT chk_patients_marital CHECK (marital_status IS NULL OR marital_status IN
  ('single','married','divorced','widowed','separated','unknown')),
CONSTRAINT chk_patients_age CHECK (age_years IS NULL OR (age_years >= 0 AND age_years <= 130))
```

**Indexes**

```sql
CREATE UNIQUE INDEX uq_patients_number ON patients (patient_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_patients_household  ON patients (household_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_patients_name       ON patients (lower(surname), lower(first_name)) WHERE deleted_at IS NULL;
CREATE INDEX idx_patients_phone      ON patients (phone) WHERE deleted_at IS NULL;
CREATE INDEX idx_patients_dob        ON patients (date_of_birth) WHERE deleted_at IS NULL;
```

The `lower(surname)` functional index is what makes the Ctrl+K patient search fast. Note it cannot use a plain `=` — search uses `LIKE 'ade%'` on the expression, so the search implementation must query `lower(surname) LIKE lower($1) || '%'` to hit the index.

**On `tribe`:** store as `TEXT`, and drive the UI with a **datalist of common values plus free entry**. A `CHECK` constraint here would be a mistake — it would reject legitimate values and force a migration every time a new one appears.

**On age/DOB consistency:** the service layer derives whichever is missing. If only `age_years` is given, derive `date_of_birth = age_recorded_at - age_years years` and mark it approximate. Never let both be `NULL` — the encounter workflow needs an age for dosing.

---

## 7. Clinical workflow

### 7.1 `encounters` 🔄 — 📅 Sprint 4

The central workflow row. **This is also the queue entry** (§2.4).

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `visit_number` | TEXT NOT NULL | e.g. `URPHC/2026/00123/03` (3rd visit). Unique |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `household_id` | UUID NULL REFERENCES households(id) | Denormalised from the patient at creation — the billing party, **snapshotted** so a later change of household does not rewrite history |
| `encounter_type` | TEXT NOT NULL DEFAULT 'opd' | `opd` \| `anc` \| `immunisation` \| `emergency` \| `follow_up` \| `inpatient` |
| `workflow_stage` | TEXT NOT NULL DEFAULT 'registered' | See the state machine below |
| `status` | TEXT NOT NULL DEFAULT 'active' | `active` \| `closed` \| `cancelled` |
| `priority` | SMALLINT NOT NULL DEFAULT 0 | 0 normal, 1 urgent, 2 emergency. Drives queue sort |
| `chief_complaint` | TEXT NULL | Captured at intake |
| `queued_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | Queue ordering |
| `started_at` | TIMESTAMPTZ NULL | When the doctor opened the consultation |
| `closed_at` | TIMESTAMPTZ NULL | |
| `closed_by` | UUID NULL REFERENCES users(id) | |
| `attending_doctor_id` | UUID NULL REFERENCES users(id) | |
| `attending_nurse_id` | UUID NULL REFERENCES users(id) | |
| `referred_from_encounter_id` | UUID NULL REFERENCES encounters(id) | Links a returning referred patient |
| `notes` | TEXT NULL | |
| sync columns | | |

```sql
CONSTRAINT chk_encounters_stage CHECK (workflow_stage IN (
  'registered','vitals_done','awaiting_doctor','in_consultation',
  'awaiting_lab','lab_done','awaiting_lab_payment',
  'awaiting_pharmacy','awaiting_drug_payment','pharmacy_done',
  'admitted','closed')),
CONSTRAINT chk_encounters_status CHECK (status IN ('active','closed','cancelled')),
CONSTRAINT chk_encounters_type CHECK (encounter_type IN
  ('opd','anc','immunisation','emergency','follow_up','inpatient'))
```

**Indexes**

```sql
CREATE UNIQUE INDEX uq_encounters_visit ON encounters (visit_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_encounters_queue  ON encounters (workflow_stage, priority DESC, queued_at)
       WHERE status = 'active' AND deleted_at IS NULL;   -- the queue board query
CREATE INDEX idx_encounters_patient ON encounters (patient_id, queued_at DESC);
CREATE INDEX idx_encounters_open    ON encounters (patient_id) WHERE status = 'active' AND deleted_at IS NULL;
```

That last partial index enforces an important business rule cheaply: **a patient should have at most one active encounter.** Make it a real unique index if you want the database to enforce it:

```sql
CREATE UNIQUE INDEX uq_encounters_one_active ON encounters (patient_id)
  WHERE status = 'active' AND deleted_at IS NULL;
```

Be aware this makes a concurrent double-registration on two offline devices fail on sync. Handle it in the sync engine as a **surfaced conflict** rather than a crash — see `02-SYNC-PROTOCOL.md` §7.3. For a first implementation, prefer the non-unique version plus a service-layer check, and add the unique index in Sprint 9 hardening once you have seen how conflicts behave in practice.

### 7.2 The workflow state machine — 📅 Sprint 4

Implement this as a single exported map in the **service** layer. It is the authority for both the API and the UI.

```
registered ──record vitals──▶ vitals_done ──enqueue──▶ awaiting_doctor
                                                            │
                                          ┌─────────────────┴──────────────────┐
                            doctor opens  │                                    │ doctor orders labs
                                          ▼                                    ▼
                                  in_consultation  ◀──results ready──  awaiting_lab
                                     │       │                              │
              doctor orders labs ────┘       │ doctor prescribes      awaiting_lab_payment
                     │                        │                              │ cashier collects
                     ▼                        ▼                              ▼
              awaiting_lab           awaiting_drug_payment ──cashier──▶ awaiting_pharmacy
                     │                        │                              │
        cashier collects │                    │ pharmacist dispenses        │
                     ▼                        ▼                              ▼
           awaiting_lab_payment       pharmacy_done ──▶ closed        pharmacy_done
                     │
        lab enters results
                     ▼
                 lab_done ──▶ awaiting_doctor
```

```go
// internal/service/workflow.go
var allowedTransitions = map[string][]string{
    "registered":             {"vitals_done"},
    "vitals_done":            {"awaiting_doctor"},
    "awaiting_doctor":        {"in_consultation"},
    "in_consultation":        {"awaiting_lab", "awaiting_drug_payment", "awaiting_pharmacy", "admitted", "closed"},
    "awaiting_lab":           {"awaiting_lab_payment", "lab_done"},
    "awaiting_lab_payment":   {"awaiting_lab"},
    "lab_done":               {"awaiting_doctor"},
    "awaiting_drug_payment":  {"awaiting_pharmacy"},
    "awaiting_pharmacy":      {"pharmacy_done"},
    "pharmacy_done":          {"closed"},
    "admitted":               {"closed"},
}
```

**Every transition is idempotent.** Re-applying the current stage is a `200 OK` no-op, not a `409`. This is what makes queued offline operations safe to retry — and it will happen constantly.

### 7.3 `vitals` 🔄 — 📅 Sprint 4

An immutable fact. A second reading is a **new row**, never an update.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `encounter_id` | UUID NULL REFERENCES encounters(id) | NULL for a standalone/admission reading |
| `admission_id` | UUID NULL REFERENCES admissions(id) | Set for ward observations |
| `recorded_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | **The clinical timestamp.** Not `created_at` |
| `bp_systolic` | SMALLINT NULL | mmHg |
| `bp_diastolic` | SMALLINT NULL | mmHg |
| `temperature_c` | NUMERIC(4,1) NULL | °C, e.g. `36.8` |
| `pulse_rate` | SMALLINT NULL | bpm |
| `respiratory_rate` | SMALLINT NULL | breaths/min |
| `height_cm` | NUMERIC(5,1) NULL | |
| `weight_kg` | NUMERIC(5,2) NULL | |
| `spo2` | SMALLINT NULL | % — cheap to add, clinically valuable |
| `bmi` | NUMERIC(4,1) NULL | Computed on the client and stored, because the formula may change |
| `pain_score` | SMALLINT NULL | 0–10 |
| `notes` | TEXT NULL | |
| `recorded_by` | UUID NOT NULL REFERENCES users(id) | |
| sync columns | | |

`temperature_c` is `NUMERIC(4,1)` not a float — thermometers report one decimal, and floats will render `36.800000000000004` in your UI at least once before you fix it.

```sql
CONSTRAINT chk_vitals_systolic  CHECK (bp_systolic  IS NULL OR bp_systolic  BETWEEN 40 AND 300),
CONSTRAINT chk_vitals_diastolic CHECK (bp_diastolic IS NULL OR bp_diastolic BETWEEN 20 AND 200),
CONSTRAINT chk_vitals_temp      CHECK (temperature_c IS NULL OR temperature_c BETWEEN 25.0 AND 45.0),
CONSTRAINT chk_vitals_pulse     CHECK (pulse_rate IS NULL OR pulse_rate BETWEEN 20 AND 300),
CONSTRAINT chk_vitals_resp      CHECK (respiratory_rate IS NULL OR respiratory_rate BETWEEN 5 AND 80),
CONSTRAINT chk_vitals_height    CHECK (height_cm IS NULL OR height_cm BETWEEN 20 AND 260),
CONSTRAINT chk_vitals_weight    CHECK (weight_kg IS NULL OR weight_kg BETWEEN 0.5 AND 400)
```

Sanity `CHECK`s like these catch data-entry slips at the database, which is the cheapest place to catch them, and they are excellent evidence of defensive design in a viva.

Index: `CREATE INDEX idx_vitals_patient_recorded ON vitals (patient_id, recorded_at DESC) WHERE deleted_at IS NULL;`

### 7.4 `diagnoses` 🔄 — 📅 Sprint 5

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `encounter_id` | UUID NOT NULL REFERENCES encounters(id) | |
| `diagnosis_type` | TEXT NOT NULL DEFAULT 'primary' | `primary` \| `secondary` \| `differential` \| `provisional` \| `final` |
| `icd10_code` | TEXT NULL | Free text with a searchable reference list — **do not** create a FK to an ICD table you have to import |
| `description` | TEXT NOT NULL | The readable diagnosis |
| `clinical_notes` | TEXT NULL | |
| `diagnosed_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `diagnosed_by` | UUID NOT NULL REFERENCES users(id) | |
| sync columns | | |

A patient can have several diagnoses per encounter, so this is one-to-many, not a column on `encounters`. That is what makes "primary plus two secondaries" expressible.

Index: `CREATE INDEX idx_diagnoses_patient ON diagnoses (patient_id, diagnosed_at DESC) WHERE deleted_at IS NULL;`

### 7.5 `lab_orders` 🔄 — 📅 Sprint 5

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `encounter_id` | UUID NOT NULL REFERENCES encounters(id) | |
| `order_number` | TEXT NOT NULL | Unique |
| `test_code` | TEXT NOT NULL | e.g. `MP`, `FBC`, `URIN` |
| `test_name` | TEXT NOT NULL | Human-readable, snapshotted from the tariff at order time |
| `specimen_type` | TEXT NULL | `blood` \| `urine` \| `stool` \| `sputum` \| `swab` \| `other` |
| `clinical_indication` | TEXT NULL | Why the doctor ordered it |
| `priority` | TEXT NOT NULL DEFAULT 'routine' | `routine` \| `urgent` \| `stat` |
| `status` | TEXT NOT NULL DEFAULT 'ordered' | Clinical status only — payment is derived, see the note |
| `ordered_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `ordered_by` | UUID NOT NULL REFERENCES users(id) | |
| `collected_at` | TIMESTAMPTZ NULL | |
| `collected_by` | UUID NULL REFERENCES users(id) | |
| `completed_at` | TIMESTAMPTZ NULL | |
| `invoice_id` | UUID NULL REFERENCES invoices(id) | The bill for this test |
| `charge_id` | UUID NULL REFERENCES charges(id) | The specific line |
| sync columns | | |

```sql
CONSTRAINT chk_lab_orders_status CHECK (status IN
  ('ordered','collected','in_progress','completed','cancelled')),
CONSTRAINT chk_lab_orders_priority CHECK (priority IN ('routine','urgent','stat'))
```

> **Critical design note — payment is NOT a column here.** The lab worklist asks "which orders are paid and ready to collect?" by joining to `charges` and `payment_allocations`. Do **not** add `is_paid BOOLEAN`. Money has exactly one representation in this system (§2.2), and duplicating it here is how you end up with an order that the lab thinks is paid and the cashier thinks is not.
>
> ```sql
> -- the lab worklist query
> SELECT lo.* FROM lab_orders lo
> JOIN charges c            ON c.id  = lo.charge_id
> LEFT JOIN payment_allocations a ON a.charge_id = c.id
>   AND lo.status = 'ordered'
>   AND lo.deleted_at IS NULL
>   AND COALESCE(SUM(a.amount_minor), 0) >= c.amount_minor   -- fully paid
> GROUP BY lo.id;
> ```
> Wrap this as `v_lab_orders_paid` in the Sprint 6 migration.

### 7.6 `lab_results` 🔄 — 📅 Sprint 5

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `lab_order_id` | UUID NOT NULL REFERENCES lab_orders(id) | |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | Denormalised for fast history queries |
| `parameter_name` | TEXT NOT NULL | e.g. `Haemoglobin` |
| `value_text` | TEXT NULL | For qualitative results: "Positive", "Not detected" |
| `value_numeric` | NUMERIC(12,4) NULL | For quantitative results |
| `unit` | TEXT NULL | `g/dL`, `mmol/L`, `×10⁹/L` |
| `reference_range` | TEXT NULL | As printed, e.g. `11.5–16.5` |
| `abnormal_flag` | TEXT NULL | `normal` \| `low` \| `high` \| `critical_low` \| `critical_high` |
| `result_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `result_by` | UUID NOT NULL REFERENCES users(id) | |
| `verified_at` | TIMESTAMPTZ NULL | Second-person verification |
| `verified_by` | UUID NULL REFERENCES users(id) | |
| `remarks` | TEXT NULL | |
| sync columns | | |

**One row per parameter**, not one row per order with a JSON blob. Multi-parameter panels (FBC, EUCr) are the norm, and rows let you chart a single parameter over time — which is exactly what a "continuous management" requirement needs.

```sql
CONSTRAINT chk_lab_results_value CHECK (value_text IS NOT NULL OR value_numeric IS NOT NULL),
CONSTRAINT chk_lab_results_flag  CHECK (abnormal_flag IS NULL OR abnormal_flag IN
  ('normal','low','high','critical_low','critical_high'))
```

### 7.7 `drug_products` 🔄 — 📅 Sprint 6

A catalogue only. **No stock, no batch, no expiry** — explicitly out of scope.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `code` | TEXT NOT NULL | Unique |
| `name` | TEXT NOT NULL | Generic name |
| `brand_name` | TEXT NULL | |
| `form` | TEXT NOT NULL | `tablet` \| `capsule` \| `syrup` \| `suspension` \| `injection` \| `cream` \| `drops` \| `inhaler` \| `suppository` |
| `strength` | TEXT NULL | e.g. `500mg`, `250mg/5ml` |
| `unit_of_issue` | TEXT NOT NULL DEFAULT 'unit' | `tablet` \| `bottle` \| `vial` \| `sachet` \| `tube` |
| `route` | TEXT NULL | `oral` \| `iv` \| `im` \| `topical` \| `rectal` \| `inhaled` |
| `is_controlled` | BOOLEAN NOT NULL DEFAULT false | Flags narcotics — extra UI warning, not a separate workflow |
| `is_active` | BOOLEAN NOT NULL DEFAULT true | |
| sync columns | | |

```sql
CONSTRAINT chk_drug_products_form CHECK (form IN
  ('tablet','capsule','syrup','suspension','injection','cream','drops','inhaler','suppository'))
```

### 7.8 `prescriptions` 🔄 — 📅 Sprint 6

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `prescription_number` | TEXT NOT NULL | Unique |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `encounter_id` | UUID NOT NULL REFERENCES encounters(id) | |
| `prescription_type` | TEXT NOT NULL DEFAULT 'opd' | `opd` \| `inpatient` \| `discharge` \| `refill` |
| `status` | TEXT NOT NULL DEFAULT 'pending_payment' | `pending_payment` \| `paid` \| `partially_dispensed` \| `dispensed` \| `cancelled` |
| `prescribed_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `prescribed_by` | UUID NOT NULL REFERENCES users(id) | |
| `invoice_id` | UUID NULL REFERENCES invoices(id) | |
| `notes` | TEXT NULL | |
| sync columns | | |

`status` here **is** a legitimate stored field, and it is not a duplication of money: it tracks *dispensing* progress, which is clinical. It advances to `paid` when the service layer observes the invoice is fully allocated. Never let the client set it directly from a local guess.

### 7.9 `prescription_items` 🔄 — 📅 Sprint 6

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `prescription_id` | UUID NOT NULL REFERENCES prescriptions(id) | |
| `drug_product_id` | UUID NULL REFERENCES drug_products(id) | NULL if a free-text drug was prescribed |
| `drug_name` | TEXT NOT NULL | Snapshotted, so history stays readable if the catalogue changes |
| `dose` | TEXT NOT NULL | e.g. `1 tablet` |
| `frequency` | TEXT NOT NULL | e.g. `BD` (twice daily) |
| `duration_days` | SMALLINT NULL | |
| `quantity_prescribed` | NUMERIC(10,2) NOT NULL | |
| `route` | TEXT NULL | |
| `instructions` | TEXT NULL | Free text for the patient |
| `charge_id` | UUID NULL REFERENCES charges(id) | Priced line, created when prescribed |
| sync columns | | |

### 7.10 `dispenses` 🔄 — 📅 Sprint 6

An immutable fact — the record of what actually left the pharmacy shelf.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `prescription_id` | UUID NOT NULL REFERENCES prescriptions(id) | |
| `prescription_item_id` | UUID NOT NULL REFERENCES prescription_items(id) | |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `drug_product_id` | UUID NULL REFERENCES drug_products(id) | |
| `drug_name` | TEXT NOT NULL | Snapshotted |
| `quantity_dispensed` | NUMERIC(10,2) NOT NULL | May be less than prescribed — partial dispensing |
| `batch_number` | TEXT NULL | Free text. **Not** inventory — just what the box said |
| `expiry_date` | DATE NULL | As printed on the packet |
| `dispensed_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `dispensed_by` | UUID NOT NULL REFERENCES users(id) | |
| `counselling_notes` | TEXT NULL | What the pharmacist told the patient |
| `substituted_from` | TEXT NULL | If a substitute was given, the original drug name |
| `substitution_reason` | TEXT NULL | |
| sync columns | | |

`quantity_dispensed < quantity_prescribed` is legal and must be recorded honestly rather than forcing the pharmacist to lie. Partial dispensing sets `prescriptions.status = 'partially_dispensed'`.

---

## 8. Billing

### 8.1 Money representation, one more time

- All amounts: `BIGINT` **minor units**. `150000` = ₦1,500.00 with `currency_minor_units = 2`.
- Column naming: `amount_minor`, `unit_price_minor`, `total_minor`.
- **Never** `NUMERIC` for money in this schema, and **never** a float.
- Rounding happens exactly once, when a line total is computed: `total_minor = round(unit_price_minor × quantity)`. Do not round again downstream.

### 8.2 `tariffs` 🔄 — 📅 Sprint 6

The price list, so prices are not hard-coded in the application.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `code` | TEXT NOT NULL | Unique |
| `name` | TEXT NOT NULL | |
| `category` | TEXT NOT NULL | `consultation` \| `lab` \| `drug` \| `procedure` \| `ward` \| `registration` \| `other` |
| `unit_price_minor` | BIGINT NOT NULL | |
| `unit` | TEXT NULL | `per_test`, `per_day`, `per_visit` |
| `is_active` | BOOLEAN NOT NULL DEFAULT true | |
| `effective_from` | DATE NOT NULL DEFAULT CURRENT_DATE | |
| `effective_to` | DATE NULL | Price history without losing old invoices |
| sync columns | | |

```sql
CONSTRAINT chk_tariffs_price CHECK (unit_price_minor >= 0),
CONSTRAINT chk_tariffs_category CHECK (category IN
  ('consultation','lab','drug','procedure','ward','registration','other'))
```

**Never change a price in place.** Close the old row (`effective_to = today`) and insert a new one. Charges snapshot the price they were created with, so historical invoices stay correct either way — but the tariff history makes the audit defensible.

### 8.3 `invoices` 🔄 — 📅 Sprint 6

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `invoice_number` | TEXT NOT NULL | Unique |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | Who received the service |
| `household_id` | UUID NULL REFERENCES households(id) | **Who pays.** Defaults from the patient, may be reassigned to the family |
| `encounter_id` | UUID NULL REFERENCES encounters(id) | |
| `admission_id` | UUID NULL REFERENCES admissions(id) | |
| `invoice_type` | TEXT NOT NULL | `consultation` \| `lab` \| `drug` \| `ward` \| `procedure` \| `mixed` |
| `status` | TEXT NOT NULL DEFAULT 'unpaid' | `unpaid` \| `partially_paid` \| `paid` \| `void` |
| `total_minor` | BIGINT NOT NULL DEFAULT 0 | Denormalised from charges **within the same transaction**, for fast list rendering |
| `issued_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `issued_by` | UUID NOT NULL REFERENCES users(id) | |
| `voided_at` | TIMESTAMPTZ NULL | |
| `voided_by` | UUID NULL REFERENCES users(id) | |
| `void_reason` | TEXT NULL | Required when voiding |
| `notes` | TEXT NULL | |
| sync columns | | |

**`total_minor` is the one permitted denormalisation**, because it is written in the same database transaction as its charge lines and is a *sum of immutable rows*, not a balance. It is not affected by whether payments have arrived. Do not extend this exception to a `balance_minor` column — that one genuinely changes when a payment lands on another device, and that is precisely the case the derived-balance rule protects you from.

Indexes:

```sql
CREATE UNIQUE INDEX uq_invoices_number ON invoices (invoice_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_invoices_patient  ON invoices (patient_id, issued_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_invoices_household ON invoices (household_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_invoices_unpaid   ON invoices (issued_at) WHERE status IN ('unpaid','partially_paid');
```

### 8.4 `charges` 🔄 — 📅 Sprint 6

A charge is what the facility remembers as owed. Once a charge is paid, it is frozen — further changes require a `payment_reversals` row (see `payment_reversals` in §8.7) and a new charge.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `invoice_id` | UUID NOT NULL REFERENCES invoices(id) | |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | Denormalised for the balance query |
| `household_id` | UUID NULL REFERENCES households(id) | **The party actually billed** |
| `charge_type` | TEXT NOT NULL | `consultation` \| `lab` \| `drug` \| `ward` \| `procedure` \| `registration` \| `other` |
| `tariff_id` | UUID NULL REFERENCES tariffs(id) | Which price list entry it came from |
| `description` | TEXT NOT NULL | Human-readable line, e.g. "Malaria parasite test" |
| `reference_id` | UUID NULL | The source row: a `lab_orders.id`, `prescription_items.id`, `admission_notes.id`, `ward_charges.id`… |
| `reference_type` | TEXT NULL | `lab_order` \| `prescription_item` \| `ward_day` \| `consultation` \| `procedure` |
| `quantity` | NUMERIC(10,2) NOT NULL DEFAULT 1 | |
| `unit_price_minor` | BIGINT NOT NULL | **Snapshotted** at creation |
| `amount_minor` | BIGINT NOT NULL | `= round(unit_price_minor × quantity)` |
| `status` | TEXT NOT NULL DEFAULT 'active' | `active` \| `void` |
| `posted_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `voided_at`, `voided_by`, `void_reason` | | |
| sync columns | | |

```sql
CONSTRAINT chk_charges_amount CHECK (amount_minor >= 0),
CONSTRAINT chk_charges_qty    CHECK (quantity > 0),
CONSTRAINT chk_charges_status CHECK (status IN ('active','void'))
```

Index: `CREATE INDEX idx_charges_balance ON charges (patient_id, status) WHERE deleted_at IS NULL;`

### 8.5 `payments` 🔄 — 📅 Sprint 6

An immutable fact. **The most safety-critical table in the system.**

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `operation_id` | UUID NOT NULL | Client-generated idempotency key |
| `receipt_number` | TEXT NOT NULL | Unique, e.g. `URPHC/RCP/2026/01007` |
| `payer_patient_id` | UUID NULL REFERENCES patients(id) | The person who handed over money |
| `household_id` | UUID NULL REFERENCES households(id) | Set when paying against the family account |
| `amount_minor` | BIGINT NOT NULL | Must be `> 0` |
| `currency_code` | CHAR(3) NOT NULL DEFAULT 'NGN' | Snapshotted from the facility |
| `payment_method` | TEXT NOT NULL DEFAULT 'cash' | `cash` \| `transfer` \| `pos_card` \| `waiver` |
| `payment_reference` | TEXT NULL | Bank/teller reference for non-cash |
| `status` | TEXT NOT NULL DEFAULT 'posted' | `posted` \| `reversed` |
| `paid_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `received_by` | UUID NOT NULL REFERENCES users(id) | |
| `reversal_of_payment_id` | UUID NULL REFERENCES payments(id) | A reversal is itself a payment row |
| `reversed_by_payment_id` | UUID NULL REFERENCES payments(id) | |
| `reversal_reason` | TEXT NULL | |
| `notes` | TEXT NULL | |
| sync columns | | |

```sql
CONSTRAINT uq_payments_operation UNIQUE (operation_id),   -- idempotency
CONSTRAINT chk_payments_amount    CHECK (amount_minor > 0),
CONSTRAINT chk_payments_method    CHECK (payment_method IN ('cash','transfer','pos_card','waiver')),
CONSTRAINT chk_payments_status    CHECK (status IN ('posted','reversed'))
```

**`UNIQUE (operation_id)` is the single most important constraint in this schema.** It is what makes a payment safe to retry. A cashier collects ₦2,000 offline; the sync push times out after the server committed it; the client retries; without this constraint the patient is charged twice and the day's reconciliation is wrong. With it, the retry is a no-op and the client is told "already applied".

`amount_minor > 0` matters too: a negative payment is a refund and must be modelled as a reversal row, so it carries a reason and a reviewer.

`receipt_number` may be assigned by the client offline, which means two offline devices could pick the same number. Two options:

1. **Recommended:** generate it client-side from `facility_code + device_short_id + date + device-local sequence` so collisions are impossible by construction.
2. Or leave it server-assigned, and print "pending" on the offline receipt — but a receipt without a number is not much of a receipt.

### 8.6 `payment_allocations` 🔄 — 📅 Sprint 6

A payment settles one or more charges; a charge may be settled by several payments. This is the join that makes partial payments expressible.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `payment_id` | UUID NOT NULL REFERENCES payments(id) | |
| `charge_id` | UUID NOT NULL REFERENCES charges(id) | |
| `amount_minor` | BIGINT NOT NULL | Portion of the payment applied to this charge |
| `allocated_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `allocated_by` | UUID NOT NULL REFERENCES users(id) | |
| sync columns | | |

```sql
CONSTRAINT uq_allocations_payment_charge UNIQUE (payment_id, charge_id),
CONSTRAINT chk_allocations_amount CHECK (amount_minor > 0)
```

**Invariants the service layer must enforce, inside one transaction:**

- `SUM(allocations.amount_minor WHERE payment_id = p) <= p.amount_minor` — you cannot allocate more than was received.
- `SUM(allocations.amount_minor WHERE charge_id = c AND payment posted) <= c.amount_minor` — a charge cannot be over-settled.

The API provides a "pay the oldest unpaid charges first" helper so the cashier rarely has to think about allocation at all; manual allocation is the exception.

### 8.7 Payment reversals

Modelled as rows in `payments`, not a separate table: `reversal_of_payment_id` points at the original, `amount_minor` equals the original, and the original's `status` becomes `reversed`. The original's allocations are deleted (or marked voided) so the balance recomputes correctly.

**Why a row and not a boolean:** a voided payment must appear in the day's report, in the audit log, and in the cashier's reconciliation — all with a reason. A boolean loses all of that.

---

## 9. Sync infrastructure (server-only) 🔒

These tables are the machinery behind `POST /sync/push` and `GET /sync/pull`. Full protocol in `02-SYNC-PROTOCOL.md`; this section is just the schema.

### 9.1 `sync_table_scopes` — 📅 Sprint 2

The single source of truth for *which* tables sync, to *whom*. Adding a new syncable table means inserting a row here — and the sync engine needs no code change.

| Column | Type | Notes |
|---|---|---|
| `table_name` | TEXT PK | |
| `sync_enabled` | BOOLEAN NOT NULL DEFAULT true | |
| `pull_roles` | TEXT[] NOT NULL | Roles allowed to pull it. `{admin}` = admin only; `{*}` = everyone |
| `push_roles` | TEXT[] NOT NULL | Roles allowed to push inserts/updates |
| `conflict_policy` | TEXT NOT NULL | `append_only` \| `lww` |
| `retention_days` | INTEGER NULL | NULL = keep forever |
| `notes` | TEXT NULL | |

Seed rows (Sprint 2 migration):

```sql
INSERT INTO sync_table_scopes
  (table_name, pull_roles, push_roles, conflict_policy) VALUES
  ('facilities',        '{*}',                              '{admin}',  'lww'),
  ('users',             '{*}',                              '{admin}',  'lww'),
  ('user_roles',        '{*}',                              '{admin}',  'lww'),
  ('devices',           '{*}',                              '{admin}',  'lww'),
  ('households',        '{front_desk,nurse,doctor,cashier,accountant,admin}', '{front_desk,admin}', 'lww'),
  ('patients',          '{*}',                              '{front_desk,nurse,doctor,admin}', 'lww'),
  ('encounters',        '{*}',                              '{front_desk,nurse,doctor}', 'lww'),
  ('vitals',            '{nurse,doctor,admin}',             '{nurse,doctor}', 'append_only'),
  ('diagnoses',         '{doctor,nurse,admin}',             '{doctor}', 'append_only'),
  ('lab_orders',        '{doctor,lab_scientist,nurse,admin}','{doctor}',  'lww'),
  ('lab_results',       '{doctor,lab_scientist,nurse,admin}','{lab_scientist}', 'append_only'),
  ('drug_products',     '{doctor,pharmacist,admin}',        '{admin,pharmacist}', 'lww'),
  ('prescriptions',     '{doctor,pharmacist,cashier,admin}','{doctor}',  'lww'),
  ('prescription_items','{doctor,pharmacist,cashier,admin}','{doctor}',  'lww'),
  ('dispenses',         '{doctor,pharmacist,admin}',        '{pharmacist}', 'append_only'),
  ('tariffs',           '{*}',                              '{admin,accountant}', 'lww'),
  ('invoices',          '{cashier,accountant,admin,doctor}','{cashier,doctor}', 'lww'),
  ('charges',           '{cashier,accountant,admin,doctor}','{cashier,doctor,nurse}', 'append_only'),
  ('payments',          '{cashier,accountant,admin}',       '{cashier}', 'append_only'),
  ('payment_allocations','{cashier,accountant,admin}',      '{cashier}', 'append_only'),
  ('admissions',        '{doctor,nurse,cashier,admin}',     '{doctor}',  'lww'),
  ('bed_assignments',   '{doctor,nurse,admin}',             '{nurse}',   'append_only'),
  ('ward_charges',      '{cashier,accountant,admin,nurse}', '{nurse,cashier}', 'append_only'),
  ('admission_notes',   '{doctor,nurse,admin}',             '{nurse,doctor}', 'append_only'),
  ('referrals',         '{doctor,nurse,admin}',             '{doctor}',  'lww');
-- Deliberately absent: audit_logs, referral_access_log, refresh_tokens,
-- sync_*, user password hashes. Server-only by omission.
```

> **Note on `users`:** the `users` row syncs (the UI needs names to display "prescribed by Dr X"), but **`password_hash` must be stripped by the sync layer**. See §9.4.

### 9.2 `sync_batches` — 📅 Sprint 2

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | = the client's `batch_id` |
| `device_id` | UUID NOT NULL REFERENCES devices(id) | |
| `user_id` | UUID NOT NULL REFERENCES users(id) | |
| `operation_count` | INTEGER NOT NULL | |
| `accepted_count` | INTEGER NOT NULL | |
| `rejected_count` | INTEGER NOT NULL | |
| `received_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `client_sent_at` | TIMESTAMPTZ NULL | For clock-skew detection |
| `highest_server_rev` | BIGINT NOT NULL | The cursor the client should pull from next |

`id` is the primary key, so a retried batch hits a PK conflict and is answered from the stored result rather than reprocessed.

### 9.3 `sync_operations` — 📅 Sprint 2

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | = the client's `operation_id` |
| `batch_id` | UUID NOT NULL REFERENCES sync_batches(id) | |
| `table_name` | TEXT NOT NULL | |
| `row_id` | UUID NOT NULL | |
| `operation` | TEXT NOT NULL | `insert` \| `update` \| `delete` |
| `result` | TEXT NOT NULL | `applied` \| `duplicate` \| `conflict_lww` \| `rejected` |
| `reason` | TEXT NULL | |
| `applied_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `server_rev` | BIGINT NULL | Rev assigned to the row |

Purpose: **per-operation idempotency**. A batch can be partially retried, and a single operation replayed inside a new batch is still recognised.

Index: `CREATE INDEX idx_sync_operations_row ON sync_operations (table_name, row_id);`

### 9.4 `sync_change_log` — 📅 Sprint 2

The pull cursor's backing store. One row per affected record per change.

| Column | Type | Notes |
|---|---|---|
| `seq` | BIGSERIAL PK | **The cursor.** Monotonic, gap-tolerant |
| `table_name` | TEXT NOT NULL | |
| `row_id` | UUID NOT NULL | |
| `operation` | TEXT NOT NULL | `upsert` \| `delete` |
| `server_rev` | BIGINT NOT NULL | The row's rev after this change |
| `changed_by` | UUID NOT NULL REFERENCES users(id) | |
| `device_id` | UUID NULL REFERENCES devices(id) | |
| `changed_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `device_recorded_at` | TIMESTAMPTZ NULL | The client's claimed `updated_at` — clock-skew evidence |

Index: `CREATE INDEX idx_change_log_pull ON sync_change_log (seq);`

**Retention:** this table grows fastest. A nightly job deletes rows older than 30 days *for rows whose current `server_rev` is still visible* — but a device offline longer than the retention window must then be forced into a **full resync** rather than an incremental pull. Implement `GET /sync/pull` to return a `full_resync_required: true` flag when the requested cursor is older than the oldest retained row. Skipping this is the bug that silently corrupts a device that was offline for two months.

**Exclusions:** `sync_change_log` is written for synced tables only. `audit_logs`, `referral_access_log` and `refresh_tokens` never appear here.

**Column-level redaction:** the change log stores only `(table, row_id)`, never the payload. The pull endpoint re-reads the live row, so redaction happens in one place — for `users`, the response builder omits `password_hash`, `failed_login_count` and `locked_until`.

### 9.5 `sync_conflicts` — 📅 Sprint 2

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `table_name` | TEXT NOT NULL | |
| `row_id` | UUID NOT NULL | |
| `device_id` | UUID NOT NULL REFERENCES devices(id) | |
| `user_id` | UUID NOT NULL REFERENCES users(id) | |
| `conflict_type` | TEXT NOT NULL | `lww_overwrite` \| `unique_violation` \| `stale_reference` \| `constraint_violation` |
| `client_revision` | BIGINT NULL | |
| `server_revision` | BIGINT NULL | |
| `winning_payload` | JSONB NULL | What the server kept |
| `losing_payload` | JSONB NULL | What was discarded — never lost, always recorded |
| `detected_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `reviewed_at` | TIMESTAMPTZ NULL | |
| `reviewed_by` | UUID NULL REFERENCES users(id) | |
| `resolution` | TEXT NULL | |

Even though the agreed policy is **automatic** (LWW / append-only), every overwrite is *recorded*. This costs almost nothing and gives you two things: a defensible audit answer, and a genuine debugging tool when two devices behave oddly. Push `sync_conflicts` to the client for admins so a manager can see "3 records were overwritten today".

### 9.6 `device_sync_state` — 📅 Sprint 2

| Column | Type | Notes |
|---|---|---|
| `device_id` | UUID PK REFERENCES devices(id) | |
| `last_pull_cursor` | BIGINT NOT NULL DEFAULT 0 | |
| `last_push_at` | TIMESTAMPTZ NULL | |
| `last_pull_at` | TIMESTAMPTZ NULL | |
| `pending_operation_count` | INTEGER NOT NULL DEFAULT 0 | Last value the device reported |
| `last_error` | TEXT NULL | |
| `last_error_at` | TIMESTAMPTZ NULL | |
| `updated_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |

This is what makes the admin's "Device health" screen possible, and it is how you diagnose a workstation that has stopped syncing.

### 9.7 `sync_probe` — 🧪 development only

Not clinical data. Three columns (`id`, `title`, `body`) plus the nine sync columns, and a
`sync_table_scopes` row granting every role read and write.

**Why it exists.** Sprint 2 must prove the engine before any clinical table exists — that ordering
is the whole point of the sprint. The probe is the smallest thing two browsers can both write,
edit and delete while the demo shows convergence, LWW and replay. It is what `/dev/sync` writes to.

It is in the schema, rather than conjured in a developer's head, so the demo is reproducible.
Nothing references it, and dropping it later is a one-migration change.

---

## 10. Admission and inpatient

### 10.1 `admissions` 🔄 — 📅 Sprint 7

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `admission_number` | TEXT NOT NULL | Unique |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `household_id` | UUID NULL REFERENCES households(id) | Billing party |
| `encounter_id` | UUID NOT NULL REFERENCES encounters(id) | |
| `admitting_diagnosis_id` | UUID NULL REFERENCES diagnoses(id) | "Admission based on diagnosis" per the requirements |
| `admission_reason` | TEXT NULL | |
| `ward` | TEXT NOT NULL | Ward name |
| `bed_number` | TEXT NULL | Current bed — denormalised for fast display |
| `current_bed_assignment_id` | UUID NULL REFERENCES bed_assignments(id) | |
| `admitted_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `admitted_by` | UUID NOT NULL REFERENCES users(id) | |
| `expected_discharge_date` | DATE NULL | |
| `discharged_at` | TIMESTAMPTZ NULL | |
| `discharged_by` | UUID NULL REFERENCES users(id) | |
| `discharge_type` | TEXT NULL | `recovered` \| `improved` \| `against_advice` \| `transferred` \| `referred` \| `died` \| `absconded` |
| `discharge_summary` | TEXT NULL | |
| `status` | TEXT NOT NULL DEFAULT 'admitted' | `admitted` \| `discharged` \| `transferred` |
| `daily_ward_rate_minor` | BIGINT NULL | Snapshotted from the tariff at admission |
| `sync columns` | | |

```sql
CONSTRAINT chk_admissions_status CHECK (status IN ('admitted','discharged','transferred')),
CONSTRAINT chk_admissions_discharge_type CHECK (discharge_type IS NULL OR discharge_type IN
  ('recovered','improved','against_advice','transferred','referred','died','absconded'))
```

**Circular FK again:** `current_bed_assignment_id → bed_assignments.id` while `bed_assignments.admission_id → admissions.id`. Same resolution as households: insert the admission with `NULL`, insert the assignment, then update. Or simply **drop `current_bed_assignment_id`** and derive the current bed from the latest open `bed_assignments` row — it is one small query, removes the cycle entirely, and there is no denormalised value to drift. **Recommended: drop it and derive.**

Also index: `CREATE UNIQUE INDEX uq_admissions_one_open ON admissions (patient_id) WHERE status = 'admitted' AND deleted_at IS NULL;` — a patient cannot be admitted twice at once.

**Daily charging:** the `ward_charges` ledger (below) accrues one row per billable day. A nightly server job creates rows for open admissions; the same job is safe to re-run because of the unique constraint on `(admission_id, charge_date)`.

### 10.2 `bed_assignments` 🔄 — 📅 Sprint 7

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `admission_id` | UUID NOT NULL REFERENCES admissions(id) | |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `ward` | TEXT NOT NULL | |
| `bed_number` | TEXT NOT NULL | |
| `assigned_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `assigned_by` | UUID NOT NULL REFERENCES users(id) | |
| `vacated_at` | TIMESTAMPTZ NULL | NULL = currently occupied |
| `vacated_by` | UUID NULL REFERENCES users(id) | |
| `transfer_reason` | TEXT NULL | |
| sync columns | | |

```sql
CREATE UNIQUE INDEX uq_bed_assignments_occupied ON bed_assignments (ward, bed_number)
  WHERE vacated_at IS NULL AND deleted_at IS NULL;
```

That partial unique index is the whole bed-occupancy system: **a bed cannot be double-booked**, enforced by the database, and it works for transfers (vacate then assign) without any extra logic. Note that two offline devices trying to assign the same bed will collide at sync time — handle it as a `unique_violation` conflict and surface it to the nurse.

### 10.3 `ward_charges` 🔄 — 📅 Sprint 7

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `admission_id` | UUID NOT NULL REFERENCES admissions(id) | |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `household_id` | UUID NULL REFERENCES households(id) | |
| `charge_date` | DATE NOT NULL | The day being billed |
| `ward` | TEXT NOT NULL | |
| `amount_minor` | BIGINT NOT NULL | Snapshotted daily rate |
| `charge_id` | UUID NULL REFERENCES charges(id) | The billing line it produced |
| `is_waived` | BOOLEAN NOT NULL DEFAULT false | Charity/waiver cases are real |
| `waived_by` | UUID NULL REFERENCES users(id) | |
| `waiver_reason` | TEXT NULL | |
| `posted_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| sync columns | | |

```sql
CONSTRAINT uq_ward_charges_day UNIQUE (admission_id, charge_date),   -- idempotent daily accrual
CONSTRAINT chk_ward_charges_amount CHECK (amount_minor >= 0)
```

**`UNIQUE (admission_id, charge_date)` is what makes the daily job safe.** Run it twice, or run it on two devices, and you still get exactly one charge per bed-day. This is the same idempotency principle as payments, applied to time.

### 10.4 `admission_notes` 🔄 — 📅 Sprint 7

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `admission_id` | UUID NOT NULL REFERENCES admissions(id) | |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `note_type` | TEXT NOT NULL | `progress` \| `nursing` \| `procedure` \| `observation` \| `review` |
| `note_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `content` | TEXT NOT NULL | |
| `vitals_id` | UUID NULL REFERENCES vitals(id) | Link to concurrent observations |
| `authored_by` | UUID NOT NULL REFERENCES users(id) | |
| sync columns | | |

Append-only. Notes are **never edited**; a correction is a new note. This is standard clinical practice and it eliminates a whole class of sync conflict.

**Discharge gate:** the service layer must refuse `discharge_type != 'against_advice'` when the derived household balance is positive, unless the discharging user is an `admin` and supplies an override reason (recorded in `discharge_summary` and the audit log). `against_advice` and `died` are permitted with an outstanding balance, because reality demands it.

---

## 11. Referrals

### 11.1 `referrals` 🔄 — 📅 Sprint 8

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `referral_number` | TEXT NOT NULL | Unique |
| `patient_id` | UUID NOT NULL REFERENCES patients(id) | |
| `encounter_id` | UUID NULL REFERENCES encounters(id) | |
| `admission_id` | UUID NULL REFERENCES admissions(id) | |
| `direction` | TEXT NOT NULL DEFAULT 'outgoing' | `outgoing` \| `incoming` |
| `referral_type` | TEXT NOT NULL DEFAULT 'clinical' | `clinical` \| `diagnostic` \| `specialist` \| `emergency` \| `transfer` |
| `priority` | TEXT NOT NULL DEFAULT 'routine' | `routine` \| `urgent` \| `emergency` |
| `to_facility_name` | TEXT NOT NULL | Free text — the receiving hospital may not be a system user |
| `to_facility_address` | TEXT NULL | |
| `to_facility_phone` | TEXT NULL | |
| `to_department` | TEXT NULL | |
| `to_clinician_name` | TEXT NULL | |
| `from_facility_id` | UUID NOT NULL REFERENCES facilities(id) | This deployment's own facility row. Kept explicit so a referral note always records **which organisation issued it** — the receiving hospital needs that on the letterhead, and it survives a future where a district runs several installations |
| `referring_clinician_id` | UUID NOT NULL REFERENCES users(id) | |
| `reason_for_referral` | TEXT NOT NULL | |
| `clinical_summary` | TEXT NULL | |
| `working_diagnosis` | TEXT NULL | Free text snapshot, not a FK |
| `investigations_done` | TEXT NULL | |
| `treatment_given` | TEXT NULL | |
| `vitals_summary` | JSONB NULL | Snapshot of the vitals at referral time |
| `allergies` | TEXT NULL | Duplicated deliberately — a receiving clinician must not miss it |
| `current_medications` | TEXT NULL | |
| `additional_notes` | TEXT NULL | |
| `status` | TEXT NOT NULL DEFAULT 'draft' | `draft` \| `issued` \| `viewed` \| `acknowledged` \| `completed` \| `cancelled` \| `expired` |
| `issued_at` | TIMESTAMPTZ NULL | |
| `access_token_hash` | TEXT NOT NULL | **SHA-256 of the URL token. The token itself is never stored** |
| `access_token_prefix` | TEXT NOT NULL | First 8 chars, for admin identification only |
| `pin_hash` | TEXT NULL | Argon2id — a 6-digit PIN is low entropy, so a slow hash is mandatory |
| `pin_required` | BOOLEAN NOT NULL DEFAULT true | |
| `pin_attempt_count` | SMALLINT NOT NULL DEFAULT 0 | |
| `pin_locked_until` | TIMESTAMPTZ NULL | Lockout after 5 failures |
| `expires_at` | TIMESTAMPTZ NOT NULL | `issued_at + REFERRAL_TTL_HOURS` (48h) |
| `first_viewed_at` | TIMESTAMPTZ NULL | |
| `view_count` | INTEGER NOT NULL DEFAULT 0 | Denormalised counter for the doctor's list view |
| `revoked_at` | TIMESTAMPTZ NULL | |
| `revoked_by` | UUID NULL REFERENCES users(id) | |
| `revoke_reason` | TEXT NULL | |
| `printed_at` | TIMESTAMPTZ NULL | When the slip was printed |
| `print_count` | SMALLINT NOT NULL DEFAULT 0 | |
| sync columns | | |

```sql
CONSTRAINT chk_referrals_status CHECK (status IN
  ('draft','issued','viewed','acknowledged','completed','cancelled','expired')),
CONSTRAINT chk_referrals_priority CHECK (priority IN ('routine','urgent','emergency')),
CONSTRAINT chk_referrals_type CHECK (referral_type IN
  ('clinical','diagnostic','specialist','emergency','transfer')),
CONSTRAINT chk_referrals_expiry CHECK (expires_at > issued_at)
```

Indexes:

```sql
CREATE UNIQUE INDEX uq_referrals_token ON referrals (access_token_hash);      -- the public lookup
CREATE UNIQUE INDEX uq_referrals_number ON referrals (referral_number) WHERE deleted_at IS NULL;
CREATE INDEX idx_referrals_patient ON referrals (patient_id, issued_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_referrals_active  ON referrals (expires_at) WHERE status IN ('issued','viewed');
```

**Security notes to state in your defence:**

- The token is a **256-bit random value from `crypto.getRandomValues`**, base64url-encoded (43 chars). It is displayed **once**, at creation, and never again — the doctor can reprint the slip, which regenerates nothing because the *token is embedded in the printed QR*, not re-derived.
  - This creates a real design question: if the doctor reprints, how do they get the same QR? Answer: **the token must be recoverable to reprint the slip**, but the server only stores its hash. The resolution is to store the token **encrypted** (AES-GCM with a server-side key from Secret Manager) in a `access_token_encrypted BYTEA` column in addition to the hash, so reprinting works and a database dump alone does not expose live links. If you would rather not build envelope encryption in Sprint 8, the acceptable simplification is: **generate the token once, store it encrypted, and log every decryption in the audit log.** Note this honestly as a trade-off.
- **PIN verification is rate-limited and locked after 5 attempts** per referral. Without this, a 6-digit PIN is 10⁶ guesses — trivial to brute-force.
- **Expiry is enforced in the SQL `WHERE` clause**, not merely in the UI, so an expired referral returns 404 regardless of what the client sends.
- The **public payload is a deliberate subset**: patient name, age, gender, the clinical summary, vitals snapshot, allergies, and the referring facility. **No address, no phone, no unrelated history, no financial data.** Whitelist the fields explicitly — do not serialise the row.

### 11.2 `referral_access_log` 🔒 — 📅 Sprint 8

Server-only, append-only, never synced.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `referral_id` | UUID NOT NULL REFERENCES referrals(id) | |
| `accessed_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `access_type` | TEXT NOT NULL | `view` \| `pin_success` \| `pin_failure` \| `pin_locked` \| `expired_attempt` \| `revoked_attempt` \| `print` |
| `ip_address` | INET NULL | |
| `user_agent` | TEXT NULL | |
| `referer` | TEXT NULL | |
| `country_code` | TEXT NULL | From Cloudflare's `CF-IPCountry` header |
| `token_prefix` | TEXT NULL | Never the full token |
| `note` | TEXT NULL | |

Index: `CREATE INDEX idx_referral_access_referral ON referral_access_log (referral_id, accessed_at DESC);`

---

## 12. Audit

### 12.1 `audit_logs` 🔒 — 📅 Sprint 6

Append-only. **Never synced to clients** — see `00-ARCHITECTURE.md` §5.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `user_id` | UUID NULL REFERENCES users(id) | NULL for unauthenticated actions |
| `device_id` | UUID NULL REFERENCES devices(id) | |
| `action` | TEXT NOT NULL | `create` \| `update` \| `delete` \| `login` \| `logout` \| `login_failed` \| `view` \| `print` \| `sync_push` \| `sync_pull` \| `payment_reversed` \| `referral_revoked` \| `device_revoked` \| `discharge_override` |
| `entity_table` | TEXT NULL | |
| `entity_id` | UUID NULL | |
| `patient_id` | UUID NULL | Denormalised so a patient's full access history is one query |
| `summary` | TEXT NOT NULL | Human-readable, e.g. "Recorded vitals for John Doe" |
| `changes` | JSONB NULL | `{ "before": {...}, "after": {...} }` for updates |
| `ip_address` | INET NULL | |
| `user_agent` | TEXT NULL | |
| `request_id` | TEXT NULL | Ties an audit row to the log line that produced it |
| `occurred_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |

Indexes:

```sql
CREATE INDEX idx_audit_entity    ON audit_logs (entity_table, entity_id, occurred_at DESC);
CREATE INDEX idx_audit_patient   ON audit_logs (patient_id, occurred_at DESC);
CREATE INDEX idx_audit_user      ON audit_logs (user_id, occurred_at DESC);
CREATE INDEX idx_audit_action    ON audit_logs (action, occurred_at DESC);
```

**Immutability at the database level**, in the migration that creates the table:

```sql
-- The API's application role may INSERT and SELECT, but never modify or erase.
REVOKE UPDATE, DELETE ON audit_logs FROM phc_app;
GRANT  INSERT, SELECT ON audit_logs TO   phc_app;
```

This is the detail that turns "we log everything" into a claim you can defend. A log the application can rewrite is not an audit trail.

**What gets logged:** every create/update/delete on clinical and financial tables; every authentication event including failures; every payment reversal; every referral issue, view and revocation; every device registration and revocation; every admin action; every discharge with an outstanding balance.

**What must never be logged:** passwords, tokens, PINs, full `password_hash` values, or complete patient records in the `changes` blob — log the changed *fields*, and redact sensitive ones to `"[redacted]"`.

**Retention:** keep for the statutory period (assume 7 years for clinical records; confirm locally). Partition by year once volume justifies it.

---

## 13. Ordered migration plan

Every file below is a goose migration. Numbers are sequential and **never reused or reordered** once applied anywhere.

| # | File | Tables | Sprint |
|---|---|---|---|
| 00001 | `00001_create_facilities.sql` | `facilities` — the single configuration row, `uq_facilities_single_org` | 0 ✅ |
| 00002 | `00002_create_users.sql` | `users` (with `citext` extension) | 0 ✅ |
| 00003 | `00003_seed_default_facility.sql` | seed | 0 ✅ |
| 00004 | `00004_seed_admin_user.sql` | seed — bootstrap admin, real Argon2id hash | 0 ✅ |
| 00005 | `00005_create_user_roles.sql` | `user_roles` | 1 ✅ |
| 00006 | `00006_create_devices.sql` | `devices` | 1 ✅ |
| 00007 | `00007_create_refresh_tokens.sql` | `refresh_tokens` (server-only) | 1 ✅ |
| 00008 | `00008_seed_admin_role.sql` | seed — grants the bootstrap admin its role | 1 ✅ |
| 00008 | `00008_create_sync_infrastructure.sql` | all `sync_*` tables + scope seeds | 2 |
| 00009 | `00009_create_households.sql` | `households` | 3 |
| 00010 | `00010_create_patients.sql` | `patients` + indexes | 3 |
| 00011 | `00011_create_encounters.sql` | `encounters` | 4 |
| 00012 | `00012_create_vitals.sql` | `vitals` | 4 |
| 00013 | `00013_create_diagnoses.sql` | `diagnoses` | 5 |
| 00014 | `00014_create_lab_orders.sql` | `lab_orders` | 5 |
| 00015 | `00015_create_lab_results.sql` | `lab_results` | 5 |
| 00016 | `00016_create_tariffs.sql` | `tariffs` + seed price list | 6 |
| 00017 | `00017_create_drug_products.sql` | `drug_products` + seed catalogue | 6 |
| 00018 | `00018_create_invoices_charges.sql` | `invoices`, `charges` | 6 |
| 00019 | `00019_create_payments.sql` | `payments`, `payment_allocations` | 6 |
| 00020 | `00020_create_prescriptions.sql` | `prescriptions`, `prescription_items` | 6 |
| 00021 | `00021_create_dispenses.sql` | `dispenses` | 6 |
| 00022 | `00022_create_audit_logs.sql` | `audit_logs` + grants | 6 |
| 00023 | `00023_create_balance_views.sql` | `v_patient_balance`, `v_household_balance`, `v_lab_orders_paid` | 6 |
| 00024 | `00024_create_admissions.sql` | `admissions` | 7 |
| 00025 | `00025_create_bed_assignments.sql` | `bed_assignments` | 7 |
| 00026 | `00026_create_ward_charges.sql` | `ward_charges` | 7 |
| 00027 | `00027_create_admission_notes.sql` | `admission_notes` | 7 |
| 00028 | `00028_create_referrals.sql` | `referrals` | 8 |
| 00029 | `00029_create_referral_access_log.sql` | `referral_access_log` | 8 |
| 00030 | `00030_add_number_sequences.sql` | document number generation helper | 9 |
| 00031 | `00031_performance_indexes.sql` | indexes found necessary by real query plans | 9 |

> **The admin seed moved from 00007 to 00004** when multi-tenancy was removed. It no longer needs to wait for `user_roles` or `devices`, and because `users` no longer references `facilities`, either table can be seeded first. The old ordering existed only to work around the circular FK that no longer exists.

**Discipline rules:**

1. **Never edit a migration that has run anywhere** — not even to fix a typo. Write `00032_fix_thing.sql`. Editing an applied migration is the single most common way a working project breaks for the person marking it.
2. **Always write a working `Down`.** If a `Down` is genuinely impossible (data loss), say so in a comment and make it a deliberate, documented no-op rather than an empty stub.
3. **One logical change per file.** `create_invoices_charges.sql` is one change: two tightly coupled tables.
4. **Test the round trip** before committing: `goose up`, `goose down`, `goose up`. If `down` then `up` does not leave you where you started, the migration is wrong.

---

## 14. Number generation

Human-readable numbers (`patient_number`, `visit_number`, `receipt_number`, `invoice_number`, `referral_number`, `family_number`, `admission_number`) must be **unique, readable, and generatable offline**.

**Format:** `{FACILITY_CODE}/{TYPE}/{YEAR}/{SEQUENCE:5}` — e.g. `URPHC/PAT/2026/00123`.

The prefix comes from `facilities.code` — the single configuration row (§5.1), which is why that value is immutable once documents exist. `TYPE` is a short mnemonic: `PAT` patient · `VIS` visit · `FAM` family · `LAB` lab order · `RX` prescription · `INV` invoice · `RCP` receipt · `ADM` admission · `REF` referral.

Offline generation cannot use a database sequence, so:

- The **client** generates the number as `{FACILITY_CODE}/{TYPE}/{YEAR}/{device_short}-{local_seq}`, where `device_short` is 3 characters derived from the device ID and `local_seq` is a per-device counter held in Dexie.
- `device_short` makes collisions structurally impossible across devices. The number is slightly less pretty than a pure sequence, and that is a fair trade for offline correctness.
- If you prefer strictly sequential numbers, generate them **server-side on sync** and treat the client value as provisional, printing `PENDING` on any offline receipt. Decide once, in Sprint 6, and document the choice.

**Uniqueness is enforced by the unique index either way**, and the sync engine must treat a `unique_violation` on a number column as a recoverable conflict (reassign and retry) rather than a fatal error — otherwise a perfectly healthy offline device gets wedged.

### 14.1 Optional helper

Migration `00030` may add a server-side function for online paths:

```sql
CREATE OR REPLACE FUNCTION next_number(
    p_type TEXT, p_year INT
) RETURNS TEXT LANGUAGE plpgsql AS $$
DECLARE
    v_seq  INT;
    v_code TEXT;
BEGIN
    INSERT INTO number_sequences (number_type, year, last_value)
    VALUES (p_type, p_year, 1)
    ON CONFLICT (number_type, year)
    DO UPDATE SET last_value = number_sequences.last_value + 1
    RETURNING last_value INTO v_seq;

    -- The single configuration row supplies the document prefix.
    SELECT code INTO v_code FROM facilities LIMIT 1;

    RETURN v_code || '/' || p_type || '/' || p_year || '/' || lpad(v_seq::TEXT, 5, '0');
END;
$$;
```

Note the `ON CONFLICT ... DO UPDATE ... RETURNING`: this is atomic under concurrency, unlike `SELECT max(...) + 1`, which is the classic race that produces duplicate patient numbers the moment two clerks work at once.

---

## 15. Entity relationships at a glance

```
facilities
   └── users ──┬── user_roles
               └── devices ── refresh_tokens

households ──┬── patients ──┬── encounters ──┬── vitals
             │              │               ├── diagnoses
             │              │               ├── lab_orders ── lab_results
             │              │               ├── prescriptions ── prescription_items ── dispenses
             │              │               ├── invoices ── charges ── payment_allocations ── payments
             │              │               ├── admissions ──┬── bed_assignments
             │              │               │                ├── ward_charges
             │              │               │                └── admission_notes
             │              │               └── referrals ── referral_access_log
             │              └── (direct) ──── vitals / lab_orders / invoices / payments
             └── (billing party for all charges)

drug_products ──── prescription_items
tariffs ────────── charges
```

**Reading it:** `patients` is the spine. Anything clinical hangs off an `encounter`; anything financial hangs off `invoices`/`charges` and is attributed to a `patient` **and** a `household`. `households` never owns clinical data.

---

## 16. Schema review checklist

Run this before you commit any migration:

- [ ] Table has all **nine** sync columns and **no `facility_id`**, or is deliberately listed as server-only **with a comment saying why**.
- [ ] `id` is `UUID PRIMARY KEY` and **no `DEFAULT`** — the client always supplies it.
- [ ] Enum-like columns have a `CHECK` constraint, named `chk_<table>_<column>`.
- [ ] Every FK has an explicit `ON DELETE` behaviour (default is `NO ACTION`; be deliberate).
- [ ] Index on `server_rev` for pull performance.
- [ ] Index on `updated_at`.
- [ ] **No** `facility_id` anywhere — this deployment is single-tenant (§3).
- [ ] Any column used in a `WHERE` on a hot path is indexed — and the index name follows `idx_<table>_<cols>`.
- [ ] Money columns are `BIGINT` and named `*_minor`.
- [ ] Timestamps are `TIMESTAMPTZ`, never `TIMESTAMP`.
- [ ] Soft-delete semantics: reads filter `deleted_at IS NULL`.
- [ ] `Down` migration genuinely reverts the change, and `up → down → up` has been run.
- [ ] New syncable table added to `sync_table_scopes` with correct `pull_roles`, `push_roles` and `conflict_policy`.
- [ ] No derived value stored unless it is written in the same transaction as its source (and say so in a comment).
