import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { ImportSurvey } from '@/components/ImportSurvey';
import { SurveyDetail } from '@/components/SurveyDetail';
import { SurveyList } from '@/components/SurveyList';
import { surveyClient } from '@/lib/client';

export function App() {
  const [selectedId, setSelectedId] = useState<string | undefined>();

  const surveysQuery = useQuery({
    queryKey: ['surveys'],
    queryFn: () => surveyClient.listSurveys({}),
  });

  const surveys = surveysQuery.data?.surveys ?? [];
  const selectedSurvey = surveys.find((s) => s.id === selectedId);

  return (
    <div className="flex h-screen flex-col bg-surface-base text-text-primary">
      <header className="border-b border-hairline bg-surface-raised px-6 py-4">
        <h1 className="text-lg font-semibold">Trellis</h1>
        <p className="text-xs text-text-muted">Wi-Fi survey heatmaps and coverage analysis</p>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <aside className="flex w-72 flex-col border-r border-hairline bg-surface-raised">
          <div className="flex-1 overflow-y-auto">
            {surveysQuery.isLoading && (
              <p className="p-4 text-sm text-text-muted">Loading surveys…</p>
            )}
            {surveysQuery.isError && (
              <p className="p-4 text-sm text-status-error">
                Failed to load surveys: {String(surveysQuery.error)}
              </p>
            )}
            {surveysQuery.data && (
              <SurveyList surveys={surveys} selectedId={selectedId} onSelect={setSelectedId} />
            )}
          </div>
          <ImportSurvey />
        </aside>

        {selectedId ? (
          <SurveyDetail surveyId={selectedId} surveyName={selectedSurvey?.name ?? 'survey'} />
        ) : (
          <div className="flex flex-1 items-center justify-center text-sm text-text-muted">
            Select a survey to view its heatmap and coverage
          </div>
        )}
      </div>
    </div>
  );
}
