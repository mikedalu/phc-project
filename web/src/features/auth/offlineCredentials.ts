import { argon2id } from 'hash-wasm';
import { db } from '@/db';
import { nowIso, type OfflineCredentialRow } from '@/db/types';
import { apiPost } from '@/lib/api';
import { fromBase64Url, toBase64Url } from '@/lib/encoding';

/**
 * Offline sign-in.
 *
 * ═══ THE PROBLEM ═══
 *
 * A PHC loses its uplink during a clinic. If signing in requires the server, the
 * device becomes a brick exactly when it is needed most. So the device keeps a
 * VERIFIER — an Argon2id hash of the password, salted with 16 random bytes this
 * device generated and never sent anywhere — and can check a password with no
 * network at all.
 *
 * ═══ WHY A VERIFIER AND NOT A HASH ═══
 *
 * Never store the server's password hash: it is a credential equivalent, and
 * anyone who reads the local replica could present it at the login endpoint.
 * Never store the password. A device-salted verifier is neither, so it cannot be
 * replayed anywhere (docs/05-SECURITY.md §3.1).
 *
 * ═══ THE RESIDUAL RISK, STATED PLAINLY ═══
 *
 * Someone with the device file can guess passwords offline at Argon2id cost. We
 * raise that cost three ways: memory-hard derivation, a 5-attempt lockout, and
 * the fact that the whole credential expires with the offline grace window.
 * Encryption at rest (full-disk encryption, a deployment requirement) is the
 * control that actually protects the file.
 */

/** Mirrors the server's account-lockout policy. */
const MAX_ATTEMPTS = 5;
const LOCKOUT_MS = 15 * 60 * 1000;

interface OfflineParams {
  algorithm: string;
  version: number;
  memoryKiB: number;
  iterations: number;
  parallelism: number;
  saltLengthBytes: number;
  keyLengthBytes: number;
}

/** Cached for the session; the parameters change only on a server release. */
let cachedParams: OfflineParams | null = null;

async function fetchParams(): Promise<OfflineParams> {
  if (cachedParams) return cachedParams;

  const { data } = await apiPost<OfflineParams>('/auth/offline-verify', undefined, {
    skipAuth: true,
    timeoutMs: 10_000,
  });
  cachedParams = data;
  return data;
}

/**
 * Derives the verifier. The expensive step — 64 MiB and 3 passes, ~100-300 ms in
 * WASM. Called once at enrolment and once per offline sign-in attempt.
 */
async function derive(
  password: string,
  salt: Uint8Array,
  params: Pick<OfflineParams, 'memoryKiB' | 'iterations' | 'parallelism'>,
): Promise<string> {
  const hex = await argon2id({
    password,
    salt,
    parallelism: params.parallelism,
    iterations: params.iterations,
    memorySize: params.memoryKiB,
    hashLength: 32,
    outputType: 'hex',
  });
  return hex;
}

/**
 * Comparison that does not leak WHERE two strings differ.
 *
 * JavaScript cannot promise true constant time — the JIT and the GC do what they
 * like — but accumulating the difference and comparing once at the end removes
 * the early-exit that makes a byte-by-byte comparison hopeless.
 */
function constantTimeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i += 1) {
    diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }
  return diff === 0;
}

export interface EnrolmentInput {
  userId: string;
  staffNumber: string;
  displayName: string;
  roles: string[];
  offlineGraceExpiresAt: string;
  password: string;
}

/**
 * Creates or refreshes the verifier. Called ONLY after a successful ONLINE
 * sign-in, where the plaintext password is available exactly this once.
 *
 * The salt is regenerated when the user changes, but not on every sign-in: an
 * unchanged salt means an unchanged verifier, which keeps the cost off the login
 * path for a returning user.
 */
export async function enrolOfflineCredential(input: EnrolmentInput): Promise<void> {
  const params = await fetchParams();

  const existing = await db.localCredentials.get('local');
  const keepSalt = existing !== undefined && existing.userId === input.userId;

  const salt = keepSalt ? fromBase64Url(existing.saltB64) : crypto.getRandomValues(new Uint8Array(16));
  const verifierHex = await derive(input.password, salt, params);

  const row: OfflineCredentialRow = {
    id: 'local',
    userId: input.userId,
    staffNumber: input.staffNumber,
    displayName: input.displayName,
    roles: input.roles,
    saltB64: toBase64Url(salt),
    verifierHex,
    params: {
      memoryKiB: params.memoryKiB,
      iterations: params.iterations,
      parallelism: params.parallelism,
    },
    offlineGraceExpiresAt: input.offlineGraceExpiresAt,
    lastOnlineAuthAt: nowIso(),
    createdAt: existing?.createdAt ?? nowIso(),
    failedAttempts: 0,
    lockedUntil: null,
  };

  await db.localCredentials.put(row);
}

/** Keeps the credential's cached grace window and roles in step with the server. */
export async function refreshOfflineCredentialMetadata(input: {
  roles: string[];
  offlineGraceExpiresAt: string;
}): Promise<void> {
  const existing = await db.localCredentials.get('local');
  if (!existing) return;

  await db.localCredentials.put({
    ...existing,
    roles: input.roles,
    offlineGraceExpiresAt: input.offlineGraceExpiresAt,
    lastOnlineAuthAt: nowIso(),
  });
}

export async function hasOfflineCredential(): Promise<boolean> {
  return (await db.localCredentials.get('local')) !== undefined;
}

export type OfflineVerifyResult =
  | { ok: true; credential: OfflineCredentialRow }
  | { ok: false; reason: 'no_credential' | 'locked' | 'mismatch'; lockedUntil?: string };

/**
 * Checks a password against the stored verifier.
 *
 * Lockout is enforced HERE, before the expensive derivation, so a script cannot
 * use this as a free password oracle at full speed.
 */
export async function verifyOfflinePassword(password: string): Promise<OfflineVerifyResult> {
  const credential = await db.localCredentials.get('local');
  if (!credential) return { ok: false, reason: 'no_credential' };

  const now = Date.now();
  if (credential.lockedUntil && Date.parse(credential.lockedUntil) > now) {
    return { ok: false, reason: 'locked', lockedUntil: credential.lockedUntil };
  }

  const salt = fromBase64Url(credential.saltB64);
  const candidate = await derive(password, salt, credential.params);

  if (constantTimeEqual(candidate, credential.verifierHex)) {
    await db.localCredentials.put({
      ...credential,
      failedAttempts: 0,
      lockedUntil: null,
    });
    return { ok: true, credential };
  }

  const failedAttempts = credential.failedAttempts + 1;
  const lockedUntil =
    failedAttempts >= MAX_ATTEMPTS ? new Date(now + LOCKOUT_MS).toISOString() : null;

  await db.localCredentials.put({ ...credential, failedAttempts, lockedUntil });

  // The attempt that TRIGGERS the lockout reports the lockout, not another wrong
  // password. Otherwise the person sees "wrong password" on the fifth attempt,
  // types it again, and only then learns they are locked out for fifteen minutes.
  if (lockedUntil) {
    return { ok: false, reason: 'locked', lockedUntil };
  }

  return { ok: false, reason: 'mismatch' };
}

/** Removes the verifier. Only for device decommissioning, not ordinary logout. */
export async function clearOfflineCredential(): Promise<void> {
  await db.localCredentials.delete('local');
}

/** Exposed for a future "device trust" screen; currently used by tests. */
export async function readOfflineCredential(): Promise<OfflineCredentialRow | undefined> {
  return db.localCredentials.get('local');
}

