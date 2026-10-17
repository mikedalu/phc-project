import Dexie, { type EntityTable } from 'dexie';
import type {
  DeviceKeyRow,
  DiagnosisRow,
  AllocationRow,
  ChargeRow,
  EncounterRow,
  InvoiceRow,
  PaymentRow,
  HouseholdRow,
  LabOrderRow,
  LabResultRow,
  LocalConflict,
  NumberCounter,
  OfflineCredentialRow,
  OutboxEntry,
  PatientRow,
  SessionRow,
  SyncMeta,
  SyncProbeRow,
  VitalsRow,
} from './types';

/**
 * The local database. In this architecture it is not a cache — it is the
 * primary store. Every screen reads from here and every clinical write lands
 * here first. See docs/00-ARCHITECTURE.md §4.
 *
 * ═══ INDEX DECLARATION RULES ═══════════════════════════════════════════
 * Dexie requires every index you query on to be declared here, and only
 * indexes you actually use — each one costs write time and disk.
 *
 * There is deliberately NO facilityId in any index. This deployment is
 * single-tenant: one installation, one health centre (00-ARCHITECTURE.md §7.4),
 * so a tenant prefix would add a column to every index and buy nothing.
 *
 * Compound-index ordering matters. `[workflowStage+priority]` serves a query
 * that constrains workflowStage; it does NOT serve a query by priority alone.
 * Getting this wrong produces a full-table scan that stays invisible until the
 * table has tens of thousands of rows.
 *
 * ═══ SCHEMA VERSIONS ARE APPEND-ONLY ═══════════════════════════════════
 * To add a table or index, add a NEW `db.version(n)` block below and leave the
 * existing ones untouched. Never edit a released version block — it silently
 * corrupts the database of any browser that already upgraded.
 */
export const db = new Dexie('phc_records') as Dexie & {
  // ── synced domain tables ────────────────────────────────────────────
  patients: EntityTable<PatientRow, 'id'>;
  households: EntityTable<HouseholdRow, 'id'>;
  encounters: EntityTable<EncounterRow, 'id'>;
  vitals: EntityTable<VitalsRow, 'id'>;
  diagnoses: EntityTable<DiagnosisRow, 'id'>;
  lab_orders: EntityTable<LabOrderRow, 'id'>;
  lab_results: EntityTable<LabResultRow, 'id'>;
  invoices: EntityTable<InvoiceRow, 'id'>;
  charges: EntityTable<ChargeRow, 'id'>;
  payments: EntityTable<PaymentRow, 'id'>;
  payment_allocations: EntityTable<AllocationRow, 'id'>;

  // ── local-only infrastructure (never synced) ────────────────────────
  outbox: EntityTable<OutboxEntry, 'id'>;
  syncMeta: EntityTable<SyncMeta, 'key'>;
  conflicts: EntityTable<LocalConflict, 'id'>;
  numberCounters: EntityTable<NumberCounter, 'key'>;
  deviceKeys: EntityTable<DeviceKeyRow, 'id'>;
  session: EntityTable<SessionRow, 'key'>;
  localCredentials: EntityTable<OfflineCredentialRow, 'id'>;
  sync_probe: EntityTable<SyncProbeRow, 'id'>;
};

db.version(1).stores({
  patients: [
    'id',
    'patientNumber',
    'householdId',
    '[surname+firstName]',
    'phone',
    'updatedAt',
    'serverRev',
    'syncState',
  ].join(', '),

  households: 'id, familyNumber, familyName, updatedAt, serverRev, syncState',

  encounters: [
    'id',
    '[workflowStage+priority]',
    'patientId',
    'visitNumber',
    'queuedAt',
    'updatedAt',
    'serverRev',
    'syncState',
  ].join(', '),

  vitals: 'id, [patientId+recordedAt], encounterId, updatedAt, serverRev, syncState',

  // [status+nextAttemptAt] lets the drain query fetch exactly the operations
  // that are due, instead of scanning the whole outbox. On a busy clinic day
  // this is the hottest table in the application.
  outbox: 'id, status, entityTable, rowId, [status+nextAttemptAt], createdAt',

  syncMeta: 'key',
  conflicts: 'id, entityTable, rowId, detectedAt, acknowledged',
  numberCounters: 'key',
});

