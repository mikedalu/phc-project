# Sync Protocol

> **Document status:** Canonical. This is the contract between the Dexie client and the Go API.
> **Prerequisite reading:** `00-ARCHITECTURE.md` §4, `01-DATA-MODEL.md` §3.
> **Last updated:** Sprint 0

---

## 1. The idea in one paragraph

Every workstation holds a complete replica in IndexedDB. Users **always** write locally and are **never** blocked by the network. A background engine drains a local **outbox** to the server (`POST /sync/push`) and fetches changes made elsewhere (`GET /sync/pull`) using a monotonic cursor. Conflicts are prevented where possible (idempotency keys, append-only facts) and resolved predictably where not (last-write-wins with an audit record). **Nothing in this document may be skipped**, because every shortcut here becomes silent data corruption later.

---

## 2. Design principles

| # | Principle | Consequence |
|---|---|---|
| 1 | **Local writes always succeed.** | The user is never shown a network error while recording care. |
| 2 | **The client generates every ID.** | `uuidv7()`. No server round-trip, no temporary IDs, no ID remapping. |
| 3 | **Every operation is idempotent.** | A client-generated `operation_id` makes a retry a no-op. |
| 4 | **Sync is resumable.** | A monotonic `seq` cursor means an interrupted pull resumes exactly where it stopped. |
| 5 | **Conflicts are recorded, never hidden.** | A lost write goes to `sync_conflicts` with both payloads. |
| 6 | **Deletes are tombstones.** | `deleted_at` replicates as a normal update. |
| 7 | **Server time wins for ordering.** | Client clocks are advisory (`device_recorded_at`); `server_rev` and `seq` are authority. |
| 8 | **The client tolerates an unknown future.** | An unrecognised enum value or extra field must not crash the app — an old device will eventually meet a newer server. |
| 9 | **Sync never blocks the UI.** | It runs on a timer and on reconnect; screens never await it. |
| 10 | **A failed sync is visible.** | The status indicator tells the truth. Silent failure is the worst outcome. |

---

## 3. Local database schema (Dexie)

`web/src/db/index.ts`, Dexie version 1.

```ts
import Dexie, { type EntityTable } from 'dexie';

export const db = new Dexie('phc_records') as Dexie & {
  // ---- synced domain tables (mirror the server) ----
  patients:       EntityTable<PatientRow, 'id'>;
  households:     EntityTable<HouseholdRow, 'id'>;
  encounters:     EntityTable<EncounterRow, 'id'>;
  vitals:         EntityTable<VitalsRow, 'id'>;
  // ... one entry per syncable table
  // ---- local-only infrastructure ----
  outbox:         EntityTable<OutboxEntry, 'id'>;
  syncMeta:       EntityTable<SyncMeta, 'key'>;
  conflicts:      EntityTable<LocalConflict, 'id'>;
  numberCounters: EntityTable<NumberCounter, 'key'>;
};

db.version(1).stores({
  patients:   'id, patientNumber, householdId, [surname+firstName], phone, updatedAt, serverRev, syncState',
  households: 'id, familyNumber, familyName, updatedAt, serverRev, syncState',
  encounters: 'id, [workflowStage+priority], patientId, visitNumber, updatedAt, serverRev, syncState',
  vitals:     'id, [patientId+recordedAt], encounterId, updatedAt, serverRev, syncState',
  // ...
  outbox:         'id, status, entityTable, rowId, createdAt, nextAttemptAt, [status+nextAttemptAt]',
  syncMeta:       'key',
  conflicts:      'id, entityTable, rowId, detectedAt',
  numberCounters: 'key',
});
```

**Notes on the index declarations:**

- Dexie requires every index you query on to be declared. Only declare indexes you actually use — each one costs write time and disk.
- `[workflowStage+priority]` is a **compound index**, and it is what makes the queue board query fast. Note that Dexie sorts compound index parts in declaration order, so this index serves `where('[workflowStage+priority]').between([stage, -Infinity], [stage, Infinity])` but **not** a query by `priority` alone. Getting this wrong produces a full table scan that is invisible until the table has 50,000 rows.
- There is **no `facilityId`** in any index. This deployment is single-tenant (`00-ARCHITECTURE.md` §7.4), so a tenant prefix would add a column to every index and buy nothing.
- Dexie's `.sortBy()` loads all matching rows into memory. For the queue board, use `.sortBy('queuedAt')` on an already-narrow result set — never across the whole table.

### 3.1 The infrastructure tables

