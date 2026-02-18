import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { isOAuthToken, makeOAuthFetch } from './oauth.js';

describe('isOAuthToken', () => {
  it('returns true for sk-ant-oat01- prefixed tokens', () => {
    expect(isOAuthToken('sk-ant-oat01-abc123')).toBe(true);
  });

  it('returns false for standard API keys', () => {
    expect(isOAuthToken('sk-ant-api03-abc123')).toBe(false);
  });

  it('returns false for empty string', () => {
    expect(isOAuthToken('')).toBe(false);
  });

  it('returns false for undefined-like values', () => {
    expect(isOAuthToken(undefined as any)).toBe(false);
    expect(isOAuthToken(null as any)).toBe(false);
  });
});

describe('makeOAuthFetch', () => {
  let originalFetch: typeof globalThis.fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it('returns a function', () => {
    const oauthFetch = makeOAuthFetch('sk-ant-oat01-test');
    expect(typeof oauthFetch).toBe('function');
  });

  it('sets Authorization Bearer header', async () => {
    const oauthFetch = makeOAuthFetch('sk-ant-oat01-mytoken');
    await oauthFetch('https://api.anthropic.com/v1/messages', {
      method: 'POST',
      headers: { 'x-api-key': 'placeholder', 'content-type': 'application/json' },
    });

    const [url, init] = (globalThis.fetch as any).mock.calls[0];
    const headers = new Headers(init.headers);

    expect(headers.get('authorization')).toBe('Bearer sk-ant-oat01-mytoken');
    expect(headers.has('x-api-key')).toBe(false);
  });

  it('adds required beta headers', async () => {
    const oauthFetch = makeOAuthFetch('sk-ant-oat01-mytoken');
    await oauthFetch('https://api.anthropic.com/v1/messages', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
    });

    const [, init] = (globalThis.fetch as any).mock.calls[0];
    const headers = new Headers(init.headers);
    const betas = headers.get('anthropic-beta')!;

    expect(betas).toContain('oauth-2025-04-20');
    expect(betas).toContain('interleaved-thinking-2025-05-14');
  });

  it('preserves existing beta headers', async () => {
    const oauthFetch = makeOAuthFetch('sk-ant-oat01-mytoken');
    await oauthFetch('https://api.anthropic.com/v1/messages', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'anthropic-beta': 'my-custom-beta-2025-01-01',
      },
    });

    const [, init] = (globalThis.fetch as any).mock.calls[0];
    const headers = new Headers(init.headers);
    const betas = headers.get('anthropic-beta')!;

    expect(betas).toContain('my-custom-beta-2025-01-01');
    expect(betas).toContain('oauth-2025-04-20');
    expect(betas).toContain('interleaved-thinking-2025-05-14');
  });

  it('adds ?beta=true to /v1/messages URL', async () => {
    const oauthFetch = makeOAuthFetch('sk-ant-oat01-mytoken');
    await oauthFetch('https://api.anthropic.com/v1/messages', {
      method: 'POST',
    });

    const [url] = (globalThis.fetch as any).mock.calls[0];
    expect(url).toContain('beta=true');
  });

  it('does not add ?beta=true to non-messages URLs', async () => {
    const oauthFetch = makeOAuthFetch('sk-ant-oat01-mytoken');
    await oauthFetch('https://api.anthropic.com/v1/complete', {
      method: 'POST',
    });

    const [url] = (globalThis.fetch as any).mock.calls[0];
    expect(url).not.toContain('beta=true');
  });

  it('sets user-agent header', async () => {
    const oauthFetch = makeOAuthFetch('sk-ant-oat01-mytoken');
    await oauthFetch('https://api.anthropic.com/v1/messages', {
      method: 'POST',
    });

    const [, init] = (globalThis.fetch as any).mock.calls[0];
    const headers = new Headers(init.headers);
    expect(headers.get('user-agent')).toBe('codex-indexer/0.1.0');
  });
});
