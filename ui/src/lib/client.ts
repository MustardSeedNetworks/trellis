import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { SurveyService } from '@/gen/trellis/survey/v1/survey_pb';

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

const transport = createConnectTransport({
  baseUrl: resolveApiBaseUrl(import.meta.env.VITE_TRELLIS_API, window.location.origin),
});

export const surveyClient = createClient(SurveyService, transport);
