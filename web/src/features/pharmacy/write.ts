import { db } from '@/db';
import { localWrite } from '@/db/write';
import { newId, type DispenseRow, type PrescriptionRow } from '@/db/types';
import { useSession } from '@/features/auth/session';

/**
 * The pharmacy counter.
 *
 * Dispensing is a FACT that stock moved, so it is an insert plus a status
 * change — never an edit. The paid-only guard is the same one the server
 * enforces, applied locally so a pharmacist working offline cannot hand over
 * drugs the patient has not paid for.
 */
export async function dispensePrescription(prescriptionId: string): Promise<DispenseRow> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot dispense without a signed-in user.');

  const rx = await db.prescriptions.get(prescriptionId);
  if (!rx) throw new Error('That prescription is not on this device.');

  if (rx.status === 'dispensed') throw new Error('This prescription has already been dispensed.');
  if (rx.status === 'pending_payment') {
    throw new Error('This prescription has not been paid for. Take payment at the cashier first.');
  }
  if (rx.status !== 'paid') {
    throw new Error(`This prescription is ${rx.status.replaceAll('_', ' ')}.`);
  }

  const now = new Date().toISOString();

  const dispense: DispenseRow = {
    id: newId(),
    prescriptionId: rx.id,
    patientId: rx.patientId,
    dispensedAt: now,
    dispensedBy: userID,
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

  await localWrite<DispenseRow>('dispenses', dispense);
  await localWrite<PrescriptionRow>('prescriptions', { ...rx, status: 'dispensed' });

  return dispense;
}

/** True when the prescription's invoice has been settled locally. */
export async function isPaid(rx: PrescriptionRow): Promise<boolean> {
  if (!rx.invoiceId) return true; // nothing to pay
  const invoice = await db.invoices.get(rx.invoiceId);
  return invoice ? invoice.status === 'paid' : false;
}
