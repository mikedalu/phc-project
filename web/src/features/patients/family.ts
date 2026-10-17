import { newId, type HouseholdRow, type PatientRow } from '@/db/types';
import { localWrite, localWriteMany } from '@/db/write';
import { useSession } from '@/features/auth/session';
import { nextFamilyNumber, nextPatientNumber } from './number';
import { deriveAge, todayIso, type NewPatientInput } from './write';

/**
 * Family registration, client side.
 *
 * ═══ WHY LOCAL, NOT THE REST ENDPOINT ═══
 *
 * The server has `POST /patients/family`, and it exists for integrations that
 * are always online. The CLERK is not: they are registering a family in a PHC
 * with an unreliable link, and a form that waits for a round trip — or refuses
 * offline — is a form they will abandon.
 *
 * So the family is written locally, all-or-nothing, and replicates through the
 * outbox like every other record.
 *
 * ═══ THE ORDER MATTERS ═══
 *
 *  1. household, with head_patient_id NULL
 *  2. its patients
 *  3. the household again, now with the head set
 *
 * A household whose head references a patient that has not been pushed yet is a
 * foreign-key violation on the server. The household is therefore enqueued
 * first, and the head is applied as a separate update afterwards.
 */

export interface FamilyMemberInput extends NewPatientInput {
  isHead: boolean;
}

export interface NewFamilyInput {
  familyName: string;
  phone: string | null;
  address: string | null;
  notes: string | null;
  members: FamilyMemberInput[];
}

export interface RegisteredFamily {
  householdId: string;
  patientIds: string[];
}

export async function registerFamily(input: NewFamilyInput): Promise<RegisteredFamily> {
  const userID = useSession.getState().user?.userId;
  if (!userID) {
    throw new Error('Cannot register a family without a signed-in user.');
  }
  if (!input.familyName.trim()) {
    throw new Error('A family name is required.');
  }
  if (input.members.length === 0) {
    throw new Error('A family needs at least one member.');
  }

  const headIndex = Math.max(
    0,
    input.members.findIndex((member) => member.isHead),
  );

  const householdId = newId();
  const familyNumber = await nextFamilyNumber();
  const now = new Date().toISOString();

  const household: HouseholdRow = {
    id: householdId,
    familyNumber,
    familyName: input.familyName.trim(),
    // Set to NULL first — see the ordering note above.
    headPatientId: null,
    phone: input.phone?.trim() || null,
    address: input.address?.trim() || null,
    notes: input.notes?.trim() || null,
    isActive: true,
    revision: 1,
    serverRev: 0,
    syncState: 'pending',
    deletedAt: null,
    createdAt: now,
    updatedAt: now,
    createdBy: userID,
    updatedBy: userID,
  };

  const patients: PatientRow[] = [];

  for (const member of input.members) {
    const derived = deriveAge(member);
    patients.push({
      id: newId(),
      patientNumber: await nextPatientNumber(),
      householdId,
      firstName: member.firstName.trim(),
      middleName: member.middleName?.trim() || null,
      surname: member.surname.trim() || input.familyName.trim(),
      dateOfBirth: derived.dateOfBirth,
      ageYears: derived.years,
      ageMonths: derived.months,
      ageRecordedAt: todayIso(),
      gender: member.gender,
      maritalStatus: member.maritalStatus,
      phone: member.phone?.trim() || input.phone?.trim() || null,
      altPhone: member.altPhone?.trim() || null,
      tribe: member.tribe?.trim() || null,
      occupation: member.occupation?.trim() || null,
      address: member.address?.trim() || input.address?.trim() || null,
      addressLandmark: member.addressLandmark?.trim() || null,
      bloodGroup: member.bloodGroup?.trim() || null,
      genotype: member.genotype?.trim() || null,
      allergies: member.allergies?.trim() || null,
      chronicConditions: null,
      isDeceased: false,
      registrationDate: todayIso(),
      isActive: true,
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

  // Household first, then its members — one transaction.
  await localWriteMany([
    { table: 'households', row: household },
    ...patients.map((patient) => ({ table: 'patients' as const, row: patient })),
  ]);

  // Finally the head, as its own queued update, so the foreign key resolves in
  // order on the server.
  await localWrite('households', {
    ...household,
    headPatientId: patients[headIndex]?.id ?? null,
    updatedAt: new Date().toISOString(),
  });

  return { householdId, patientIds: patients.map((patient) => patient.id) };
}
