import { db } from '@/db';
import { newId, type OutboxEntry, type SyncProbeRow } from '@/db/types';
import { apiFetch, apiPath } from '@/lib/api';
import { loadOrCreateDeviceKey, signRequest } from './deviceKey';

/**
 * The outbox drain.
 *
 * ═══ RULES THAT MATTER ═══
 *
 *  - SINGLE-FLIGHT. A reconnect event, the 60 s timer and a write debounce can
 *    all fire at once. Without a guard that is three concurrent pushes of the
 *    same operations; idempotency saves the data, but the traffic and the logs
 *    are confusing for no reason.
 *  - Mark `inflight` BEFORE the request. A crash mid-flight then neither
 *    double-sends nor loses the entries.
 *  - `rejected` is NOT retried. It is permanent; retrying it for ever is how a
 *    device hammers the server and never drains. It is parked in `conflicts`
 *    so a human can act on it.
 *  - Backoff is exponential WITH FULL JITTER, so a clinic's twelve workstations
 *    do not all retry in lockstep when the uplink returns.
 */

const MAX_BATCH = 200;
const BASE_DELAY_MS = 1_000;
const MAX_DELAY_MS = 5 * 60_000;

export interface PushOpResult {
  operationId: string;
  status: 'applied' | 'duplicate' | 'conflict_lww' | 'rejected';
  rowId: string;
  serverRev?: number;
  reason?: string;
  keptServerVersion?: boolean;
  conflictId?: string;
  serverPayload?: Record<string, unknown>;
}

export interface PushResponse {
  batchId: string;
  serverTime: string;
  results: PushOpResult[];
  summary: { applied: number; duplicate: number; conflictLww: number; rejected: number };
  nextCursor: number;
  clientClockSkewSeconds: number;
}

/** Exponential backoff with full jitter. */
export function backoff(attempts: number): number {
  const capped = Math.min(BASE_DELAY_MS * 2 ** attempts, MAX_DELAY_MS);
  return Math.random() * capped;
}

/**
 * Drains one batch. Returns true when there may be more work.
 *
 * Never throws for a network failure: the caller is a background loop, and an
 * exception there would be an unhandled rejection with no user-visible effect.
 */
export async function pushOnce(): Promise<boolean> {
  const key = await loadOrCreateDeviceKey();
  if (!key.registered) {
    // A device that cannot sign cannot push. This is the window between login
    // and POST /devices; the next successful sign-in retries registration.
    return false;
  }

  const due = await db.outbox
    .where('[status+nextAttemptAt]')
    .between(['pending', 0], ['pending', Date.now()], true, true)
    .limit(MAX_BATCH)
    .toArray();

  if (due.length === 0) return false;

  await db.outbox.bulkUpdate(
    due.map((entry) => ({ key: entry.id, changes: { status: 'inflight' as const } })),
  );

  const pendingAfter = await db.outbox.where('status').equals('pending').count();

  const body = JSON.stringify({
    batchId: newId(),
    deviceId: key.deviceId,
    clientSentAt: new Date().toISOString(),
    schemaVersion: 1,
    pendingOperationCount: Math.max(0, pendingAfter - due.length),
    operations: due.map((entry) => ({
      operationId: entry.id,
      table: entry.entityTable,
      rowId: entry.rowId,
      operation: entry.operation,
      baseServerRev: entry.baseServerRev,
      deviceRecordedAt: entry.deviceRecordedAt,
      payload: entry.payload,
    })),
  });

  try {
    // Sign the EXACT bytes above, then send those same bytes. Re-serialising in
    // the client would change them and invalidate the signature.
    const headers = await signRequest({
      privateKey: key.privateKey,
      method: 'POST',
      path: apiPath('/sync/push'),
      deviceId: key.deviceId,
      body,
    });

    const { data } = await apiFetch<PushResponse>('/sync/push', {
      method: 'POST',
      rawBody: body,
      headers: { ...headers } as Record<string, string>,
      timeoutMs: 60_000,
    });

    await applyResults(due, data);
    return due.length === MAX_BATCH;
  } catch (error) {
    await db.outbox.bulkUpdate(
      due.map((entry) => ({
        key: entry.id,
        changes: {
          status: 'pending' as const,
          attempts: entry.attempts + 1,
          lastError: error instanceof Error ? error.message : String(error),
          nextAttemptAt: Date.now() + backoff(entry.attempts),
        },
      })),
    );
    throw error;
  }
}

