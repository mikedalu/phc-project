import { db } from '@/db';

/**
 * Every table that replicates, in ONE place.
 *
 * ═══ WHY THIS FILE EXISTS ═══
 *
 * The pusher and the puller each need the full list: Dexie refuses to touch a
 * store that is not in the enclosing transaction, and a table missing from the
 * list fails in the worst possible way — AFTER the outbox row is marked done,
 * leaving the row at serverRev 0 for ever while the outbox says "sent".
 *
 * That bug was written three times (probe → patients → encounters) because the
 * list was typed out at each call site. One export, two call sites, no fourth
 * time.
 */
export const SYNCED_TABLES = ['sync_probe', 'patients', 'households', 'encounters', 'vitals'] as const;

export type SyncedTableName = (typeof SYNCED_TABLES)[number];

/** The Dexie handles, for `db.transaction('rw', ...)`. */
export function syncedTableHandles() {
  return [db.sync_probe, db.patients, db.households, db.encounters, db.vitals];
}