```ts
export interface OutboxEntry {
  id: string;              // uuidv7 — this is the operation_id
  entityTable: string;     // 'patients', 'payments', ...
  rowId: string;
  operation: 'insert' | 'update' | 'delete';
  payload: unknown;        // the FULL row as it should exist on the server
  baseServerRev: number | null;  // the serverRev this edit was based on; null for insert
  status: 'pending' | 'inflight' | 'failed' | 'done';
  attempts: number;
  lastError: string | null;
  nextAttemptAt: number;   // epoch ms — backoff gate
  createdAt: number;
  deviceRecordedAt: string; // ISO — the client's claimed updatedAt, for skew detection
}

export interface SyncMeta {
  key: string;   // 'pullCursor' | 'lastPushAt' | 'lastPullAt' | 'lastError' | 'deviceId' | 'bootstrapped'
  value: unknown;
}

export interface LocalConflict {
  id: string;
  entityTable: string;
  rowId: string;
  conflictType: 'lww_lost' | 'lww_won' | 'unique_violation' | 'rejected';
  serverRow: unknown | null;
  localRow: unknown | null;
  reason: string;
  detectedAt: number;
  acknowledged: 0 | 1;    // 0/1 rather than boolean so it can be indexed and filtered
}
```

> **Why `status` plus `nextAttemptAt` and not just a timestamp:** Dexie can filter on an index but not on an arbitrary expression cheaply. Declaring `[status+nextAttemptAt]` lets the engine fetch exactly the due operations — `where('[status+nextAttemptAt]').between(['pending', 0], ['pending', Date.now()])` — instead of scanning every row in the outbox. On a busy day the outbox is the hottest table in the app.

---

## 4. The write path

### 4.1 The one helper every write goes through

**This is the most important function in the frontend.** Every clinical mutation in the entire application calls it. If a developer ever calls `db.patients.put(...)` directly, the record is written locally and **never synced** — a bug that is nearly impossible to notice during development because everything looks correct on that one machine.

`web/src/db/write.ts`

```ts
import { db } from './index';
import { uuidv7 } from 'uuidv7';
import type { SyncableTable } from './tables';

export interface LocalWriteOptions {
  /** Existing serverRev when updating; omit for a new row. */
  baseServerRev?: number;
  /**
   * Set false ONLY for genuinely local-only tables (numberCounters, syncMeta).
   * Defaults to true — i.e. everything is queued for sync unless you say otherwise.
   */
  enqueue?: boolean;
}

/**
 * Write a row locally AND enqueue it for sync, atomically.
 *
 * The two writes MUST share one Dexie transaction. If they do not, a tab closed
 * between them loses the record from the server's point of view while the user
 * believes it was saved.
 */
export async function localWrite<T extends { id: string }>(
  tableName: SyncableTable,
  row: T,
  operation: 'insert' | 'update' | 'delete',
  opts: LocalWriteOptions = {},
): Promise<T> {
  const enqueue = opts.enqueue ?? true;

  // uuidv7 is required: it is time-sortable, which keeps IndexedDB and
  // Postgres B-tree inserts at the right-hand edge instead of scattering them.
  const rowId = row.id ?? uuidv7();
  const nowIso = new Date().toISOString();

  const toStore = {
    ...row,
    id: rowId,
    updatedAt: nowIso,
    syncState: 'pending' as const,
  };

  await db.transaction('rw', [db.table(tableName), db.outbox], async () => {
    await db.table(tableName).put(toStore);

    if (enqueue) {
      await db.outbox.add({
        id: uuidv7(),
        entityTable: tableName,
        rowId,
        operation,
        payload: toStore,
        baseServerRev: opts.baseServerRev ?? null,
        status: 'pending',
        attempts: 0,
        lastError: null,
        nextAttemptAt: Date.now(),
        createdAt: Date.now(),
        deviceRecordedAt: nowIso,
      });
    }
  });

  return toStore as T;
}

/** Soft delete — never a hard delete, so the tombstone can replicate. */
export async function localDelete(
  tableName: SyncableTable,
  rowId: string,
  baseServerRev: number,
): Promise<void> {
  const existing = await db.table(tableName).get(rowId);
  if (!existing) return; // idempotent
  await localWrite(
    tableName,
    { ...existing, deletedAt: new Date().toISOString() },
    'delete',
    { baseServerRev },
  );
}
```

### 4.2 Rules for callers

1. **Never** call `db.<table>.put()` / `.add()` / `.delete()` outside this module.
2. The `payload` is the **complete row**, not a diff. The server applies it wholesale for LWW tables, which keeps the server logic simple. (Diffs are a legitimate optimisation later; they are not worth the complexity now.)
3. Set the domain columns (e.g. `recordedAt`, `paidAt`) yourself. `updatedAt` is managed by the helper.
4. When updating, **always pass `baseServerRev`** — taken from the row you loaded. Without it the server cannot detect a concurrent edit and will blind-overwrite.
5. Wrap multi-table logical operations (invoice + charges) in **one** outer Dexie transaction, calling `localWrite` inside it. Dexie nests transactions correctly, so this works — and it is the difference between an invoice with five charges and an invoice with three.

```ts
// Correct: one outer transaction, several rows, one atomic outcome.
await db.transaction('rw', [db.invoices, db.charges, db.outbox], async () => {
  await localWrite('invoices', invoice, 'insert');
  for (const charge of charges) {
    await localWrite('charges', charge, 'insert');
  }
});
```

