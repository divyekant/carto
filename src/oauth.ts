/**
 * OAuth support for Anthropic subscription tokens (sk-ant-oat01-).
 * Based on the pattern from WebChat/llm.js.
 *
 * When a user provides an OAuth token instead of a standard API key,
 * we intercept fetch calls to:
 * 1. Replace x-api-key with Authorization: Bearer
 * 2. Add required beta headers
 * 3. Handle token refresh when expired
 */

const OAUTH_CLIENT_ID = '9d1c250a-e61b-44d9-88ed-5944d1962f5e';
const OAUTH_TOKEN_URL = 'https://console.anthropic.com/v1/oauth/token';
const REQUIRED_BETAS = ['oauth-2025-04-20', 'interleaved-thinking-2025-05-14'];

interface OAuthState {
  accessToken: string;
  refreshToken: string | null;
  expiresAt: number | null;
}

let oauthState: OAuthState | null = null;

export function isOAuthToken(key: string): boolean {
  return !!key && key.startsWith('sk-ant-oat01-');
}

async function refreshOAuthToken(): Promise<boolean> {
  if (!oauthState || !oauthState.refreshToken) return false;

  try {
    const res = await fetch(OAUTH_TOKEN_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        grant_type: 'refresh_token',
        refresh_token: oauthState.refreshToken,
        client_id: OAUTH_CLIENT_ID,
      }),
    });

    if (!res.ok) {
      console.error('OAuth token refresh failed:', res.status);
      return false;
    }

    const data = await res.json() as {
      access_token: string;
      refresh_token: string;
      expires_in: number;
    };

    oauthState.accessToken = data.access_token;
    oauthState.refreshToken = data.refresh_token;
    oauthState.expiresAt = Date.now() + data.expires_in * 1000;
    return true;
  } catch (err: any) {
    console.error('OAuth token refresh error:', err.message);
    return false;
  }
}

/**
 * Build a custom fetch function that converts OAuth tokens into
 * the correct Authorization: Bearer headers, adds required betas,
 * and strips x-api-key.
 */
export function makeOAuthFetch(accessToken: string): typeof fetch {
  oauthState = { accessToken, refreshToken: null, expiresAt: null };

  return async function oauthFetch(
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> {
    // Refresh if expired
    if (oauthState && oauthState.expiresAt && oauthState.expiresAt < Date.now()) {
      const refreshed = await refreshOAuthToken();
      if (refreshed) accessToken = oauthState!.accessToken;
    }

    // Merge headers
    const headers = new Headers(init?.headers || {});

    // Set Bearer auth, remove x-api-key
    headers.set('authorization', `Bearer ${accessToken}`);
    headers.delete('x-api-key');

    // Merge beta headers
    const existingBetas = (headers.get('anthropic-beta') || '')
      .split(',')
      .map(s => s.trim())
      .filter(Boolean);
    const allBetas = [...new Set([...existingBetas, ...REQUIRED_BETAS])];
    headers.set('anthropic-beta', allBetas.join(','));

    // Set user-agent
    headers.set('user-agent', 'codex-indexer/0.1.0');

    // Modify URL — add ?beta=true for /v1/messages
    let url = typeof input === 'string'
      ? input
      : input instanceof Request
        ? input.url
        : String(input);

    const urlObj = new URL(url);
    if (urlObj.pathname === '/v1/messages') {
      urlObj.searchParams.set('beta', 'true');
    }

    return fetch(urlObj.toString(), {
      ...init,
      headers,
    });
  } as typeof fetch;
}
