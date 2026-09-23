import { Code, ConnectError, createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SurveyService } from '@/gen/trellis/survey/v1/survey_pb';
import { currentCsrfToken, fetchSession, logout, onSessionLost, resetCsrfToken } from './auth';
import { sessionInterceptor } from './client';

/**
 * These drive a real Connect transport rather than a hand-built `next`, so the
 * retry is proven to replay a request the way the browser sends it. The
 * transport binds its fetch at construction, which is why each test builds its
 * own instead of going through the module's singleton.
 */

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

/**
 * FakeDaemon honours exactly one CSRF token at a time, as trellisd does: a
 * refresh revokes the session it spends and the token minted for it.
 */
class FakeDaemon {
  live = 'tok-1';
  issued = 1;
  /** The X-CSRF-Token each ListSurveys call carried, in order. */
  rpcTokens: (string | null)[] = [];
  refreshes = 0;
  refuseRefresh = false;
  denyEveryRpc = false;
  /** When set, each refresh waits for release() so callers are truly concurrent. */
  hold = false;
  /** When set, the next refused RPC's answer waits for releaseRefusal(). */
  holdNextRefusal = false;
  private open: () => void = () => {};
  private gate = new Promise<void>((resolve) => {
    this.open = resolve;
  });
  private openRefusals: () => void = () => {};
  private refusalGate = new Promise<void>((resolve) => {
    this.openRefusals = resolve;
  });

  readonly fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = input instanceof Request ? input.url : String(input);
    const token = new Headers(init?.headers).get('X-CSRF-Token');
    if (url.endsWith('/auth/session')) {
      return json(200, { authenticated: true, username: 'surveyor', csrfToken: this.live });
    }
    if (url.endsWith('/auth/refresh')) {
      this.refreshes++;
      if (this.hold) {
        await this.gate;
      }
      if (this.refuseRefresh) {
        return json(401, { error: 'invalid refresh token' });
      }
      this.issued++;
      this.live = `tok-${this.issued}`;
      return json(200, { username: 'surveyor', csrfToken: this.live });
    }
    if (url.endsWith('/auth/logout')) {
      return new Response(null, { status: 204 });
    }
    if (url.endsWith('/trellis.survey.v1.SurveyService/ListSurveys')) {
      this.rpcTokens.push(token);
      if (this.denyEveryRpc) {
        return json(403, { code: 'permission_denied', message: 'request not permitted' });
      }
      if (token === this.live) {
        return json(200, {});
      }
      if (this.holdNextRefusal) {
        this.holdNextRefusal = false;
        await this.refusalGate;
      }
      return json(401, { code: 'unauthenticated', message: 'authentication required' });
    }
    throw new Error(`unexpected fetch ${url}`);
  });

  release() {
    this.open();
  }

  releaseRefusal() {
    this.openRefusals();
  }

  /** The access token lapses: the browser still holds a token nothing honours. */
  expire() {
    this.live = 'lapsed';
  }

  client() {
    vi.stubGlobal('fetch', this.fetch);
    return createClient(
      SurveyService,
      createConnectTransport({
        baseUrl: 'https://trellis.test',
        fetch: this.fetch,
        interceptors: [sessionInterceptor],
      }),
    );
  }
}

async function signIn(daemon: FakeDaemon) {
  const client = daemon.client();
  await fetchSession();
  expect(currentCsrfToken()).toBe(daemon.live);
  return client;
}

describe('sessionInterceptor', () => {
  beforeEach(() => resetCsrfToken());
  afterEach(() => vi.unstubAllGlobals());

  it('refreshes an expired session once and retries with the new CSRF token', async () => {
    const daemon = new FakeDaemon();
    const client = await signIn(daemon);
    daemon.expire();

    await expect(client.listSurveys({})).resolves.toBeDefined();
    expect(daemon.refreshes).toBe(1);
    // The refresh revoked the old token with the old session, so the retry
    // must carry the one the refresh returned.
    expect(daemon.rpcTokens).toEqual(['tok-1', 'tok-2']);
    expect(currentCsrfToken()).toBe('tok-2');
  });

  it('shares one refresh between concurrent calls, and refreshes again later', async () => {
    const daemon = new FakeDaemon();
    const client = await signIn(daemon);
    daemon.expire();
    daemon.hold = true;

    const calls = [client.listSurveys({}), client.listSurveys({}), client.listSurveys({})];
    await vi.waitFor(() => expect(daemon.rpcTokens).toHaveLength(3));
    daemon.release();
    await expect(Promise.all(calls)).resolves.toHaveLength(3);
    // A second refresh would spend the first one's session under its retry.
    expect(daemon.refreshes).toBe(1);

    // The shared attempt is finished, not cached: the next expiry refreshes.
    daemon.hold = false;
    daemon.expire();
    await expect(client.listSurveys({})).resolves.toBeDefined();
    expect(daemon.refreshes).toBe(2);
  });

  it('retries without refreshing when another call already renewed the session', async () => {
    const daemon = new FakeDaemon();
    const client = await signIn(daemon);
    daemon.expire();

    // `late` leaves with the lapsed token but its refusal arrives only after
    // `early` has refreshed. Refreshing again would revoke the session `early`
    // just renewed, so `late` must retry on it instead.
    daemon.holdNextRefusal = true;
    const late = client.listSurveys({});
    await vi.waitFor(() => expect(daemon.rpcTokens).toHaveLength(1));
    await expect(client.listSurveys({})).resolves.toBeDefined();
    daemon.releaseRefusal();

    await expect(late).resolves.toBeDefined();
    expect(daemon.refreshes).toBe(1);
    expect(daemon.rpcTokens).toEqual(['tok-1', 'tok-1', 'tok-2', 'tok-2']);
  });

  it('returns to sign-in when the refresh is refused, without retrying', async () => {
    const daemon = new FakeDaemon();
    const client = await signIn(daemon);
    daemon.expire();
    daemon.refuseRefresh = true;
    const lost = vi.fn();
    const stop = onSessionLost(lost);

    const err = await client.listSurveys({}).catch((e: unknown) => e);
    stop();

    expect(ConnectError.from(err).code).toBe(Code.Unauthenticated);
    expect(daemon.refreshes).toBe(1);
    expect(daemon.rpcTokens).toHaveLength(1);
    expect(lost).toHaveBeenCalledOnce();
    expect(currentCsrfToken()).toBe('');
  });

  // permission_denied is a present session without a proven CSRF token. A new
  // session does not fix that, which is why the daemon's refusal() splits it
  // from unauthenticated.
  it('does not refresh on permission_denied', async () => {
    const daemon = new FakeDaemon();
    const client = await signIn(daemon);
    daemon.denyEveryRpc = true;

    const err = await client.listSurveys({}).catch((e: unknown) => e);
    expect(ConnectError.from(err).code).toBe(Code.PermissionDenied);
    expect(daemon.refreshes).toBe(0);
    expect(daemon.rpcTokens).toHaveLength(1);
  });

  // D-TRL-8: logout must still end the server session, which after a refresh
  // means proving the new session, not the one the refresh spent.
  it('signs out with the CSRF token of the refreshed session', async () => {
    const daemon = new FakeDaemon();
    const client = await signIn(daemon);
    daemon.expire();
    await client.listSurveys({});

    await logout();
    const [url, init] = daemon.fetch.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe('/auth/logout');
    expect(new Headers(init.headers).get('X-CSRF-Token')).toBe('tok-2');
  });
});
