import { apiGet, apiPost } from '@/lib/api';

/**
 * Auth endpoint shapes. These mirror docs/07-API-CONTRACT.md §3 exactly; if the
 * server changes, this file changes with it.
 */

export interface AuthUser {
  id: string;
  staffNumber: string;
  firstName: string;
  lastName: string;
  email?: string;
  roles: string[];
  mustChangePassword: boolean;
}

export interface LoginResult {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
  user: AuthUser;
  device: { id: string; name?: string };
  sessionId: string;
  offlineGraceExpiresAt: string;
}

export interface LoginInput {
  staffNumber: string;
  password: string;
  deviceId: string;
  deviceName: string;
}

/**
 * POST /auth/login.
 *
 * `skipAuth` because there is no token yet, and a retry-with-refresh loop on the
 * login endpoint is how a client hammers the server during an outage.
 */
export function login(input: LoginInput) {
  return apiPost<LoginResult>('/auth/login', input, {
    skipAuth: true,
    timeoutMs: 20_000,
  });
}

export function refreshSession(input: { refreshToken: string; deviceId: string }) {
  return apiPost<LoginResult>('/auth/refresh', input, { skipAuth: true });
}

export function logoutRequest(refreshToken: string) {
  return apiPost<void>('/auth/logout', { refreshToken }, { skipAuth: true });
}

export interface RegisteredDevice {
  id: string;
  name: string;
  platform?: string;
  algorithm: string;
  publicKeyFingerprint: string;
  registered: boolean;
  revoked: boolean;
  lastSeenAt?: string;
  createdAt: string;
}

export interface RegisterDeviceInput {
  deviceId: string;
  publicKey: string;
  label: string;
  platform?: string;
  appVersion?: string;
}

/**
 * POST /devices — bind this workstation's public key.
 *
 * Authenticated with the access token from login, which is why this is called
 * immediately afterwards and not before.
 */
export function registerDeviceKey(input: RegisterDeviceInput) {
  return apiPost<RegisteredDevice>('/devices', {
    deviceId: input.deviceId,
    publicKey: input.publicKey,
    algorithm: 'ES256',
    label: input.label,
    platform: input.platform,
    appVersion: input.appVersion,
  });
}

export interface ServerSession {
  user: AuthUser;
  roles: string[];
  device: { id: string; revoked: boolean };
  offlineGraceExpiresAt: string;
}

/** GET /auth/session — who am I, and is this device still trusted? */
export function fetchSession() {
  return apiGet<ServerSession>('/auth/session');
}