---

## 5. Push — `POST /sync/push`

### 5.1 Request

```http
POST /api/v1/sync/push
Authorization: Bearer <access_token>
X-Device-Timestamp: 2026-02-14T09:31:07.221Z
X-Device-Nonce: 3q2-7wTkR9mZ1sVbQ0cJx4LpA6nH8yGd
X-Device-Signature: base64url(ECDSA-P256-SHA256(canonicalString))
Content-Type: application/json
```

> There is **no `X-Device-Id` header.** The device is the access token's `device_id` claim, which
> is authenticated. A header would be client-chosen, so a signature could claim to be another
> device — which defeats the binding the signature exists to provide. The nonce is 32 random
> bytes, base64url (43 characters), not hex.

```jsonc
{
  "batchId": "018f3c2a-1111-7abc-8def-000000000001",
  "deviceId": "018f3c2a-....",
  "clientSentAt": "2026-02-14T09:31:07.221Z",
  "schemaVersion": 1,
  "operations": [
    {
      "operationId": "018f3c2a-2222-7abc-8def-000000000002",
      "table": "patients",
      "rowId": "018f3c2a-3333-7abc-8def-000000000003",
      "operation": "insert",
      "baseServerRev": null,
      "deviceRecordedAt": "2026-02-14T09:28:44.000Z",
      "payload": {
        "id": "018f3c2a-3333-7abc-8def-000000000003",
        "patientNumber": "URPHC/PAT/2026/00451",
        "firstName": "Amina",
        "surname": "Bello",
        "gender": "female",
        "dateOfBirth": "1991-04-12",
        "phone": "08031234567",
        "createdAt": "2026-02-14T09:28:44.000Z",
        "updatedAt": "2026-02-14T09:28:44.000Z",
        "createdBy": "018f1a00-....",
        "updatedBy": "018f1a00-....",
        "revision": 1
      }
    }
    // ... up to SYNC_MAX_BATCH_SIZE, default 200
  ]
}
```

**Batching rules**

- Maximum `SYNC_MAX_BATCH_SIZE` operations (default **200**) or **1 MB** of JSON, whichever comes first. Cloud Run request limits and Neon latency both punish large batches, and a partially failed 500-operation batch is a miserable debugging experience.
- Order within a batch **must** preserve causal order: the patient insert before their encounter insert. The server applies operations **sequentially** in array order and stops relying on that only when it can prove independence — it cannot, so just preserve the order.
- One batch contains operations from **one device and one user**. If the session changes, close the batch and start a new one.

### 5.2 Why the request is signed

The body is signed with the device's private key so a stolen access token alone is not enough to inject records. The canonical string is constructed **exactly** as follows, and both sides must agree byte-for-byte (a mismatch produces a signature failure that is painful to debug):

```
CANONICAL = HTTP_METHOD + "\n"
          + REQUEST_PATH + "\n"                    // "/api/v1/sync/push" — no query string
          + DEVICE_ID + "\n"
          + DEVICE_TIMESTAMP + "\n"                // the X-Device-Timestamp header verbatim
          + DEVICE_NONCE + "\n"
          + BASE64URL(SHA256(rawRequestBodyBytes))  // hash the RAW bytes, before JSON parsing
```

Client side:

```ts
// web/src/sync/deviceKey.ts
const CRYPTO_ALGO = { name: 'ECDSA', namedCurve: 'P-256' } as const;
const SIGN_ALGO   = { name: 'ECDSA', hash: 'SHA-256' } as const;

export async function signRequest(
  privateKey: CryptoKey,
  method: string,
  path: string,
  deviceId: string,
  timestamp: string,
  nonce: string,
  rawBody: ArrayBuffer,
): Promise<string> {
  const bodyHash = await crypto.subtle.digest('SHA-256', rawBody);
  const canonical = [
    method, path, deviceId, timestamp, nonce, b64url(new Uint8Array(bodyHash)),
  ].join('\n');

  const sig = await crypto.subtle.sign(
    SIGN_ALGO, privateKey, new TextEncoder().encode(canonical),
  );
  return b64url(new Uint8Array(sig));
}
```

> **Why ECDSA P-256 and not Ed25519:** Ed25519 in WebCrypto is not available in every browser you will meet on an old PHC workstation. P-256 is universally supported by `crypto.subtle`, and Go verifies it with the standard library (`ecdsa.VerifyASN1` for the ASN.1 signature WebCrypto produces — note it is **not** the raw `r||s` form, which is a classic interop trap).

**Replay protection (server):**

- Reject if `|serverNow − deviceTimestamp| > 5 minutes`.
- Reject if the `nonce` was seen for this device within the last 10 minutes (a small in-memory LRU plus a `last_nonce` column on `devices` for the single-instance case; for multiple Cloud Run instances use a short-lived `sync_nonces` table with a TTL sweep — do **not** try to solve this with an in-process map alone once you scale past one instance, and say so in your write-up).
- Reject if the device is revoked (`devices.revoked_at IS NOT NULL`).

