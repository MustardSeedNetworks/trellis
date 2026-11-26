import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CaptureCapabilityNotice } from '@/components/CaptureCapabilityNotice';
import { SurveyCreateForm } from '@/components/SurveyCreateForm';
import { SurveyDetail } from '@/components/SurveyDetail';
import { SurveyList } from '@/components/SurveyList';
import { surveyClient } from '@/lib/client';
import { Alert } from '@/shared/Alert';
import { Card } from '@/shared/Card';

/**
 * Surveys — create, list, and one survey's detail.
 *
 * The walk starts here. The service has offered create, start, capture, pause,
 * complete and delete since the capture backend landed; until this page called
 * them Trellis could only analyse other tools' captures. The new-survey form
 * sits above the list so a created survey appears where it will be selected,
 * and the selected survey opens beneath the list at full width: beside it, at
 * 390px, the two could not share a row (trellis#473).
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

  return (
    <div className="flex flex-col gap-4">
      <CaptureCapabilityNotice />

      <Card title={t('pages:surveys.newSurvey')}>
        <SurveyCreateForm onCreated={setSelectedId} primary={selectedSurvey === undefined} />
      </Card>

      {/* A failed load and an empty list both have zero surveys, so they are
          different states here: the failure says so rather than showing an
          empty table that reads as "no surveys yet". */}
      {surveysQuery.isError ? (
        <Alert tone="error" title={t('pages:surveys.notArriving')}>
          {t('pages:surveys.surveyServiceSilent', { error: String(surveysQuery.error) })}
        </Alert>
      ) : (
        <SurveyList
          surveys={surveys}
          loading={surveysQuery.isLoading}
          selectedId={selectedId}
          onSelect={setSelectedId}
        />
      )}

      {selectedSurvey ? (
        <SurveyDetail survey={selectedSurvey} onDeleted={() => setSelectedId(undefined)} />
      ) : null}
    </div>
  );
}
