import { db } from '@/db';
import { localWrite } from '@/db/write';
import { newId, type AdmissionRow, type WardChargeRow } from '@/db/types';
import { useSession } from '@/features/auth/session';

/**
 * The ward, client side.
 *
 * The bed-day guard is applied HERE as well as in the database: a night is
 * identified by its DATE, so the second device to charge 2026-10-15 for the same
 * admission is told the night is already billed instead of billing it again.
 * Doing it locally means a ward running offline cannot double-charge either.
 */

/** UTC midnight for a date, so two timezones agree on which night this is. */
export function nightOf(date: Date): string {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()))
    .toISOString()
    .slice(0, 10);
}

export async function admitPatient(input: {
  patientId: string;
  encounterId: string;
  ward: string;
  bedNumber: string | null;
}): Promise<AdmissionRow> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot admit without a signed-in user.');
  if (!input.ward.trim()) throw new Error('A ward is required.');

  const open = await db.admissions
    .where('patientId')
    .equals(input.patientId)
    .filter((a) => a.status === 'admitted')
    .first();

  if (open) throw new Error('This patient is already admitted.');

  const now = new Date().toISOString();

  return localWrite<AdmissionRow>('admissions', {
    id: newId(),
    patientId: input.patientId,
    encounterId: input.encounterId,
    ward: input.ward.trim(),
    bedNumber: input.bedNumber,
    status: 'admitted',
    admittedAt: now,
    admittedBy: userID,
    dischargedAt: null,
    dischargedBy: null,
    dischargeNotes: null,
    invoiceId: null,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: userID,
    updatedBy: userID,
  });
}

/**
 * Bills one night. Returns false when the night was already billed — a no-op,
 * not an error, exactly as the server reports it.
 */
export async function chargeNight(input: {
  admissionId: string;
  night: string;
  unitPriceMinor: number;
}): Promise<boolean> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot bill a bed-day without a signed-in user.');
  if (input.unitPriceMinor <= 0) throw new Error('A bed-day needs a price.');

  const admission = await db.admissions.get(input.admissionId);
  if (!admission) throw new Error('That admission is not on this device.');
  if (admission.status !== 'admitted') {
    throw new Error(`This admission is ${admission.status}; no further bed-days can be charged.`);
  }

  const existing = await db.ward_charges
    .where('admissionId')
    .equals(admission.id)
    .filter((c) => c.chargeDate === input.night)
    .first();

  if (existing) return false;

  const now = new Date().toISOString();

  const row: WardChargeRow = {
    id: newId(),
    admissionId: admission.id,
    patientId: admission.patientId,
    chargeDate: input.night,
    ward: admission.ward,
    unitPriceMinor: input.unitPriceMinor,
    amountMinor: input.unitPriceMinor,
    invoiceId: null,
    chargeId: null,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: userID,
    updatedBy: userID,
  };

  await localWrite<WardChargeRow>('ward_charges', row);
  return true;
}

export async function dischargePatient(admissionId: string, notes: string): Promise<void> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot discharge without a signed-in user.');

  const admission = await db.admissions.get(admissionId);
  if (!admission) throw new Error('That admission is not on this device.');
  if (admission.status !== 'admitted') throw new Error(`Already ${admission.status}.`);

  await localWrite<AdmissionRow>('admissions', {
    ...admission,
    status: 'discharged',
    dischargedAt: new Date().toISOString(),
    dischargedBy: userID,
    dischargeNotes: notes || null,
  });
}
