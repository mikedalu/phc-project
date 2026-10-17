import { useSyncExternalStore } from 'react';
import { useLiveQuery } from 'dexie-react-hooks';
import { db } from '@/db';

/* ────────────────────────────────────────────────────────────────────────
 *  Connectivity
 * ──────────────────────────────────────────────────────────────────────── */

function subscribeToOnline(callback: () => void): () => void {
  window.addEventListener('online', callback);
  window.addEventListener('offline', callback);
  return () => {
    window.removeEventListener('online', callback);
    window.removeEventListener('offline', callback);
  };
}

/**
 * Tracks browser connectivity.
 *
 * `navigator.onLine` is a coarse signal — it reports a connected interface, not
 * a working internet connection. A workstation on a LAN with a dead uplink
 * reports online. That is acceptable here because it is only used to decide
 * whether to *attempt* a sync; the request itself is the real test, and a
 * failure is handled as a normal offline condition.
 */
export function useOnline(): boolean {
  return useSyncExternalStore(
    subscribeToOnline,
    () => navigator.onLine,
    () => true, // assume online during SSR/prerender
  );
}

/* ────────────────────────────────────────────────────────────────────────
 *  Sync status
 * ──────────────────────────────────────────────────────────────────────── */

/**
 * The six states the UI must be able to express.
 * See docs/03-UI-PATTERNS.md §4 and docs/02-SYNC-PROTOCOL.md §8.1.
 *
 * Note that `offline` is NOT an error. It is the expected state in a PHC, and
 * it must never be rendered in a danger colour — a user who sees red assumes
 * something is broken and stops working.
 */
export type SyncStatus =
  | 'synced'
  | 'syncing'
  | 'pending'
  | 'offline'
  | 'error'
  | 'blocked';

export interface SyncState {
  status: SyncStatus;
  pendingCount: number;
  conflictCount: number;
  lastError: string | null;
}

/* ────────────────────────────────────────────────────────────────────────
 *  Engine state — a tiny external store
 *
 *  The engine runs outside React (timers, listeners, background loops), so its
 *  progress cannot live in component state. `useSyncExternalStore` is the
 *  supported way to read a mutable value from an external source without
 *  tearing, and it costs about fifteen lines.
 * ──────────────────────────────────────────────────────────────────────── */

interface EngineState {
  syncing: boolean;
  lastError: string | null;
  lastSyncAt: number | null;
}

let engineState: EngineState = { syncing: false, lastError: null, lastSyncAt: null };
const engineListeners = new Set<() => void>();

export function setEngineState(patch: Partial<EngineState>): void {
  engineState = { ...engineState, ...patch };
  for (const listener of engineListeners) listener();
}

function subscribeEngine(listener: () => void): () => void {
  engineListeners.add(listener);
  return () => {
    engineListeners.delete(listener);
  };
}

export function useEngineState(): EngineState {
  return useSyncExternalStore(subscribeEngine, () => engineState, () => engineState);
}

/**
 * Live sync state, derived entirely from Dexie.
 *
 * There is no polling and no React state copy of the data: the outbox count
 * comes from IndexedDB through useLiveQuery, so it is correct the instant an
 * operation is queued or acknowledged.
 *
 * Sprint 2 wires the actual drain/reconcile loop; today this reports the truth
 * about an empty outbox.
 */
export function useSyncState(): SyncState {
  const online = useOnline();

  const pendingCount = useLiveQuery(
    () => db.outbox.where('status').anyOf('pending', 'inflight').count(),
    [],
    0,
  );

  const conflictCount = useLiveQuery(
    () => db.conflicts.where('acknowledged').equals(0).count(),
    [],
    0,
  );

  const engine = useEngineState();

  let status: SyncStatus = 'synced';
  if (!online) {
    // Offline is NOT an error: the records are safe on this device. It must
    // never be rendered in a danger colour.
    status = 'offline';
  } else if (engine.lastError) {
    status = 'error';
  } else if (engine.syncing) {
    status = 'syncing';
  } else if ((pendingCount ?? 0) > 0) {
    status = 'pending';
  }

  return {
    status,
    pendingCount: pendingCount ?? 0,
    conflictCount: conflictCount ?? 0,
    lastError: engine.lastError,
  };
}

/** Placeholder for the Sprint 2 offline grace countdown. */
export function useOfflineGrace(): { expired: boolean; hoursLeft: number } {
  // Sprint 1 supplies the real deadline; Sprint 0 has nothing to count down.
  return { expired: false, hoursLeft: 168 };
}
