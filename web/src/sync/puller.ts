import { db } from '@/db';
import type { SyncProbeRow } from '@/db/types';
import { apiGet } from '@/lib/api';

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

  await db.transaction('rw', db.sync_probe, db.syncMeta, async () => {
    for (const [table, rows] of Object.entries(data.rows)) {
      if (table !== 'sync_probe') continue;

      for (const raw of rows) {
        const incoming = toProbeRow(raw);
        const local = await db.sync_probe.get(incoming.id);

        // A row with local edits queued must not be clobbered by an older
        // server copy: our push will resolve it through LWW and come back with
        // the authoritative result.
        if (local && local.syncState === 'pending') continue;

        // Never move a row backwards. Out-of-order re-application is possible
        // when a cursor is stale, and it must be harmless.
        if (local && local.serverRev > incoming.serverRev) continue;

        await db.sync_probe.put(incoming);
        applied += 1;
      }
    }

    // Cursor LAST, in the same transaction.
    await db.syncMeta.put({ key: 'pullCursor', value: data.nextCursor });
    await db.syncMeta.put({ key: 'lastPullAt', value: Date.now() });
  });

  return { applied, hasMore: data.hasMore, needsResync: false };
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
