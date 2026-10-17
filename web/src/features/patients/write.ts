import { localWrite } from '@/db/write';
import { newId, type PatientRow } from '@/db/types';
import { useSession } from '@/features/auth/session';
import { nextPatientNumber } from './number';

/**
 * Registers a patient on this device.
 *
 * Goes through `localWrite`, so the row and its outbox entry commit together and
 * the record reaches the server on the next drain. Registration never waits for
 * the network.
 */

export interface NewPatientInput {
  firstName: string;
  middleName: string | null;
  surname: string;
  gender: 'male' | 'female' | 'other' | 'unknown';
  /** Exactly one of these is required. */
  dateOfBirth: string | null; // YYYY-MM-DD
  ageYears: number | null;
  ageMonths: number | null;
  maritalStatus: string | null;
  phone: string | null;
  altPhone: string | null;
  tribe: string | null;
  occupation: string | null;
  address: string | null;
  addressLandmark: string | null;
  bloodGroup: string | null;
  genotype: string | null;
  allergies: string | null;
  hospitalNumber?: string;
}

/** Today as YYYY-MM-DD in LOCAL time — a date of birth is a calendar date. */
export function todayIso(): string {
  const now = new Date();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const day = String(now.getDate()).padStart(2, '0');
  return `${now.getFullYear()}-${month}-${day}`;
}

/**
 * Derives age from a date of birth, or a date of birth from an age.
 *
 * Mirrors `deriveAge` in the server's patient service. Both sides implement it
 * because both can be the first to see a registration: the device when offline,
 * the server when a client sends only one of the two.
 */
export function deriveAge(input: {
  dateOfBirth: string | null;
  ageYears: number | null;
  ageMonths: number | null;
}): { dateOfBirth: string; years: number; months: number } {
  if (input.dateOfBirth) {
    const dob = new Date(`${input.dateOfBirth}T00:00:00`);
    const now = new Date();
    let months = (now.getFullYear() - dob.getFullYear()) * 12 + (now.getMonth() - dob.getMonth());
    if (now.getDate() < dob.getDate()) months -= 1;
    if (months < 0) months = 0;
    return { dateOfBirth: input.dateOfBirth, years: Math.floor(months / 12), months };
  }

  const total = (input.ageYears ?? 0) * 12 + (input.ageMonths ?? 0);
  const approx = new Date();
  approx.setMonth(approx.getMonth() - total);
  const iso = `${approx.getFullYear()}-${String(approx.getMonth() + 1).padStart(2, '0')}-${String(
    approx.getDate(),
  ).padStart(2, '0')}`;
  return { dateOfBirth: iso, years: Math.floor(total / 12), months: total };
}

export async function registerPatient(input: NewPatientInput): Promise<PatientRow> {
  const userID = useSession.getState().user?.userId;
  if (!userID) {
    throw new Error('Cannot register a patient without a signed-in user.');
  }

  const derived = deriveAge(input);
  const now = new Date().toISOString();

  const row: PatientRow = {
    id: newId(),
    patientNumber: input.hospitalNumber || (await nextPatientNumber()),
    householdId: null,
    firstName: input.firstName.trim(),
    middleName: input.middleName?.trim() || null,
    surname: input.surname.trim(),
    dateOfBirth: derived.dateOfBirth,
    ageYears: derived.years,
    ageMonths: derived.months,
    ageRecordedAt: todayIso(),
    gender: input.gender,
    maritalStatus: input.maritalStatus,
    phone: input.phone?.trim() || null,
    altPhone: input.altPhone?.trim() || null,
    tribe: input.tribe?.trim() || null,
    occupation: input.occupation?.trim() || null,
    address: input.address?.trim() || null,
    addressLandmark: input.addressLandmark?.trim() || null,
    bloodGroup: input.bloodGroup?.trim() || null,
    genotype: input.genotype?.trim() || null,
    allergies: input.allergies?.trim() || null,
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
  };

  return localWrite('patients', row);
}

/**
 * Looks for people who might already be registered.
 *
 * The same forgiving rule the server uses: same surname (case-insensitive), a
 * first name with the same initial, and either a shared date of birth or a
 * shared phone. ADVISORY ONLY — the caller warns and lets the clerk decide.
 */
export async function findLocalDuplicates(input: {
  firstName: string;
  surname: string;
  dateOfBirth: string | null;
  phone: string | null;
}): Promise<PatientRow[]> {
  const surname = input.surname.trim().toLowerCase();
  const initial = input.firstName.trim().charAt(0).toLowerCase();
  if (!surname || !initial) return [];

  const candidates = await import('@/db').then(({ db }) =>
    db.patients.where('patientNumber').notEqual('').toArray(),
  );

  return candidates.filter((row) => {
    if (row.surname.toLowerCase() !== surname) return false;
    if (!row.firstName.toLowerCase().startsWith(initial)) return false;
    const sameDob = input.dateOfBirth !== null && row.dateOfBirth === input.dateOfBirth;
    const samePhone = input.phone !== null && row.phone === input.phone;
    return sameDob || samePhone || (input.dateOfBirth === null && input.phone === null);
  });
}
