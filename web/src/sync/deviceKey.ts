/**
 * The device's ECDSA P-256 identity: keypair, persistence, and request signing.
 *
 * ─── WHY A DEVICE KEY AT ALL, WHEN A JWT ALREADY AUTHENTICATES THE USER ────
 * A JWT answers "is this a valid session for this user?". It cannot answer "is
 * this the same physical machine?". With a full local replica a device IS a data
 * store, so losing a laptop must be revocable WITHOUT locking the nurse out of
 * every other workstation. That surgical lever is what this file buys.
 * See docs/05-SECURITY.md §4.1.
 *
 * ─── THE INTEROP TRAP ─────────────────────────────────────────────────────
 * WebCrypto emits ECDSA signatures as ASN.1 DER. Go must verify them with
 * ecdsa.VerifyASN1 — NOT ecdsa.Verify, which expects the raw r||s form. The
 * wrong one produces a signature that is mathematically perfect and always
 * fails, which costs an afternoon and looks like a key problem.
 *
 * Likewise exportKey('raw') THROWS for P-256 public keys. Export 'spki'.
 *
 * ─── CANONICAL STRING ─────────────────────────────────────────────────────
 * Fixed by docs/02-SYNC-PROTOCOL.md §5.2: six fields joined by "\n", no prefix,
 * no trailing newline. The Go verifier builds the identical string. Do not
 * "tidy" the order or the encoding: a one-character divergence yields a
 * signature that is valid and never verifies.
 */

import { db } from '@/db';
import { newId, nowIso, type DeviceKeyRow } from '@/db/types';
import { toBase64Url, toHex } from '@/lib/encoding';

/** P-256 is fixed by the curve; the server's CHECK constraint accepts ES256 only. */
const GEN_ALGO = { name: 'ECDSA', namedCurve: 'P-256' } as const;
const SIGN_ALGO = { name: 'ECDSA', hash: 'SHA-256' } as const;

/** The three headers a signed request carries. Spread them into fetch headers. */
export interface DeviceSignatureHeaders {
  'X-Device-Signature': string;
  'X-Device-Timestamp': string;
  'X-Device-Nonce': string;
}

/** A loaded keypair, ready to sign. Never contains the JWK. */
export interface DeviceKeyHandle {
  deviceId: string;
  /** Re-imported non-extractable: `privateKey.extractable === false`. */
  privateKey: CryptoKey;
  /** base64url SPKI DER — what the server stores. */
  publicKeyB64: string;
  /** SHA-256 of the SPKI DER, lowercase hex. */
  fingerprint: string;
  registered: boolean;
}

/**
 * Fails loudly outside a secure context.
 *
 * `crypto.subtle` is undefined on plain HTTP. On a LAN over http:// that means
 * device identity does not work at all, and without this check the failure
 * surfaces later as "undefined is not an object" somewhere far from the cause.
 * Deploy HTTPS even internally.
 */
function assertSecureContext(): void {
  if (typeof crypto === 'undefined' || crypto.subtle === undefined) {
    throw new Error(
      'WebCrypto is unavailable. This app requires a secure context (HTTPS or localhost).',
    );
  }
}

async function fingerprintOfSpki(spkiDer: ArrayBuffer): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', spkiDer);
  return toHex(new Uint8Array(digest));
}

async function generateRow(): Promise<DeviceKeyRow> {
  // extractable = true is required for exactly one reason: the JWK has to be
  // exported once in order to be stored. The live key is never used in this
  // form — after storage it is re-imported non-extractable (see restore()).
  const pair = (await crypto.subtle.generateKey(GEN_ALGO, true, [
    'sign',
    'verify',
  ])) as CryptoKeyPair;

  const spkiDer = await crypto.subtle.exportKey('spki', pair.publicKey);

  // Export the JWK and keep it INTACT. Hand-building {kty, crv, x, y, d} works
  // until you forget a field, and then importKey fails with a vague DOMException.
  const privateKeyJwk = await crypto.subtle.exportKey('jwk', pair.privateKey);

  return {
    id: 'local',
    deviceId: newId(),
    algorithm: 'ES256',
    privateKeyJwk,
    publicKeyB64: toBase64Url(new Uint8Array(spkiDer)),
    fingerprint: await fingerprintOfSpki(spkiDer),
    registered: false,
    createdAt: nowIso(),
  };
}

