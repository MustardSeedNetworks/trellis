/**
 * Operator session for a daemon that asks for one.
 *
 * trellisd serves loopback without a credential — the desktop app of ADR-0007 —
 * and registers no /auth routes at all in that mode. So a 404 from the session
 * probe is the answer "this daemon wants no login", not a failure, and the UI
 * renders exactly as it always has.
 *
 * The CSRF token lives in memory rather than in storage. It is re-fetched from
 * the session probe on every load, so keeping a copy on disk would add a way
 * for it to be stale or stolen without adding a way for it to be useful.
 *
 * The access token lasts fifteen minutes. The transport renews it through
 * refreshSession when an RPC is refused as unauthenticated, and a renewal the
 * daemon refuses is reported to onSessionLost, which puts the operator back
 * on the sign-in form.
 */
const SESSION_URL = '/auth/session';
const LOGIN_URL = '/auth/login';
const LOGOUT_URL = '/auth/logout';
const REFRESH_URL = '/auth/refresh';

export interface Session {
  /** False only when this daemon wants a login and we do not have one. */
  authenticated: boolean;
  username?: string;
  /** True when the daemon registered no auth routes, i.e. loopback desktop mode. */
  authDisabled?: boolean;
}

let csrfToken = '';

/** currentCsrfToken returns the token the transport must echo, '' if none. */
export function currentCsrfToken(): string {
  return csrfToken;
}

/** Exported for tests, which need to start from a known state. */
export function resetCsrfToken(): void {
  csrfToken = '';
}

interface SessionResponse {
  authenticated?: boolean;
  username?: string;
  csrfToken?: string;
}

async function readSession(response: Response): Promise<Session> {
  const body = (await response.json()) as SessionResponse;
  csrfToken = body.csrfToken ?? '';
  return { authenticated: body.authenticated ?? false, username: body.username };
}

/** fetchSession asks the daemon whether we are logged in, and whether it cares. */
export async function fetchSession(): Promise<Session> {
  const response = await fetch(SESSION_URL, { credentials: 'same-origin' });
  if (response.status === 404) {
    return { authenticated: true, authDisabled: true };
  }
  if (!response.ok) {
    return { authenticated: false };
  }
  return readSession(response);
}

const sessionLost = new Set<() => void>();

/** onSessionLost calls listener when the daemon refuses to renew the session. */
export function onSessionLost(listener: () => void): () => void {
  sessionLost.add(listener);
  return () => {
    sessionLost.delete(listener);
  };
}

let refreshing: Promise<boolean> | null = null;

/**
 * refreshSession exchanges the refresh cookie for a new session and resolves
 * true once the new CSRF token is in hand.
 *
 * Callers share one attempt. The daemon rotates on every refresh, revoking the
 * session it spends, so a second refresh racing the first would revoke the
 * session the first caller is about to retry with.
 */
export function refreshSession(): Promise<boolean> {
  refreshing ??= renew().finally(() => {
    refreshing = null;
  });
  return refreshing;
}

async function renew(): Promise<boolean> {
  let response: Response;
  try {
    response = await fetch(REFRESH_URL, { method: 'POST', credentials: 'same-origin' });
  } catch {
    // An unreachable daemon has not refused the session; as in AuthGate, the
    // page's own error state says what is wrong, not a sign-in form.
    return false;
  }
  if (response.ok) {
    await readSession(response);
    return true;
  }
  csrfToken = '';
  for (const listener of sessionLost) {
    listener();
  }
  return false;
}

/** LoginFailure distinguishes the two answers the form must word differently. */
export type LoginFailure = 'invalid' | 'rateLimited' | 'unavailable';

export class LoginError extends Error {
  // Declared rather than a constructor parameter property: tsconfig sets
  // erasableSyntaxOnly, which refuses syntax that emits runtime code.
  readonly reason: LoginFailure;

  constructor(reason: LoginFailure) {
    super(reason);
    this.name = 'LoginError';
    this.reason = reason;
  }
}

export async function login(username: string, password: string): Promise<Session> {
  let response: Response;
  try {
    response = await fetch(LOGIN_URL, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
  } catch {
    throw new LoginError('unavailable');
  }
  if (response.status === 429) {
    throw new LoginError('rateLimited');
  }
  if (!response.ok) {
    throw new LoginError('invalid');
  }
  const session = await readSession(response);
  return { ...session, authenticated: true };
}

export async function logout(): Promise<void> {
  try {
    await fetch(LOGOUT_URL, {
      method: 'POST',
      credentials: 'same-origin',
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
    });
  } finally {
    csrfToken = '';
  }
}
