import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { currentCsrfToken, fetchSession, LoginError, login, logout, resetCsrfToken } from './auth';

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('auth', () => {
  beforeEach(() => {
    resetCsrfToken();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('reads a daemon with no auth routes as needing no login', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 404 })));
    await expect(fetchSession()).resolves.toEqual({ authenticated: true, authDisabled: true });
  });

  it('reports an anonymous session on a daemon that wants one', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { authenticated: false })));
    const session = await fetchSession();
    expect(session.authenticated).toBe(false);
    expect(session.authDisabled).toBeUndefined();
    expect(currentCsrfToken()).toBe('');
  });

  it('recovers the CSRF token for a session that survived a reload', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(
          jsonResponse(200, { authenticated: true, username: 'surveyor', csrfToken: 'tok-1' }),
        ),
    );
    await expect(fetchSession()).resolves.toMatchObject({
      authenticated: true,
      username: 'surveyor',
    });
    expect(currentCsrfToken()).toBe('tok-1');
  });

  it('keeps the CSRF token from a successful login', async () => {
    const send = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, { username: 'surveyor', csrfToken: 'tok-2' }));
    vi.stubGlobal('fetch', send);

    await expect(login('surveyor', 'correct-horse-battery')).resolves.toMatchObject({
      authenticated: true,
      username: 'surveyor',
    });
    expect(currentCsrfToken()).toBe('tok-2');

    const [url, init] = send.mock.calls[0] as [string, RequestInit];
    expect(init.credentials).toBe('same-origin');
    // The password belongs in the body, never in the URL, where it would reach
    // logs and history.
    expect(url).not.toContain('correct-horse-battery');
  });

  it.each([
    [401, 'invalid'],
    [429, 'rateLimited'],
  ])('maps HTTP %i to the %s failure', async (status, reason) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(status, { error: 'no' })));
    await expect(login('surveyor', 'wrong')).rejects.toThrow(
      expect.objectContaining({ reason }) as Error,
    );
  });

  it('reports an unreachable daemon as unavailable, not as bad credentials', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network')));
    await expect(login('surveyor', 'correct-horse-battery')).rejects.toBeInstanceOf(LoginError);
    await expect(login('surveyor', 'correct-horse-battery')).rejects.toMatchObject({
      reason: 'unavailable',
    });
  });

  it('drops the CSRF token on logout even when the call fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { csrfToken: 'tok-3' })));
    await fetchSession();
    expect(currentCsrfToken()).toBe('tok-3');

    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network')));
    await expect(logout()).rejects.toBeTruthy();
    expect(currentCsrfToken()).toBe('');
  });
});
