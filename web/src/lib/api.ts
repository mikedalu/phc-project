/**
 * The single place that knows how to talk to the API.
 *
 * Every error the server returns uses the same envelope
 * (`{ error: { code, message, details, requestId } }` — see
 * docs/07-API-CONTRACT.md §1.3), so it is unwrapped exactly once, here.
 *
 * ═══ WHY THE AUTH HOOK EXISTS ═══
 *
 * This module must attach the access token and, on expiry, refresh it once and
 * replay the request. But the token and the refresh token live in the session
 * store, which itself calls this module. Importing the store here would create a
 * cycle, and the usual workaround — a global `window.token` — is worse.
 *
 * So the direction is inverted: the store calls `configureAuth(...)` at module
 * init and this file only ever calls through the interface. One registration,
 * one direction of dependency, no cycle.
 */

/**
 * In development this is a relative path and Vite proxies /api to :8080, so the
 * browser sees a single origin and CORS never applies locally. In production
 * VITE_API_URL points at the Cloud Run service and CORS does apply.
 *
 * `||`, NOT `??`. `web/.env` commits `VITE_API_URL=` as an EMPTY STRING for
 * local development, and `??` only falls back on null/undefined — so with `??`
 * every local request would go to `/auth/login` instead of `/api/v1/auth/login`
 * and 404 against the Vite dev server.
 */
const API_BASE = import.meta.env.VITE_API_URL || '/api/v1';

/**
 * The origin root, for the two endpoints that are NOT under /api/v1.
 *
 * `/health` and `/ready` are mounted at the root because Cloud Run's probes
 * expect them there — they are infrastructure, not part of the versioned API.
 * Prefixing them with /api/v1 produces a 404 that the dashboard reports as a
 * failed check, which looks like the server is down when it is fine.
 */
const ROOT_BASE = API_BASE.endsWith('/api/v1') ? API_BASE.slice(0, -'/api/v1'.length) : API_BASE;

export interface ApiErrorBody {
  code?: string;
  message?: string;
  details?: unknown;
  requestId?: string;
}

/**
 * A typed API failure.
 *
 * `code` is stable and machine-readable — switch on it.
 * `message` is human-readable and may change — never switch on it.
 * `status` is the HTTP status, useful for coarse branching (401 → re-auth).
 */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly details: unknown;
  readonly requestId: string | undefined;

  constructor(
    code: string,
    message: string,
    status: number,
    details?: unknown,
    requestId?: string,
  ) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.details = details;
    this.requestId = requestId;
  }

  /** True when retrying could plausibly succeed. */
  get isRetryable(): boolean {
    return this.status >= 500 || this.status === 429 || this.code === 'NETWORK_ERROR';
  }

  /** True when the failure is the absence of a network, not a rejection. */
  get isOffline(): boolean {
    return this.code === 'NETWORK_ERROR' || this.code === 'REQUEST_TIMEOUT';
  }
}

/**
 * The path a signed request must use in its canonical string.
 *
 * The signature covers the PATH, not the origin, so an absolute production API
 * URL has to be reduced to its pathname. Getting this wrong fails verification
 * only in production, which is the worst place to discover it.
 */
export function apiPath(path: string): string {
  if (API_BASE.startsWith('http://') || API_BASE.startsWith('https://')) {
    try {
      const base = new URL(API_BASE).pathname.replace(/\/$/, '');
      return `${base}${path}`;
    } catch {
      return path;
    }
  }
  return `${API_BASE}${path}`;
}

/* ─── Auth bridge ────────────────────────────────────────────────────── */

/**
 * The result of an attempted refresh.
 *
 * The `reason` is not decoration. A refresh can fail because the network is
 * gone (the session is FINE, the device is simply offline) or because the server
 * rejected the refresh token (the session is DEAD). Treating both as "dead"
 * would sign a nurse out every time the wifi dropped, which is the single worst
 * bug an offline-first app can have.
 */
export type RefreshOutcome =
  | { token: string; reason?: undefined }
  | { token: null; reason: 'offline' | 'dead' };

