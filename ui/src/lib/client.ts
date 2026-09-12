import { createClient, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { SurveyService } from '@/gen/trellis/survey/v1/survey_pb';
import { currentCsrfToken } from '@/lib/auth';

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
 */
export const csrfInterceptor: Interceptor = (next) => (req) => {
  const token = currentCsrfToken();
  if (token) {
    req.header.set('X-CSRF-Token', token);
  }
  return next(req);
};

const transport = createConnectTransport({
  baseUrl: resolveApiBaseUrl(import.meta.env.VITE_TRELLIS_API, window.location.origin),
  interceptors: [csrfInterceptor],
});

export const surveyClient = createClient(SurveyService, transport);
