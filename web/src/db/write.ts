import type { EntityTable } from 'dexie';
import { db } from './index';
import { newId, nowIso, type BaseRow, type OutboxEntry, type SyncProbeRow } from './types';
import { assertCanWrite } from '@/features/auth/offline';
import { useSession } from '@/features/auth/session';

/**
 * The ONLY way clinical data is written on this device.
 *
 * ═══ WHY THIS FUNCTION EXISTS ═══
 *
 * `db.sync_probe.put(row)` would save the row and never sync it. The record
 * looks perfect on the machine that created it, for ever, and only the person
 * on the OTHER computer notices that it is missing. That is the single worst
 * failure mode in this architecture, so the two writes are welded together here
 * and every screen must come through this door.
 *
 * The row write and the outbox append share ONE Dexie transaction. Two
 * transactions would leave a window where a row exists with no queued
 * operation — invisible, permanent, and silent.
 */

/** Every table the client may write through `localWrite`. */
export type SyncableTable = 'sync_probe';

interface RowBase extends BaseRow {
  id: string;
}

function tableHandle(table: SyncableTable): EntityTable<RowBase, 'id'> {
  // One concrete handle shape for every synced table. The cast is contained
  // here rather than repeated at every call site.
  return db[table] as unknown as EntityTable<RowBase, 'id'>;
}

/**
 * Writes a row and queues it for sync, atomically.
 *
 * The caller supplies the domain fields; identity, revision, ownership and the
 * sync columns are owned here. A caller that set `serverRev` or `syncState`
 * itself would be corrupting the one thing the server is authoritative about.
 */
export async function localWrite<T extends RowBase>(
  table: SyncableTable,
  row: T,
): Promise<T> {
  // Refuses when the offline window has closed under `block_writes`. Called
  // BEFORE anything is persisted, so a blocked write leaves no half-state.
  assertCanWrite();

  const userID = useSession.getState().user?.userId;
  if (!userID) {
    throw new Error('Cannot write without a signed-in user: the row could not be attributed.');
  }

  const handle = tableHandle(table);
  const existing = await handle.get(row.id);
  const now = nowIso();

  const prepared = {
    ...row,
    revision: existing ? existing.revision + 1 : 1,
    // NEVER client-owned in the end: the server overwrites this. Sending the
    // value we last saw is what lets the server detect a concurrent edit.
    serverRev: existing?.serverRev ?? 0,
    syncState: 'pending' as const,
    deletedAt: row.deletedAt ?? null,
    createdAt: existing?.createdAt ?? row.createdAt ?? now,
    updatedAt: now,
    createdBy: existing?.createdBy ?? userID,
    updatedBy: userID,
  } as T;

  const entry: OutboxEntry = {
    id: newId(),
    entityTable: table,
    rowId: row.id,
    operation: existing ? 'update' : 'insert',
    payload: prepared,
    baseServerRev: existing ? existing.serverRev : null,
    status: 'pending',
    attempts: 0,
    lastError: null,
    nextAttemptAt: 0,
    createdAt: Date.now(),
    deviceRecordedAt: prepared.updatedAt,
  };

  await db.transaction('rw', handle, db.outbox, async () => {
    await handle.put(prepared);
    await db.outbox.put(entry);
  });

  // Debounced drain: keeps the badge near zero in normal use without a request
  // per keystroke. Imported lazily to avoid a module cycle with the engine.
  void import('@/sync/engine').then((m) => m.requestSync()).catch(() => {});

  return prepared;
}

/**
 * Soft-deletes a row and queues the deletion.
 *
 * NEVER a hard delete: the tombstone has to replicate, or the row lives for ever
 * on every other device. `deleted_at` is a normal update as far as the engine is
 * concerned, which is why a delete needs no special server handling.
 */
export async function localDelete(table: SyncableTable, id: string): Promise<void> {
  assertCanWrite();

  const userID = useSession.getState().user?.userId;
  if (!userID) {
    throw new Error('Cannot delete without a signed-in user.');
  }

  const handle = tableHandle(table);
  const existing = await handle.get(id);
  if (!existing) return;

  const now = nowIso();
  const prepared = {
    ...existing,
    revision: existing.revision + 1,
    syncState: 'pending' as const,
    deletedAt: now,
    updatedAt: now,
    updatedBy: userID,
  };

  const entry: OutboxEntry = {
    id: newId(),
    entityTable: table,
    rowId: id,
    operation: 'delete',
    payload: prepared,
    baseServerRev: existing.serverRev,
    status: 'pending',
    attempts: 0,
    lastError: null,
    nextAttemptAt: 0,
    createdAt: Date.now(),
    deviceRecordedAt: now,
  };

  await db.transaction('rw', handle, db.outbox, async () => {
    await handle.put(prepared);
    await db.outbox.put(entry);
  });

  void import('@/sync/engine').then((m) => m.requestSync()).catch(() => {});
}

/** Convenience for the probe screen. */
export function writeProbe(input: {
  id?: string;
  title: string;
  body: string | null;
}): Promise<SyncProbeRow> {
  const now = nowIso();
  return localWrite<SyncProbeRow>('sync_probe', {
    id: input.id ?? newId(),
    title: input.title,
    body: input.body,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: '',
    updatedBy: '',
  });
}