export interface AuthBridge {
  /** The in-memory access token, or null when signed out. */
  getAccessToken: () => string | null;
  /**
   * Obtains a new access token.
   *
   * MUST be single-flight: ten parallel 401s must cause one refresh, not ten.
   * The store guarantees that; this module relies on it.
   */
  refresh: () => Promise<RefreshOutcome>;
  /** Called when refresh proves the session is dead (not when merely offline). */
  onSessionLost?: () => void;
}

let authBridge: AuthBridge | null = null;

/** Registers the session store as the token source. Called once at module init. */
export function configureAuth(bridge: AuthBridge | null): void {
  authBridge = bridge;
}

/* ─── Envelope ───────────────────────────────────────────────────────── */

interface ApiMeta {
  requestId?: string;
  serverTime?: string;
  pagination?: { limit: number; hasMore: boolean; nextCursor?: string };
}

interface ApiEnvelope<T> {
  data: T;
  meta?: ApiMeta;
}

export interface ApiResult<T> {
  data: T;
  meta: ApiMeta | undefined;
}

export interface ApiFetchOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
  /**
   * A body that is ALREADY serialised. Used by the sync push, where the exact
   * bytes must be signed and then sent unchanged: passing an object would let
   * this module serialise it a second time, and any difference in key order or
   * spacing invalidates the signature.
   */
  rawBody?: string;
  /** Milliseconds. Sync pushes need a long timeout because of Cloud Run cold starts. */
  timeoutMs?: number;
  /** Value for the Idempotency-Key header, for money-affecting POSTs. */
  idempotencyKey?: string;
  /** Skip the Authorization header: login, refresh, and public referral reads. */
  skipAuth?: boolean;
  /** Override the path prefix. Only /health and /ready need this. */
  basePath?: string;
}

type Outcome<T> = { result: ApiResult<T>; error?: undefined } | { result?: undefined; error: ApiError };

/** Performs exactly one HTTP exchange. No retries, no token logic. */
async function send<T>(
  path: string,
  options: ApiFetchOptions,
  accessToken: string | null,
): Promise<Outcome<T>> {
  const {
    body,
    rawBody,
    timeoutMs = 30_000,
    idempotencyKey,
    headers,
    skipAuth: _skipAuth,
    basePath = API_BASE,
    ...init
  } = options;

  // AbortSignal.timeout is widely supported and avoids hand-rolled timer
  // bookkeeping. A hung fetch to a cold Cloud Run instance would otherwise
  // leave the UI waiting indefinitely.
  const signal = init.signal ?? AbortSignal.timeout(timeoutMs);

  const requestHeaders: Record<string, string> = {
    Accept: 'application/json',
    ...(headers as Record<string, string> | undefined),
  };

  if (accessToken) {
    requestHeaders['Authorization'] = `Bearer ${accessToken}`;
  }
  if (body !== undefined || rawBody !== undefined) {
    requestHeaders['Content-Type'] = 'application/json';
  }
  if (idempotencyKey) {
    requestHeaders['Idempotency-Key'] = idempotencyKey;
  }

  let response: Response;
  try {
    response = await fetch(`${basePath}${path}`, {
      ...init,
      signal,
      headers: requestHeaders,
      body: rawBody ?? (body === undefined ? undefined : JSON.stringify(body)),
    });
  } catch (cause) {
    // A network failure or timeout is NOT an ApiError from the server; it is
    // the absence of one. Offline-first code must distinguish the two: one is
    // an expected state, the other is a real rejection.
    const aborted = cause instanceof DOMException && cause.name === 'TimeoutError';
    return {
      error: new ApiError(
        aborted ? 'REQUEST_TIMEOUT' : 'NETWORK_ERROR',
        aborted
          ? 'The server took too long to respond. It may be starting up.'
          : 'Could not reach the server. Work continues offline and will sync later.',
        0,
      ),
    };
  }

  // 204 has no body to parse.
  if (response.status === 204) {
    return { result: { data: undefined as T, meta: undefined } };
  }

  let parsed: unknown = null;
  try {
    parsed = await response.json();
  } catch {
    parsed = null;
  }

  if (!response.ok) {
    const envelope = parsed as { error?: ApiErrorBody } | null;
    const err = envelope?.error;

    return {
      error: new ApiError(
        err?.code ?? 'UNKNOWN_ERROR',
        err?.message ?? `Request failed with status ${response.status}.`,
        response.status,
        err?.details,
        err?.requestId,
      ),
    };
  }

  const envelope = parsed as ApiEnvelope<T>;
  return { result: { data: envelope.data, meta: envelope.meta } };
}