/**
 * Applies each per-operation result to the local replica.
 *
 * All six statuses are handled explicitly. The one that is easy to get wrong is
 * `rejected`: it must be parked, not dropped and not retried.
 */
async function applyResults(due: OutboxEntry[], response: PushResponse): Promise<void> {
  const byId = new Map(response.results.map((r) => [r.operationId, r]));
  const now = Date.now();

  // EVERY synced table must be in the transaction scope, not just the probe.
  //
  // Dexie throws NotFoundError on a store that is not part of the enclosing
  // transaction, so marking a patient synced outside this list silently failed
  // AFTER the outbox row was already marked done — the work was sent, the row
  // still said "never sent", and nothing reconciled them.
  const syncedTables = [db.sync_probe, db.patients, db.households];

  await db.transaction('rw', [db.outbox, db.conflicts, ...syncedTables], async () => {
    for (const entry of due) {
      const result = byId.get(entry.id);
      if (!result) {
        // The server did not mention this operation. Treat it as a transient
        // failure rather than silently losing it.
        await db.outbox.update(entry.id, {
          status: 'pending',
          attempts: entry.attempts + 1,
          lastError: 'missing from server response',
          nextAttemptAt: Date.now() + backoff(entry.attempts),
        });
        continue;
      }

      switch (result.status) {
        case 'applied':
        case 'duplicate': {
          await markDone(entry, result.serverRev);
          break;
        }

        case 'conflict_lww': {
          if (result.keptServerVersion && result.serverPayload) {
            // The server's copy won: overwrite the local row with it, so the
            // device stops showing a value that lost.
            await db.sync_probe.put(serverPayloadToRow(result.serverPayload));
            await db.conflicts.put({
              id: result.conflictId ?? entry.id,
              entityTable: entry.entityTable,
              rowId: entry.rowId,
              conflictType: 'lww_lost',
              serverRow: result.serverPayload,
              localRow: entry.payload,
              reason: result.reason ?? 'server_copy_newer',
              detectedAt: now,
              acknowledged: 0,
            });
          } else {
            await db.conflicts.put({
              id: result.conflictId ?? entry.id,
              entityTable: entry.entityTable,
              rowId: entry.rowId,
              conflictType: 'lww_won',
              serverRow: null,
              localRow: entry.payload,
              reason: result.reason ?? 'client_copy_newer',
              detectedAt: now,
              acknowledged: 0,
            });
          }
          await markDone(entry, result.serverRev);
          break;
        }

        case 'rejected': {
          // Permanent. Park it and show a human; NEVER retry.
          await db.outbox.update(entry.id, {
            status: 'failed',
            lastError: result.reason ?? 'rejected',
          });
          await db.conflicts.put({
            id: result.conflictId ?? entry.id,
            entityTable: entry.entityTable,
            rowId: entry.rowId,
            conflictType: 'rejected',
            serverRow: null,
            localRow: entry.payload,
            reason: result.reason ?? 'rejected',
            detectedAt: now,
            acknowledged: 0,
          });
          break;
        }
      }
    }
  });
}

async function markDone(entry: OutboxEntry, serverRev: number | undefined): Promise<void> {
  await db.outbox.update(entry.id, { status: 'done' });

  if (typeof serverRev !== 'number') return;

  // GENERIC on purpose. Hard-coding 'sync_probe' here marked the outbox done
  // while leaving the patient row at serverRev 0 / pending for ever: the outbox
  // said "sent", the row said "never sent", and nothing reconciled them.
  const table = (db as unknown as Record<string, { get: Function; put: Function }>)[
    entry.entityTable
  ];
  if (!table) return;

  const row = (await table.get(entry.rowId)) as Record<string, unknown> | undefined;
  if (row) {
    await table.put({ ...row, serverRev, syncState: 'synced' });
  }
}

/** Maps a server row (camelCase) back onto the local shape. */
function serverPayloadToRow(payload: Record<string, unknown>): SyncProbeRow {
  const value = (key: string) => payload[key];
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
    createdBy: '',
    updatedBy: '',
  };
}
