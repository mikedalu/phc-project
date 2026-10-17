import { db } from '@/db';
import { localWrite } from '@/db/write';
import { newId, type LabOrderRow, type LabResultRow } from '@/db/types';
import { useSession } from '@/features/auth/session';

/**
 * The lab bench, client side.
 *
 * A result is a FACT, so it is inserted and never updated: a corrected value is
 * a new row. The order row itself IS updated (status advances), which is why
 * lab_orders is LWW and lab_results is append-only.
 */

/** Marks a specimen as taken and moves the order to `collected`. */
export async function collectOrder(orderId: string): Promise<void> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot collect a specimen without a signed-in user.');

  const order = await db.lab_orders.get(orderId);
  if (!order) throw new Error('That order is not on this device.');

  // The same guard the server applies: a result against a specimen nobody took
  // is a data-entry fiction, so the door closes here too.
  if (order.status !== 'ordered') {
    throw new Error(`This order is already ${order.status}.`);
  }

  const now = new Date().toISOString();

  await localWrite<LabOrderRow>('lab_orders', {
    ...order,
    status: 'collected',
    collectedAt: now,
    collectedBy: userID,
  });
}

export async function recordResult(input: {
  orderId: string;
  parameterName: string;
  valueText: string | null;
  valueNumeric: number | null;
  unit: string | null;
  referenceRange: string | null;
  abnormalFlag: LabResultRow['abnormalFlag'];
}): Promise<LabResultRow> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot record a result without a signed-in user.');
  if (!input.parameterName.trim()) throw new Error('A result needs a parameter name.');
  if (input.valueText === null && input.valueNumeric === null) {
    throw new Error('A result needs a value — text or a number.');
  }

  const order = await db.lab_orders.get(input.orderId);
  if (!order) throw new Error('That order is not on this device.');
  if (order.status !== 'collected' && order.status !== 'resulted') {
    throw new Error(`Results need a collected specimen; this order is ${order.status}.`);
  }

  const now = new Date().toISOString();

  const row = await localWrite<LabResultRow>('lab_results', {
    id: newId(),
    labOrderId: order.id,
    patientId: order.patientId,
    parameterName: input.parameterName.trim(),
    valueText: input.valueText,
    valueNumeric: input.valueNumeric,
    unit: input.unit,
    referenceRange: input.referenceRange,
    abnormalFlag: input.abnormalFlag,
    resultAt: now,
    resultBy: userID,
    remarks: null,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: userID,
    updatedBy: userID,
  });

  // The first result closes the order; later parameters still land, because a
  // full blood count has several and closing the door would make it impossible.
  if (order.status === 'collected') {
    await localWrite<LabOrderRow>('lab_orders', {
      ...order,
      status: 'resulted',
      completedAt: now,
    });
  }

  return row;
}