/**
 * Performs a request and unwraps the envelope.
 *
 * Throws ApiError on any non-2xx response, so callers never have to check
 * `res.ok` themselves.
 *
 * An expired access token is refreshed ONCE and the request replayed. Everything
 * else — including a 401 for a bad token — is thrown, because retrying a
 * rejected credential is how a client ends up in an infinite loop hammering the
 * login endpoint.
 */
export async function apiFetch<T>(
  path: string,
  options: ApiFetchOptions = {},
): Promise<ApiResult<T>> {
  let token = options.skipAuth ? null : (authBridge?.getAccessToken() ?? null);

  // After a reload the store holds a REFRESH token but no access token — the
  // access token is memory-only by design. Without this, the first request would
  // go out with no Authorization header, the server would answer UNAUTHENTICATED
  // rather than TOKEN_EXPIRED, the refresh branch below would never run, and a
  // signed-in user would look signed out on every page load.
  if (!token && !options.skipAuth && authBridge) {
    const outcome = await authBridge.refresh();
    if (outcome.token) {
      token = outcome.token;
    } else if (outcome.reason === 'dead') {
      authBridge.onSessionLost?.();
    }
  }

  const first = await send<T>(path, options, token);
  if (first.result) return first.result;

  const error = first.error;
  const expired = error.code === 'TOKEN_EXPIRED';

  if (expired && !options.skipAuth && authBridge) {
    const outcome = await authBridge.refresh();

    if (outcome.token) {
      const second = await send<T>(path, options, outcome.token);
      if (second.result) return second.result;
      throw second.error;
    }

    if (outcome.reason === 'dead') {
      // The refresh token itself was rejected (revoked, reused, or the account
      // was suspended). The session is over.
      authBridge.onSessionLost?.();
      throw error;
    }

    // Offline. The session is intact; this request simply cannot be completed.
    // Report it as a network failure so the caller can show "offline" rather
    // than bouncing the user to a login screen.
    throw new ApiError(
      'NETWORK_ERROR',
      'The session could not be refreshed while offline. Work continues locally.',
      0,
    );
  }

  throw error;
}

/** GET helper — the common case. */
export function apiGet<T>(path: string, options?: ApiFetchOptions): Promise<ApiResult<T>> {
  return apiFetch<T>(path, { ...options, method: 'GET' });
}

/** POST helper. */
export function apiPost<T>(
  path: string,
  body?: unknown,
  options?: ApiFetchOptions,
): Promise<ApiResult<T>> {
  return apiFetch<T>(path, { ...options, method: 'POST', body });
}

/** PATCH helper — partial updates, used by admin screens. */
export function apiPatch<T>(
  path: string,
  body?: unknown,
  options?: ApiFetchOptions,
): Promise<ApiResult<T>> {
  return apiFetch<T>(path, { ...options, method: 'PATCH', body });
}

/* ─── Response shapes for the endpoints that exist today ─────────────── */

/** GET /meta — everything the client needs to configure itself at boot. */
export interface MetaResponse {
  schemaVersion: number;
  serverTime: string;
  referralTtlHours: number;
  offlineGracePeriodHours: number;
  offlinePolicy: 'block_writes' | 'warn_only';
  syncMaxBatchSize: number;
  minClientVersion: string;
  serverVersion: string;
  facilityCount: number;
}

export function fetchMeta(): Promise<ApiResult<MetaResponse>> {
  return apiGet<MetaResponse>('/meta');
}

/** GET /health — liveness. Never touches the database. */
export interface HealthResponse {
  status: string;
  version: string;
  time: string;
}

export function fetchHealth(): Promise<ApiResult<HealthResponse>> {
  return apiGet<HealthResponse>('/health', { basePath: ROOT_BASE });
}

/** GET /ready — readiness. DOES ping the database. */
export interface ReadyResponse {
  status: string;
  database: string;
}

export function fetchReady(): Promise<ApiResult<ReadyResponse>> {
  return apiGet<ReadyResponse>('/ready', { basePath: ROOT_BASE });
}
