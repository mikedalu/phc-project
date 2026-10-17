import { db } from '@/db';
import { getDeviceId } from '@/sync/deviceKey';

/**
 * Document numbers, generated on the device.
 *
 * ═══ WHY NOT A SERVER SEQUENCE ═══
 *
 * A patient is registered with the network down. A database sequence is
 * unreachable exactly when it is needed, so the number must be produced locally
 * — and must not collide with one another device produced.
 *
 * Format: `{FACILITY_CODE}/PAT/{YEAR}/{DEVICE_SHORT}-{SEQ}`
 *   DEMO/PAT/2026/A3F-0007
 *
 * The device segment is what makes collisions structurally impossible. It is
 * uglier than a clean sequence and that is a fair trade: a duplicate number is a
 * support call, a collision is a data corruption.
 */
export async function nextPatientNumber(): Promise<string> {
  const year = new Date().getFullYear();

  const [codeEntry, deviceId] = await Promise.all([
    db.syncMeta.get('facilityCode'),
    getDeviceId(),
  ]);

  const code = typeof codeEntry?.value === 'string' && codeEntry.value ? codeEntry.value : 'PHC';
  const deviceShort = shortDevice(deviceId);

  // The counter is per device AND per year: numbering restarts each January, and
  // two devices never share a counter because their prefixes differ.
  const key = `patient:${year}`;
  const seq = await db.transaction('rw', db.numberCounters, async () => {
    const current = await db.numberCounters.get(key);
    const next = (current?.lastValue ?? 0) + 1;
    await db.numberCounters.put({ key, lastValue: next });
    return next;
  });

  return `${code}/PAT/${year}/${deviceShort}-${String(seq).padStart(4, '0')}`;
}

/** Three characters from the device id — stable for this browser profile. */
function shortDevice(deviceId: string | null): string {
  if (!deviceId) return 'UNK';
  const hex = deviceId.replace(/-/g, '').toUpperCase();
  return hex.slice(-3);
}
