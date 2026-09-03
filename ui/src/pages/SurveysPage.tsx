import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SurveyCreateForm } from '@/components/SurveyCreateForm';
import { SurveyDetail } from '@/components/SurveyDetail';
import { SurveyList } from '@/components/SurveyList';
import { surveyClient } from '@/lib/client';
import { type RollupState, StatusRollup } from '@/ui/StatusRollup';

/**
 * Surveys — List + detail.
 *
 * The rollup leads because this page can be wrong: the survey list is the only
 * thing the rest of the product is built on, and a failed load has to say so
 * rather than render an empty list that looks like "no surveys yet".
 *
 * The walk starts here. The service has offered create, start, capture, pause,
 * complete and delete since the capture backend landed; until this page called
 * them Trellis could only analyse other tools' captures. The new-survey form
 * sits above the list so a created survey appears where it will be selected.
 */
export function SurveysPage() {
  const { t } = useTranslation(['common', 'pages']);
  const [selectedId, setSelectedId] = useState<string | undefined>();

  const surveysQuery = useQuery({
    queryKey: ['surveys'],
    queryFn: () => surveyClient.listSurveys({}),
  });

  const surveys = surveysQuery.data?.surveys ?? [];
  const selectedSurvey = surveys.find((s) => s.id === selectedId);

  /* An empty list and a failed request look identical if both render as zero,
     so they are different states here. Loading is not "ok" either. */
  const state: RollupState = surveysQuery.isError
    ? 'unknown'
    : surveysQuery.isLoading
      ? 'unknown'
      : 'ok';

  const headline = surveysQuery.isError
    ? t('pages:surveys.notArriving')
    : surveysQuery.isLoading
      ? t('pages:surveys.loading')
      : surveys.length > 0
        ? t('pages:surveys.available', { count: surveys.length })
        : t('pages:surveys.noneCaptured');

  const body = surveysQuery.isError
    ? t('pages:surveys.surveyServiceSilent', { error: String(surveysQuery.error) })
    : surveys.length === 0 && !surveysQuery.isLoading
      ? t('pages:surveys.emptyBody')
      : undefined;

  return (
    <div className="flex flex-1 flex-col gap-6 overflow-hidden p-6">
      <StatusRollup
        state={state}
        headline={headline}
        body={body}
        figures={[{ label: t('common:labels.surveys'), value: String(surveys.length) }]}
      />

      <div className="flex flex-1 gap-6 overflow-hidden">
        <aside className="panel flex w-72 shrink-0 flex-col overflow-hidden">
          <SurveyCreateForm onCreated={setSelectedId} />
          <div className="flex-1 overflow-y-auto">
            {surveysQuery.isSuccess ? (
              <SurveyList surveys={surveys} selectedId={selectedId} onSelect={setSelectedId} />
            ) : null}
          </div>
        </aside>

        {selectedSurvey ? (
          <SurveyDetail survey={selectedSurvey} onDeleted={() => setSelectedId(undefined)} />
        ) : (
          <div className="panel flex flex-1 items-center justify-center text-sm text-text-muted">
            {t('pages:surveys.selectPrompt')}
          </div>
        )}
      </div>
    </div>
  );
}
