import { localWrite } from '@/db/write';
import { newId, type DiagnosisRow, type LabOrderRow } from '@/db/types';
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
