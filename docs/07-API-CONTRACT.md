# API Contract

> **Base URL (dev):** `http://localhost:8080/api/v1`
> **Base URL (prod):** `https://phc-api-xxxx.run.app/api/v1`
> **Document status:** Canonical. The frontend and backend are written against this document, not against each other.
> **Last updated:** Sprint 0

---

## 1. Conventions

### 1.1 Versioning

All routes are prefixed `/api/v1`. The version only changes for a **breaking** change. Adding a field or an endpoint is not breaking. If you ever need `/api/v2`, both must run simultaneously — a PHC cannot pause work to update a browser.

### 1.2 Authentication

```http
Authorization: Bearer <access_token>
```

Bearer tokens, **not cookies** — see `00-ARCHITECTURE.md` §3.5. Tokens are never placed in a query string, because query strings land in logs and browser history.

Endpoints marked 🔓 are public. Everything else requires a valid token.

### 1.3 The response envelope

**Success — single resource:**

```json
{ "data": { "id": "...", "firstName": "Amina" }, "meta": { "requestId": "..." } }
```

**Success — collection:**

```json
{
  "data": [ { "id": "..." } ],
  "meta": {
    "requestId": "...",
    "pagination": { "limit": 50, "hasMore": true, "nextCursor": "018f..." }
  }
}
```

