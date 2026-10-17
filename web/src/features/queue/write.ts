import { db } from '@/db';
import { localWrite } from '@/db/write';
import { newId, type EncounterRow, type VitalsRow } from '@/db/types';
import { useSession } from '@/features/auth/session';
import { nextVisitNumber } from '@/features/patients/number';

/**
 * The workflow, client side.
 *
 * ═══ THE TRANSITION TABLE IS DUPLICATED HERE ON PURPOSE ═══
 *
 * The server is the authority and will refuse an illegal move. But the queue
 * board has to render *something* while offline, and "no buttons at all" is
 * worse than a copy of the rules. The copy is kept honest by the server's
 * refusal: if it drifts, the user sees an error rather than a silent corruption.
 *
 * Every write goes through `localWrite`, so a visit started with the network
 * down still reaches the server later — and the queue is on every device.
 */

const TRANSITIONS: Record<string, string[]> = {
  registered: ['vitals_done'],
  vitals_done: ['awaiting_doctor'],
  awaiting_doctor: ['in_consultation'],
  in_consultation: ['awaiting_lab', 'awaiting_drug_payment', 'awaiting_pharmacy', 'admitted', 'closed'],
  awaiting_lab: ['awaiting_lab_payment', 'lab_done'],
  awaiting_lab_payment: ['awaiting_lab'],
  lab_done: ['awaiting_doctor'],
  awaiting_drug_payment: ['awaiting_pharmacy'],
  awaiting_pharmacy: ['pharmacy_done'],
  pharmacy_done: ['closed'],
  admitted: ['closed'],
};

export function nextStages(stage: string): string[] {
  return TRANSITIONS[stage] ?? [];
}

/** Starts a visit, which puts the patient in the queue. */
export async function startVisit(input: {
  patientId: string;
  householdId: string | null;
  encounterType?: EncounterRow['encounterType'];
  priority?: 0 | 1 | 2;
  chiefComplaint?: string | null;
}): Promise<EncounterRow> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot start a visit without a signed-in user.');

  // One open visit per patient. The server enforces this too; catching it here
  // means a double-click does not create a queue entry that the server then
  // rejects, leaving a phantom row on this device for ever.
  const existing = await db.encounters
    .where('patientId')
    .equals(input.patientId)
    .filter((e) => e.status === 'active')
    .first();

  if (existing) {
    throw new Error(`This patient already has an open visit (${existing.visitNumber}).`);
  }

  const now = new Date().toISOString();

  return localWrite<EncounterRow>('encounters', {
    id: newId(),
    visitNumber: await nextVisitNumber(),
    patientId: input.patientId,
    householdId: input.householdId,
    encounterType: input.encounterType ?? 'opd',
    workflowStage: 'registered',
    status: 'active',
    priority: input.priority ?? 0,
    chiefComplaint: input.chiefComplaint ?? null,
    queuedAt: now,
    startedAt: null,
    closedAt: null,
    attendingDoctorId: null,
    attendingNurseId: null,
    notes: null,
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

/** Records an observation and moves a `registered` visit to `vitals_done`. */
export async function recordVitals(input: {
  patientId: string;
  encounterId: string | null;
  bpSystolic: number | null;
  bpDiastolic: number | null;
  temperatureC: number | null;
  pulseRate: number | null;
  respiratoryRate: number | null;
  heightCm: number | null;
  weightKg: number | null;
  spo2: number | null;
}): Promise<VitalsRow> {
  const userID = useSession.getState().user?.userId;
  if (!userID) throw new Error('Cannot record vitals without a signed-in user.');

  const now = new Date().toISOString();

  const bmi =
    input.heightCm && input.weightKg && input.heightCm > 0
      ? input.weightKg / (input.heightCm / 100) ** 2
      : null;

  const row = await localWrite<VitalsRow>('vitals', {
    id: newId(),
    patientId: input.patientId,
    encounterId: input.encounterId,
    admissionId: null,
    recordedAt: now,
    bpSystolic: input.bpSystolic,
    bpDiastolic: input.bpDiastolic,
    temperatureC: input.temperatureC,
    pulseRate: input.pulseRate,
    respiratoryRate: input.respiratoryRate,
    heightCm: input.heightCm,
    weightKg: input.weightKg,
    spo2: input.spo2,
    bmi: bmi === null ? null : Math.round(bmi * 10) / 10,
    painScore: null,
    notes: null,
    recordedBy: userID,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: userID,
    updatedBy: userID,
  });

  if (input.encounterId) {
    const encounter = await db.encounters.get(input.encounterId);
    if (encounter && encounter.workflowStage === 'registered') {
      await moveStage(input.encounterId, 'vitals_done');
    }
  }

  return row;
}

/** Moves a visit to a new stage. The server re-checks the move. */
export async function moveStage(encounterId: string, to: string): Promise<void> {
  const encounter = await db.encounters.get(encounterId);
  if (!encounter) throw new Error('That visit is not on this device.');

  if (!nextStages(encounter.workflowStage).includes(to)) {
    throw new Error(
      `A visit cannot move from ${encounter.workflowStage.replaceAll('_', ' ')} to ${to.replaceAll('_', ' ')}.`,
    );
  }

  await localWrite<EncounterRow>('encounters', {
    ...encounter,
    workflowStage: to as EncounterRow['workflowStage'],
    status: to === 'closed' ? 'closed' : encounter.status,
    closedAt: to === 'closed' ? new Date().toISOString() : encounter.closedAt,
  });
}
