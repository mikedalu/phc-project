import { db } from '@/db';
import { apiGet } from '@/lib/api';
import { setEngineState } from './status';
import { pushOnce } from './pusher';
import { pullOnce, toProbeRow } from './puller';

/**
 * The sync engine's conductor.
 *
 * ═══ WHEN IT RUNS (docs/02-SYNC-PROTOCOL.md §5.6) ═══
 *
 *  - App start, after the session is restored — catches up on everything queued
 *    while the tab was closed.
 *  - `online` — the fastest path back.
 *  - Every 60 s while online — the steady-state drain.
 *  - ~2 s after any local write — keeps the pending badge near zero in normal
 *    use without a request per keystroke.
 *
 * Every entry point funnels through `runSync`, which is single-flight.
 */

const PUSH_DEBOUNCE_MS = 2_000;
const POLL_INTERVAL_MS = 60_000;

let debounceTimer: number | null = null;
let running = false;
let started = false;

/** Schedules a sync. Safe to call on every write. */
export function requestSync(delayMs = PUSH_DEBOUNCE_MS): void {
  if (debounceTimer !== null) window.clearTimeout(debounceTimer);
  debounceTimer = window.setTimeout(() => {
    debounceTimer = null;
    void runSync();
  }, delayMs);
}

/** Drains the outbox, then pulls. Single-flight. */
export async function runSync(): Promise<void> {
  if (running) return;
  if (!navigator.onLine) return;
  if (!db.isOpen()) return;

  running = true;
  setEngineState({ syncing: true, lastError: null });

  try {
    // Push first, ALWAYS. Pulling first would apply server rows over local
    // pending edits that have not been sent yet, which then lose for no reason.
    for (let round = 0; round < 5; round += 1) {
      const more = await pushOnce();
      if (!more) break;
    }

    for (let round = 0; round < 20; round += 1) {
      const outcome = await pullOnce();
      if (outcome.needsResync) {
        await rebootstrap();
        break;
      }
      if (!outcome.hasMore) break;
    }

    setEngineState({ syncing: false, lastError: null, lastSyncAt: Date.now() });
  } catch (error) {
    // A background loop must not throw an unhandled rejection; the indicator is
    // where the user learns about it.
    setEngineState({
      syncing: false,
      lastError: error instanceof Error ? error.message : String(error),
    });
  } finally {
    running = false;
  }
}

/**
 * Full re-bootstrap after the server says our cursor predates retention.
 *
 * Snapshot the probe table page by page, then adopt `cursorAtStart` — the seq
 * captured BEFORE the first page was read, so changes made during the bootstrap
 * are not skipped.
 */
async function rebootstrap(): Promise<void> {
  let after = '';
  let cursorAtStart = 0;

  for (let page = 0; page < 200; page += 1) {
    const query = `/sync/snapshot?table=sync_probe${after ? `&after=${after}` : ''}`;
    const { data } = await apiGet<{
      table: string;
      rows: Record<string, unknown>[];
      hasMore: boolean;
      cursorAtStart: number;
    }>(query);

    cursorAtStart = data.cursorAtStart;

    await db.transaction('rw', db.sync_probe, async () => {
      for (const raw of data.rows) {
        await db.sync_probe.put(toProbeRow(raw));
      }
    });

    if (!data.hasMore) break;
    const last = data.rows[data.rows.length - 1];
    if (!last) break;
    after = String(last.id);
  }

  await db.transaction('rw', db.syncMeta, async () => {
    await db.syncMeta.put({ key: 'pullCursor', value: cursorAtStart });
    await db.syncMeta.put({ key: 'fullResyncRequired', value: false });
  });
}

/** Wires the triggers. Called once at bootstrap. */
export function startSync(): void {
  if (started) return;
  started = true;

  window.addEventListener('online', () => requestSync(250));
  window.setInterval(() => {
    if (navigator.onLine) void runSync();
  }, POLL_INTERVAL_MS);

  requestSync(1_000);
}
