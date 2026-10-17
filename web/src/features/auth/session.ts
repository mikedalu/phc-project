import { create } from 'zustand';
import { db } from '@/db';
import { nowIso, type SessionRow } from '@/db/types';
import { ApiError, configureAuth, type RefreshOutcome } from '@/lib/api';
import { loadOrCreateDeviceKey, markDeviceRegistered } from '@/sync/deviceKey';
import * as authApi from './api';
import {
  enrolOfflineCredential,
  refreshOfflineCredentialMetadata,
  verifyOfflinePassword,
} from './offlineCredentials';
import { APP_VERSION, describeDevice, describePlatform } from './device-label';

/**
 * The session store.
 *
 * ═══ WHAT LIVES WHERE, AND WHY ═══
 *
 *  - The ACCESS token is in memory only. It expires in 15 minutes and is
 *    worthless on disk; persisting it would put a bearer credential at rest for
 *    no benefit.
 *  - The REFRESH token is persisted to IndexedDB. Offline operation needs it:
 *    when the network returns, the device trades it for an access token, and a
 *    reload must not lose it.
 *  - The offline deadline is persisted because it came from the SERVER. The
 *    client never computes or extends it.
 *
 * ═══ WHY NOT ZUSTAND's `persist` MIDDLEWARE ═══
 *
 * `persist` hydrates asynchronously in the background, so a route guard can run
 * while `status` still says "loading" and flash a redirect to /login for a user
 * who is signed in. Hydration here is an explicit step (`hydrateSession`) that
 * bootstrap awaits BEFORE the router renders. One less race, and the ordering is
 * visible in the code rather than in a middleware's internals.
 */

export type SessionStatus = 'loading' | 'authenticated' | 'anonymous';

export interface SessionUser {
  userId: string;
  staffNumber: string;
  displayName: string;
  roles: string[];
  deviceId: string;
}

interface SessionState {
  status: SessionStatus;
  user: SessionUser | null;
  /** MEMORY ONLY. Never written to Dexie. */
  accessToken: string | null;
  refreshToken: string | null;
  offlineGraceExpiresAt: string | null;
  lastOnlineAuthAt: string | null;
  mustChangePassword: boolean;
  /** False when the public key has not yet reached the server. */
  deviceKeyRegistered: boolean;
  /** Set when the server says this device has been revoked. */
  deviceRevoked: boolean;
}

export const useSession = create<SessionState>(() => ({
  status: 'loading',
  user: null,
  accessToken: null,
  refreshToken: null,
  offlineGraceExpiresAt: null,
  lastOnlineAuthAt: null,
  mustChangePassword: false,
  deviceKeyRegistered: false,
  deviceRevoked: false,
}));

function toSnapshot(row: SessionRow): SessionUser {
  return {
    userId: row.userId,
    staffNumber: row.staffNumber,
    displayName: row.displayName,
    roles: row.roles,
    deviceId: row.deviceId,
  };
}

/**
 * Writes the session to Dexie and to memory.
 *
 * Called by login, by refresh (which ROTATES the refresh token, so the stored
 * copy must be replaced every time), and by revalidation.
 */
/**
 * Written when an offline sign-in cannot proceed. A distinct type so the login
 * screen can show the right sentence instead of "something went wrong".
 */
export class OfflineSignInError extends Error {
  readonly code:
    | 'NO_CREDENTIAL'
    | 'WRONG_PASSWORD'
    | 'LOCKED'
    | 'WRONG_STAFF_NUMBER'
    | 'NOT_ENROLLED';

  constructor(code: OfflineSignInError['code'], message: string) {
    super(message);
    this.code = code;
    this.name = 'OfflineSignInError';
  }
}

async function persistSession(result: authApi.LoginResult): Promise<void> {
  const row: SessionRow = {
    key: 'current',
    userId: result.user.id,
    staffNumber: result.user.staffNumber,
    displayName: `${result.user.firstName} ${result.user.lastName}`.trim(),
    roles: result.user.roles,
    refreshToken: result.refreshToken,
    deviceId: result.device.id,
    offlineGraceExpiresAt: result.offlineGraceExpiresAt,
    lastOnlineAuthAt: nowIso(),
    sessionId: result.sessionId,
  };

  await db.session.put(row);

  useSession.setState({
    status: 'authenticated',
    user: toSnapshot(row),
    accessToken: result.accessToken,
    refreshToken: row.refreshToken,
    offlineGraceExpiresAt: row.offlineGraceExpiresAt,
    lastOnlineAuthAt: row.lastOnlineAuthAt,
    mustChangePassword: result.user.mustChangePassword,
    deviceRevoked: false,
  });
}

/**
 * Restores the session from Dexie. Awaited during bootstrap, before rendering.
 *
 * The access token is deliberately NOT restored: it is gone, and the first
 * authenticated request will 401 and refresh. That is the designed path, not a
 * failure — and it means a device that starts offline still has a usable session
 * for local reads.
 */
