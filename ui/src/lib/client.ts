import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { SurveyService } from '@/gen/trellis/survey/v1/survey_pb';

const DEFAULT_API_BASE_URL = 'http://127.0.0.1:8080';

const transport = createConnectTransport({
  baseUrl: import.meta.env.VITE_TRELLIS_API ?? DEFAULT_API_BASE_URL,
});

export const surveyClient = createClient(SurveyService, transport);
