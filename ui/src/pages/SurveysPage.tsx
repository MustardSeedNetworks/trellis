import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CaptureCapabilityNotice } from '@/components/CaptureCapabilityNotice';
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
/** How often the list refreshes while a continuous capture is running. */
const WALK_POLL_MS = 2000;

export function SurveysPage() {
  const { t } = useTranslation(['common', 'pages']);
  const [selectedId, setSelectedId] = useState<string | undefined>();

  // Polled only while a walk is running. A continuous capture stores a point
  // every few seconds, and the sample count, the capture's position and the
  // reason it stopped all ride on this reply — an unpolled list would show a
  // walk frozen at whatever it looked like when the page loaded.
  const [walking, setWalking] = useState(false);
  const surveysQuery = useQuery({
    queryKey: ['surveys'],
    queryFn: () => surveyClient.listSurveys({}),
    refetchInterval: walking ? WALK_POLL_MS : false,
  });

  const surveys = surveysQuery.data?.surveys ?? [];
  const anyWalking = surveys.some((s) => s.capture?.running === true);
  if (anyWalking !== walking) {
    setWalking(anyWalking);
  }

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
      {/* No figures: the headline already counts the surveys, and a "2 SURVEYS"
          face beside "2 surveys available" read as two quantities that happen
          to agree rather than as one stated twice (trellis#476). */}
      <StatusRollup state={state} headline={headline} body={body} />

      <CaptureCapabilityNotice />

      {/* List beside detail on a laptop, stacked on a phone: at 390px the
          288px list and the detail cannot share a row, and the detail was
          pushed off the right edge entirely (trellis#473). Below md the page
          itself scrolls instead of each pane. */}
      <div className="flex flex-1 flex-col gap-6 overflow-y-auto md:flex-row md:overflow-hidden">
        <aside className="panel flex w-full shrink-0 flex-col md:w-72 md:overflow-hidden">
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
          /* Not stretched to the full height of the row. With nothing
             selected there is one sentence to show, and a full-height bordered
             box holding it reads as a pane that failed to load rather than as
             an invitation — the same dead-panel shape the density pass removed
             from stem's idle results card (UI-TRL-13). `md:self-start` lets it
             size to its content; the space below is page, not an empty box. */
          <div className="panel flex flex-1 items-center justify-center p-6 text-sm text-text-muted md:self-start">
            {t('pages:surveys.selectPrompt')}
          </div>
        )}
      </div>
    </div>
  );
}