export async function hydrateSession(): Promise<void> {
  const row = await db.session.get('current');

  if (!row) {
    useSession.setState({ status: 'anonymous' });
    return;
  }

  useSession.setState({
    status: 'authenticated',
    user: toSnapshot(row),
    accessToken: null,
    refreshToken: row.refreshToken,
    offlineGraceExpiresAt: row.offlineGraceExpiresAt,
    lastOnlineAuthAt: row.lastOnlineAuthAt,
  });
}

/** True when the server-issued offline window has passed. */
export function isOfflineGraceExpired(): boolean {
  const at = useSession.getState().offlineGraceExpiresAt;
  if (!at) return false;
  return Date.parse(at) <= Date.now();
}

/**
 * Signs in: creates or loads the device key, logs in, then registers the key.
 *
 * Order is forced. The key must exist BEFORE login because its device id is part
 * of the login request; the key can only be REGISTERED after login because
 * `POST /devices` needs an access token.
 */
export async function signIn(input: { staffNumber: string; password: string }): Promise<void> {
  const key = await loadOrCreateDeviceKey();

  const { data } = await authApi.login({
    staffNumber: input.staffNumber.trim(),
    password: input.password,
    deviceId: key.deviceId,
    deviceName: describeDevice(),
  });

  // Persist first: the registration call below needs the access token, and the
  // token only becomes available to apiFetch once the store holds it.
  await persistSession(data);

  if (!key.registered) {
    try {
      await authApi.registerDeviceKey({
        deviceId: key.deviceId,
        publicKey: key.publicKeyB64,
        label: describeDevice(),
        platform: describePlatform(),
        appVersion: APP_VERSION,
      });
      await markDeviceRegistered();
      useSession.setState({ deviceKeyRegistered: true });
    } catch (error) {
      // NOT fatal, and deliberately not rethrown. The user is signed in and can
      // read the local replica; only pushing is blocked until the key lands. A
      // failed key registration that logged the user out would be a worse outcome
      // than a device that cannot sync for one session.
      console.warn('Device key registration failed; will retry at the next sign-in.', error);
      useSession.setState({ deviceKeyRegistered: false });
    }
  } else {
    useSession.setState({ deviceKeyRegistered: true });
  }

  // Enrol for offline sign-in. This is the ONLY moment the plaintext password is
  // available, so it is the only moment a verifier can be derived.
  try {
    await enrolOfflineCredential({
      userId: data.user.id,
      staffNumber: data.user.staffNumber,
      displayName: `${data.user.firstName} ${data.user.lastName}`.trim(),
      roles: data.user.roles,
      offlineGraceExpiresAt: data.offlineGraceExpiresAt,
      password: input.password,
    });
  } catch (error) {
    // Same reasoning as above: the user is signed in, and a missing verifier only
    // means the NEXT sign-in cannot be done offline.
    console.warn('Offline credential enrolment failed.', error);
  }
}

/**
 * Signs in with no network, using the device's stored verifier.
 *
 * The resulting session has NO refresh token — the server never issued one — so
 * it cannot sync. It exists to let the device keep being usable for the rest of
 * its server-issued grace window. When the deadline passes, the banner blocks
 * writes; when the device reconnects, the user signs in online and sync resumes.
 */
export async function signInOffline(input: {
  staffNumber: string;
  password: string;
}): Promise<void> {
  const attempt = await verifyOfflinePassword(input.password);

  if (!attempt.ok) {
    switch (attempt.reason) {
      case 'no_credential':
        throw new OfflineSignInError(
          'NO_CREDENTIAL',
          'This device has not been set up for offline sign-in. Sign in once while connected.',
        );
      case 'locked':
        throw new OfflineSignInError(
          'LOCKED',
          'Too many failed offline attempts. Wait 15 minutes, or connect and sign in online.',
        );
      default:
        throw new OfflineSignInError('WRONG_PASSWORD', 'That password does not match this device.');
    }
  }

  const credential = attempt.credential;

  // Checked AFTER the password so a wrong staff number cannot be probed for free,
  // but it still stops "any password works as any user" on a shared workstation.
  if (credential.staffNumber.toLowerCase() !== input.staffNumber.trim().toLowerCase()) {
    throw new OfflineSignInError(
      'WRONG_STAFF_NUMBER',
      `This device is enrolled for ${credential.staffNumber}, not that account.`,
    );
  }

  const key = await loadOrCreateDeviceKey();

  const row: SessionRow = {
    key: 'current',
    userId: credential.userId,
    staffNumber: credential.staffNumber,
    displayName: credential.displayName,
    roles: credential.roles,
    // Empty, deliberately: no server token exists, so this session cannot sync
    // or call authenticated endpoints. That is the honest representation.
    refreshToken: '',
    deviceId: key.deviceId,
    offlineGraceExpiresAt: credential.offlineGraceExpiresAt,
    lastOnlineAuthAt: credential.lastOnlineAuthAt,
    sessionId: '',
  };

  await db.session.put(row);

  useSession.setState({
    status: 'authenticated',
    user: toSnapshot(row),
    accessToken: null,
    refreshToken: '',
    offlineGraceExpiresAt: row.offlineGraceExpiresAt,
    lastOnlineAuthAt: row.lastOnlineAuthAt,
    deviceKeyRegistered: key.registered,
    deviceRevoked: false,
  });
}

