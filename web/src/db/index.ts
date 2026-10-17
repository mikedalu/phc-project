import Dexie, { type EntityTable } from 'dexie';
import type {
  DeviceKeyRow,
  EncounterRow,
  HouseholdRow,
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
