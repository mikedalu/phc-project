import { beforeEach, describe, expect, it } from 'vitest';
import { db } from './index';
import { localDelete, localWrite } from './write';
import { useSession } from '@/features/auth/session';
import { OfflineGraceError } from '@/features/auth/offline';
import { backoff } from '@/sync/pusher';
import type { SyncProbeRow } from './types';

/**
 * Tests for the two things that silently corrupt an offline-first app:
 * the row/outbox pairing, and writing after the offline window has closed.
 */

function probe(id: string, title = 'probe'): SyncProbeRow {
  const now = new Date().toISOString();
  return {
    id,
    title,
    body: null,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    // localWrite overwrites both from the session; the fixture only needs
    // the type to be satisfied.
    createdBy: '',
    updatedBy: '',
  };
}

beforeEach(async () => {
  await db.open();
  await db.outbox.clear();
  await db.sync_probe.clear();
  await db.syncMeta.clear();

  useSession.setState({
    status: 'authenticated',
    user: {
      userId: '018f0000-0000-7000-8000-0000000000aa',
      staffNumber: 'DEMO/STF/0001',
      displayName: 'Test Admin',
      roles: ['admin'],
      deviceId: '018f0000-0000-7000-8000-0000000000d1',
    },
    offlineGraceExpiresAt: new Date(Date.now() + 86_400_000).toISOString(),
  });
});

describe('localWrite', () => {
  it('writes the row and queues exactly one operation', async () => {
    const id = crypto.randomUUID();
    await localWrite('sync_probe', probe(id, 'hello'));

    const row = await db.sync_probe.get(id);
    const queue = await db.outbox.toArray();

    expect(row?.title).toBe('hello');
    expect(row?.syncState).toBe('pending');
    expect(queue).toHaveLength(1);
    expect(queue[0]?.rowId).toBe(id);
    expect(queue[0]?.operation).toBe('insert');
    expect(queue[0]?.baseServerRev).toBeNull();
    // The client never owns the server's revision.
    expect(row?.serverRev).toBe(0);
  });

  it('turns a second write into an update that carries the base revision', async () => {
    const id = crypto.randomUUID();
    await localWrite('sync_probe', probe(id, 'first'));

    // Pretend the server acknowledged this row.
    await db.sync_probe.update(id, { serverRev: 4, syncState: 'synced' });

    await localWrite('sync_probe', { ...(await db.sync_probe.get(id))!, title: 'second' });

    const queue = await db.outbox.orderBy('createdAt').toArray();
    expect(queue).toHaveLength(2);
    expect(queue[1]?.operation).toBe('update');
    expect(queue[1]?.baseServerRev).toBe(4);
    expect((await db.sync_probe.get(id))?.revision).toBe(2);
  });

  it('refuses to write when the offline window has closed under block_writes', async () => {
    useSession.setState({
      offlineGraceExpiresAt: new Date(Date.now() - 60_000).toISOString(),
    });

    const id = crypto.randomUUID();
    await expect(localWrite('sync_probe', probe(id))).rejects.toBeInstanceOf(OfflineGraceError);

    // The guard runs BEFORE the write, so nothing is half-applied.
    expect(await db.sync_probe.get(id)).toBeUndefined();
    expect(await db.outbox.count()).toBe(0);
  });
});

describe('localDelete', () => {
  it('soft-deletes and queues the tombstone instead of removing the row', async () => {
    const id = crypto.randomUUID();
    await localWrite('sync_probe', probe(id));
    await localDelete('sync_probe', id);

    const row = await db.sync_probe.get(id);
    expect(row).toBeDefined();
    expect(row?.deletedAt).not.toBeNull();
    expect((await db.outbox.toArray()).some((entry) => entry.operation === 'delete')).toBe(true);
  });
});

describe('backoff', () => {
  it('stays bounded and never shrinks below zero', () => {
    for (let attempts = 0; attempts < 12; attempts += 1) {
      const delay = backoff(attempts);
      expect(delay).toBeGreaterThanOrEqual(0);
      // Full jitter: the ceiling doubles, the value never exceeds it.
      expect(delay).toBeLessThanOrEqual(Math.min(1_000 * 2 ** attempts, 300_000));
    }
  });
});
