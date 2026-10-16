/**
 * The single place that knows how to talk to the API.
 *
 * Every error the server returns uses the same envelope
 * (`{ error: { code, message, details, requestId } }` — see
 * docs/07-API-CONTRACT.md §1.3), so it is unwrapped exactly once, here.
 */

/**
 * In development this is a relative path and Vite proxies /api to :8080, so the
 * browser sees a single origin and CORS never applies locally. In production
 * VITE_API_URL points at the Cloud Run service and CORS does apply.
 */
const API_BASE = import.meta.env.VITE_API_URL ?? '/api/v1';

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
}

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
  /** Milliseconds. Sync pushes need a long timeout because of Cloud Run cold starts. */
  timeoutMs?: number;
  /** Value for the Idempotency-Key header, for money-affecting POSTs. */
  idempotencyKey?: string;
}

/**
 * Performs a request and unwraps the envelope.
 *
 * Throws ApiError on any non-2xx response, so callers never have to check
 * `res.ok` themselves.
 */
export async function apiFetch<T>(
  path: string,
  options: ApiFetchOptions = {},
): Promise<ApiResult<T>> {
  const { body, timeoutMs = 30_000, idempotencyKey, headers, ...init } = options;

  // AbortSignal.timeout is widely supported and avoids hand-rolled timer
  // bookkeeping. A hung fetch to a cold Cloud Run instance would otherwise
  // leave the UI waiting indefinitely.
  const signal = init.signal ?? AbortSignal.timeout(timeoutMs);

  const requestHeaders: Record<string, string> = {
    Accept: 'application/json',
    ...(headers as Record<string, string> | undefined),
  };

  if (body !== undefined) {
    requestHeaders['Content-Type'] = 'application/json';
  }
  if (idempotencyKey) {
    requestHeaders['Idempotency-Key'] = idempotencyKey;
  }

  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...init,
      signal,
      headers: requestHeaders,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch (cause) {
    // A network failure or timeout is NOT an ApiError from the server; it is
    // the absence of one. Offline-first code must distinguish the two: one is
    // an expected state, the other is a real rejection.
    const aborted = cause instanceof DOMException && cause.name === 'TimeoutError';
    throw new ApiError(
      aborted ? 'REQUEST_TIMEOUT' : 'NETWORK_ERROR',
      aborted
        ? 'The server took too long to respond. It may be starting up.'
        : 'Could not reach the server. Work continues offline and will sync later.',
      0,
    );
  }

  // 204 has no body to parse.
  if (response.status === 204) {
    return { data: undefined as T, meta: undefined };
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

    throw new ApiError(
      err?.code ?? 'UNKNOWN_ERROR',
      err?.message ?? `Request failed with status ${response.status}.`,
      response.status,
      err?.details,
      err?.requestId,
    );
  }

  const envelope = parsed as ApiEnvelope<T>;
  return { data: envelope.data, meta: envelope.meta };
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
  return apiGet<HealthResponse>('/health');
}

/** GET /ready — readiness. DOES ping the database. */
export interface ReadyResponse {
  status: string;
  database: string;
}

export function fetchReady(): Promise<ApiResult<ReadyResponse>> {
  return apiGet<ReadyResponse>('/ready');
}
