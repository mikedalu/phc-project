import { db } from '@/db';
import { localWrite, localWriteMany } from '@/db/write';
import {
  newId,
  type ChargeRow,
  type DiagnosisRow,
  type InvoiceRow,
  type LabOrderRow,
  type PrescriptionItemRow,
  type PrescriptionRow,
} from '@/db/types';
import { useSession } from '@/features/auth/session';
import { nextNumber } from '@/features/patients/number';

/**
 * The doctor's writes, all local-first.
 *
 * A consultation is where the record is created that everything downstream
 * bills against, so it must work with the network down — the same rule as the
 * rest of the clinical record.
 */

export async function addDiagnosis(input: {
  patientId: string;
  encounterId: string;
  diagnosisType: DiagnosisRow['diagnosisType'];
  icd10Code: string | null;
  description: string;
  clinicalNotes: string | null;
}): Promise<DiagnosisRow> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot record a diagnosis without a signed-in user.');
  if (!input.description.trim()) throw new Error('A diagnosis needs a description.');

  const now = new Date().toISOString();

  return localWrite<DiagnosisRow>('diagnoses', {
    id: newId(),
    patientId: input.patientId,
    encounterId: input.encounterId,
    diagnosisType: input.diagnosisType,
    icd10Code: input.icd10Code?.trim() || null,
    description: input.description.trim(),
    clinicalNotes: input.clinicalNotes?.trim() || null,
    diagnosedAt: now,
    diagnosedBy: userID,
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

export async function addLabOrder(input: {
  patientId: string;
  encounterId: string;
  testCode: string;
  testName: string;
  specimenType: LabOrderRow['specimenType'];
  clinicalIndication: string | null;
  priority: LabOrderRow['priority'];
}): Promise<LabOrderRow> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot raise a lab order without a signed-in user.');

  const now = new Date().toISOString();

  return localWrite<LabOrderRow>('lab_orders', {
    id: newId(),
    patientId: input.patientId,
    encounterId: input.encounterId,
    orderNumber: await nextNumber('LAB'),
    testCode: input.testCode.trim(),
    testName: input.testName.trim(),
    specimenType: input.specimenType,
    clinicalIndication: input.clinicalIndication?.trim() || null,
    priority: input.priority,
    status: 'ordered',
    orderedAt: now,
    orderedBy: userID,
    collectedAt: null,
    collectedBy: null,
    completedAt: null,
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
 * Writes a prescription with its bill, entirely on this device.
 *
 * ═══ WHY NOT THE REST ENDPOINT ═══
 *
 * `POST /consultations/:id/prescriptions` exists and does the same thing inside
 * one server transaction. But the doctor may be offline, and a prescribing form
 * that refuses without a connection is a form the ward will work around on paper.
 *
 * Everything is written in ONE local transaction and in DEPENDENCY ORDER —
 * invoice, charge, prescription, items — because the sync applier inserts them
 * in the order the outbox holds them, and a prescription that references an
 * invoice the server has not seen yet is a foreign-key violation.
 */
export interface PrescriptionLine {
  drugName: string;
  dose: string;
  frequency: string;
  durationDays: number | null;
  quantity: number;
  unitPriceMinor: number;
}

export async function addPrescription(input: {
  patientId: string;
  encounterId: string;
  lines: PrescriptionLine[];
  notes?: string | null;
}): Promise<{ prescriptionId: string; totalMinor: number }> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot prescribe without a signed-in user.');
  if (input.lines.length === 0) throw new Error('A prescription needs at least one drug.');

  const lines = input.lines.filter((l) => l.drugName.trim() && l.quantity > 0);
  if (lines.length === 0) throw new Error('A prescription needs at least one drug.');

  const now = new Date().toISOString();
  const total = lines.reduce((sum, l) => sum + Math.round(l.unitPriceMinor * l.quantity), 0);

  const patient = await db.patients.get(input.patientId);
  if (!patient) throw new Error('That patient is not on this device.');

  const prescriptionId = newId();
  const invoiceId = newId();
  const chargeId = newId();

  const invoice: InvoiceRow = {
    id: invoiceId,
    invoiceNumber: await nextNumber('INV'),
    patientId: patient.id,
    householdId: patient.householdId,
    encounterId: input.encounterId,
    invoiceType: 'drug',
    status: 'unpaid',
    totalMinor: total,
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

  const charge: ChargeRow = {
    id: chargeId,
    invoiceId,
    patientId: patient.id,
    householdId: patient.householdId,
    chargeType: 'drug',
    tariffId: null,
    description: `Prescription · ${lines.length} item${lines.length === 1 ? '' : 's'}`,
    referenceId: prescriptionId,
    referenceType: 'prescription',
    quantity: 1,
    unitPriceMinor: total,
    amountMinor: total,
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

  const prescription: PrescriptionRow = {
    id: prescriptionId,
    prescriptionNumber: await nextNumber('RX'),
    patientId: patient.id,
    encounterId: input.encounterId,
    prescriptionType: 'opd',
    status: 'pending_payment',
    prescribedAt: now,
    prescribedBy: userID,
    invoiceId,
    notes: input.notes?.trim() || null,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: userID,
    updatedBy: userID,
  };

  const items: PrescriptionItemRow[] = lines.map((line) => ({
    id: newId(),
    prescriptionId,
    drugName: line.drugName.trim(),
    dose: line.dose.trim(),
    frequency: line.frequency.trim(),
    durationDays: line.durationDays,
    quantityPrescribed: line.quantity,
    route: null,
    instructions: null,
    chargeId,
    unitPriceMinor: line.unitPriceMinor,
    amountMinor: Math.round(line.unitPriceMinor * line.quantity),
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: userID,
    updatedBy: userID,
  }));

  // Dependency order: invoice -> charge -> prescription -> items.
  await localWriteMany([
    { table: 'invoices', row: invoice },
    { table: 'charges', row: charge },
    { table: 'prescriptions', row: prescription },
    ...items.map((item) => ({ table: 'prescription_items' as const, row: item })),
  ]);

  return { prescriptionId, totalMinor: total };
}