let refreshInFlight: Promise<RefreshOutcome> | null = null;

/**
 * Refreshes the access token, single-flight.
 *
 * Ten parallel requests hitting 401 must cause ONE refresh. The server rotates
 * refresh tokens and treats reuse as theft — it revokes the whole family — so a
 * second concurrent refresh with the same token would log the user out. This
 * function is therefore not an optimisation; it is the correctness fix for
 * refresh-token reuse detection.
 */
export function refreshAccessToken(): Promise<RefreshOutcome> {
  if (refreshInFlight) return refreshInFlight;

  refreshInFlight = doRefresh().finally(() => {
    refreshInFlight = null;
  });

  return refreshInFlight;
}

async function doRefresh(): Promise<RefreshOutcome> {
  const { refreshToken, user } = useSession.getState();

  // An EMPTY refresh token means this is an offline-only session. That is not
  // evidence the session is dead — it is evidence there is nothing to refresh
  // with — so it must NOT trigger sign-out. Reporting 'dead' here would log a
  // nurse out the moment her device regained a signal.
  if (!refreshToken || !user) {
    return { token: null, reason: 'offline' };
  }

  try {
    const { data } = await authApi.refreshSession({
      refreshToken,
      deviceId: user.deviceId,
    });

    // The response carries a ROTATED refresh token. Storing it is mandatory:
    // the old one has just been consumed, and reusing it next time would trip
    // reuse detection and revoke the family.
    await persistSession(data);
    return { token: data.accessToken };
  } catch (error) {
    if (error instanceof ApiError) {
      if (error.isOffline) {
        // The session is fine. The device is offline. Do NOT sign out.
        return { token: null, reason: 'offline' };
      }
      if (error.status === 401 || error.status === 403) {
        return { token: null, reason: 'dead' };
      }
    }
    // A 5xx or an unexpected failure is the server's problem, not proof that the
    // session is dead. Keep it and let the caller retry later.
    return { token: null, reason: 'offline' };
  }
}

/**
 * Ends the session locally, then best-effort revokes it on the server.
 *
 * Local first, and always: if the network is down the user must still be able to
 * get off a shared workstation. The server-side revocation then happens whenever
 * the device next reaches the API (or the refresh token expires on its own).
 */
export async function signOut(): Promise<void> {
  const refreshToken = useSession.getState().refreshToken;

  useSession.setState({
    status: 'anonymous',
    user: null,
    accessToken: null,
    refreshToken: null,
    offlineGraceExpiresAt: null,
    lastOnlineAuthAt: null,
    mustChangePassword: false,
    deviceKeyRegistered: false,
    deviceRevoked: false,
  });

  await db.session.delete('current');

  if (refreshToken) {
    try {
      await authApi.logoutRequest(refreshToken);
    } catch {
      // Offline. The local session is already gone; the refresh token will
      // expire server-side without ever being used again.
    }
  }
}

/**
 * Re-reads the session from the server: current roles, device status, and a new
 * offline deadline.
 *
 * This is how an offline device discovers a revoked role or a revoked device.
 * It is cheap and idempotent, so call it on app start and on reconnect.
 */
export async function revalidateSession(): Promise<void> {
  if (useSession.getState().status !== 'authenticated') return;

  // Offline-only session (signed in with the local verifier). There is nothing
  // to revalidate against, and calling the API would 401 and sign the user out
  // for the crime of reconnecting.
  if (!useSession.getState().refreshToken) return;

  try {
    const { data } = await authApi.fetchSession();

    if (data.device.revoked) {
      useSession.setState({ deviceRevoked: true });
      await signOut();
      return;
    }

    const row = await db.session.get('current');
    if (!row) return;

    const updated: SessionRow = {
      ...row,
      roles: data.roles,
      offlineGraceExpiresAt: data.offlineGraceExpiresAt,
      lastOnlineAuthAt: nowIso(),
    };
    await db.session.put(updated);

    useSession.setState({
      user: toSnapshot(updated),
      offlineGraceExpiresAt: updated.offlineGraceExpiresAt,
      lastOnlineAuthAt: updated.lastOnlineAuthAt,
      mustChangePassword: data.user.mustChangePassword,
    });

    // Keep the offline verifier's cached window and roles in step, so a device
    // that goes offline tomorrow signs in with today's deadline, not last week's.
    await refreshOfflineCredentialMetadata({
      roles: data.roles,
      offlineGraceExpiresAt: data.offlineGraceExpiresAt,
    });
  } catch (error) {
    if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
      await signOut();
      return;
    }
    // Offline or server trouble: keep working with what we have. The offline
    // deadline check elsewhere decides when that stops being acceptable.
  }
}

// Register the bridge once, at module init. From here on, lib/api.ts can attach
// the token and refresh it without importing this file — no import cycle.
configureAuth({
  getAccessToken: () => useSession.getState().accessToken,
  refresh: refreshAccessToken,
  onSessionLost: () => {
    void signOut();
  },
});
