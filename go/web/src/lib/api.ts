/**
 * Centralized API client for Carto.
 *
 * All fetch calls to /api/* must go through `apiFetch` to ensure:
 * - Bearer token authentication is attached when a server token is configured
 * - Consistent JSON error parsing (throws ApiError on non-OK responses)
 * - Automatic token invalidation and page reload on 401
 *
 * Usage:
 *   const data = await apiFetch<Project[]>('/projects')
 *   const result = await apiFetch('/projects/index', { method: 'POST', body: JSON.stringify(payload) })
 */

/** Base URL prefix for all API requests. Change here to update all callers. */
export const API_BASE = '/api'

/** Typed error thrown by apiFetch when the server returns a non-OK status. */
export class ApiError extends Error {
  readonly status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/**
 * apiFetch wraps the native fetch API with:
 * - Automatic Bearer token injection from localStorage
 * - JSON Content-Type header
 * - Structured error handling (throws ApiError on non-OK responses)
 * - 401 auto-logout: clears the stored token and reloads the page
 *
 * @param path  API path relative to API_BASE (e.g. '/projects', '/config')
 * @param init  Optional RequestInit override (method, body, extra headers, etc.)
 * @returns     Parsed JSON response body typed as T
 * @throws      ApiError if the server responds with a non-OK status code
 */
export async function apiFetch<T = unknown>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const token = localStorage.getItem('carto_token') ?? ''

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...(init?.headers ?? {}),
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
  })

  // 401 Unauthorized → stored token is stale or auth was just enabled on the
  // server. Clear the token so AuthGuard will prompt for a new one.
  if (res.status === 401) {
    localStorage.removeItem('carto_token')
    window.location.reload()
    // Unreachable after reload, but satisfies TypeScript's control-flow analysis.
    throw new ApiError(401, 'Unauthorized — please re-authenticate')
  }

  if (!res.ok) {
    let message = res.statusText
    try {
      const body = await res.json()
      if (typeof body?.error === 'string') {
        message = body.error
      }
    } catch {
      // Body is not JSON — use statusText as the error message.
    }
    throw new ApiError(res.status, message)
  }

  // Return parsed JSON for all OK responses.
  return res.json() as Promise<T>
}

/**
 * apiFetchRaw is the same as apiFetch but returns the raw Response object
 * instead of parsing JSON. Use this for endpoints that return SSE streams,
 * binary data, or where you need response headers.
 */
export async function apiFetchRaw(
  path: string,
  init?: RequestInit,
): Promise<Response> {
  const token = localStorage.getItem('carto_token') ?? ''

  const headers: HeadersInit = {
    ...(init?.headers ?? {}),
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
  })

  if (res.status === 401) {
    localStorage.removeItem('carto_token')
    window.location.reload()
    throw new ApiError(401, 'Unauthorized — please re-authenticate')
  }

  return res
}
