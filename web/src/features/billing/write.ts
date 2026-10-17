import { db } from '@/db';
import { localWrite, localWriteMany } from '@/db/write';
import { newId, type AllocationRow, type InvoiceRow, type PaymentRow } from '@/db/types';
import { useSession } from '@/features/auth/session';
import { nextNumber } from '@/features/patients/number';

/**
 * The cashier's writes.
 *
 * ═══ THE IDEMPOTENCY KEY IS GENERATED HERE ═══
 *
 * `operationId` is created ONCE, when the cashier presses Pay, and is stored on
 * the payment row. Every retry of that outbox entry carries the same value, so
 * the server's unique constraint turns a replayed batch into a duplicate instead
 * of a second charge. Generating it at push time would defeat the whole point.
 */

/** What is still owed on one invoice, derived from charges minus allocations. */
export async function invoiceBalance(invoiceId: string): Promise<number> {
  const charges = await db.charges.where('invoiceId').equals(invoiceId).toArray();
  const active = charges.filter((c) => c.status === 'active');

  let paid = 0;
  for (const charge of active) {
    const allocations = await db.payment_allocations.where('chargeId').equals(charge.id).toArray();
    paid += allocations.reduce((sum, a) => sum + a.amountMinor, 0);
  }

  const total = active.reduce((sum, c) => sum + c.amountMinor, 0);
  return total - paid;
}

export interface TakePaymentInput {
  invoiceId: string;
  amountMinor: number;
  method: PaymentRow['paymentMethod'];
  reference?: string | null;
}

/**
 * Records a payment and allocates it across the invoice's outstanding charges.
 *
 * The allocation loop mirrors the server's: oldest-first, never more than a
 * charge is still owed. Doing it locally means the balance is right OFFLINE,
 * which is the state the cashier is actually in half the time.
 */
export async function takePayment(input: TakePaymentInput): Promise<PaymentRow> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot take a payment without a signed-in user.');
  if (input.amountMinor <= 0) throw new Error('A payment must be more than zero.');

  const invoice = await db.invoices.get(input.invoiceId);
  if (!invoice) throw new Error('That invoice is not on this device.');
  if (invoice.status === 'void') throw new Error('This invoice has been voided.');

  const outstanding = await invoiceBalance(invoice.id);
  if (input.amountMinor > outstanding) {
    throw new Error(
      `That is more than the outstanding balance of ${formatMoney(outstanding)}.`,
    );
  }

  const now = new Date().toISOString();
  const operationId = newId();

  const payment: PaymentRow = {
    id: newId(),
    operationId,
    receiptNumber: await nextNumber('RCP'),
    payerPatientId: invoice.patientId,
    householdId: invoice.householdId,
    amountMinor: input.amountMinor,
    currencyCode: 'NGN',
    paymentMethod: input.method,
    paymentReference: input.reference ?? null,
    status: 'posted',
    paidAt: now,
    receivedBy: userID,
    reversalOfPaymentId: null,
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

  const charges = (await db.charges.where('invoiceId').equals(invoice.id).toArray())
    .filter((c) => c.status === 'active')
    .sort((a, b) => a.postedAt.localeCompare(b.postedAt));

  const allocations: AllocationRow[] = [];
  let remaining = input.amountMinor;

  for (const charge of charges) {
    if (remaining <= 0) break;

    const already = await db.payment_allocations.where('chargeId').equals(charge.id).toArray();
    const settled = already.reduce((sum, a) => sum + a.amountMinor, 0);
    const due = charge.amountMinor - settled;
    if (due <= 0) continue;

    const amount = Math.min(due, remaining);
    allocations.push({
      id: newId(),
      paymentId: payment.id,
      chargeId: charge.id,
      amountMinor: amount,
      allocatedAt: now,
      allocatedBy: userID,
      revision: 1,
      serverRev: 0,
      syncState: 'pending',
      deletedAt: null,
      createdAt: now,
      updatedAt: now,
      createdBy: userID,
      updatedBy: userID,
    });
    remaining -= amount;
  }

  // One transaction: money and what it settled must never be separated.
  await localWriteMany([
    { table: 'payments', row: payment },
    ...allocations.map((a) => ({ table: 'payment_allocations' as const, row: a })),
  ]);

  // The status label is derived, but storing it keeps list rendering cheap and
  // matches the server's own denormalisation of the label (not the balance).
  const after = outstanding - input.amountMinor;
  const status: InvoiceRow['status'] = after <= 0 ? 'paid' : 'partially_paid';
  if (status !== invoice.status) {
    await localWrite<InvoiceRow>('invoices', { ...invoice, status });
  }

  return payment;
}

export function formatMoney(minor: number): string {
  const sign = minor < 0 ? '-' : '';
  const abs = Math.abs(minor);
  return `${sign}₦${Math.floor(abs / 100).toLocaleString()}.${String(abs % 100).padStart(2, '0')}`;
}