### 5.3 Server processing — the algorithm

Execute inside **one database transaction**:

```
1. Authenticate the user from the JWT → user_id and roles.
2. Verify the device:
     - devices.id = the JWT's device_id claim exists and revoked_at IS NULL
     - public_key is registered (not the empty placeholder from login)
     - signature valid for the canonical string
     - timestamp within tolerance; nonce unused
3. Idempotency at batch level:
     INSERT INTO sync_batches (id, ...) — if PK conflict, this batch was already
     processed. Return the STORED result (200) and stop. Do not reprocess.
4. For each operation, in array order:
     a. If sync_operations.id already exists → result = 'duplicate', skip.
     b. Look up sync_table_scopes[table] to get push_roles and conflict_policy.
        If the user's role is not permitted → result = 'rejected', reason = 'forbidden'.
     c. Validate the payload against the server-side Zod-equivalent schema for
        that table. Failure → 'rejected', reason = 'validation:<field>'.
     d. Apply per the conflict policy (section 6).
     e. Assign server_rev, write the row, and append to sync_change_log.
     f. Insert a sync_operations row with the result.
5. Grant privileges: the client never sets server_rev, sync_state or updated_by
   for audit purposes — the server overrides updated_by with the session user
   if they disagree.
6. UPDATE devices SET last_seen_at = now(), last_sync_cursor = <max seq written>.
7. Respond with the per-operation results and the new cursor.
```

> **Why there is no facility check in the loop above.** An earlier draft of this design included a `facility_mismatch` rejection, defending against one PHC writing into another's records. This deployment is single-tenant (`00-ARCHITECTURE.md` §7.4) — there is one health centre, one database, and no tenant column to compare against — so the check has nothing to verify and has been removed rather than left as dead code.

> **Point 4e is the heart of the pull mechanism.** Every accepted write appends to `sync_change_log`. If you insert a row without appending to the change log, other devices will **never** see it, and the bug looks like "sometimes records don't appear on the other computer", which is a nightmare to diagnose. Consider enforcing it with a trigger, or at minimum wrapping both writes in a helper function that makes the pairing impossible to forget.

### 5.4 Response

```jsonc
{
  "batchId": "018f3c2a-1111-7abc-8def-000000000001",
  "serverTime": "2026-02-14T09:31:07.900Z",
  "results": [
    {
      "operationId": "018f3c2a-2222-7abc-8def-000000000002",
      "status": "applied",
      "rowId": "018f3c2a-3333-7abc-8def-000000000003",
      "serverRev": 1
    },
    {
      "operationId": "018f3c2a-4444-7abc-8def-000000000004",
      "status": "conflict_lww",
      "rowId": "018f3c2a-5555-7abc-8def-000000000005",
      "serverRev": 7,
      "reason": "server_copy_newer",
      "keptServerVersion": true,
      "conflictId": "018f3c2a-6666-7abc-8def-000000000006"
    }
  ],
  "summary": { "applied": 1, "duplicate": 0, "conflictLww": 1, "rejected": 0 },
  "nextCursor": 10432,
  "clientClockSkewSeconds": -42
}
```

**Per-operation statuses — the client must handle all six:**

| Status | Meaning | Client action |
|---|---|---|
| `applied` | Written, `serverRev` assigned | Mark outbox `done`; record `serverRev` on the local row |
| `duplicate` | Already applied earlier | Mark `done` — the work is done, treat as success |
| `conflict_lww` + `keptServerVersion: true` | Server's copy won | Overwrite the local row from the returned server state; log to `db.conflicts`; if a human opened this record, show a toast |
| `conflict_lww` + `keptServerVersion: false` | Client's copy won but overwrote another device | Mark `done`; log to `db.conflicts` for the record |
| `rejected` | Permanent failure (forbidden, validation, stale reference) | Mark `failed`; **never retry**; park it in `db.conflicts` and surface it to the user — a rejected operation must not silently vanish |
| `error` | Transient server failure | Retry with backoff |

> **`rejected` is the status students get wrong.** Retrying a permanently rejected operation forever produces a device that hammers the server and never drains. Worse, if you drop it silently, a nurse's vitals reading disappears with no trace. Neither is acceptable: park it, show it, let a human act.

### 5.5 The push loop (client)

