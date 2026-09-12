import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetCsrfToken } from '@/lib/auth';
import { LoginPage } from './LoginPage';

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('LoginPage', () => {
  beforeEach(() => resetCsrfToken());
  afterEach(() => vi.unstubAllGlobals());

  it('reports the operator in on a successful login', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse(200, { username: 'surveyor', csrfToken: 'tok' })),
    );
    const onAuthenticated = vi.fn();
    render(<LoginPage onAuthenticated={onAuthenticated} />);

    await userEvent.type(screen.getByTestId('login-username'), 'surveyor');
    await userEvent.type(screen.getByTestId('login-password'), 'correct-horse-battery');
    await userEvent.click(screen.getByTestId('login-submit'));

    await waitFor(() => expect(onAuthenticated).toHaveBeenCalledWith('surveyor'));
  });

  it('says the credentials were wrong and clears only the password', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(401, { error: 'no' })));
    render(<LoginPage onAuthenticated={vi.fn()} />);

    await userEvent.type(screen.getByTestId('login-username'), 'surveyor');
    await userEvent.type(screen.getByTestId('login-password'), 'wrong');
    await userEvent.click(screen.getByTestId('login-submit'));

    expect(await screen.findByTestId('login-error')).toBeInTheDocument();
    expect(screen.getByTestId('login-password')).toHaveValue('');
    expect(screen.getByTestId('login-username')).toHaveValue('surveyor');
  });

  it('words a rate-limited refusal differently from a wrong password', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(429, { error: 'slow down' })));
    render(<LoginPage onAuthenticated={vi.fn()} />);

    await userEvent.type(screen.getByTestId('login-username'), 'surveyor');
    await userEvent.type(screen.getByTestId('login-password'), 'correct-horse-battery');
    await userEvent.click(screen.getByTestId('login-submit'));

    const rateLimited = (await screen.findByTestId('login-error')).textContent;

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(401, { error: 'no' })));
    await userEvent.type(screen.getByTestId('login-password'), 'correct-horse-battery');
    await userEvent.click(screen.getByTestId('login-submit'));

    await waitFor(() =>
      expect(screen.getByTestId('login-error').textContent).not.toBe(rateLimited),
    );
  });

  it('does not call the daemon with an empty field', async () => {
    const send = vi.fn();
    vi.stubGlobal('fetch', send);
    render(<LoginPage onAuthenticated={vi.fn()} />);

    await userEvent.click(screen.getByTestId('login-submit'));

    expect(send).not.toHaveBeenCalled();
    expect(await screen.findByTestId('login-error')).toBeInTheDocument();
  });
});
