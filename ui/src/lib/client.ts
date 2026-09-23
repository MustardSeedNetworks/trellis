import { Code, ConnectError, createClient, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { SurveyService } from '@/gen/trellis/survey/v1/survey_pb';
import { currentCsrfToken, refreshSession } from '@/lib/auth';

/**
 * resolveApiBaseUrl decides which host the UI talks to.
 *
 * trellisd embeds this bundle and serves it from the same listener as the API,
 * so the answer is normally "wherever this page came from". Baking an address
 * in at compile time instead breaks the moment the daemon moves — `TRELLIS_ADDR`
 * exists so it can — and is actively wrong over the network, where a loopback
 * address means the viewer's own machine rather than the server's.
 *
 * The override is for running Vite's dev server on 5173 against a daemon
 * somewhere else, which is the one case where same-origin is not what we want.
 */
export function resolveApiBaseUrl(override: string | undefined, origin: string): string {
  const configured = override?.trim();
  return configured ? configured : origin;
}

/**
 * A protected daemon checks a per-session CSRF token on every RPC, so the token
 * the session probe or the login handed back travels with each call. A daemon
 * on loopback registers no auth at all and there is no token to send, which is
 * why this adds the header only when one exists rather than always.
 *
 * An RPC refused as unauthenticated has outlived its fifteen-minute access
 * token, so it renews the session once and is sent again with the new token.
 * Sending it again is safe for a mutation too: the daemon refuses before any
 * handler runs. If a concurrent call already renewed the session while this
 * one was in flight, it retries on that session rather than spending it with
 * a second refresh. permission_denied (a CSRF miss) is left alone, as a new
 * session would not fix it. Every RPC is unary; a stream could not be replayed.
 */
export const sessionInterceptor: Interceptor = (next) => async (req) => {
  const sent = withCsrfToken(req.header);
  try {
    return await next(req);
  } catch (err) {
    if (req.stream || ConnectError.from(err).code !== Code.Unauthenticated) {
      throw err;
    }
    const current = currentCsrfToken();
    const renewed = (current !== '' && current !== sent) || (await refreshSession());
    if (!renewed) {
      throw err;
    }
    withCsrfToken(req.header);
    return next(req);
  }
};

function withCsrfToken(header: Headers): string {
  const token = currentCsrfToken();
  if (token) {
    header.set('X-CSRF-Token', token);
  } else {
    header.delete('X-CSRF-Token');
  }
  return token;
}

const transport = createConnectTransport({
  baseUrl: resolveApiBaseUrl(import.meta.env.VITE_TRELLIS_API, window.location.origin),
  interceptors: [sessionInterceptor],
});

export const surveyClient = createClient(SurveyService, transport);