```ts
// web/src/sync/pusher.ts
const BASE_DELAY_MS = 1_000;
const MAX_DELAY_MS  = 5 * 60_000;
const MAX_BATCH     = 200;

export async function drainOutbox(): Promise<void> {
  if (!navigator.onLine) return;
  if (isSyncing) return;              // single-flight guard: no concurrent pushes
  isSyncing = true;

  try {
    for (;;) {
      const due = await db.outbox
        .where('[status+nextAttemptAt]')
        .between(['pending', 0], ['pending', Date.now()])
        .limit(MAX_BATCH)
        .toArray();

      if (due.length === 0) break;

      // Mark inflight BEFORE the request so a crash mid-flight does not
      // double-send and does not lose the entries either.
      await db.outbox.bulkUpdate(
        due.map(o => ({ key: o.id, changes: { status: 'inflight' as const } })),
      );

      try {
        const response = await pushBatch(due);
        await applyPushResults(due, response);
      } catch (err) {
        // Network/5xx: return to pending with exponential backoff + jitter.
        await db.outbox.bulkUpdate(
          due.map(o => ({
            key: o.id,
            changes: {
              status: 'pending' as const,
              attempts: o.attempts + 1,
              lastError: String(err),
              nextAttemptAt: Date.now() + backoff(o.attempts),
            },
          })),
        );
        throw err;
      }
    }
    await db.syncMeta.put({ key: 'lastPushAt', value: Date.now() });
  } finally {
    isSyncing = false;
  }
}

/** Exponential backoff with full jitter — avoids a thundering herd of
 *  workstations all retrying in lockstep after the internet returns. */
function backoff(attempts: number): number {
  const capped = Math.min(BASE_DELAY_MS * 2 ** attempts, MAX_DELAY_MS);
  return Math.random() * capped;
}
```

**The single-flight guard is not optional.** Without it, a reconnect event, a timer tick and a user action can start three concurrent drains, and you will send the same operations three times. Idempotency saves your data, but the wasted traffic and confusing logs are avoidable.

### 5.6 When the pusher runs

| Trigger | Notes |
|---|---|
| App start, after the session is restored | Catches up on everything queued while closed |
| `window.addEventListener('online', ...)` | The fastest path back to sync |
| Every 60 seconds while online and the outbox is non-empty | Steady-state drain |
| Immediately after a write, debounced ~2 s | Keeps the pending count near zero in normal use |
| Manual "Sync now" from the status popover | Gives the user agency and makes your demo easy |

**Do not** push after *every* individual write with no debounce. A cashier taking five payments in a row would fire five HTTP round-trips in a second, and each one competes for the same connection.

---

## 6. Conflict resolution

`sync_table_scopes.conflict_policy` declares one of two policies per table. **The policy is data, not code** — adding a table requires no engine change.

### 6.1 `append_only`

Applies to: `vitals`, `lab_results`, `dispenses`, `charges`, `payments`, `payment_allocations`, `ward_charges`, `admission_notes`, `bed_assignments`, `diagnoses`.

**Rule:** an insert always succeeds. An update is only permitted for a whitelist of lifecycle fields, and a delete is only permitted within the same working day (`deleted_at <= created_at + 1 day`) to allow the correction of an obvious mistake.

Two nurses recording a BP on two offline devices produce **two rows**. That is correct — they are two observations. The consultation screen shows the latest by `recorded_at` and lists the rest beneath it.

Updates to append-only tables are restricted because letting a client rewrite a payment is exactly the attack you want to prevent. Permitted update columns per table are declared in `sync_table_scopes.notes` as a comma-separated list, and anything else is `rejected`.

### 6.2 `lww` — last write wins

Applies to: `patients`, `households`, `encounters`, `users`, `user_roles`, `devices`, `drug_products`, `tariffs`, `invoices`, `lab_orders`, `prescriptions`, `prescription_items`, `admissions`, `referrals`.

```
if operation.baseServerRev is NULL:
      # client says "this is new"
      if the row already exists on the server → treat as an update, fall through to LWW
      else → INSERT with server_rev = 1

elif operation.baseServerRev == row.server_rev:
      # no concurrent edit — the common case, fast path
      APPLY, server_rev = row.server_rev + 1

else:
      # concurrent edit detected
      if client_updated_at > server_updated_at:
          APPLY; server_rev = row.server_rev + 1
          log conflict (winner = client), keep losing server payload
          result = 'conflict_lww', keptServerVersion = false
      else:
          DO NOT APPLY
          log conflict (winner = server), keep losing client payload
          result = 'conflict_lww', keptServerVersion = true
```

**The tiebreaker is `updated_at` from the payload, then `deviceRecordedAt`, then `server_rev`** — in that order. Record every comparison in `sync_conflicts` with **both** payloads. Storage is cheap; a lost clinical edit is not.

**Milestone-level refinement (recommended, and worth doing):** for `patients`, do not apply the whole row. Apply only the fields whose `updated_at` is newer than the server row's. This is *per-field* LWW, and it is what your agreed policy actually says. Implement it as a field mask:

```go
// Only fields the client actually changed move across.
changed := diffFields(existing, incoming)   // compares against baseServerRev snapshot
for _, f := range changed {
    if incoming.UpdatedAt.After(existing.UpdatedAt) {
        apply(existing, f, incoming)
    }
}
```

To make this possible the client should send the row **as it was when loaded** in `baseServerRevSnapshot`, or simply accept whole-row LWW in Sprint 2 and upgrade to per-field in Sprint 9. **Recommendation: whole-row in Sprint 2** so the sync engine works end-to-end early, and per-field as a clearly-scoped Sprint 9 improvement. Say exactly that in your report — an honest account of the trade-off reads far better than an unfinished attempt at the harder version.