async function restore(row: DeviceKeyRow): Promise<DeviceKeyHandle> {
  // Re-import NON-extractable. After this line the running session cannot export
  // the key again; the JWK in IndexedDB is the only copy of the raw bytes.
  const privateKey = await crypto.subtle.importKey(
    'jwk',
    row.privateKeyJwk,
    GEN_ALGO,
    false,
    ['sign'],
  );

  return {
    deviceId: row.deviceId,
    privateKey,
    publicKeyB64: row.publicKeyB64,
    fingerprint: row.fingerprint,
    registered: row.registered,
  };
}

/**
 * Loads this browser profile's keypair, generating one on first run.
 *
 * Cheap enough to call at bootstrap: one indexed lookup plus one importKey.
 */
export async function loadOrCreateDeviceKey(): Promise<DeviceKeyHandle> {
  assertSecureContext();

  const existing = await db.deviceKeys.get('local');
  if (existing) {
    return restore(existing);
  }

  // Generate OUTSIDE the transaction. generateKey is not a Dexie promise, and
  // awaiting a foreign promise inside a Dexie transaction makes it commit early
  // ("premature commit") and silently drop the write.
  const candidate = await generateRow();

  // Two tabs opened back-to-back on a first run both reach this point. The
  // transaction makes the first writer win; the loser adopts the winner's row.
  // Without this, the second tab signs with a keypair the server has never seen
  // and every push fails verification with a perfectly valid signature.
  const winner = await db.transaction('rw', db.deviceKeys, async () => {
    const current = await db.deviceKeys.get('local');
    if (current) return current;
    await db.deviceKeys.put(candidate);
    return candidate;
  });

  return restore(winner);
}

/** The device id, or null before the keypair has ever been created. */
export async function getDeviceId(): Promise<string | null> {
  const row = await db.deviceKeys.get('local');
  return row?.deviceId ?? null;
}

/** Records that the server has stored this device's public key. */
export async function markDeviceRegistered(): Promise<void> {
  await db.deviceKeys.update('local', { registered: true });
}

/** The fields of the canonical string, in contract order. */
export interface CanonicalParts {
  method: string;
  path: string;
  deviceId: string;
  timestamp: string;
  nonce: string;
  bodyHashB64: string;
}

/**
 * Builds the exact bytes that get signed.
 *
 * docs/02-SYNC-PROTOCOL.md §5.2. Six fields, "\n"-joined, nothing else:
 *
 *   POST\n/api/v1/sync/push\n0192f7…\n2026-02-14T09:31:07.221Z\nkQ7v…\nb64url(sha256(body))
 */
export function buildCanonicalString(p: CanonicalParts): string {
  return [p.method.toUpperCase(), p.path, p.deviceId, p.timestamp, p.nonce, p.bodyHashB64].join(
    '\n',
  );
}

export interface SignedRequestParts {
  privateKey: CryptoKey;
  method: string;
  /** '/api/v1/sync/push' — the path exactly as sent, WITHOUT the query string. */
  path: string;
  deviceId: string;
  /**
   * The EXACT body string being sent. Serialise once and pass that same string
   * to fetch; serialising twice risks different bytes and a failed signature.
   */
  body: string;
}

/**
 * Signs a request and returns the headers to attach.
 *
 * The timestamp is ISO-8601 and must travel VERBATIM in X-Device-Timestamp —
 * that header is one of the signed fields, so a client that re-formats it and a
 * server that re-serialises it will disagree by milliseconds and fail.
 */
export async function signRequest(p: SignedRequestParts): Promise<DeviceSignatureHeaders> {
  assertSecureContext();

  const bodyHash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(p.body));

  const timestamp = new Date().toISOString();
  const nonce = toBase64Url(crypto.getRandomValues(new Uint8Array(32)));

  const canonical = buildCanonicalString({
    method: p.method,
    path: p.path,
    deviceId: p.deviceId,
    timestamp,
    nonce,
    bodyHashB64: toBase64Url(new Uint8Array(bodyHash)),
  });

  const signature = await crypto.subtle.sign(
    SIGN_ALGO,
    p.privateKey,
    new TextEncoder().encode(canonical),
  );

  return {
    'X-Device-Signature': toBase64Url(new Uint8Array(signature)),
    'X-Device-Timestamp': timestamp,
    'X-Device-Nonce': nonce,
  };
}
