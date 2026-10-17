import { db } from '@/db';
import type {
  DiagnosisRow,
  EncounterRow,
  HouseholdRow,
  LabOrderRow,
  LabResultRow,
  PatientRow,
  SyncProbeRow,
  VitalsRow,
} from '@/db/types';
import { apiGet } from '@/lib/api';
import { syncedTableHandles } from './tables';

/**
 * The pull loop: server changes → local replica.
 *
 * ═══ THE ONE RULE ═══
 *
 * The cursor is committed INSIDE the same Dexie transaction that applies the
 * rows, and only after they are written. If the tab closes between the two, the
 * cursor is stale and some rows are re-applied — which is safe, because applying
 * an upsert twice must be harmless. Advancing first would lose rows for ever.
 *
 * Bias always toward re-applying.
 */

interface PullChange {
  seq: number;
  table: string;
  rowId: string;
  operation: 'upsert' | 'delete';
  serverRev: number;
}

interface PullResponse {
  changes: PullChange[];
  rows: Record<string, Record<string, unknown>[]>;
  nextCursor: number;
  hasMore: boolean;
  fullResyncRequired: boolean;
  reason?: string;
  oldestAvailableSeq?: number;
  serverTime: string;
}

export interface PullOutcome {
  applied: number;
  hasMore: boolean;
  needsResync: boolean;
}

export async function pullOnce(): Promise<PullOutcome> {
  const cursorEntry = await db.syncMeta.get('pullCursor');
  const since = typeof cursorEntry?.value === 'number' ? cursorEntry.value : 0;

  const { data } = await apiGet<PullResponse>(`/sync/pull?since=${since}`);

  if (data.fullResyncRequired) {
    await db.syncMeta.put({ key: 'fullResyncRequired', value: true });
    await db.syncMeta.put({ key: 'lastError', value: data.reason ?? 'resync_required' });
    return { applied: 0, hasMore: false, needsResync: true };
  }

  let applied = 0;

  // EVERY synced table must be in the transaction scope, and every table needs
  // an adapter. Hard-coding the probe here is how a patient could be pushed,
  // acknowledged by the server, and then never appear on the second device —
  // the pull simply skipped it.
  await db.transaction('rw', [...syncedTableHandles(), db.syncMeta], async () => {
    for (const [table, rows] of Object.entries(data.rows)) {
      const adapter = ADAPTERS[table];
      if (!adapter) continue;

      for (const raw of rows) {
        const incoming = adapter.adapt(raw);
        const local = await adapter.table.get(incoming.id);

        // A row with local edits queued must not be clobbered by an older
        // server copy: our push will resolve it through LWW and come back with
        // the authoritative result.
        if (local && local.syncState === 'pending') continue;

        // Never move a row backwards. Out-of-order re-application is possible
        // when a cursor is stale, and it must be harmless.
        if (local && local.serverRev > incoming.serverRev) continue;

        await adapter.table.put(incoming);
        applied += 1;
      }
    }

    // Cursor LAST, in the same transaction.
    await db.syncMeta.put({ key: 'pullCursor', value: data.nextCursor });
    await db.syncMeta.put({ key: 'lastPullAt', value: Date.now() });
  });

  return { applied, hasMore: data.hasMore, needsResync: false };
}

export interface RowLike {
  id: string;
  serverRev: number;
  syncState: 'pending' | 'synced' | 'conflict';
}

/**
 * One adapter per synced table.
 *
 * `table` is the Dexie handle, so the loop can read the local row and decide
 * whether the incoming one is newer without a switch on the table name.
 */
interface RowStore {
  get: (id: string) => Promise<RowLike | undefined>;
  put: (row: RowLike) => Promise<unknown>;
}

const ADAPTERS: Record<string, { table: RowStore; adapt: (raw: Record<string, unknown>) => RowLike }> = {
  // The cast is contained here: the loop only needs `id`, `serverRev` and
  // `syncState`, and each table's real type is enforced by its own adapter.
  sync_probe: { table: db.sync_probe as unknown as RowStore, adapt: toProbeRow },
  patients: { table: db.patients as unknown as RowStore, adapt: toPatientRow },
  households: { table: db.households as unknown as RowStore, adapt: toHouseholdRow },
  encounters: { table: db.encounters as unknown as RowStore, adapt: toEncounterRow },
  lab_orders: { table: db.lab_orders as unknown as RowStore, adapt: toLabOrderRow },
  lab_results: { table: db.lab_results as unknown as RowStore, adapt: toLabResultRow },
  diagnoses: { table: db.diagnoses as unknown as RowStore, adapt: toDiagnosisRow },
  vitals: { table: db.vitals as unknown as RowStore, adapt: toVitalsRow },
};

/**
 * Server rows arrive in the camelCase wire shape, which is exactly the local
 * row shape apart from the sync state. A cast is honest here: the server is the
 * authority for these columns, and inventing defaults for a missing one would
 * hide a contract mismatch.
 */
function toSynced<T extends RowLike>(raw: Record<string, unknown>): T {
  return {
    ...(raw as unknown as T),
    syncState: 'synced',
  };
}

export function toPatientRow(raw: Record<string, unknown>): PatientRow {
  return toSynced<PatientRow>(raw);
}

export function toHouseholdRow(raw: Record<string, unknown>): HouseholdRow {
  return toSynced<HouseholdRow>(raw);
}

export function toEncounterRow(raw: Record<string, unknown>): EncounterRow {
  return toSynced<EncounterRow>(raw);
}

export function toVitalsRow(raw: Record<string, unknown>): VitalsRow {
  return toSynced<VitalsRow>(raw);
}

export function toDiagnosisRow(raw: Record<string, unknown>): DiagnosisRow {
  return toSynced<DiagnosisRow>(raw);
}

export function toLabOrderRow(raw: Record<string, unknown>): LabOrderRow {
  return toSynced<LabOrderRow>(raw);
}

export function toLabResultRow(raw: Record<string, unknown>): LabResultRow {
  return toSynced<LabResultRow>(raw);
}

export function toProbeRow(raw: Record<string, unknown>): SyncProbeRow {
  const value = (key: string) => raw[key];
  const now = new Date().toISOString();
  return {
    id: String(value('id')),
    title: String(value('title') ?? ''),
    body: (value('body') as string | null) ?? null,
    revision: Number(value('revision') ?? 1),
    serverRev: Number(value('serverRev') ?? 0),
    syncState: 'synced',
    deletedAt: (value('deletedAt') as string | null) ?? null,
    createdAt: String(value('createdAt') ?? now),
    updatedAt: String(value('updatedAt') ?? now),
    createdBy: String(value('createdBy') ?? ''),
    updatedBy: String(value('updatedBy') ?? ''),
  };
}