### 6.3 Unique-constraint violations

Some operations can fail on a unique index: a duplicate `patient_number`, a double-booked bed (`uq_bed_assignments_occupied`), two active encounters for one patient.

```
ON unique_violation:
    INSERT INTO sync_conflicts (conflict_type = 'unique_violation', ...)
    result = 'rejected', reason = 'unique_violation:<constraint_name>'
```

The client must then offer a resolution path. For numbers, **reassign and retry automatically** — pull a fresh number from the device-local counter namespace, update the row, and re-enqueue. For beds, show the nurse the conflicting assignment.

**Never** silently discard. A rejected insert is a patient who was registered and now does not exist.

> **Idempotency exception:** if the operation is a re-insert of a row that already exists **with the same `operation_id`**, it is a `duplicate`, not a conflict. Check `sync_operations` first, always.

### 6.4 Stale references

A row can reference a parent the client has never seen — a charge pointing at an invoice that exists on the server but was never pulled, because the client has not pulled since. Because the client generates IDs, this is rare but real when a device has been offline for a long time.

**Server response:** `rejected`, `reason = 'stale_reference'`, listing the missing parent ID. The client's recovery is to **force a pull and retry once** before parking the operation as failed. Implement this recovery explicitly; do not let it loop.

---

## 7. Pull — `GET /sync/pull`

### 7.1 Request

```http
GET /api/v1/sync/pull?since=10234&limit=500
Authorization: Bearer <access_token>
```

- `since` — the last `seq` the client successfully applied. Omit or `0` for the initial bootstrap.
- `limit` — max change-log rows to return (default 500, max 2000).

### 7.2 Response

```jsonc
{
  "changes": [
    { "seq": 10235, "table": "patients",  "rowId": "...", "operation": "upsert", "serverRev": 12 },
    { "seq": 10236, "table": "vitals",    "rowId": "...", "operation": "upsert", "serverRev": 3  },
    { "seq": 10237, "table": "households","rowId": "...", "operation": "delete", "serverRev": 8  }
  ],
  "rows": {
    "patients":   [ { "id": "...", "firstName": "Amina", "syncState": "synced", "serverRev": 12, "...": "..." } ],
    "vitals":     [ { "id": "...", "bpSystolic": 118, "...": "..." } ]
  },
  "nextCursor": 10237,
  "hasMore": true,
  "fullResyncRequired": false,
  "serverTime": "2026-02-14T09:33:01.000Z",
  "schemaVersion": 1
}
```

Note the **split between `changes` and `rows`**: `changes` is the ordered manifest (and it is what advances the cursor), while `rows` carries each distinct current row **once**, however many times it changed. A record edited five times in one window appears as five `changes` entries but one row — this keeps the payload small without losing ordering.

### 7.3 The cursor — how to get this right

```sql
-- One statement, one snapshot. REPEATABLE READ is what makes it consistent.
BEGIN ISOLATION LEVEL REPEATABLE READ;

SELECT seq, table_name, row_id, operation, server_rev
FROM sync_change_log
WHERE seq > $1
ORDER BY seq
LIMIT $2;

-- nextCursor = the seq of the LAST row returned (not max(seq) of the table)
COMMIT;
```

**Two mistakes to avoid, both of which cause permanently missing records:**

1. **Using `max(seq)` instead of the last returned `seq`.** If a write lands while you are reading, `max(seq)` skips it forever.
2. **Querying outside a transaction.** A row could be committed with `seq = 10240` while rows `10238` and `10239` are still uncommitted; your query sees `10240` and advances past the gap. `seq` values are assigned by a sequence and are **not gap-free**. The cursor is therefore "the last `seq` I have *seen*", not "everything with a smaller `seq` is applied" — and `REPEATABLE READ` is what makes that distinction safe.

The client applies the changes then stores `nextCursor` in `db.syncMeta`. Only after the local writes commit — never before:

```ts
await db.transaction('rw', [...tables, db.syncMeta], async () => {
  await applyRows(response.rows);
  await db.syncMeta.put({ key: 'pullCursor', value: response.nextCursor });
});
```

If the tab closes between these two, the cursor is stale (safe, some rows are re-applied — which is why applying an upsert twice must be harmless) rather than advanced (unsafe, rows are lost forever). **Always bias toward re-applying.**

### 7.4 Server-side pull filtering

The pull is scoped in two ways:

1. **Table scope** — the change log rows are filtered against `sync_table_scopes.pull_roles` for the caller's roles. A `lab_scientist` does not receive `payments`.
2. **Row filters** — a small per-table predicate where needed. The only one required today: `users` rows always have `password_hash`, `failed_login_count` and `locked_until` stripped by the response builder.

There is **no facility filter**. In a single-tenant deployment there is exactly one organisation's data in the database, and scoping by a tenant column that does not exist would be meaningless.

