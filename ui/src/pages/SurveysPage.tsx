import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { ImportSurvey } from '@/components/ImportSurvey';
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
 */
export function SurveysPage() {
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
    ? 'Survey data is not arriving'
    : surveysQuery.isLoading
      ? 'Loading surveys'
      : surveys.length > 0
        ? `${surveys.length} survey${surveys.length === 1 ? '' : 's'} available`
        : 'No surveys captured yet';

  const body = surveysQuery.isError
    ? `The survey service did not answer: ${String(surveysQuery.error)}`
    : surveys.length === 0 && !surveysQuery.isLoading
      ? 'Import a survey to build a heatmap and coverage analysis from it.'
      : undefined;

  return (
    <div className="flex flex-1 flex-col gap-6 overflow-hidden p-6">
      <StatusRollup
        state={state}
        headline={headline}
        body={body}
        figures={[{ label: 'Surveys', value: String(surveys.length) }]}
      />

      <div className="flex flex-1 gap-6 overflow-hidden">
        <aside className="panel flex w-72 shrink-0 flex-col overflow-hidden">
          <div className="flex-1 overflow-y-auto">
            {surveysQuery.isSuccess ? (
              <SurveyList surveys={surveys} selectedId={selectedId} onSelect={setSelectedId} />
            ) : null}
          </div>
          <ImportSurvey />
        </aside>

        {selectedId ? (
          <SurveyDetail surveyId={selectedId} surveyName={selectedSurvey?.name ?? 'survey'} />
        ) : (
          <div className="panel flex flex-1 items-center justify-center text-sm text-text-muted">
            Select a survey to view its heatmap and coverage
          </div>
        )}
      </div>
    </div>
  );
}