/**
 * v2 — device identity.
 *
 * Dexie applies only the tables NAMED in a version block; everything declared in
 * v1 carries forward untouched. Re-listing the v1 tables here is the usual
 * mistake and the usual way an index silently disappears — name only what
 * changes.
 */
db.version(2).stores({
  deviceKeys: 'id, deviceId',
});

/**
 * v3 — the signed-in session.
 *
 * Keyed by the constant 'current': one browser profile has one active session at
 * a time. The refresh token is stored here because offline operation needs it —
 * see the note on SessionRow for why the ACCESS token is not.
 */
db.version(3).stores({
  session: 'key',
});

/**
 * v4 — the offline sign-in verifier.
 *
 * Kept OUT of the `session` row on purpose. Logging out deletes the session;
 * the verifier must survive, or "sign out and back in while offline" — the exact
 * flow a shared workstation needs — would be impossible.
 */
db.version(4).stores({
  localCredentials: 'id, userId',
});

/**
 * v5 — the development probe table.
 *
 * A real synced table, wired exactly like the clinical tables will be: written
 * through `localWrite`, drained by the pusher, applied by the puller. If the
 * engine works here, it works for `patients`.
 */
db.version(5).stores({
  sync_probe: 'id, updatedAt, serverRev, syncState',
});

/**
 * v6 — the patient registry and households.
 *
 * Note what is NOT here: no Dexie foreign keys. Dexie has no real FK support,
 * and pretending otherwise creates ordering constraints during bootstrap that
 * the server already enforces. Indexes only.
 */
/**
 * v7 — the clinical workflow.
 *
 * The queue is NOT a separate store: an encounter IS the queue entry, exactly as
 * on the server. Two rows for one visit would be two things to keep in step.
 */
/**
 * v8 — the consultation's output: diagnoses and lab orders.
 *
 * Diagnoses are append-only on the server, so nothing here is ever updated; the
 * index exists to read a patient's history without scanning.
 */
/**
 * v9 — lab results.
 *
 * Append-only like vitals: a corrected value is a NEW row, so this store is
 * never updated after insert and never needs a mutable index.
 */
/**
 * v10 — money.
 *
 * Balances are NOT stored here either. They are computed from charges minus
 * allocations on every render, for the same reason as on the server: a stored
 * balance is right until two devices write at once.
 */
db.version(10).stores({
  invoices: 'id, patientId, householdId, encounterId, status, issuedAt, serverRev, syncState',
  charges: 'id, invoiceId, patientId, householdId, status, serverRev, syncState',
  payments: 'id, operationId, receiptNumber, householdId, paidAt, serverRev, syncState',
  payment_allocations: 'id, paymentId, chargeId, serverRev, syncState',
});

db.version(9).stores({
  lab_results: 'id, labOrderId, patientId, resultAt, serverRev, syncState',
});

db.version(8).stores({
  diagnoses: 'id, patientId, encounterId, diagnosedAt, serverRev, syncState',
  lab_orders: 'id, patientId, encounterId, orderNumber, status, updatedAt, serverRev, syncState',
});

db.version(7).stores({
  encounters: 'id, patientId, workflowStage, status, priority, queuedAt, updatedAt, serverRev, syncState',
  vitals: 'id, patientId, encounterId, recordedAt, updatedAt, serverRev, syncState',
});

db.version(6).stores({
  patients: [
    'id',
    'patientNumber',
    'householdId',
    '[surname+firstName]',
    'phone',
    'updatedAt',
    'serverRev',
    'syncState',
  ].join(', '),
  households: 'id, familyNumber, familyName, updatedAt, serverRev, syncState',
});

/**
 * Opens the database.
 *
 * Call once during app bootstrap and await it before rendering screens that
 * read data — an unopened Dexie database throws on first query, and the error
 * surfaces far from its cause.
 */
export async function openDatabase(): Promise<void> {
  if (!db.isOpen()) {
    await db.open();
  }
}

/**
 * True on the very first run in this browser profile.
 *
 * Used to decide whether a full bootstrap from /sync/snapshot is required.
 * A device with no cursor has never synced.
 */
export async function isFirstRun(): Promise<boolean> {
  const cursor = await db.syncMeta.get('pullCursor');
  return cursor === undefined;
}