**Redaction happens in exactly one place** — the pull response builder. Do not scatter `if table == "users"` checks through the sync package; that is how a hash eventually leaks.

### 7.5 Initial bootstrap

A brand-new device has `pullCursor = 0`, which would mean replaying the entire change log. For a clinic with a year of history, that is enormous and duplicative. Bootstrap therefore uses a **snapshot** endpoint:

```http
GET /api/v1/sync/snapshot?table=patients&after=018f...&limit=500
```

Returns the current state of one table, keyset-paginated by `id` (not `OFFSET`, which degrades badly). The client loops until `hasMore: false`, then sets `pullCursor` to the current maximum `seq` **captured before the snapshot began**, which the endpoint returns as `cursorAtStart`.

**Bootstrap order — parents before children**, so foreign keys resolve:

```
facilities → users → user_roles → devices → households → patients →
encounters → vitals → drug_products → tariffs → diagnoses →
lab_orders → lab_results → prescriptions → prescription_items →
invoices → charges → payments → payment_allocations → dispenses →
admissions → bed_assignments → ward_charges → admission_notes → referrals
```

Two practical notes:

- **Do not declare Dexie foreign keys.** Dexie has no real FK support; use indexes only and rely on the server for referential integrity. This removes the ordering problem as a source of hard failures and lets bootstrap be more forgiving.
- Show a real progress indicator during bootstrap. It is the one moment a user genuinely waits, and on a slow link it is the moment they decide whether to trust the application.

### 7.6 Long-offline devices and `fullResyncRequired`

`sync_change_log` is pruned on a retention window (default 30 days). A device whose cursor is older than the oldest retained row **cannot** catch up incrementally. The server must detect this and refuse to pretend otherwise:

```jsonc
{ "fullResyncRequired": true, "reason": "cursor_expired", "oldestAvailableSeq": 88410 }
```

The client then wipes its replica and bootstraps from scratch. **Handle this before you prune the log.** A device offline for two months that silently applies a partial delta is corrupted in a way nobody will notice until a patient's history is missing — and the client must **never** push queued operations against a replica it is about to wipe, or those operations reference parents that no longer exist locally. Order of operations: stop sync → drain the outbox → wipe → bootstrap → resume.

---

## 8. Sync orchestration

```ts
// web/src/sync/engine.ts
export async function runSyncCycle(): Promise<SyncResult> {
  if (!navigator.onLine) return { skipped: 'offline' };

  // 1. PUSH FIRST. Local truth goes up before remote truth comes down.
  //    This ordering matters: if we pulled first, a local edit based on an
  //    older serverRev would be marked conflicted unnecessarily.
  await drainOutbox();

  // 2. THEN PULL. Apply everything that changed elsewhere.
  await pullChanges();

  return { ok: true };
}
```

**Why push-before-pull:** pulling first updates local rows and their `serverRev` values, which would make the in-flight outbox entries' `baseServerRev` values stale — manufacturing conflicts that never actually happened. Push first, and most conflicts evaporate.

### 8.1 The status machine

Exposed through a Zustand store; the UI reads it (see `03-UI-PATTERNS.md` §5).

```ts
export type SyncStatus =
  | 'synced'     // outbox empty, last cycle succeeded
  | 'syncing'    // a cycle is in flight
  | 'pending'    // N operations queued, online, not yet drained
  | 'offline'    // no connection — NOT an error
  | 'error'      // last cycle failed; needs attention
  | 'blocked';   // offline grace period expired; re-authentication required
```

```ts
export const useSyncStore = create<SyncState>((set) => ({
  status: 'synced',
  pendingCount: 0,
  lastSyncedAt: null,
  lastError: null,
  conflictCount: 0,
  setStatus: (status) => set({ status }),
  // ...
}));
```

`pendingCount` is kept live from Dexie:

```ts
useLiveQuery(() => db.outbox.where('status').anyOf('pending', 'inflight').count())
  .then(count => useSyncStore.setState({ pendingCount: count ?? 0 }));
```

---

## 9. Edge cases — the checklist you will be glad you read

