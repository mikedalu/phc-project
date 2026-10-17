import { db } from '@/db';
import { apiGet } from '@/lib/api';
import { setEngineState } from './status';
import { pushOnce } from './pusher';
import { applySnapshotRows, pullOnce } from './puller';
import { SYNCED_TABLES } from './tables';

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

/**
 * A whole sync run must FINISH, one way or another.
 *
 * ═══ THE FAILURE THIS PREVENTS ═══
 *
 * `runSync` is single-flight: while a run is in progress every other trigger is
 * merely queued. If any stage of a run never settles — a fetch that hangs, a
 * transaction that never commits — the engine stays `syncing` for ever, drops
 * every subsequent trigger, and shows no error. The device looks idle and empty
 * with nothing to press.
 *
 * A timeout converts "stuck for ever" into "failed, try again", which the poll
 * then does 60 s later.
 */
// 120 s, not 30: a FIRST run includes the per-table snapshot bootstrap, which on
// a cold Neon takes a few seconds per table. The timeout exists to break a wedge,
// not to police a slow — but healthy — first sync.
const SYNC_TIMEOUT_MS = 120_000;

let debounceTimer: number | null = null;
let running = false;
let queued = false;
let started = false;

/** Schedules a sync. Safe to call on every write. */
export function requestSync(delayMs = PUSH_DEBOUNCE_MS): void {
  if (debounceTimer !== null) window.clearTimeout(debounceTimer);
  debounceTimer = window.setTimeout(() => {
    debounceTimer = null;
    void runSync();
  }, delayMs);
}

/**
 * Drains the outbox, then pulls. Single-flight, but a request that arrives
 * DURING a run is remembered and honoured afterwards.
 *
 * ═══ WHY THE QUEUE MATTERS ═══
 *
 * Without it, a "Sync now" press that lands while the boot sync is still in
 * flight is silently dropped — and the caller then waits up to 60 s for the poll.
 * That is not a theoretical edge case: on a slow link the boot sync takes
 * several seconds, which is exactly when an impatient user presses the button.
 */
export async function runSync(): Promise<void> {
  if (running) {
    queued = true;
    return;
  }
  if (!navigator.onLine) return;

  // ── A closed database means "not yet", never "give up" ─────────────────
  //
  // The engine is asked to run 1 s after boot, and the initial catch-up can
  // arrive while Dexie is still opening. Returning silently for that case left
  // a FRESH device showing an empty app for a whole poll interval (60 s) — no
  // patients, no queue, looking broken — and the only cure was pressing Sync
  // now. Re-arming costs one timer and removes that dead minute entirely.
  if (!db.isOpen()) {
    requestSync(300);
    return;
  }

  running = true;
  setEngineState({ syncing: true, lastError: null });

  try {
    do {
      queued = false;
      await withTimeout(runOnce(), SYNC_TIMEOUT_MS);
    } while (queued);

    setEngineState({ syncing: false, lastError: null, lastSyncAt: Date.now() });
  } finally {
    running = false;
  }
}

/**
 * Fills a brand-new device table by table.
 *
 * Runs BEFORE the first pull, and never on a device that already has a cursor:
 * the cursor is the contract that says "I have everything up to here".
 */
async function bootstrapFromScratch(): Promise<void> {
  let cursor = 0;

  for (const table of SYNCED_TABLES) {
    let after = '';

    try {
    for (let page = 0; page < 200; page += 1) {
      const query = `/sync/snapshot?table=${table}${after ? `&after=${after}` : ''}`;
      const { data } = await apiGet<{
        table: string;
        rows: Record<string, unknown>[];
        hasMore: boolean;
        cursorAtStart: number;
      }>(query);

      // Take the MINIMUM, never the maximum.
      //
      // Each table's snapshot reports the seq at ITS start, and the tables are
      // read one after another, so the last number is the largest. Adopting the
      // largest would skip every row written between the first table's snapshot
      // and the last — which is exactly the window a second device is active in.
      // The symptom is a patient registered on device A that device B never
      // receives, with both cursors looking perfectly healthy.
      //
      // Replaying a change that was already snapshotted is harmless; skipping
      // one is not.
      const start = data.cursorAtStart ?? 0;
      cursor = cursor === 0 ? start : Math.min(cursor, start);
      await applySnapshotRows(table, data.rows);

      if (!data.hasMore) break;
      const last = data.rows[data.rows.length - 1];
      if (!last) break;
      after = String(last.id);
    }
    } catch (error) {
      // A role does not get every table: a lab scientist cannot read households,
      // and a cashier cannot read clinical notes. Skipping the ones this user
      // may not see is correct; aborting the bootstrap would leave a device with
      // NO cursor and an empty app, which is exactly the bug this fixes.
      if (!isSkippableTableError(error)) throw error;
    }
  }

  // Adopt the cursor captured BEFORE the first page was read, so changes made
  // during the bootstrap are replayed by the next pull rather than skipped.
  await db.syncMeta.put({ key: 'pullCursor', value: cursor });
}

/** True for "you may not read that table", which must not abort a bootstrap. */
function isSkippableTableError(error: unknown): boolean {
  const code = (error as { code?: string } | null)?.code;
  return code === 'FORBIDDEN' || code === 'NOT_FOUND' || code === 'VALIDATION_FAILED';
}

function withTimeout<T>(work: Promise<T>, ms: number): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = window.setTimeout(() => {
      reject(new Error(`Sync timed out after ${Math.round(ms / 1000)}s.`));
    }, ms);
    work.then(
      (value) => {
        window.clearTimeout(timer);
        resolve(value);
      },
      (error: unknown) => {
        window.clearTimeout(timer);
        reject(error);
      },
    );
  });
}

async function runOnce(): Promise<void> {
  try {
    // A device with NO cursor has never synced. Fill it from SNAPSHOTS, not from
    // `pull?since=0`: the pull has to stream the whole change log plus every row
    // in one response, which on a cold Neon outlasts the client's timeout — and
    // the device then sits there looking empty with nothing to press.
    if (!(await db.syncMeta.get('pullCursor'))) {
      await bootstrapFromScratch();
    }

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
  } catch (error) {
    // A background loop must not throw an unhandled rejection; the indicator is
    // where the user learns about it.
    setEngineState({
      syncing: false,
      lastError: error instanceof Error ? error.message : String(error),
    });
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
  // A full re-bootstrap is exactly what a first-time device does, so it is the
  // same routine: snapshot every table, then adopt the cursor captured before
  // the first page. The earlier version restored ONLY the probe table, which
  // left a resynced device with an empty app and a cursor claiming otherwise.
  await bootstrapFromScratch();
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
