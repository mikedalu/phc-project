import { db } from '@/db';
import { fetchMeta } from '@/lib/api';
import { isOfflineGraceExpired, useSession } from './session';

/**
 * Offline policy: what a device is allowed to do once its grace window closes.
 *
 * Two values, both legitimate, both demonstrable in a viva (docs/05-SECURITY.md
 * §3.5):
 *
 *  - `block_writes` (default): reads keep working, writes are refused. A nurse
 *    mid-shift does not lose a patient's history; she just cannot add to it.
 *  - `warn_only`: writes are allowed with a visible warning.
 *
 * ═══ WHAT NO POLICY MAY WEAKEN ═══
 *
 * Server-side REVOCATION overrides both. A revoked device must stop working
 * immediately and unconditionally, however forgiving the grace policy is. If
 * `warn_only` ever let a revoked device keep writing, a convenience setting
 * would have become a security hole.
 */

export type OfflinePolicy = 'block_writes' | 'warn_only';

const POLICY_KEY = 'offlinePolicy';

// Cached in memory after boot so assertCanWrite() stays synchronous — it is
// called inside every local write, including from Dexie transactions, and an
// await there would split the transaction.
let currentPolicy: OfflinePolicy = 'block_writes';

/**
 * Loads the policy: cached value first (works offline), then refresh if online.
 *
 * Called once during bootstrap, before render. Never awaited by a write path.
 */
export async function primeOfflinePolicy(): Promise<void> {
  try {
    const cached = await db.syncMeta.get(POLICY_KEY);
    if (cached?.value === 'warn_only' || cached?.value === 'block_writes') {
      currentPolicy = cached.value;
    }
  } catch {
    // A failed read is not worth blocking boot for; keep the safe default.
  }

  if (!navigator.onLine) return;

  try {
    const { data } = await fetchMeta();
    currentPolicy = data.offlinePolicy;
    await db.syncMeta.put({ key: POLICY_KEY, value: data.offlinePolicy });
  } catch {
    // Offline or a slow cold start. The cached value stands.
  }
}

export function offlinePolicy(): OfflinePolicy {
  return currentPolicy;
}

/** True when the device has been offline past its server-issued deadline. */
export function graceExpired(): boolean {
  return isOfflineGraceExpired();
}

/** True when writes must be refused right now. */
export function writesBlocked(): boolean {
  return graceExpired() && currentPolicy === 'block_writes';
}

/**
 * Thrown by write paths when the offline window has closed under block_writes.
 *
 * A distinct error, not a generic Error, because the UI must say "connect and
 * sign in again" rather than "something went wrong".
 */
export class OfflineGraceError extends Error {
  readonly code = 'OFFLINE_GRACE_EXPIRED';

  constructor() {
    super('Offline access has expired. Connect this device to the internet and sign in again.');
    this.name = 'OfflineGraceError';
  }
}

/**
 * Guards a clinical write. Called by `localWrite` before anything is persisted.
 *
 * Synchronous on purpose: it runs inside a Dexie transaction, where an await on
 * a foreign promise would cause the transaction to commit early and silently
 * drop the write.
 */
export function assertCanWrite(): void {
  if (!writesBlocked()) return;
  throw new OfflineGraceError();
}

/** Milliseconds until the grace window closes; negative once it has. */
export function graceMillisRemaining(): number {
  const at = useSession.getState().offlineGraceExpiresAt;
  if (!at) return 0;
  return Date.parse(at) - Date.now();
}
