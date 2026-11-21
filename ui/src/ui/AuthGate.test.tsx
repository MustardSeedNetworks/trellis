import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { refreshSession } from '@/lib/auth';
import { AuthGate, PROBE_RETRY_MS, useAuth } from './AuthGate';

function SignOutProbe() {
  const { signOut } = useAuth();
  return (
    <button type="button" data-testid="shell" onClick={signOut}>
      sign out
    </button>
  );
}

function session(body: object): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('AuthGate', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  // Blank is indistinguishable from broken: until the probe answers, the
  // screen says what it is waiting for.
  it('shows a checking state, not a blank screen, while the probe is pending', async () => {
    let answer: (response: Response) => void = () => {};
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise<Response>((resolve) => (answer = resolve))),
    );
    render(
      <AuthGate>
        <div data-testid="shell" />
      </AuthGate>,
    );

    expect(screen.getByTestId('auth-checking')).toHaveTextContent('Checking your session');
    expect(screen.queryByTestId('shell')).not.toBeInTheDocument();

    await act(async () => answer(new Response(null, { status: 404 })));
    expect(screen.queryByTestId('auth-checking')).not.toBeInTheDocument();
    expect(screen.getByTestId('shell')).toBeInTheDocument();
  });

  // The loopback desktop daemon registers no auth routes at all. The app must
  // render for it exactly as before this feature existed.
  it('renders the app when the daemon has no auth routes', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 404 })));
    render(
      <AuthGate>
        <div data-testid="shell" />
      </AuthGate>,
    );
    expect(await screen.findByTestId('shell')).toBeInTheDocument();
  });

  it('shows the login form when the daemon wants a session we do not have', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ authenticated: false }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    );
    render(
      <AuthGate>
        <div data-testid="shell" />
      </AuthGate>,
    );
    expect(await screen.findByTestId('login-form')).toBeInTheDocument();
    expect(screen.queryByTestId('shell')).not.toBeInTheDocument();
  });

  // An unreachable daemon is not an authentication answer: asking for a
  // password at nothing teaches the operator to type it anywhere, and a shell
  // of pages that all fail says nothing about why. It says so, and asks again.
  it('says the daemon is unreachable and retries until it answers, never asking for a password', async () => {
    vi.useFakeTimers();
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError('network'))
      .mockRejectedValueOnce(new TypeError('network'))
      .mockResolvedValue(session({ authenticated: false }));
    vi.stubGlobal('fetch', fetchMock);
    render(
      <AuthGate>
        <div data-testid="shell" />
      </AuthGate>,
    );

    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(screen.getByTestId('daemon-unreachable')).toHaveTextContent(
      "Can't reach the Trellis daemon",
    );
    expect(screen.queryByTestId('login-form')).not.toBeInTheDocument();
    expect(screen.queryByTestId('shell')).not.toBeInTheDocument();

    await act(() => vi.advanceTimersByTimeAsync(PROBE_RETRY_MS));
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(screen.getByTestId('daemon-unreachable')).toBeInTheDocument();

    await act(() => vi.advanceTimersByTimeAsync(PROBE_RETRY_MS));
    expect(screen.queryByTestId('daemon-unreachable')).not.toBeInTheDocument();
    expect(screen.getByTestId('login-form')).toBeInTheDocument();
  });

  // The Vite dev server has no daemon behind it and answers the probe with
  // HTML. That is a reply, not silence: the app renders and says what is wrong.
  it('renders the app when the probe is answered with something that is not a session', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('<!doctype html>', { status: 200 })),
    );
    render(
      <AuthGate>
        <div data-testid="shell" />
      </AuthGate>,
    );
    expect(await screen.findByTestId('shell')).toBeInTheDocument();
    expect(screen.queryByTestId('daemon-unreachable')).not.toBeInTheDocument();
  });

  // A refresh the daemon refuses (revoked, or past its week) means the session
  // is over; the operator must see the sign-in form, not a page of errors.
  it('returns to the login form when the session cannot be renewed', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) =>
        url === '/auth/refresh'
          ? new Response(JSON.stringify({ error: 'invalid refresh token' }), { status: 401 })
          : new Response(
              JSON.stringify({ authenticated: true, username: 'surveyor', csrfToken: 'tok-1' }),
              { status: 200, headers: { 'Content-Type': 'application/json' } },
            ),
      ),
    );
    render(
      <AuthGate>
        <div data-testid="shell" />
      </AuthGate>,
    );
    expect(await screen.findByTestId('shell')).toBeInTheDocument();

    await act(async () => {
      await refreshSession();
    });
    expect(screen.getByTestId('login-form')).toBeInTheDocument();
    expect(screen.queryByTestId('shell')).not.toBeInTheDocument();
    expect(screen.getByTestId('login-session-expired')).toHaveTextContent(
      'Your session expired. Sign in again.',
    );
  });

  // Signing out on purpose is not an expiry; telling the operator their
  // session expired after they ended it would be wrong.
  it('does not claim the session expired when the operator signed out', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => session({ authenticated: true, username: 'surveyor', csrfToken: 'tok' })),
    );
    render(
      <AuthGate>
        <SignOutProbe />
      </AuthGate>,
    );

    fireEvent.click(await screen.findByTestId('shell'));

    expect(await screen.findByTestId('login-form')).toBeInTheDocument();
    expect(screen.queryByTestId('login-session-expired')).not.toBeInTheDocument();
  });
});