| # | Case | Required behaviour |
|---|---|---|
| 1 | Tab closed mid-push | Entries are `inflight`, not lost. On next start, reset any `inflight` older than 2 minutes back to `pending` — the server's idempotency makes the resend safe. |
| 2 | Same record edited on two devices | LWW per §6.2; conflict recorded with both payloads. |
| 3 | Device clock wrong by days | Server rejects a signature timestamp beyond ±5 min with `clock_skew`; the client shows "This device's clock is incorrect — please fix it" and **blocks sync** until corrected, because an unfixable clock silently corrupts ordering. |
| 4 | Two devices pick the same receipt number | Unique violation → auto-reassign from the device-scoped counter and retry (§6.3). |
| 5 | Same bed assigned twice offline | Unique violation on `uq_bed_assignments_occupied`; nurse sees a conflict. |
| 6 | Patient registered twice offline | Both rows insert (different IDs). Duplicate detection is a **separate, best-effort feature**: the server flags probable duplicates via a fuzzy name + DOB + phone match into a `duplicate_candidates` view for the front desk to merge, later. Do not try to prevent it — offline devices cannot see each other. |
| 7 | Payment pushed twice | Prevented at three levels: `sync_operations` idempotency, `uq_payments_operation`, and the client's `operation_id`. |
| 8 | Payment recorded, patient leaves, receipt must print offline | The receipt prints from the local row immediately. Number comes from the device-scoped counter, so it is globally unique. |
| 9 | Device offline beyond the grace window | Behaviour is governed by `OFFLINE_POLICY` (`00-ARCHITECTURE.md` §5.1). Under the default `block_writes`: status becomes `blocked`, **read-only** browsing continues, new writes are refused with a clear explanation, and an online re-auth on any device unlocks it. Under `warn_only`: writes continue with a persistent warning. **Default is `block_writes`; the window is `OFFLINE_GRACE_PERIOD` (default 168h).** |
| 10 | Sync mid-shift causes a record to appear twice on screen | Almost always a `useLiveQuery` keyed on a stale array, or the same row written under two IDs. Check ID generation first. |
| 11 | A migration changes a table's columns | Bump the Dexie version **and** the `schemaVersion` in the sync payload. An old client receiving unknown fields must ignore them; a new client receiving missing fields must apply defaults. Never break a client that is one version behind — a PHC cannot pause admissions to update a browser. |
| 12 | Outbox grows unboundedly | Cap it. If `pendingCount > 5000`, surface a prominent warning and pause non-essential writes; a device this far behind will not realistically catch up, and a full resync is the honest remedy. |
| 13 | User logs out with pending operations | **Refuse to log out** while the outbox is non-empty, unless the user explicitly chooses "log out and discard N unsynced records" after being told what will be lost. Silent data loss on logout is unacceptable. |
| 14 | Two tabs open on one workstation | Dexie's `BroadcastChannel` (or its `storagemutated` events) keeps live queries in sync between tabs. Test this explicitly — it is a real PHC usage pattern. |
| 15 | Service worker serves a stale shell after deploy | Version the payload, show a "new version available — reload" prompt, and let the server accept the older `schemaVersion` (case 11). |

---

## 10. Diagnostics and observability

Give yourself these from Sprint 2, before you need them:

- **Sync log ring buffer** — the last 200 push/pull attempts in memory, viewable at `/admin/sync-log`, with counts, durations and errors. This is how you will debug in Sprint 6 what you broke in Sprint 3.
- **A `/admin/devices` health screen** backed by `device_sync_state`: last push, last pull, pending count, last error, app version. It answers "is reception's computer syncing?" in one glance.
- **A `/admin/conflicts` screen** fed by `sync_conflicts`, so overwrites are visible rather than theoretical.
- **Server-side structured logs** with `requestId`, `deviceId`, `userId`, `batchId`, and per-operation outcome counts. Never log payloads containing patient data.
- **One metric that matters**: sync cycle duration p95. When it degrades, something is wrong with your queries.

---

## 11. Testing the sync engine

This is the part of the project that most deserves real tests, because manual testing cannot reproduce the interesting cases.

**Unit (Vitest + `fake-indexeddb`):**

- `localWrite` enqueues exactly one outbox entry per call, and both writes land in one transaction (test by forcing a throw on the outbox add and asserting the row is absent).
- Backoff produces increasing delays and never exceeds the cap.
- The conflict decision function: given `(baseServerRev, serverRev, clientUpdatedAt, serverUpdatedAt)`, returns the correct winner for all six combinations.
- Cursor advancement only happens after rows are applied.

**Integration (Go, `testcontainers` or a disposable Neon branch):**

- The same operation pushed twice → one row, first `applied`, second `duplicate`.
- Two clients editing the same patient → exactly one `conflict_lww` result and one `sync_conflicts` row.
- Pushing with an expired JWT → 401; with a revoked device → 403.
- A push referencing a parent row the server has never seen → `rejected:stale_reference`, and **no** row written.
- Pull with `since` older than the retention window → `fullResyncRequired: true`.

**End-to-end (Playwright) — the four scenarios that prove the thesis:**

1. **Two browsers, one offline.** Browser A registers a patient while B is offline; A syncs; B comes online, syncs, and sees the patient. *This is the headline demo.*
2. **Conflict.** Both offline, both edit the same patient's phone number, both come online. Assert exactly one wins and the conflict is visible at `/admin/conflicts`.
3. **Idempotency.** Intercept the push request with Playwright's route handler and abort it *after* the server processed it (or simply repeat the same batch body twice). Assert only one payment row exists.
4. **Offline soak.** `context.setOffline(true)`, run a full clinical flow — register, vitals, queue, consult, lab order, pay, dispense — then restore the network and assert every record arrives and the derived balance is correct.

Scenario 4 is the one you run in your defence. Practise it until it is boring.
