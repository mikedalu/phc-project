/**
 * Byte/string conversions, in one place.
 *
 * These look trivial and are not. WebCrypto hands you an ArrayBuffer, IndexedDB
 * will happily store an empty Uint8Array, and the server compares base64url
 * strings. One definition of each conversion means one place to get it wrong.
 *
 * The signature contract (docs/02-SYNC-PROTOCOL.md §5.2) depends on these bytes
 * byte-for-byte, so do not inline a conversion "just here".
 */

/**
 * Unpadded base64url (RFC 4648 §5).
 *
 * UNPADDED is not a preference. The signature travels in an HTTP header and is
 * compared as text; padding on one side and not the other is a mismatch that
 * looks like a broken key. `+/` become `-_` for the same reason — every
 * base64url implementation you will ever compare against uses `-_`.
 */
export function toBase64Url(bytes: Uint8Array): string {
  let binary = '';
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

/** Decodes unpadded base64url. Throws rather than returning partial bytes. */
export function fromBase64Url(text: string): Uint8Array {
  const normalised = text.replace(/-/g, '+').replace(/_/g, '/');
  const padded = normalised.padEnd(Math.ceil(normalised.length / 4) * 4, '=');

  let binary: string;
  try {
    binary = atob(padded);
  } catch {
    throw new Error('Invalid base64url input.');
  }

  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    out[i] = binary.charCodeAt(i);
  }
  return out;
}

/**
 * Lowercase hex.
 *
 * Used where the CONTRACT says hex — database fingerprints. Never for sync
 * signatures, which are base64url. Mixing the two is the interop trap this file
 * exists to prevent.
 */
export function toHex(bytes: Uint8Array): string {
  let out = '';
  for (const byte of bytes) {
    out += byte.toString(16).padStart(2, '0');
  }
  return out;
}
