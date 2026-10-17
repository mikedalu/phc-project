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

/**
 * Keys in the `syncMeta` k/v store.
 *
 * Note what is deliberately NOT here: `deviceId`. The device id lives in the
 * `deviceKeys` row and nowhere else. Two sources of truth for an identity value
 * is how one half of the app ends up signing as a device the server has never
 * seen.
 */
export type SyncMetaKey =
  | 'pullCursor'
  | 'lastPushAt'
  | 'lastPullAt'
  | 'lastError'
  | 'bootstrapped'
  | 'offlineSince'
  /** Cached copy of the server's OFFLINE_POLICY, so the rule works offline. */
  | 'offlinePolicy';

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

/**
 * The device's ECDSA P-256 keypair. ONE row, `id = 'local'`.
 *
 * This is the most sensitive row in the local database: it signs every sync
 * push. It is stored as an exported JWK because a non-extractable `CryptoKey`
 * cannot be persisted — the bytes cannot be read out, so they cannot be written
 * down. docs/05-SECURITY.md §4.2 weighs the options; Option B is this one,
 * chosen because a PHC workstation reboots daily and re-registering a key on
 * every boot would make the device control unusable.
 *
 * The JWK is exported exactly ONCE, at creation. On every subsequent load it is
 * re-imported non-extractable, so for the whole session the key cannot be read
 * back out by JavaScript — including by this app.
 */
export interface DeviceKeyRow {
  /** Constant 'local'. There is exactly one identity per browser profile. */
  id: 'local';
  /** Sent to the server; also the JWT's `device_id` claim. */
  deviceId: string;
  algorithm: 'ES256';
  /** Private key as JWK. Never log this, never sync it, never put it in a URL. */
  privateKeyJwk: JsonWebKey;
  /** base64url SPKI DER — the exact string POST /api/v1/devices stores. */
  publicKeyB64: string;
  /** SHA-256 of the SPKI DER, lowercase hex. The admin-facing identity of this key. */
  fingerprint: string;
  /** False until the server has confirmed it stored this public key. */
  registered: boolean;
  createdAt: string;
}

/**
 * The signed-in session, as it survives a reload. ONE row, `key = 'current'`.
 *
 * ═══ WHAT IS DELIBERATELY ABSENT ═══
 * The ACCESS token. It lives in memory only, because it is worthless after
 * fifteen minutes and writing it down would put a bearer credential at rest for
 * no benefit. The REFRESH token is here because offline operation requires it:
 * the device must be able to trade it for an access token when the network comes
 * back, and that trade cannot happen if the token was lost on reload.
 *
 * ═══ THE OFFLINE DEADLINE IS SERVER-ISSUED ═══
 * `offlineGraceExpiresAt` comes from the API, never from this device's clock.
 * A device that rolls its clock back can only make the local check stricter,
 * never looser — which is the whole point (docs/05-SECURITY.md §3.5).
 */
export interface SessionRow {
  key: 'current';
  userId: string;
  staffNumber: string;
  displayName: string;
  roles: string[];
  refreshToken: string;
  /** The JWT's device_id claim; also the signing device. */
  deviceId: string;
  /** ISO-8601, from the server. */
  offlineGraceExpiresAt: string;
  /** ISO-8601. When this device last successfully authenticated online. */
  lastOnlineAuthAt: string;
  sessionId: string;
}

/** Generates a time-ordered UUID. The ONLY way IDs are created in this app. */
export function newId(): string {
  return uuidv7();
}

/** Single source of "now". Never call new Date() inline — see CODING-STANDARDS. */
export function nowIso(): string {
  return new Date().toISOString();
}
