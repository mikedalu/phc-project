import { uuidv7 } from 'uuidv7';

/** A row generated on another device, arriving via sync. */
export interface BaseRow {
  id: string;
  /** Client-side edit counter. Advisory — the server's serverRev is authority. */
  revision: number;
  /** Server-assigned, monotonic per row. Assigned by the API, never by us. */
  serverRev: number;
  /** 'pending' until pushed and acknowledged; 'conflict' if the server refused. */
  syncState: 'pending' | 'synced' | 'conflict';
  /** Soft delete ONLY. A row is never hard-deleted, so the tombstone replicates. */
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

/** Domain-specific columns are added per table; see the interfaces below. */

export interface PatientRow extends BaseRow {
  patientNumber: string;
  householdId: string | null;
  firstName: string;
  middleName: string | null;
  surname: string;
  dateOfBirth: string | null;
  ageYears: number | null;
  ageMonths: number | null;
  ageRecordedAt: string | null;
  gender: 'male' | 'female' | 'other' | 'unknown';
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
  chronicConditions: string | null;
  isDeceased: boolean;
  registrationDate: string;
  isActive: boolean;
}

export interface HouseholdRow extends BaseRow {
  familyNumber: string;
  familyName: string;
  headPatientId: string | null;
  phone: string | null;
  address: string | null;
  notes: string | null;
  isActive: boolean;
}

export type WorkflowStage =
  | 'registered'
  | 'vitals_done'
  | 'awaiting_doctor'
  | 'in_consultation'
  | 'awaiting_lab'
  | 'lab_done'
  | 'awaiting_lab_payment'
  | 'awaiting_pharmacy'
  | 'awaiting_drug_payment'
  | 'pharmacy_done'
  | 'admitted'
  | 'closed';

export interface EncounterRow extends BaseRow {
  visitNumber: string;
  patientId: string;
  householdId: string | null;
  encounterType: 'opd' | 'anc' | 'immunisation' | 'emergency' | 'follow_up' | 'inpatient';
  workflowStage: WorkflowStage;
  status: 'active' | 'closed' | 'cancelled';
  priority: 0 | 1 | 2;
  chiefComplaint: string | null;
  queuedAt: string;
  startedAt: string | null;
  closedAt: string | null;
  attendingDoctorId: string | null;
  attendingNurseId: string | null;
  notes: string | null;
}

export interface VitalsRow extends BaseRow {
  patientId: string;
  encounterId: string | null;
  admissionId: string | null;
  recordedAt: string;
  bpSystolic: number | null;
  bpDiastolic: number | null;
  temperatureC: number | null;
  pulseRate: number | null;
  respiratoryRate: number | null;
  heightCm: number | null;
  weightKg: number | null;
  spo2: number | null;
  bmi: number | null;
  painScore: number | null;
  notes: string | null;
  recordedBy: string;
}

/** ────────────────────────────────────────────────────────────────────────
 *  Local-only infrastructure tables. These never sync.
 *  See docs/02-SYNC-PROTOCOL.md §3.1.
 *  ──────────────────────────────────────────────────────────────────────── */

/**
 * One queued local mutation awaiting push.
 *
 * This is the table that makes offline writes work. A row lands here in the
 * SAME Dexie transaction as the domain row — see `localWrite` in ./write.ts.
 * If the two writes are ever separated, a record can exist locally and never
 * reach the server, which looks perfectly fine on that one machine forever.
 */
export interface OutboxEntry {
  /** This is the operation_id sent to the server for idempotency. */
  id: string;
  entityTable: string;
  rowId: string;
  operation: 'insert' | 'update' | 'delete';
  /** The FULL row, not a diff. The server applies it wholesale for LWW tables. */
  payload: unknown;
  /** The serverRev this edit was based on; null for an insert. */
  baseServerRev: number | null;
  status: 'pending' | 'inflight' | 'failed' | 'done';
  attempts: number;
  lastError: string | null;
  /** Epoch ms. Backoff gate, indexed so the drain query stays cheap. */
  nextAttemptAt: number;
  createdAt: number;
  /** The client's claimed updatedAt, kept for clock-skew detection. */
  deviceRecordedAt: string;
}

export type SyncMetaKey =
  | 'pullCursor'
  | 'lastPushAt'
  | 'lastPullAt'
  | 'lastError'
  | 'deviceId'
  | 'bootstrapped'
  | 'offlineSince';

export interface SyncMeta {
  key: SyncMetaKey;
  value: unknown;
}

/** A conflict the server reported, parked for a human to look at. */
export interface LocalConflict {
  id: string;
  entityTable: string;
  rowId: string;
  conflictType: 'lww_lost' | 'lww_won' | 'unique_violation' | 'rejected';
  serverRow: unknown | null;
  localRow: unknown | null;
  reason: string;
  detectedAt: number;
  /** 0/1 rather than boolean so Dexie can index and filter it. */
  acknowledged: 0 | 1;
}

/** Per-device document-number counter, so numbers are unique while offline. */
export interface NumberCounter {
  key: string;
  lastValue: number;
}

/** Generates a time-ordered UUID. The ONLY way IDs are created in this app. */
export function newId(): string {
  return uuidv7();
}

/** Single source of "now". Never call new Date() inline — see CODING-STANDARDS. */
export function nowIso(): string {
  return new Date().toISOString();
}