**Error — always this shape:**

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Date of birth must not be in the future.",
    "details": [ { "field": "dateOfBirth", "issue": "must_be_past" } ],
    "requestId": "..."
  }
}
```

`code` is **machine-readable and stable** — the frontend switches on it. `message` is **human-readable and may change** — never switch on it.

### 1.4 Error codes

| HTTP | `code` | Meaning | Client action |
|---|---|---|---|
| 400 | `VALIDATION_FAILED` | Body/params failed validation | Show field errors |
| 400 | `MALFORMED_REQUEST` | Unparseable JSON or missing header | Fix the client — a bug |
| 401 | `UNAUTHENTICATED` | No token, or token malformed | Redirect to login |
| 401 | `TOKEN_EXPIRED` | Access token expired | Refresh, then retry once |
| 401 | `INVALID_CREDENTIALS` | Wrong username or password | Show the login error |
| 401 | `CLOCK_SKEW` | Device time off by > 5 min, or an unparseable timestamp | Show "fix your clock" and block sync |
| 401 | `DEVICE_SIGNATURE_MISSING` | A signed endpoint was called without the three device headers | Register the device, then retry |
| 401 | `DEVICE_SIGNATURE_INVALID` | The device signature did not verify | Re-register the device key; do not retry blindly |
| 403 | `FORBIDDEN` | Authenticated, wrong role | Hide the action; do not retry |
| 403 | `DEVICE_REVOKED` | Device key revoked, or the device id is unknown | Wipe local data, force re-registration |
| 403 | `DEVICE_KEY_NOT_REGISTERED` | The device has no public key yet (the login → register gap) | Call `POST /devices` |
| 409 | `DEVICE_NONCE_REPLAYED` | These exact bytes were already received | Treat as already applied; a fresh request needs a fresh nonce |
| 403 | `OFFLINE_GRACE_EXPIRED` | Offline window passed under `block_writes` | Require an online re-auth |
| 404 | `NOT_FOUND` | No such resource — **also used for expired referrals** | Show a friendly not-found |
| 409 | `CONFLICT` | State conflict (`UNIQUE_VIOLATION`, `STALE_STATE`) | Reload and re-apply |
| 409 | `DUPLICATE_OPERATION` | Idempotency key already used… | Treat as success, or reconcile |
| 413 | `PAYLOAD_TOO_LARGE` | Batch too big | Split and resend |
| 422 | `BUSINESS_RULE_VIOLATION` | Valid input, illegal transition | Explain what is blocking |
| 429 | `RATE_LIMITED` | Too many requests | Back off using `Retry-After` |
| 500 | `INTERNAL_ERROR` | Unhandled | Retry with backoff |
| 503 | `DATABASE_UNAVAILABLE` | Neon unreachable | Retry with backoff |

> **`404` for a verified-but-expired referral is deliberate.** Returning `410 Gone` would confirm the token was once valid, which is a small information leak. A uniform `404` reveals nothing.

### 1.5 Pagination

**Cursor-based, not offset-based.** Offsets break when new records arrive mid-pagination, which in a health centre happens constantly.

```
GET /patients?limit=50&cursor=018f3c2a-...
```

`nextCursor` is the `id` of the last row. `hasMore: false` means stop. `limit` defaults to 50, max 200.

### 1.6 Idempotency for direct (non-sync) writes

Any `POST` that creates something financially significant accepts:

```http
Idempotency-Key: <uuidv7>
```

The server stores the key with the response for 24 hours; a repeat with the same key returns the **stored** response with `200` instead of creating a duplicate. This protects the cashier's browser retry path — the sync path has its own idempotency via `operation_id`.

### 1.7 Standard request headers

| Header | Required on | Purpose |
|---|---|---|
| `Authorization` | all protected | `Bearer <token>` |
| `X-Request-Id` | optional | Client-generated correlation ID; echoed back |
| ~~`X-Device-Id`~~ | — | **Not used.** The device is taken from the access token's `device_id` claim, which is authenticated. A header would be client-chosen, so a signature could claim another device. |
| `X-Device-Signature` | sync endpoints | ECDSA P-256 over the canonical string |
| `X-Device-Timestamp` | sync endpoints | ISO-8601, ±5 min tolerance |
| `X-Device-Nonce` | sync endpoints | Replay prevention |
| `Idempotency-Key` | selected POSTs | Duplicate-submission protection |

### 1.8 Role notation

`[FD]` front_desk · `[NU]` nurse · `[DR]` doctor · `[LS]` lab_scientist · `[PH]` pharmacist · `[CA]` cashier · `[AC]` accountant · `[AD]` admin

---

## 2. Health and meta

### `GET /health` 🔓
Liveness. Never touches the database.

```json
{ "status": "ok", "version": "0.3.1", "time": "2026-02-14T09:31:07Z" }
```

### `GET /ready` 🔓
Readiness. **Does** ping the database with a 2-second timeout. Cloud Run uses this to decide whether to route traffic to the instance.

```json
{ "status": "ready", "database": "ok", "latencyMs": 12 }
```
Returns `503` with `{"status":"degraded","database":"unreachable"}` when Neon is down.

> Keep these two separate. A health check that pings the database will fail during a Neon cold start and cause Cloud Run to kill a perfectly healthy instance.

### `GET /meta` 🔓
Everything the client needs at boot, in one request.

```json
{
  "data": {
    "schemaVersion": 1,
    "serverTime": "2026-02-14T09:31:07Z",
    "referralTtlHours": 48,
    "offlineGracePeriodHours": 168,
    "offlinePolicy": "block_writes",
    "syncMaxBatchSize": 200,
    "minClientVersion": "0.3.0",
    "features": { "referralPin": true, "posPrinting": true }
  }
}
```

`minClientVersion` is how you tell an ancient client to update without breaking it silently.

---

## 3. Authentication

### `POST /auth/login` 🔓

```json
{ "staffNumber": "URPHC/STF/004", "password": "...", "deviceId": "018f...", "deviceName": "Reception Desk 1" }
```

```json
{
  "data": {
    "accessToken": "eyJ...",
    "refreshToken": "9f2b...",
    "expiresIn": 900,
    "user": {
      "id": "...", "staffNumber": "...", "firstName": "...", "lastName": "...",
      "roles": ["front_desk"], "facilityName": "...",
      "mustChangePassword": false
    },
    "device": { "id": "...", "registered": true, "publicKeyRegistered": true }
  }
}
```

Rate-limited: **5 attempts per staff number per 15 minutes**, and separately **20 attempts per IP per 15 minutes**. Both limits are required — per-account alone lets an attacker spray many accounts; per-IP alone lets a distributed attack target one account.

Partial-failure responses:

| Situation | Status | `code` | Body extra |
|---|---|---|---|
| Account locked | 403 | `ACCOUNT_LOCKED` | `lockedUntil` |
| Device fingerprint mismatch | 403 | `DEVICE_MISMATCH` | `requiresDeviceRegistration: true` |
| Unknown device | 200 | — | `device.registered: false` |

### `POST /auth/refresh` 🔓

```json
{ "refreshToken": "9f2b...", "deviceId": "018f..." }
```

Returns a **new** access token and a **new** refresh token. The old refresh token is marked used. If a used token is presented again, the entire token **family** is revoked — that is the signature of a stolen token, and the safe response is to log everyone out.

### `POST /auth/logout` 🔐

```json
{ "refreshToken": "9f2b...", "revokeDevice": false }
```

Revokes the refresh token family. `revokeDevice: true` additionally revokes the device key — used when a workstation is being decommissioned.

### `POST /auth/offline-verify` 🔓

Used by the SPA on **first** login only, to learn the Argon2id cost parameters it should use for
its local offline verifier.

```json
{
  "data": {
    "algorithm": "argon2id",
    "version": 19,
    "memoryKiB": 65536,
    "iterations": 3,
    "parallelism": 4,
    "saltLengthBytes": 16,
    "keyLengthBytes": 32
  }
}
```

**No salt is returned, deliberately.** The salt is generated on the device (16 bytes from
`crypto.getRandomValues`) and never transmitted, so the resulting verifier is not comparable to
anything the server holds — not the password hash, not another device's verifier. The server's
useful contribution is the **cost**: raise the parameters centrally and every device picks them up
at its next online sign-in, with no client release.

**WebAssembly needs a CSP opt-in.** `hash-wasm` compiles a WASM module at runtime, which
`script-src 'self'` blocks. Without `'wasm-unsafe-eval'` in the policy, every offline sign-in fails
silently and the failure looks like a wrong password.

> The client derives its **own** verifier from the password; the server never sends a password hash. Offline verification is then a local Argon2id computation. See `05-SECURITY.md` §3.

### `POST /auth/change-password` 🔐

```json
{ "currentPassword": "...", "newPassword": "..." }
```

Revokes all other sessions. Returns fresh tokens for the current device.

### `GET /auth/session` 🔐

Who am I, according to the server. Used to revalidate after offline work.

```json
{ "data": { "user": { "...": "..." }, "roles": ["nurse"], "device": { "id": "...", "revoked": false }, "offlineGraceExpiresAt": "2026-02-21T09:31:07Z" } }
```

### `POST /devices` 🔐

Binds this workstation's ECDSA P-256 public key to the device named in the access token. Called
once after login, before the first sync push. See `05-SECURITY.md` §4.3.

```json
{
  "deviceId":   "018f3c2a-...",
  "publicKey":  "<base64url SPKI>",
  "algorithm":  "ES256",
  "label":      "Triage laptop 2",
  "platform":   "Windows 11 / Chrome 141",
  "appVersion": "0.1.0"
}
```

```json
{
  "data": {
    "id": "018f3c2a-...",
    "name": "Triage laptop 2",
    "platform": "Windows 11 / Chrome 141",
    "algorithm": "ES256",
    "publicKeyFingerprint": "746c91ed971d55d9…",
    "registered": true,
    "revoked": false,
    "lastSeenAt": "2026-02-14T09:31:07Z",
    "createdAt": "2026-02-14T09:20:00Z"
  }
}
```

| Situation | Status | `code` |
|---|---|---|
| First registration | 200 | — |
| Same key again (idempotent retry) | 200 | — |
| A different key already registered | 409 | `CONFLICT` |
| `deviceId` differs from the token | 400 | `VALIDATION_FAILED` |
| Public key does not parse | 400 | `VALIDATION_FAILED` |
| Device is revoked | 403 | `DEVICE_REVOKED` |

The fingerprint is computed **server-side** from the decoded DER. It is never accepted from the
client, and the public key is never echoed back.

### `POST /admin/devices/:id/revoke` 🔐 `[AD]`

```json
{ "reason": "laptop stolen from records room 2026-01-14" }
```

Marks the device revoked and **revokes every refresh-token family bound to it, in the same
transaction**. Signed requests from that device fail on the next request with `403 DEVICE_REVOKED`.
Access tokens already issued are not denylisted (no `jti` denylist yet), so JWT-only endpoints keep
working until they expire — at most 15 minutes.

Idempotent: revoking an already-revoked device returns 200 with `revokedSessions: 0`. `reason` is
required (5–500 characters) — a revocation nobody can explain later is a revocation that gets
overturned by accident.

```json
{ "data": { "device": { "...": "..." }, "revokedSessions": 2 } }
```

### `GET /admin/users` 🔐 `[AD]`

Staff administration. **Online-only and not synced**: a role change made against a
six-day-old offline copy of the staff list is a change made against stale information.

```json
{
  "data": {
    "users": [
      {
        "id": "018f0000-0000-7000-8000-0000000000b3",
        "staffNumber": "DEMO/STF/0004",
        "firstName": "Emeka",
        "lastName": "Nwosu",
        "email": "doctor@demo.phc",
        "roles": ["doctor"],
        "isActive": true,
        "mustChangePassword": true,
        "lastLoginAt": "2026-02-14T09:31:07Z",
        "createdAt": "2026-02-01T08:00:00Z"
      }
    ]
  }
}
```

Ordered by `staffNumber`, so the table does not reshuffle between refreshes. One page of up to
100; staff lists do not need cursor pagination, and pretending they do adds a state machine
nobody will exercise.

### `PATCH /admin/users/:id` 🔐 `[AD]`

```json
{ "role": "doctor", "isActive": true, "reason": "covering the consulting room" }
```

| Field | Semantics |
|---|---|
| `role` | A **toggle**. Granted when the user does not hold it, revoked when they do. Sending the full role set instead would let a stale client silently remove a grant made seconds earlier. |
| `isActive` | Opt in or out. Optional pointer in the API so `false` and "absent" differ. |
| `reason` | Recorded on a revocation or a disable. Defaults to `administrator change`; minimum 5 characters, enforced by the DB CHECK as well. |

Refusals, all server-side:

| Situation | Status | `code` |
|---|---|---|
| No `role` and no `isActive` | 400 | `VALIDATION_FAILED` |
| Unknown role name | 400 | `VALIDATION_FAILED` |
| Actor changes their own role or disables themselves | 400 | `VALIDATION_FAILED` |
| Disabling the last active administrator | 400 | `VALIDATION_FAILED` |
| Actor is not an admin (checked against the database, not the token) | 403 | `FORBIDDEN` |

Disabling an account also revokes **every** refresh-token family for it, in the same
transaction. Otherwise a dismissed employee keeps working for up to thirty days.

---

## 4. Sync

Full protocol in `02-SYNC-PROTOCOL.md`. Endpoints and contracts only here.

### `POST /sync/push` 🔐 + signed
Batched outbox drain. Max 200 operations (`SYNC_MAX_BATCH_SIZE`) or 1 MB.

Request/response shapes: `02-SYNC-PROTOCOL.md` §5. The response is wrapped in the standard
envelope, so the client reads `data.results`, `data.summary`, `data.nextCursor`.

| Situation | Status | `code` |
|---|---|---|
| Missing/expired device headers | 401 | `DEVICE_SIGNATURE_MISSING`, `CLOCK_SKEW`, `DEVICE_SIGNATURE_INVALID` |
| Device revoked or unknown | 403 | `DEVICE_REVOKED` |
| Device has no public key yet | 403 | `DEVICE_KEY_NOT_REGISTERED` |
| Replayed nonce | 409 | `DEVICE_NONCE_REPLAYED` |
| Malformed envelope, bad ids, batch too large | 400 | `VALIDATION_FAILED` |
| Body over 1 MB | 413 | `PAYLOAD_TOO_LARGE` |

Per-operation failures are **not** HTTP errors: the batch succeeds and each rejected operation is
reported in `results[]` with `status: "rejected"` and a `reason` (`forbidden`, `unknown_table`,
`validation:<field>`, `stale_reference`).

### `GET /sync/pull` 🔐

**JWT-only, not device-signed.** A read cannot change anything, and signing it adds CPU to the
request most likely to hit a Cloud Run cold start (see `05-SECURITY.md` §4.4.1). It is still
authenticated, scoped and rate-limited.

| Param | Type | Default | Notes |
|---|---|---|---|
| `since` | int64 | 0 | Last applied `seq`. 0 means bootstrap — use `/sync/snapshot` |
| `limit` | int | 500 | Max 2000 |

| Situation | Status | `code` |
|---|---|---|
| `limit` out of range, or non-numeric `since` | 400 | `VALIDATION_FAILED` |
| No token | 401 | `UNAUTHENTICATED` |

Returns `changes`, `rows`, `nextCursor`, `hasMore`, `fullResyncRequired`. `02-SYNC-PROTOCOL.md` §7.

**`nextCursor` is the seq of the LAST CHANGE READ**, not `max(seq)` of the table, and the read
runs inside one `REPEATABLE READ` transaction. `seq` is not gap-free; outside a snapshot a reader
can see seq 10240 while 10238–10239 are uncommitted, advance past the gap, and lose those rows for
ever.

| Field | Meaning |
|---|---|
| `changes` | Ordered manifest. Filtered by `pull_roles`, but the cursor still advances past filtered entries — otherwise a client would re-request the same window for ever |
| `rows` | Each distinct current row **once**, however many times it changed in the window |
| `fullResyncRequired` | `true` with `reason: "cursor_expired"` and `oldestAvailableSeq` when the cursor predates retention. A device that old cannot catch up incrementally; the client must re-bootstrap |
| `serverTime` | Server clock, for skew display |

**Redaction.** `users` rows are built from an explicit column allowlist that does **not** include
`password_hash`, `password_changed_at`, `failed_login_count` or `locked_until`; the delete pass in
`redactRows` is defence in depth. There is one place rows are serialised, so there is one place a
hash could leak.

### `GET /sync/snapshot` 🔐

**JWT-only**, like pull.

| Param | Type | Notes |
|---|---|---|
| `table` | string | Required; must be in `sync_table_scopes` **and** have a column allowlist |
| `after` | uuid | Keyset pagination cursor (last `id` seen). Keyset, never `OFFSET` |
| `limit` | int | Default 500, max 1000 |

| Situation | Status | `code` |
|---|---|---|
| Missing or unknown `table` | 400 | `VALIDATION_FAILED` |
| Role may not read that table | 403 | `FORBIDDEN` |

```json
{
  "data": { "rows": [ { "...": "..." } ] },
  "meta": { "hasMore": true, "cursorAtStart": 88410, "table": "patients" }
}
```

`cursorAtStart` is captured **before** the first page is read; the client stores it as its `pullCursor` only after bootstrap finishes. Otherwise changes made during a long bootstrap are lost.

### `GET /sync/status` 🔐

```json
{
  "data": {
    "serverCursor": 10432,
    "device": { "id": "...", "lastPushAt": "...", "lastPullAt": "...", "pendingOperationCount": 0, "lastError": null },
    "conflicts": { "unreviewed": 3 },
    "schemaVersion": 1
  }
}
```

### `POST /sync/resolve-conflict` 🔐 `[AD]`

```json
{ "conflictId": "...", "resolution": "accepted_server" | "accepted_client" | "manual", "manualPayload": { } }
```

---

## 5. Patients and households

### `GET /patients` 🔐 `[FD][NU][DR][CA][AC][AD]`

| Param | Notes |
|---|---|
| `q` | Free-text; matches name, patient number, phone |
| `householdId` | Members of one family |
| `gender`, `isActive` | Filters |
| `limit`, `cursor` | Pagination |

Search implementation note: the query is written as `lower(surname) LIKE lower($1) || '%'` so it can use the functional index from `01-DATA-MODEL.md` §6.2. A leading-wildcard search (`%amina%`) cannot use that index and will scan — acceptable for the small `q`-contains fallback, but keep the prefix search as the primary path.

### `POST /patients` 🔐 `[FD][AD]`

```json
{
  "firstName": "Amina",
  "middleName": null,
  "surname": "Bello",
  "gender": "female",
  "dateOfBirth": "1991-04-12",
  "ageYears": null,
  "maritalStatus": "married",
  "phone": "08031234567",
  "tribe": "Hausa",
  "occupation": "Trader",
  "address": "12 Zaria Road",
  "addressLandmark": "Opposite the market gate",
  "householdId": null,
  "registrationDate": "2026-02-14"
}
```

| Field | Required | Validation |
|---|---|---|
| `firstName` | ✅ | 1–100 chars |
| `middleName` | — | ≤100 chars |
| `surname` | ✅ | 1–100 chars |
| `gender` | ✅ | enum |
| `dateOfBirth` | ⚠️ | Past date. **Required unless `ageYears` is supplied** |
| `ageYears` | ⚠️ | 0–130. Required unless `dateOfBirth` is supplied |
| `registrationDate` | ✅ | Not in the future; defaults to today |
| `maritalStatus` | — | enum |
| `phone` | — | Nigerian format, normalised to `0XXXXXXXXXX` |
| everything else | — | free text |

Returns `201` with the created patient including the server-generated `patientNumber`.

**`422` cases:** `AGE_OR_DOB_REQUIRED`, `REGISTRATION_DATE_IN_FUTURE`.

> Note that `patientNumber` is assigned by the **server** for direct (online) writes, and by the **client** in the offline path. Both are globally unique in a single-tenant deployment. See `01-DATA-MODEL.md` §14.

### `GET /patients/:id` 🔐
Full record. Includes `household` summary and `activeEncounterId` if one exists.

### `PATCH /patients/:id` 🔐 `[FD][AD]`
Partial update. Send only the changed fields. Requires `If-Unmodified-Since` or a `baseServerRev` in the body — otherwise two clerks editing the same patient will silently clobber each other even **online**, which is the same bug the sync engine solves offline.

### `GET /patients/:id/history` 🔐 `[NU][DR][AD]`
Timeline: encounters, vitals, diagnoses, labs, prescriptions, admissions, referrals. Cursor-paginated. **This deliberately spans all clinical data for one patient** — it is the "continuous management" requirement.

### `GET /patients/:id/balance` 🔐 `[CA][AC][AD][FD]`

```json
{
  "data": {
    "patientBalanceMinor": 450000,
    "householdBalanceMinor": 1250000,
    "currency": "NGN",
    "currencyMinorUnits": 2,
    "unpaidInvoices": [ { "id": "...", "invoiceNumber": "...", "totalMinor": 200000, "paidMinor": 50000 } ]
  }
}
```

Derived on every call — never cached in a table.

### `GET /patients/duplicates` 🔐 `[FD][AD]`

Probable duplicates, scored. `?threshold=0.8`.

```json
{
  "data": [
    {
      "score": 0.94,
      "reason": ["same_phone", "similar_name", "same_dob"],
      "patients": [ { "id": "...", "patientNumber": "..." }, { "id": "...", "patientNumber": "..." } ]
    }
  ]
}
```

### `POST /patients/merge` 🔐 `[AD]`

```json
{ "primaryPatientId": "...", "mergedPatientIds": ["..."], "reason": "Registered twice offline" }
```

Reassigns all child records to the primary, sets `deleted_at` on the others, writes an audit entry. **Requires `admin`** — a merge is irreversible in practice and must leave a named trail.

### Households

| Method | Path | Roles | Notes |
|---|---|---|---|
| `GET` | `/households` | FD NU DR CA AC AD | `?q=` searches family name and number |
| `POST` | `/households` | FD AD | `{ familyName, phone, address, notes }` → `201` with `familyNumber` |
| `GET` | `/households/:id` | FD NU DR CA AC AD | Includes members and the derived family balance |
| `PATCH` | `/households/:id` | FD AD | |
| `POST` | `/households/:id/members` | FD AD | `{ patientId }` — links an existing patient |
| `DELETE` | `/households/:id/members/:patientId` | FD AD | Unlinks (sets `household_id = NULL`) |
| `GET` | `/households/:id/balance` | CA AC AD FD | Derived |
| `GET` | `/households/:id/statement` | CA AC AD FD | All invoices, payments, allocations for the family |

### `POST /patients/family` 🔐 `[FD][AD]`

Registers a whole family in one transaction — the flow the staff specifically asked for. One `household`, N `patients`, all linked.

```json
{
  "household": { "familyName": "Bello", "phone": "08031234567", "address": "12 Zaria Road" },
  "members": [
    { "firstName": "Amina", "surname": "Bello", "gender": "female", "dateOfBirth": "1991-04-12", "isHead": true },
    { "firstName": "Yusuf", "surname": "Bello", "gender": "male",   "ageYears": 8 }
  ]
}
```

Returns `201` with the household and all created patients. **All-or-nothing** — a family half-registered is worse than one not registered at all.

---

## 6. Encounters, queue and vitals

### `POST /encounters` 🔐 `[FD][NU][DR]`

```json
{ "patientId": "...", "encounterType": "opd", "priority": 0, "chiefComplaint": "Fever for 3 days" }
```

Returns `201` with `visitNumber` and `workflowStage: "registered"`.

**`422 ONE_ACTIVE_ENCOUNTER_EXISTS`** if the patient already has an active encounter — with the existing `encounterId` in `details` so the UI can offer "resume that visit?" instead of a dead end.

### `GET /encounters` 🔐
The queue. The most-used endpoint in the system.

| Param | Notes |
|---|---|
| `stage` | Repeatable: `?stage=vitals_done&stage=awaiting_doctor` |
| `status` | Default `active` |
| `date` | Defaults to today, in the clinic's configured timezone (`facilities.timezone`) |
| `priority` | Minimum priority |
| `patientId` | All encounters for a patient |

Ordered by `priority DESC, queued_at ASC` — emergencies first, then longest-waiting.

```json
{
  "data": [
    {
      "id": "...", "visitNumber": "URPHC/2026/00451/01",
      "workflowStage": "awaiting_doctor", "priority": 0, "queuedAt": "...",
      "waitingMinutes": 23,
      "patient": { "id": "...", "patientNumber": "...", "firstName": "Amina", "surname": "Bello", "gender": "female", "ageYears": 34, "householdId": "..." },
      "hasVitals": true, "latestVitals": { "bpSystolic": 118, "bpDiastolic": 76, "temperatureC": 37.2 }
    }
  ]
}
```

`waitingMinutes` is computed server-side from the **server's** clock so a workstation with a wrong clock cannot display nonsense waiting times.

### `GET /encounters/:id` 🔐
Full encounter with patient, vitals, diagnoses, orders, prescriptions, invoices and stage history.

### `POST /encounters/:id/transition` 🔐 `[NU][DR][FD]`

```json
{ "toStage": "awaiting_doctor", "reason": null }
```

Validates against the state machine in `01-DATA-MODEL.md` §7.2.

- **`200`** on success.
- **`200`** (no-op) if already in `toStage` — **transitions are idempotent**, which is what makes offline retries safe.
- **`422 INVALID_TRANSITION`** with `details: { from, to, allowed: [...] }`.

### Vitals

| Method | Path | Roles | Notes |
|---|---|---|---|
| `POST` | `/encounters/:id/vitals` | NU DR | Records vitals; advances the stage to `vitals_done` if it was `registered` |
| `POST` | `/patients/:id/vitals` | NU DR | Standalone (e.g. a walk-in BP check, no encounter) |
| `GET` | `/patients/:id/vitals` | NU DR AD | `?limit=20` — newest first, for trend display |
| `GET` | `/vitals/:id` | NU DR AD | |
| `PATCH` | `/vitals/:id` | NU DR | **Only within 15 minutes of `recorded_at`** — corrections of typos, not rewrites of history |

```json
// POST /encounters/:id/vitals
{
  "bpSystolic": 118, "bpDiastolic": 76,
  "temperatureC": 37.2, "pulseRate": 82, "respiratoryRate": 18,
  "heightCm": 164, "weightKg": 62.5, "spo2": 98, "painScore": 2,
  "notes": null, "recordedAt": "2026-02-14T09:45:00Z"
}
```

**All fields optional, but at least one measurement is required** (`422 NO_MEASUREMENT_PROVIDED`). `bmi` is computed server-side from height and weight if both are present, and validated against the `CHECK` constraints — an out-of-range value returns `422 VITALS_OUT_OF_RANGE` with the field named, rather than a `500` from a constraint violation.

---

## 7. Clinical

### Diagnoses

| Method | Path | Roles |
|---|---|---|
| `POST` | `/encounters/:id/diagnoses` | DR |
| `GET` | `/encounters/:id/diagnoses` | DR NU AD |
| `GET` | `/patients/:id/diagnoses` | DR NU AD |
| `PATCH` | `/diagnoses/:id` | DR (same day only) |

```json
{ "diagnosisType": "primary", "icd10Code": "B50.9", "description": "Plasmodium falciparum malaria, unspecified", "clinicalNotes": "..." }
```

### `GET /icd10/search` 🔐 `[DR][NU][AD]`
`?q=malaria&limit=20`. Serves a bundled reference list — **not** a database table. Ship it as a JSON asset; importing the full ICD-10 catalogue is a distraction and costs you a day.

### Lab orders

| Method | Path | Roles | Notes |
|---|---|---|---|
| `POST` | `/encounters/:id/lab-orders` | DR | Creates the order **and** its invoice + charge in one transaction |
| `GET` | `/lab-orders` | LS DR AD | The lab worklist |
| `GET` | `/lab-orders/:id` | LS DR NU AD | |
| `POST` | `/lab-orders/:id/collect` | LS | Records specimen collection |
| `POST` | `/lab-orders/:id/results` | LS | Submits results |
| `PATCH` | `/lab-orders/:id` | DR | Cancel (`status: cancelled`), same day |
| `GET` | `/lab-orders/catalogue` | DR LS AD | Available tests and prices from `tariffs` |

`GET /lab-orders` filters:

| Param | Notes |
|---|---|
| `status` | `ordered`, `collected`, `in_progress`, `completed` |
| `paidOnly` | `true` (default) — only orders whose charge is fully settled |
| `date`, `priority` | |

```json
// POST /encounters/:id/lab-orders
{
  "tests": [
    { "testCode": "MP", "testName": "Malaria Parasite", "specimenType": "blood", "priority": "routine", "clinicalIndication": "Fever 3 days" },
    { "testCode": "FBC", "testName": "Full Blood Count", "specimenType": "blood", "priority": "routine" }
  ]
}
```

**Response `201` includes the created invoice** so the UI can immediately show "₦3,500 due — send to cashier".

```json
{
  "data": {
    "labOrders": [ { "id": "...", "testName": "Malaria Parasite" } ],
    "invoice": { "id": "...", "invoiceNumber": "...", "totalMinor": 350000, "status": "unpaid" }
  }
}
```

> **The billing coupling is deliberate and is the requirement.** A doctor orders a test; the test cannot be performed until it is paid; therefore the order and its invoice are created together. Splitting them would allow an unpaid, unperformable order to exist.

```json
// POST /lab-orders/:id/results
{
  "results": [
    { "parameterName": "Malaria Parasite", "valueText": "Positive (P. falciparum trophozoites)", "abnormalFlag": "high" },
    { "parameterName": "Haemoglobin", "valueNumeric": 9.8, "unit": "g/dL", "referenceRange": "11.5-16.5", "abnormalFlag": "low" }
  ],
  "remarks": "Severe parasitaemia"
}
```

Each result requires `valueText` **or** `valueNumeric`. Submitting results sets the order to `completed` and appends a `lab_done` transition, returning the encounter to `awaiting_doctor`.

### Prescriptions

| Method | Path | Roles | Notes |
|---|---|---|---|
| `POST` | `/encounters/:id/prescriptions` | DR | Creates the prescription, its items, and the drug invoice + charges |
| `GET` | `/prescriptions` | PH DR CA AD | Pharmacist worklist |
| `GET` | `/prescriptions/:id` | PH DR CA AD | |
| `POST` | `/prescriptions/:id/dispense` | PH | Records dispensing |
| `PATCH` | `/prescriptions/:id` | DR | Cancel, same day, only if not dispensed |
| `GET` | `/drug-products` | DR PH AD | Catalogue search, `?q=`, `?form=` |

```json
// POST /encounters/:id/prescriptions
{
  "prescriptionType": "opd",
  "notes": "Review in 3 days if no improvement",
  "items": [
    { "drugProductId": "...", "drugName": "Artemether/Lumefantrine 20/120mg", "dose": "4 tablets",
      "frequency": "BD", "durationDays": 3, "quantityPrescribed": 24, "route": "oral",
      "instructions": "Take with fatty food" }
  ]
}
```

Response `201` includes the created invoice, exactly like lab orders.

```json
// POST /prescriptions/:id/dispense
{
  "items": [
    { "prescriptionItemId": "...", "quantityDispensed": 24, "batchNumber": "LOT-4471", "expiryDate": "2027-06-30" }
  ],
  "counsellingNotes": "Advised to complete the full course"
}
```

A `quantityDispensed` **less** than prescribed is valid (partial dispensing) and sets `prescriptions.status = 'partially_dispensed'`. The remaining quantity stays on the worklist for a later visit.

**`422 PRESCRIPTION_NOT_PAID`** if the linked invoice is not fully settled — unless the dispenser is `admin` and sends `overrideReason`, which is audited.

---

## 8. Billing

### Tariffs

| Method | Path | Roles |
|---|---|---|
| `GET` | `/tariffs` | all (read) |
| `POST` | `/tariffs` | AC AD |
| `PATCH` | `/tariffs/:id` | AC AD |
| `POST` | `/tariffs/:id/close` | AC AD — sets `effective_to = today` |

> **There is no `DELETE /tariffs/:id`.** Prices are closed, never deleted, because historical charges reference them.

### Invoices

| Method | Path | Roles | Notes |
|---|---|---|---|
| `GET` | `/invoices` | CA AC AD DR | `?status=unpaid&patientId=&householdId=&from=&to=` |
| `GET` | `/invoices/:id` | CA AC AD DR | With charges and allocation detail |
| `POST` | `/invoices` | DR CA NU | Ad-hoc invoice (e.g. a procedure) |
| `POST` | `/invoices/:id/void` | CA AD | Requires `reason`; audited |

```json
// GET /invoices/:id
{
  "data": {
    "id": "...", "invoiceNumber": "URPHC/INV/2026/00871", "invoiceType": "mixed",
    "status": "partially_paid",
    "totalMinor": 350000, "paidMinor": 200000, "outstandingMinor": 150000,
    "currency": "NGN", "currencyMinorUnits": 2,
    "patient": { "id": "...", "firstName": "Amina", "surname": "Bello" },
    "household": { "id": "...", "familyName": "Bello", "familyNumber": "URPHC/FAM/2026/00042" },
    "charges": [
      { "id": "...", "chargeType": "lab", "description": "Malaria Parasite", "amountMinor": 150000, "paidMinor": 150000, "outstandingMinor": 0, "referenceType": "lab_order", "referenceId": "..." },
      { "id": "...", "chargeType": "lab", "description": "Full Blood Count", "amountMinor": 200000, "paidMinor": 50000, "outstandingMinor": 150000, "referenceType": "lab_order", "referenceId": "..." }
    ],
    "payments": [ { "id": "...", "receiptNumber": "...", "amountMinor": 200000, "paymentMethod": "cash", "paidAt": "..." } ]
  }
}
```

Per-charge `paidMinor` / `outstandingMinor` come from `payment_allocations` — **derived, never stored**.

### `GET /patients/:id/outstanding` 🔐 `[CA][AC][AD][FD]`
What the cashier sees when a patient arrives at the desk: every unpaid charge with its description, in a sensible payment order (oldest first, ward charges before drugs).

### Payments — the most safety-critical endpoints

### `POST /payments` 🔐 `[CA][AD]`

Requires `Idempotency-Key`.

```json
{
  "patientId": "...",
  "householdId": "...",
  "amountMinor": 500000,
  "paymentMethod": "cash",
  "paymentReference": null,
  "allocations": [
    { "chargeId": "...", "amountMinor": 350000 },
    { "chargeId": "...", "amountMinor": 150000 }
  ],
  "autoAllocate": false,
  "notes": null
}
```

- `autoAllocate: true` applies the payment to the oldest outstanding charges and ignores `allocations`. **This is the default path in the UI** — a cashier should not have to think about FIFO allocation.
- Manual `allocations` are for the exception case (a patient paying for one specific test only).

**`422` cases, all with actionable detail:**

| Code | When | `details` |
|---|---|---|
| `ALLOCATION_EXCEEDS_PAYMENT` | Sum of allocations > amount | `{ allocatedMinor, amountMinor }` |
| `CHARGE_OVER_SETTLED` | A charge would be overpaid | `{ chargeId, chargeMinor, alreadyPaidMinor, attemptedMinor }` |
| `AMOUNT_MUST_BE_POSITIVE` | amount ≤ 0 | — |
| `CHARGE_NOT_FOUND` / `CHARGE_VOID` | Bad reference | `{ chargeId }` |

**Duplicate `Idempotency-Key` returns `200` with the original payment**, not `409`. A retried payment is a user who did not see the first response — which is the normal case on a flaky link.

Response `201`:

```json
{
  "data": {
    "payment": { "id": "...", "receiptNumber": "URPHC/RCP-01/2026/01007", "amountMinor": 500000, "paidAt": "..." },
    "receipt": { "printable": true, "width": "80mm", "template": "payment" },
    "invoices": [ { "id": "...", "status": "paid", "outstandingMinor": 0 } ],
    "householdBalanceMinor": 750000,
    "patientBalanceMinor": 0
  }
}
```

### `POST /payments/:id/reverse` 🔐 `[CA][AD]`

```json
{ "reason": "Wrong patient selected at the desk", "supervisorId": "..." }
```

Creates a reversing payment, marks the original `reversed`, voids its allocations. **Never a delete.** Requires a reason of at least 10 characters, and an `admin` supervisor for amounts above a configurable threshold.

### `GET /payments` 🔐 `[CA][AC][AD]`
`?from=&to=&method=&userId=`. The day's collections.

### `GET /payments/:id/receipt` 🔐 `[CA][AC][AD]`

```json
{
  "data": {
    "receiptNumber": "...", "width": "80mm",
    "clinic": { "name": "...", "address": "...", "phone": "...", "code": "URPHC" },
    "patient": { "name": "...", "patientNumber": "..." },
    "lines": [ { "description": "Malaria Parasite", "amountMinor": 150000 } ],
    "totalMinor": 500000, "amountPaidMinor": 500000, "changeMinor": 0,
    "balanceMinor": 750000,
    "receivedBy": "Grace Okon", "paidAt": "...",
    "qrPayload": "https://phc.example.org/verify/r/..."
  }
}
```

The receipt is rendered **client-side** into the POS layout — the API returns structured data, never HTML. This keeps printing concerns in the print stylesheet where they belong (see `03-UI-PATTERNS.md` §7).

### `GET /reports/daily-collections` 🔐 `[CA][AC][AD]`
`?date=2026-02-14` → totals by method, by user, with reversal count. The cashier's end-of-day reconciliation.

---

## 9. Admissions

| Method | Path | Roles | Notes |
|---|---|---|---|
| `POST` | `/admissions` | DR | `{ patientId, encounterId, admittingDiagnosisId, ward, bedNumber, admissionReason, expectedDischargeDate }` |
| `GET` | `/admissions` | DR NU CA AC AD | `?status=admitted&ward=` |
| `GET` | `/admissions/:id` | DR NU CA AC AD | Full episode: notes, vitals, bed history, accrued charges, balance |
| `POST` | `/admissions/:id/transfer` | NU DR | `{ ward, bedNumber, transferReason }` — vacates the old bed and assigns the new |
| `POST` | `/admissions/:id/notes` | NU DR | `{ noteType, content }` |
| `POST` | `/admissions/:id/discharge` | DR | `{ dischargeType, dischargeSummary, overrideReason? }` |
| `GET` | `/admissions/:id/charges` | CA AC AD DR | Accrued ward charges and balance |
| `GET` | `/beds` | NU DR AD | `{ ward, bedNumber, occupied, patientId?, admissionId? }` |
| `POST` | `/admissions/accrue-daily` | system/AD | Idempotent daily accrual; normally a scheduled job |

```json
// POST /admissions/:id/discharge
{ "dischargeType": "recovered", "dischargeSummary": "Afebrile for 48 hours. Completed ACT course." }
```

**`422 OUTSTANDING_BALANCE`** when the derived balance is positive:

```json
{
  "error": {
    "code": "OUTSTANDING_BALANCE",
    "message": "This admission has an outstanding balance of ₦2,500.",
    "details": { "outstandingMinor": 250000, "currency": "NGN", "admissionId": "..." }
  }
}
```

Permitted without clearing: `dischargeType` of `against_advice`, `died`, or `absconded`. Anything else requires an `admin` and an `overrideReason` of ≥10 characters, which is audited as `discharge_override`.

```json
// POST /admissions/accrue-daily  →  run for all open admissions
{ "date": "2026-02-14" }
```
```json
{ "data": { "date": "2026-02-14", "admissionsProcessed": 7, "chargesCreated": 7, "chargesSkipped": 2, "totalAccruedMinor": 1750000 } }
```

`chargesSkipped` counts admissions already charged for that date — the `UNIQUE (admission_id, charge_date)` constraint doing its job. Re-running is always safe. **Say this in your defence:** it is a small, concrete demonstration of idempotency designed in rather than bolted on.

---

## 10. Referrals

### `POST /referrals` 🔐 `[DR]`

```json
{
  "patientId": "...",
  "encounterId": "...",
  "referralType": "clinical",
  "priority": "urgent",
  "toFacilityName": "Federal Medical Centre, Keffi",
  "toFacilityAddress": "Keffi, Nasarawa State",
  "toFacilityPhone": "08099887766",
  "toDepartment": "Internal Medicine",
  "toClinicianName": "Dr. Musa",
  "reasonForReferral": "Unresolving fever with severe anaemia",
  "clinicalSummary": "...",
  "workingDiagnosis": "Severe malaria with anaemia",
  "investigationsDone": "MP positive, Hb 9.8 g/dL",
  "treatmentGiven": "IV Artesunate commenced",
  "allergies": "None known",
  "currentMedications": "Artemether/Lumefantrine",
  "includeVitalsSnapshot": true,
  "pinRequired": true,
  "status": "issued"
}
```

`status: "draft"` saves without issuing (no token, no expiry). `"issued"` generates the token and PIN immediately.

**Response `201` — the only time the raw token is ever returned in plaintext, other than a reprint:**

```json
{
  "data": {
    "referral": { "id": "...", "referralNumber": "URPHC/REF/2026/00087", "status": "issued", "issuedAt": "...", "expiresAt": "2026-02-16T09:31:07Z" },
    "access": {
      "url": "https://phc.example.org/r/8Kd2mQ_9xR4vB1nP7wZ3sL6tY0aH5cJ2eF4gU8iM1oQ",
      "pin": "472913",
      "expiresAt": "2026-02-16T09:31:07Z",
      "pinRequired": true
    },
    "print": { "template": "referral-slip", "width": "58mm", "qrDataUrl": "data:image/png;base64,..." }
  }
}
```

> **Return the QR as a `dataUrl` from the API.** The `qrcode` library can render client-side, but generating it server-side guarantees that what is printed and what is stored are provably the same string — which matters when a referral cannot be opened at the receiving end and you have to debug it.

### `GET /referrals` 🔐 `[DR][NU][AD]`
`?patientId=&status=&from=&to=`. Includes `viewCount` and `firstViewedAt` so a doctor can see whether the receiving facility actually opened it.

### `GET /referrals/:id` 🔐 `[DR][NU][AD]`
Full note. **Does not return the URL or PIN** — those are only available through `/reprint`.

### `POST /referrals/:id/reprint` 🔐 `[DR][AD]`
Regenerates the printable slip (same token and PIN, retrieved via the encrypted copy, `print_count++`, audited). This is why the token is stored encrypted in addition to hashed — see `01-DATA-MODEL.md` §11.1.

### `POST /referrals/:id/revoke` 🔐 `[DR][AD]`
`{ "reason": "..." }` — sets `status = 'cancelled'`, `revoked_at`, and makes the public link return `404` immediately.

### `GET /referrals/:id/access-log` 🔐 `[DR][AD]`
Every view, PIN attempt and print, with IP and timestamp. Excellent evidence for your defence.

### Public referral access 🔓

### `GET /r/:token` 🔓
**No authentication.** Deliberately returns a minimal payload.

```json
{
  "data": {
    "referralNumber": "URPHC/REF/2026/00087",
    "issuedAt": "...", "expiresAt": "...",
    "priority": "urgent",
    "patient": { "name": "Amina Bello", "ageYears": 34, "gender": "female" },
    "fromFacility": { "name": "Ungwan Rimi PHC", "phone": "..." },
    "toFacility": { "name": "Federal Medical Centre, Keffi", "department": "Internal Medicine" },
    "referringClinician": "Dr. Ibrahim Sani",
    "reasonForReferral": "...",
    "clinicalSummary": "...",
    "workingDiagnosis": "...",
    "investigationsDone": "...",
    "treatmentGiven": "...",
    "vitalsSnapshot": { "bpSystolic": 118, "bpDiastolic": 76, "temperatureC": 38.4, "pulseRate": 104 },
    "allergies": "None known",
    "currentMedications": "...",
    "pinRequired": true
  }
}
```

**Field whitelisting is mandatory.** This response is built from an explicit struct — never by serialising the `referrals` row, because the row carries `access_token_hash`, `pin_hash`, `pin_attempt_count` and internal IDs that must never leave the server. Write the response struct first, then map into it.

What is **excluded**: patient address, phone, patient number, full clinical history, financial data, all internal IDs, the token hash, the PIN hash.

**Errors:**

| Case | Status | `code` |
|---|---|---|
| Unknown token | 404 | `REFERRAL_NOT_FOUND` |
| Expired | 404 | `REFERRAL_NOT_FOUND` — **deliberately indistinguishable** |
| Revoked | 404 | `REFERRAL_NOT_FOUND` |
| PIN required, not yet supplied | 401 | `PIN_REQUIRED` |
| Wrong PIN | 401 | `INVALID_PIN` + `attemptsRemaining` |
| Too many wrong PINs | 429 | `PIN_LOCKED` + `lockedUntil` |

Uniform `404` prevents token enumeration: an attacker cannot distinguish "never existed" from "expired".

### `POST /r/:token/verify-pin` 🔓

```json
{ "pin": "472913" }
```

Returns the referral payload on success. Rate-limited: **5 attempts per referral per hour, hard lock after 5 failures.** On success, writes `pin_success` to `referral_access_log` and sets `status = 'viewed'`, `first_viewed_at` if unset, and increments `view_count`.

### `POST /r/:token/acknowledge` 🔓

```json
{ "receivingClinicianName": "Dr. Musa", "notes": "Patient seen, admitted to ward 3" }
```

Lets the receiving facility close the loop, which sets `status = 'acknowledged'` and shows the referring doctor that the patient arrived. **This is the feature that turns a printed note into a referral *system*** — make a point of demonstrating it.

---

## 11. Administration

| Method | Path | Roles | Notes |
|---|---|---|---|
| `GET` | `/admin/users` | AD | ✅ Built — list staff with their roles |
| `PATCH` | `/admin/users/:id` | AD | ✅ Built — `{ role, isActive, reason }`; `role` toggles |
| `POST` | `/admin/users` | AD | ⬜ Not built — create a staff account |
| `POST` | `/admin/users/:id/reset-password` | AD | ⬜ Not built — temporary password + `mustChangePassword` |
| `GET` | `/admin/devices` | AD | ⬜ Not built — with sync health from `device_sync_state` |
| `POST` | `/admin/devices/:id/revoke` | AD | ✅ Built — `{ reason }`, immediate |
| `GET` | `/admin/audit-logs` | AD AC | `?entityTable=&entityId=&userId=&action=&from=&to=` |
| `GET` | `/admin/sync-conflicts` | AD | Unreviewed conflicts |
| `GET` | `/admin/sync-log` | AD | Recent batches from `sync_batches` |
| `GET` | `/admin/health` | AD | DB latency, pool stats, disk, version |

### `GET /admin/devices`

```json
{
  "data": [
    {
      "id": "...", "name": "Reception Desk 1", "fingerprint": "a3:f2:...",
      "registeredBy": "Grace Okon", "lastSeenAt": "2026-02-14T09:28:00Z",
      "revokedAt": null, "appVersion": "0.3.1",
      "sync": { "lastPullCursor": 10432, "pendingOperationCount": 0, "lastError": null, "minutesSinceLastPush": 4 }
    }
  ]
}
```

---

## 12. Sprint-to-endpoint map

| Sprint | Endpoints introduced |
|---|---|
| 0 | `GET /health`, `GET /ready`, `GET /meta` |
| 1 | `POST /auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/offline-verify`, `/auth/change-password`, `GET /auth/session` |
| 2 | `POST /sync/push`, `GET /sync/pull`, `/sync/snapshot`, `/sync/status`, `POST /sync/resolve-conflict` |
| 3 | `GET/POST /patients`, `GET/PATCH /patients/:id`, `/patients/:id/history`, `/patients/:id/balance`, `/patients/duplicates`, `/patients/merge`, `POST /patients/family`, all `/households/*` |
| 4 | `POST /encounters`, `GET /encounters`, `/encounters/:id`, `/encounters/:id/transition`, all vitals endpoints |
| 5 | diagnoses endpoints, lab order endpoints, `GET /icd10/search` |
| 6 | tariffs, invoices, charges, `POST /payments`, `/payments/:id/reverse`, `/payments/:id/receipt`, `GET /reports/daily-collections`, prescription and dispense endpoints, `/drug-products` |
| 7 | admissions, transfers, notes, discharge, `/beds`, `/admissions/accrue-daily` |
| 8 | `POST /referrals`, `GET /referrals`, `/referrals/:id/reprint`, `/revoke`, `/access-log`, `GET /r/:token`, `POST /r/:token/verify-pin`, `/acknowledge` |
| 9 | `GET /admin/*`, reports, exports |

---

## 13. Contract review checklist

Before you consider an endpoint done:

- [ ] Route registered under `/api/v1` with the correct role middleware.
- [ ] Request struct has `binding` tags **and** explicit validation in the service layer.
- [ ] Response uses the standard envelope with `requestId`.
- [ ] Every error path returns a **stable `code`** from the table in §1.4.
- [ ] Role check exists in the **service**, not only in the route middleware.
- [ ] The endpoint is idempotent where it touches money, or requires an `Idempotency-Key`.
- [ ] Audit log written for any state change.
- [ ] No sensitive field can reach the response — verify `password_hash`, tokens and PIN hashes are absent from every DTO.
- [ ] Documented here in the same commit as the code.
