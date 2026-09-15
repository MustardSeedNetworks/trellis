import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthGate } from './AuthGate';

describe('AuthGate', () => {
  afterEach(() => vi.unstubAllGlobals());

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
  // password at nothing teaches the operator to type it anywhere.
  it('renders the app rather than a login form when the probe fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network')));
    render(
      <AuthGate>
        <div data-testid="shell" />
      </AuthGate>,
    );
    expect(await screen.findByTestId('shell')).toBeInTheDocument();
  });
});
