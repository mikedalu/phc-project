import { db } from '@/db';
import { localWriteMany, type WriteItem } from '@/db/write';
import {
  newId,
  type AdmissionRow,
  type ChargeRow,
  type EncounterRow,
  type InvoiceRow,
  type WardChargeRow,
} from '@/db/types';
import { useSession } from '@/features/auth/session';
import { nextStages } from '@/features/queue/write';
import { formatMoney, invoiceBalance } from '@/features/billing/write';
import { nextNumber } from '@/features/patients/number';

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

  const encounter = await db.encounters.get(input.encounterId);
  if (!encounter) throw new Error('That visit is not on this device.');
  if (encounter.patientId !== input.patientId) {
    throw new Error('That visit belongs to a different patient.');
  }

  const now = new Date().toISOString();

  const admission: AdmissionRow = {
    id: newId(),
    patientId: input.patientId,
    encounterId: input.encounterId,
    ward: input.ward.trim(),
    bedNumber: input.bedNumber?.trim() || null,
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
  };

  const writes: WriteItem[] = [{ table: 'admissions', row: admission }];

  // The server moves the encounter to `admitted` in the same transaction. Do
  // the same locally, otherwise an offline admission leaves the queue showing a
  // patient in consultation while they are lying in a bed.
  if (encounter.workflowStage !== 'admitted' && nextStages(encounter.workflowStage).includes('admitted')) {
    writes.push({
      table: 'encounters',
      row: { ...encounter, workflowStage: 'admitted' } as EncounterRow,
    });
  }

  await localWriteMany(writes);
  return admission;
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
  const patient = await db.patients.get(admission.patientId);
  const patientHouseholdID = patient?.householdId ?? null;

  // Every bed-day must land on a bill. The cashier needs to see the balance on
  // this device even when the clinic is offline, so create the invoice and line
  // locally and let the server's ward-charge applier confirm them on sync. A
  // client-created invoice number is safe: numbers include the device prefix,
  // and the server's unique constraint handles any collision like any other.
  let invoice = admission.invoiceId ? await db.invoices.get(admission.invoiceId) : undefined;
  let invoiceWrites: WriteItem[] = [];

  if (!invoice) {
    const invoiceID = newId();
    const fresh: InvoiceRow = {
      id: invoiceID,
      invoiceNumber: await nextNumber('INV'),
      patientId: admission.patientId,
      householdId: patientHouseholdID,
      encounterId: admission.encounterId,
      invoiceType: 'ward',
      status: 'unpaid',
      totalMinor: input.unitPriceMinor,
      issuedAt: now,
      issuedBy: userID,
      voidedAt: null,
      voidReason: null,
      notes: null,
      revision: 1,
      serverRev: 0,
      syncState: 'pending',
      deletedAt: null,
      createdAt: now,
      updatedAt: now,
      createdBy: userID,
      updatedBy: userID,
    };
    invoice = fresh;
    invoiceWrites = [{ table: 'invoices', row: fresh }];
  } else {
    invoiceWrites = [
      {
        table: 'invoices',
        row: {
          ...invoice,
          totalMinor: invoice.totalMinor + input.unitPriceMinor,
          // A new night on a previously settled invoice reopens the bill; the
          // cashier must take the balance before the patient can be discharged.
          status: invoice.status === 'paid' ? 'unpaid' : invoice.status,
        } as InvoiceRow,
      },
    ];
  }

  const chargeID = newId();
  const line: ChargeRow = {
    id: chargeID,
    invoiceId: invoice.id,
    patientId: admission.patientId,
    householdId: patientHouseholdID,
    chargeType: 'ward',
    tariffId: null,
    description: `Ward bed · ${admission.ward} · ${input.night}`,
    referenceId: null,
    referenceType: null,
    quantity: 1,
    unitPriceMinor: input.unitPriceMinor,
    amountMinor: input.unitPriceMinor,
    status: 'active',
    postedAt: now,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: userID,
    updatedBy: userID,
  };

  const row: WardChargeRow = {
    id: newId(),
    admissionId: admission.id,
    patientId: admission.patientId,
    chargeDate: input.night,
    ward: admission.ward,
    unitPriceMinor: input.unitPriceMinor,
    amountMinor: input.unitPriceMinor,
    invoiceId: invoice.id,
    chargeId: chargeID,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: userID,
    updatedBy: userID,
  };

  const writes: WriteItem[] = [
    ...invoiceWrites,
    { table: 'charges', row: line },
    { table: 'ward_charges', row: row },
  ];

  if (admission.invoiceId !== invoice.id) {
    writes.push({
      table: 'admissions',
      row: { ...admission, invoiceId: invoice.id } as AdmissionRow,
    });
  }

  // Dependency order matters: invoice -> charge -> ward charge -> admission.
  await localWriteMany(writes);
  return true;
}

export async function dischargePatient(admissionId: string, notes: string): Promise<void> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot discharge without a signed-in user.');

  const admission = await db.admissions.get(admissionId);
  if (!admission) throw new Error('That admission is not on this device.');
  if (admission.status !== 'admitted') throw new Error(`Already ${admission.status}.`);

  // The server is the authority, but an offline workstation must apply the same
  // rule. If the ward invoice is already on this device and has a balance, say so
  // before writing anything; the server repeats the check on sync.
  if (admission.invoiceId) {
    const due = await invoiceBalance(admission.invoiceId);
    if (due > 0) {
      throw new Error(`Settle the outstanding balance of ${formatMoney(due)} before discharge.`);
    }
  }

  const now = new Date().toISOString();
  const writes: WriteItem[] = [
    {
      table: 'admissions',
      row: {
        ...admission,
        status: 'discharged',
        dischargedAt: now,
        dischargedBy: userID,
        dischargeNotes: notes || null,
      } as AdmissionRow,
    },
  ];

  const encounter = await db.encounters.get(admission.encounterId);
  if (encounter && encounter.status === 'active') {
    writes.push({
      table: 'encounters',
      row: {
        ...encounter,
        workflowStage: 'closed',
        status: 'closed',
        closedAt: now,
      } as EncounterRow,
    });
  }

  await localWriteMany(writes);
}
